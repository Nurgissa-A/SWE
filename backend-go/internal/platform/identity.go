package platform

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"

	"github.com/biletflow/backend-go/internal/auth"
)

// Viewer returns the authenticated principal, or ErrUnauthenticated.
func Viewer(r *http.Request) (auth.Principal, error) {
	p, ok := auth.Current(r.Context())
	if !ok || p.ID == "" {
		return auth.Principal{}, ErrUnauthenticated
	}
	return p, nil
}

// OptionalViewer returns the principal when present; anonymous visitors get a zero value.
func OptionalViewer(r *http.Request) auth.Principal {
	p, _ := auth.Current(r.Context())
	return p
}

func NormalizeLocale(l string) string {
	switch l {
	case "kk", "ru", "en":
		return l
	}
	return "ru"
}

// DirectoryMiddleware refreshes the user_directory snapshot after the auth middleware
// has resolved live identity, so Go can address notifications and staff lists without
// copying credentials or writing identity tables.
func DirectoryMiddleware(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := auth.Current(r.Context()); ok && p.ID != "" && p.Email != "" {
			if err := UpsertDirectory(r.Context(), db, p); err != nil {
				log.Printf("directory refresh failed: %v", err)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func UpsertDirectory(ctx context.Context, q Querier, p auth.Principal) error {
	var display any
	if strings.TrimSpace(p.DisplayName) != "" {
		display = p.DisplayName
	}
	_, err := q.ExecContext(ctx, `INSERT INTO user_directory(user_id, email, display_name, locale, role)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id) DO UPDATE SET email=EXCLUDED.email, display_name=EXCLUDED.display_name,
			locale=EXCLUDED.locale, role=EXCLUDED.role, updated_at=now()
		WHERE (user_directory.email, user_directory.display_name, user_directory.locale, user_directory.role)
			IS DISTINCT FROM (EXCLUDED.email, EXCLUDED.display_name, EXCLUDED.locale, EXCLUDED.role)`,
		p.ID, strings.ToLower(p.Email), display, NormalizeLocale(p.Locale), p.Role)
	return err
}

// DirectoryUser is the cached contact data of one identity user.
type DirectoryUser struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"displayName"`
	Locale      string  `json:"locale"`
}

// Name returns the display name, falling back to the e-mail address.
func (u DirectoryUser) Name() string {
	if u.DisplayName != nil && *u.DisplayName != "" {
		return *u.DisplayName
	}
	return u.Email
}

func LookupDirectory(ctx context.Context, q Querier, userID string) (DirectoryUser, bool) {
	var u DirectoryUser
	var display sql.NullString
	err := q.QueryRowContext(ctx, `SELECT user_id, email, display_name, locale FROM user_directory WHERE user_id=$1`, userID).
		Scan(&u.ID, &u.Email, &display, &u.Locale)
	if err != nil {
		return u, false
	}
	u.DisplayName = NullString(display)
	return u, true
}
