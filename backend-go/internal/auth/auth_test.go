package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testUser = "00000000-0000-4000-8000-000000000001"

func testKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	data, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return key, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: data})
}

func sign(t *testing.T, key *rsa.PrivateKey, changes map[string]interface{}, kid string) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": testUser, "jti": "00000000-0000-4000-8000-000000000002", "role": "organizer", "ver": 0,
		"iss": "issuer", "aud": "audience", "iat": time.Now().Add(-time.Second).Unix(), "nbf": time.Now().Add(-time.Second).Unix(), "exp": time.Now().Add(time.Minute).Unix()}
	for name, value := range changes {
		if value == nil {
			delete(claims, name)
		} else {
			claims[name] = value
		}
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestVerifierAcceptsLiveRole(t *testing.T) {
	key, public := testKey(t)
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users/me" || r.Header.Get("Authorization") == "" {
			t.Error("identity request missing contract")
		}
		_ = json.NewEncoder(w).Encode(Principal{ID: testUser, Role: "attendee", EmailVerified: true})
	}))
	defer live.Close()
	verifier, err := NewVerifier(public, "issuer", "audience", "key-1", live.URL)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		p, ok := Current(r.Context())
		if !ok || p.ID != testUser || p.Role != "attendee" {
			t.Error("live role must be authoritative")
		}
	}))
	req := httptest.NewRequest("POST", "/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+sign(t, key, nil, "key-1"))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if !called || recorder.Code != 200 {
		t.Fatalf("expected authenticated request, got %d", recorder.Code)
	}
}

func TestVerifierRejectsInvalidTokensBeforeIdentityCall(t *testing.T) {
	key, public := testKey(t)
	wrong, _ := testKey(t)
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("invalid JWT reached identity") }))
	defer live.Close()
	verifier, _ := NewVerifier(public, "issuer", "audience", "key-1", live.URL)
	cases := []struct {
		name    string
		changes map[string]interface{}
		kid     string
		key     *rsa.PrivateKey
	}{
		{"expired", map[string]interface{}{"exp": time.Now().Add(-time.Minute).Unix()}, "key-1", key},
		{"signature", nil, "key-1", wrong},
		{"issuer", map[string]interface{}{"iss": "wrong"}, "key-1", key},
		{"audience", map[string]interface{}{"aud": "wrong"}, "key-1", key},
		{"role", map[string]interface{}{"role": "root"}, "key-1", key},
		{"subject", map[string]interface{}{"sub": "forged"}, "key-1", key},
		{"missing-exp", map[string]interface{}{"exp": nil}, "key-1", key},
		{"missing-iat", map[string]interface{}{"iat": nil}, "key-1", key},
		{"missing-nbf", map[string]interface{}{"nbf": nil}, "key-1", key},
		{"missing-ver", map[string]interface{}{"ver": nil}, "key-1", key},
		{"future-nbf", map[string]interface{}{"nbf": time.Now().Add(time.Hour).Unix()}, "key-1", key},
		{"kid", nil, "unknown", key},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("invalid JWT accepted") }))
			req := httptest.NewRequest("POST", "/api/events", nil)
			req.Header.Set("Authorization", "Bearer "+sign(t, tc.key, tc.changes, tc.kid))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != 401 || recorder.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("got %d", recorder.Code)
			}
		})
	}
}

func TestVerifierFailsClosedOnLiveIdentity(t *testing.T) {
	key, public := testKey(t)
	for _, status := range []int{401, 403, 500, 302} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer live.Close()
			verifier, _ := NewVerifier(public, "issuer", "audience", "key-1", live.URL)
			req := httptest.NewRequest("POST", "/api/events", nil)
			req.Header.Set("Authorization", "Bearer "+sign(t, key, nil, "key-1"))
			recorder := httptest.NewRecorder()
			verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("identity failure admitted") })).ServeHTTP(recorder, req)
			expected := 503
			if status == 401 || status == 403 {
				expected = 401
			}
			if recorder.Code != expected {
				t.Fatalf("got %d want %d", recorder.Code, expected)
			}
		})
	}
}

func TestVerifierRejectsMalformedLiveIdentity(t *testing.T) {
	key, public := testKey(t)
	for _, principal := range []Principal{{ID: testUser, Role: "organizer"}, {ID: "other", Role: "organizer", EmailVerified: true}, {ID: testUser, Role: "root", EmailVerified: true}} {
		live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(principal) }))
		verifier, _ := NewVerifier(public, "issuer", "audience", "key-1", live.URL)
		req := httptest.NewRequest("POST", "/api/events", nil)
		req.Header.Set("Authorization", "Bearer "+sign(t, key, nil, "key-1"))
		recorder := httptest.NewRecorder()
		verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("malformed identity admitted") })).ServeHTTP(recorder, req)
		live.Close()
		if recorder.Code != 401 {
			t.Fatal(recorder.Code)
		}
	}
}

func TestMissingMalformedAndOversizedHeaders(t *testing.T) {
	_, public := testKey(t)
	verifier, _ := NewVerifier(public, "issuer", "audience", "key-1", "http://127.0.0.1:1")
	for _, value := range []string{"", "Basic abc", "Bearer invalid", "Bearer a b", "Bearer " + strings.Repeat("a", 8193)} {
		req := httptest.NewRequest("POST", "/api/events", nil)
		req.Header.Set("Authorization", value)
		recorder := httptest.NewRecorder()
		verifier.Middleware(http.NotFoundHandler()).ServeHTTP(recorder, req)
		if recorder.Code != 401 {
			t.Fatal(recorder.Code)
		}
	}
}

func TestAnonymousDetailOnlyAndUnavailableIdentity(t *testing.T) {
	key, public := testKey(t)
	verifier, _ := NewVerifier(public, "issuer", "audience", "key-1", "http://127.0.0.1:1")
	recorder := httptest.NewRecorder()
	verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).ServeHTTP(recorder, httptest.NewRequest("GET", "/api/events/id", nil))
	if recorder.Code != 204 {
		t.Fatal(recorder.Code)
	}
	req := httptest.NewRequest("POST", "/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+sign(t, key, nil, "key-1"))
	recorder = httptest.NewRecorder()
	verifier.Middleware(http.NotFoundHandler()).ServeHTTP(recorder, req)
	if recorder.Code != 503 {
		t.Fatal(recorder.Code)
	}
}

func TestVerifierConfigurationRejectsBadKeysAndURLs(t *testing.T) {
	_, public := testKey(t)
	if _, err := NewVerifier([]byte("bad"), "issuer", "audience", "key", "http://localhost"); err == nil {
		t.Fatal("bad key")
	}
	for _, url := range []string{"", "file:///tmp/socket", "http://user:password@localhost", "http://localhost?token=x"} {
		if _, err := NewVerifier(public, "issuer", "audience", "key", url); err == nil {
			t.Fatal("bad URL accepted")
		}
	}
}
