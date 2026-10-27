package event

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
)

func ptr[T any](v T) *T { return &v }

var testOwner = auth.Principal{ID: "00000000-0000-4000-8000-000000000001", Role: "organizer", EmailVerified: true}
var testOther = auth.Principal{ID: "00000000-0000-4000-8000-000000000002", Role: "organizer", EmailVerified: true}

func as(p auth.Principal) context.Context { return auth.WithPrincipal(context.Background(), p) }
func validInput() CreateEventInput {
	return CreateEventInput{Title: "Concert", StartsAt: time.Date(2030, 1, 1, 10, 0, 0, 0, time.UTC), VenueID: ptr("00000000-0000-4000-8000-000000000010")}
}
func TestValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*CreateEventInput)
	}{
		{"blank title", func(i *CreateEventInput) { i.Title = " \n " }},
		{"missing start", func(i *CreateEventInput) { i.StartsAt = time.Time{} }},
		{"invalid visibility", func(i *CreateEventInput) { i.Visibility = "secret" }},
		{"zero capacity", func(i *CreateEventInput) { i.Capacity = ptr(0) }},
		{"negative capacity", func(i *CreateEventInput) { i.Capacity = ptr(-1) }},
		{"capacity overflow", func(i *CreateEventInput) { i.Capacity = ptr(2147483648) }},
		{"invalid venue", func(i *CreateEventInput) { i.VenueID = ptr("not-a-uuid") }},
		{"end before start", func(i *CreateEventInput) { i.EndsAt = ptr(i.StartsAt.Add(-time.Hour)) }},
		{"end equal start", func(i *CreateEventInput) { i.EndsAt = ptr(i.StartsAt) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewService(NewInMemoryRepository())
			i := validInput()
			tc.change(&i)
			if _, err := s.Create(as(testOwner), i); !errors.Is(err, ErrValidation) {
				t.Fatalf("expected validation: %v", err)
			}
		})
	}
}
func TestServiceAuthorization(t *testing.T) {
	s := NewService(NewInMemoryRepository())
	e, err := s.Create(as(testOwner), validInput())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"anonymous", context.Background(), ErrUnauthorized},
		{"other organizer", as(testOther), ErrForbidden},
		{"attendee", as(auth.Principal{ID: testOwner.ID, Role: "attendee", EmailVerified: true}), ErrForbidden},
		{"unverified", as(auth.Principal{ID: testOwner.ID, Role: "organizer"}), ErrForbidden},
		{"admin", as(auth.Principal{ID: testOwner.ID, Role: "platform_admin", EmailVerified: true}), ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Update(tc.ctx, e.ID, UpdateEventInput{Title: ptr("forged")}); !errors.Is(err, tc.want) {
				t.Fatalf("update: %v", err)
			}
			if _, err := s.Publish(tc.ctx, e.ID); !errors.Is(err, tc.want) {
				t.Fatalf("publish: %v", err)
			}
			if _, err := s.Get(tc.ctx, e.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("draft leaked: %v", err)
			}
		})
	}
}
func TestPartialUpdateRollbackAndLifecycle(t *testing.T) {
	s := NewService(NewInMemoryRepository())
	ctx := as(testOwner)
	in := validInput()
	in.VenueID = nil
	e, err := s.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if !validID(e.ID) {
		t.Fatal("invalid UUID")
	}
	if _, err = s.Update(ctx, e.ID, UpdateEventInput{}); !errors.Is(err, ErrValidation) {
		t.Fatal("empty service patch accepted", err)
	}
	if _, err = s.Publish(ctx, e.ID); !errors.Is(err, ErrValidation) {
		t.Fatal(err)
	}
	e, err = s.Update(ctx, e.ID, UpdateEventInput{VenueID: validInput().VenueID, Description: ptr("Description"), Capacity: ptr(50)})
	if err != nil {
		t.Fatal(err)
	}
	if e.Title != in.Title || !e.StartsAt.Equal(in.StartsAt) {
		t.Fatal("omitted fields overwritten")
	}
	if _, err = s.Update(ctx, e.ID, UpdateEventInput{Title: ptr("changed"), Capacity: ptr(-5)}); !errors.Is(err, ErrValidation) {
		t.Fatal(err)
	}
	e, _ = s.Get(ctx, e.ID)
	if e.Title != in.Title || *e.Capacity != 50 {
		t.Fatal("invalid patch was persisted")
	}
	e, err = s.Publish(ctx, e.ID)
	if err != nil || e.Status != StatusPublished {
		t.Fatalf("publish %v", err)
	}
	if _, err = s.Publish(ctx, e.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("repeat publication", err)
	}
	if _, err = s.Update(ctx, e.ID, UpdateEventInput{Title: ptr("published edit")}); !errors.Is(err, ErrConflict) {
		t.Fatal("published update", err)
	}
}
func TestVisibilityMatrix(t *testing.T) {
	for _, visibility := range []Visibility{VisibilityPublic, VisibilityUnlisted, VisibilityPrivate} {
		t.Run(string(visibility), func(t *testing.T) {
			s := NewService(NewInMemoryRepository())
			in := validInput()
			in.Visibility = visibility
			e, _ := s.Create(as(testOwner), in)
			for _, status := range []Status{StatusDraft, StatusPublished} {
				if status == StatusPublished {
					if _, err := s.Publish(as(testOwner), e.ID); err != nil {
						t.Fatal(err)
					}
				}
				for _, p := range []auth.Principal{testOwner, testOther, {ID: testOther.ID, Role: "attendee", EmailVerified: true}, {}} {
					got, err := s.Get(as(p), e.ID)
					want := p == testOwner || (status == StatusPublished && visibility != VisibilityPrivate)
					if (err == nil) != want {
						t.Fatalf("detail %s %s %+v: %v %+v", status, visibility, p, err, got)
					}
					if p.ID == "" {
						continue
					}
					rows, err := s.List(as(p), nil)
					if err != nil {
						t.Fatal(err)
					}
					wantList := p == testOwner || (status == StatusPublished && visibility == VisibilityPublic)
					if (len(rows) == 1) != wantList {
						t.Fatal("list visibility", p, status, visibility)
					}
				}
			}
		})
	}
}
func TestConcurrentPublication(t *testing.T) {
	s := NewService(NewInMemoryRepository())
	e, _ := s.Create(as(testOwner), validInput())
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Publish(as(testOwner), e.ID); results <- err }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal(success)
	}
}
func TestHTTPValidation(t *testing.T) {
	mux := http.NewServeMux()
	newTestHandler().RegisterRoutes(mux)
	for _, body := range []string{`null`, `[]`, `{}`, `{"title":"x","title":"y","startsAt":"2030-01-01T10:00:00Z"}`, `{"title":"x","startsAt":"bad"}`, `{"title":null,"startsAt":"2030-01-01T10:00:00Z"}`, `{"title":"x","startsAt":"2030-01-01T10:00:00Z","visibility":""}`, strings.Repeat("x", (1<<20)+1)} {
		if r := requestAs(mux, "POST", "/api/events", body, testOwner); r.Code != 400 {
			t.Fatalf("create: %d", r.Code)
		}
	}
	e, _ := newID()
	for _, body := range []string{`{}`, `null`, `{"title":null}`, `{"capacity":null}`, `{"venueId":null}`, `{"organizerId":"forged"}`, `{"status":"published"}`, `{"title":"x"} {}`, `{"title":"x","title":"y"}`} {
		if r := requestAs(mux, "PATCH", "/api/events/"+e, body, testOwner); r.Code != 400 {
			t.Fatalf("patch %s: %d", body, r.Code)
		}
	}
	for _, path := range []string{"/api/events/bad", "/api/events?status=no", "/api/events?status=", "/api/events?status=draft&status=published"} {
		if r := requestAs(mux, "GET", path, "", testOwner); r.Code != 400 {
			t.Fatal(path, r.Code)
		}
	}
	if r := requestAs(mux, "GET", "/api/events", "", auth.Principal{}); r.Code != 401 {
		t.Fatal(r.Code)
	}
	if r := requestAs(mux, "GET", "/api/events/"+e, "", testOwner); r.Code != 404 {
		t.Fatal(r.Code)
	}
}

type failingRepository struct {
	Repository
	err error
}

func (r failingRepository) Create(context.Context, *Event) error        { return r.err }
func (r failingRepository) Get(context.Context, string) (*Event, error) { return nil, r.err }
func (r failingRepository) List(context.Context, string, *Status) ([]*Event, error) {
	return nil, r.err
}
func (r failingRepository) Mutate(context.Context, string, func(*Event) error) (*Event, error) {
	return nil, r.err
}
func TestStorageErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{{errors.New("sensitive database details"), 500}, {context.DeadlineExceeded, 503}} {
		mux := http.NewServeMux()
		NewHandler(NewService(failingRepository{err: tc.err})).RegisterRoutes(mux)
		id := "00000000-0000-4000-8000-000000000010"
		for _, req := range []struct{ method, path, body string }{{"POST", "/api/events", `{"title":"x","startsAt":"2030-01-01T10:00:00Z"}`}, {"GET", "/api/events", ""}, {"GET", "/api/events/" + id, ""}, {"PATCH", "/api/events/" + id, `{"title":"x"}`}, {"POST", "/api/events/" + id + "/publish", ""}} {
			r := requestAs(mux, req.method, req.path, req.body, testOwner)
			if r.Code != tc.want || strings.Contains(r.Body.String(), "sensitive") {
				t.Fatal(req.method, r.Code, r.Body.String())
			}
		}
	}
}

func TestUUIDValidation(t *testing.T) {
	for _, id := range []string{"bad", "00000000-0000-0000-0000-000000000000", "--000000-0000-4000-8000-000000000001", "00000000_0000_4000_8000_000000000001"} {
		if validID(id) {
			t.Fatalf("invalid UUID accepted: %s", id)
		}
	}
	for i := 0; i < 100; i++ {
		id, err := newID()
		if err != nil || !validID(id) || id[14] != '4' {
			t.Fatal(id, err)
		}
	}
}
func TestServiceCreateAuthorization(t *testing.T) {
	s := NewService(NewInMemoryRepository())
	for _, ctx := range []context.Context{context.Background(), as(auth.Principal{ID: testOwner.ID, Role: "attendee", EmailVerified: true}), as(auth.Principal{ID: testOwner.ID, Role: "organizer"})} {
		if _, err := s.Create(ctx, validInput()); err == nil {
			t.Fatal("service allowed untrusted creator")
		}
	}
}
