// Package auth verifies Python-issued tokens and resolves authoritative live identity.
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	ID            string `json:"id"`
	Role          string `json:"role"`
	EmailVerified bool   `json:"emailVerified"`
	// Display data from the live identity response; never used for authorization.
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Locale      string `json:"locale,omitempty"`
}

type contextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

func Current(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(Principal)
	return p, ok
}

type Claims struct {
	Role    string `json:"role"`
	Version *int   `json:"ver"`
	jwt.RegisteredClaims
}

type Verifier struct {
	key                                  *rsa.PublicKey
	issuer, audience, keyID, identityURL string
	client                               *http.Client
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func NewVerifier(publicPEM []byte, issuer, audience, keyID, identityURL string) (*Verifier, error) {
	key, err := jwt.ParseRSAPublicKeyFromPEM(publicPEM)
	if err != nil || key.N.BitLen() < 2048 {
		return nil, errors.New("a valid RSA public key of at least 2048 bits is required")
	}
	parsed, err := url.Parse(identityURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return nil, errors.New("IDENTITY_URL must be a trusted http(s) base URL")
	}
	if issuer == "" || audience == "" || keyID == "" {
		return nil, errors.New("JWT issuer, audience and key ID are required")
	}
	return &Verifier{key: key, issuer: issuer, audience: audience, keyID: keyID,
		identityURL: strings.TrimRight(identityURL, "/"), client: &http.Client{Timeout: 2 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func validRole(role string) bool {
	return role == "attendee" || role == "organizer" || role == "event_admin" || role == "platform_admin"
}

func (v *Verifier) authenticate(r *http.Request, raw string) (Principal, int) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Header["kid"] != v.keyID {
			return nil, errors.New("unknown key")
		}
		return v.key, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(v.issuer), jwt.WithAudience(v.audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !uuidPattern.MatchString(claims.Subject) || !uuidPattern.MatchString(claims.ID) || !validRole(claims.Role) || claims.Version == nil || *claims.Version < 0 || claims.IssuedAt == nil || claims.NotBefore == nil {
		return Principal{}, http.StatusUnauthorized
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, v.identityURL+"/api/users/me", nil)
	if err != nil {
		return Principal{}, http.StatusServiceUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+raw)
	response, err := v.client.Do(req)
	if err != nil {
		return Principal{}, http.StatusServiceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return Principal{}, http.StatusUnauthorized
	}
	if response.StatusCode != http.StatusOK {
		return Principal{}, http.StatusServiceUnavailable
	}
	var principal Principal
	if json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&principal) != nil || principal.ID != claims.Subject || !validRole(principal.Role) || !principal.EmailVerified {
		return Principal{}, http.StatusUnauthorized
	}
	// The live DB-backed role is authoritative after onboarding/operator changes.
	return principal, 0
}

func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": map[string]string{"code": strconv.Itoa(status), "message": message}})
}

// AnonymousReadable lists the GET paths that work without a token.
func AnonymousReadable(path string) bool {
	return path == "/api/events" || strings.HasPrefix(path, "/api/events/") || strings.HasPrefix(path, "/api/public/")
}

func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		// Anonymous visitors may read public event pages and listings (SRS §4.2) and open
		// campaign links; handlers still hide drafts, private events and other users' data.
		if header == "" && r.Method == http.MethodGet && AnonymousReadable(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 8192 {
			WriteError(w, http.StatusUnauthorized, "Invalid or missing access token")
			return
		}
		principal, status := v.authenticate(r, parts[1])
		if status != 0 {
			message := "Invalid access token"
			if status == http.StatusServiceUnavailable {
				message = "Identity service unavailable"
			}
			WriteError(w, status, message)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

// LookedUpUser is a verified account found by e-mail through the identity service.
type LookedUpUser struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"displayName"`
	Locale      string  `json:"locale"`
}

// ErrNoSuchUser means the identity service has no verified, active account with that e-mail.
var ErrNoSuchUser = errors.New("no such user")

// LookupUser asks the identity service for a verified user by e-mail, authorised with the
// caller's own bearer token (organizers adding door staff). The identity service decides
// whether the caller may look users up.
func (v *Verifier) LookupUser(ctx context.Context, bearer, email string) (LookedUpUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.identityURL+"/api/users/lookup?email="+url.QueryEscape(email), nil)
	if err != nil {
		return LookedUpUser{}, err
	}
	req.Header.Set("Authorization", bearer)
	res, err := v.client.Do(req)
	if err != nil {
		return LookedUpUser{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return LookedUpUser{}, ErrNoSuchUser
	}
	if res.StatusCode != http.StatusOK {
		return LookedUpUser{}, errors.New("identity lookup failed: " + res.Status)
	}
	var u LookedUpUser
	if err := json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&u); err != nil || !uuidPattern.MatchString(u.ID) {
		return LookedUpUser{}, errors.New("malformed identity lookup response")
	}
	return u, nil
}
