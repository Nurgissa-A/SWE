// Package testutil runs the real application against a real PostgreSQL database.
//
// Set GO_TEST_DATABASE_URL to a disposable database, e.g.
//
//	postgres://biletflow:secret@localhost:5432/biletflow_go_test?sslmode=disable
//
// Each test package gets its own schema (dropped and recreated per test), so packages can run
// in parallel. Authentication is replaced by a header carrying the test principal.
package testutil

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/biletflow/backend-go/internal/app"
	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/event"
	"github.com/biletflow/backend-go/internal/platform"
)

const principalHeader = "X-Test-Principal"

// QRKey is the ticket-signing key used in tests.
var QRKey = []byte("test-ticket-signing-key-0123456789abcdef")

// Env is one isolated application instance.
type Env struct {
	T       testing.TB
	DB      *sql.DB
	App     *app.App
	Handler http.Handler
	// Identity simulates the Python identity service for staff lookups.
	Identity map[string]auth.Principal
}

var unsafe = regexp.MustCompile(`[^a-z0-9_]`)

// New creates a fresh schema, migrates it and builds the app. It skips when no test database is configured.
func New(t testing.TB) *Env {
	t.Helper()
	base := os.Getenv("GO_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("GO_TEST_DATABASE_URL is not set; skipping database-backed test")
	}
	name := strings.ToLower(t.Name())
	if i := strings.IndexByte(name, '/'); i > 0 {
		name = name[:i]
	}
	schema := "t_" + unsafe.ReplaceAllString(name, "_")
	if len(schema) > 50 {
		schema = schema[:50]
	}
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	// Test packages run in parallel. On a fresh database two of them can create the extension at
	// the same moment and one gets a duplicate-key error, so accept it if the extension now exists.
	if _, err := admin.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public`); err != nil {
		var installed bool
		if qerr := admin.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pgcrypto')`).Scan(&installed); qerr != nil || !installed {
			t.Fatalf("pgcrypto: %v", err)
		}
	}
	if _, err := admin.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE; CREATE SCHEMA %s`, schema, schema)); err != nil {
		t.Fatalf("schema: %v", err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	db, err := platform.Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := platform.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	env := &Env{T: t, DB: db, Identity: map[string]auth.Principal{}}
	lookup := func(ctx context.Context, bearer, email string) (platform.DirectoryUser, error) {
		p, ok := env.Identity[strings.ToLower(email)]
		if !ok {
			return platform.DirectoryUser{}, event.ErrUserNotFound
		}
		name := p.DisplayName
		return platform.DirectoryUser{ID: p.ID, Email: p.Email, DisplayName: &name, Locale: platform.NormalizeLocale(p.Locale)}, nil
	}
	env.App = app.New(app.Config{DB: db, Authenticate: fakeAuth, Lookup: lookup, QRKey: QRKey, AppURL: "https://biletflow.test", RateLimitScale: 1000})
	env.Handler = env.App.Handler
	return env
}

func fakeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if raw := r.Header.Get(principalHeader); raw != "" {
			var p auth.Principal
			if err := json.Unmarshal([]byte(raw), &p); err == nil && p.ID != "" {
				r = r.WithContext(auth.WithPrincipal(r.Context(), p))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// User makes a verified principal with the given role and registers it with the fake identity service.
func (e *Env) User(role, name string) auth.Principal {
	id := newUUID()
	p := auth.Principal{ID: id, Role: role, EmailVerified: true, Email: strings.ToLower(strings.ReplaceAll(name, " ", ".")) + "." + id[:4] + "@example.kz", DisplayName: name, Locale: "en"}
	e.Identity[p.Email] = p
	return p
}

// Response wraps a recorded response.
type Response struct {
	*httptest.ResponseRecorder
	t testing.TB
}

// JSON decodes the body into v (failing the test on malformed JSON).
func (r Response) JSON(v any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body.Bytes(), v); err != nil {
		r.t.Fatalf("decode %q: %v", r.Body.String(), err)
	}
}

// Map decodes an object body.
func (r Response) Map() map[string]any {
	m := map[string]any{}
	r.JSON(&m)
	return m
}

// ErrorCode returns error.code from an error response.
func (r Response) ErrorCode() string {
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &e)
	return e.Error.Code
}

// Do sends a request as p (zero principal = anonymous). body may be nil, a string or any JSON value.
func (e *Env) Do(method, path string, body any, p auth.Principal) Response {
	e.T.Helper()
	var reader *bytes.Reader
	switch b := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(b))
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			e.T.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if p.ID != "" {
		raw, _ := json.Marshal(p)
		req.Header.Set(principalHeader, string(raw))
	}
	rec := httptest.NewRecorder()
	e.Handler.ServeHTTP(rec, req)
	return Response{rec, e.T}
}

// Must sends a request and fails the test unless the status matches.
func (e *Env) Must(status int, method, path string, body any, p auth.Principal) Response {
	e.T.Helper()
	res := e.Do(method, path, body, p)
	if res.Code != status {
		e.T.Fatalf("%s %s: want %d, got %d: %s", method, path, status, res.Code, res.Body.String())
	}
	return res
}

// EventOptions customise a test event.
type EventOptions struct {
	Capacity   int
	Visibility string
	Types      []map[string]any
	PaidActive bool
}

// PublishedEvent creates and publishes an event; ticket types default to one free type of 100.
// It returns the event ID and ticket type IDs in order.
func (e *Env) PublishedEvent(org auth.Principal, title string, o EventOptions) (string, []string) {
	e.T.Helper()
	body := map[string]any{"title": title, "startsAt": "2031-06-01T14:00:00Z", "endsAt": "2031-06-01T18:00:00Z", "venueName": "Astana Arena", "city": "Astana"}
	if o.Capacity > 0 {
		body["capacity"] = o.Capacity
	}
	if o.Visibility != "" {
		body["visibility"] = o.Visibility
	}
	var ev struct{ ID string }
	e.Must(201, "POST", "/api/events", body, org).JSON(&ev)
	types := o.Types
	if len(types) == 0 {
		types = []map[string]any{{"name": "General", "priceKzt": 0, "quantity": 100}}
	}
	var ids []string
	for _, t := range types {
		var tt struct{ ID string }
		e.Must(201, "POST", "/api/events/"+ev.ID+"/ticket-types", t, org).JSON(&tt)
		ids = append(ids, tt.ID)
	}
	e.Must(200, "POST", "/api/events/"+ev.ID+"/publish", nil, org)
	if o.PaidActive {
		e.ActivatePaidSales(org, ev.ID)
	}
	return ev.ID, ids
}

// ActivatePaidSales completes the whole activation checklist with the simulator.
func (e *Env) ActivatePaidSales(org auth.Principal, eventID string) {
	e.T.Helper()
	e.Must(200, "PUT", "/api/organizer/payment-setup/identity", map[string]any{"legalName": "Aigerim Organizer", "iin": "990101300123", "consent": true}, org)
	e.Must(200, "PUT", "/api/organizer/payment-setup/payout", map[string]any{"bankName": "Halyk", "accountHolder": "Aigerim Organizer", "iban": "KZ123456789012345678"}, org)
	e.Must(200, "POST", "/api/events/"+eventID+"/activation/terms", nil, org)
	e.Must(200, "POST", "/api/events/"+eventID+"/activation/fee", map[string]any{"cardNumber": "4242 4242 4242 4242"}, org)
	e.Must(200, "POST", "/api/events/"+eventID+"/activation/activate", nil, org)
}

// Buy reserves tickets and, for paid orders, pays with the success card. It returns the order JSON.
func (e *Env) Buy(buyer auth.Principal, eventID string, items map[string]int, extra map[string]any) map[string]any {
	e.T.Helper()
	var list []map[string]any
	for id, q := range items {
		list = append(list, map[string]any{"ticketTypeId": id, "quantity": q})
	}
	body := map[string]any{"eventId": eventID, "items": list, "buyer": map[string]any{"name": buyer.DisplayName, "email": buyer.Email}}
	for k, v := range extra {
		body[k] = v
	}
	order := e.Must(201, "POST", "/api/checkout/reserve", body, buyer).Map()
	if order["status"] == "pending" {
		order = e.Must(200, "POST", "/api/checkout/"+order["id"].(string)+"/pay", map[string]any{"cardNumber": "4242424242424242"}, buyer).Map()
	}
	return order
}
