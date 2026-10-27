package event

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
)

func requestAs(mux *http.ServeMux, method, path, body string, p auth.Principal) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if p.ID != "" {
		r = r.WithContext(auth.WithPrincipal(r.Context(), p))
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, r)
	return recorder
}

func TestEventAuthorizationAndVisibility(t *testing.T) {
	repo := NewInMemoryRepository()
	service := NewService(repo)
	h := NewHandler(service)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	owner := auth.Principal{ID: "00000000-0000-4000-8000-000000000001", Role: "organizer", EmailVerified: true}
	other := auth.Principal{ID: "00000000-0000-4000-8000-000000000002", Role: "organizer", EmailVerified: true}
	input := `{"title":"Private draft","startsAt":"2030-01-01T10:00:00Z","visibility":"private","venueId":"00000000-0000-4000-8000-000000000010"}`
	created := requestAs(mux, "POST", "/api/events", input, owner)
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var e Event
	_ = json.Unmarshal(created.Body.Bytes(), &e)
	if e.OrganizerID != owner.ID {
		t.Fatal("organizer must be authenticated subject")
	}
	if requestAs(mux, "POST", "/api/events/"+e.ID+"/publish", "", other).Code != 403 {
		t.Fatal("non-owner publish")
	}
	if requestAs(mux, "GET", "/api/events/"+e.ID, "", other).Code != 404 {
		t.Fatal("draft leaked")
	}
	list := requestAs(mux, "GET", "/api/events", "", other)
	var rows []Event
	_ = json.Unmarshal(list.Body.Bytes(), &rows)
	if len(rows) != 0 {
		t.Fatal("list leaked private draft")
	}
	if requestAs(mux, "POST", "/api/events/"+e.ID+"/publish", "", owner).Code != 200 {
		t.Fatal("owner cannot publish")
	}
	if requestAs(mux, "GET", "/api/events/"+e.ID, "", auth.Principal{}).Code != 404 {
		t.Fatal("private published event leaked")
	}
	public, _ := service.Create(auth.WithPrincipal(context.Background(), owner), CreateEventInput{Title: "Public", StartsAt: time.Now(), Visibility: VisibilityPublic, VenueID: ptr("00000000-0000-4000-8000-000000000010")})
	_, _ = service.Publish(auth.WithPrincipal(context.Background(), owner), public.ID)
	if requestAs(mux, "GET", "/api/events/"+public.ID, "", auth.Principal{}).Code != 200 {
		t.Fatal("public published detail unavailable")
	}
}

func TestNonOrganizerCannotCreateOrPublish(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	body := `{"title":"Event","startsAt":"2030-01-01T10:00:00Z"}`
	if requestAs(mux, "POST", "/api/events", body, auth.Principal{}).Code != 401 {
		t.Fatal("anonymous create")
	}
	for _, role := range []string{"attendee", "event_admin", "platform_admin"} {
		p := auth.Principal{ID: "00000000-0000-4000-8000-000000000001", Role: role, EmailVerified: true}
		if requestAs(mux, "POST", "/api/events", body, p).Code != 403 {
			t.Fatal("privileged event role inferred incorrectly")
		}
		if requestAs(mux, "POST", "/api/events/anything/publish", "", p).Code != 403 {
			t.Fatal("global publish granted")
		}
	}
}

func TestCreateRejectsSuppliedOrganizerAndTrailingJSON(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	owner := auth.Principal{ID: "00000000-0000-4000-8000-000000000001", Role: "organizer", EmailVerified: true}
	for _, body := range []string{`{"title":"x","startsAt":"2030-01-01T10:00:00Z","organizerId":"forged"}`, `{"title":"x","startsAt":"2030-01-01T10:00:00Z"} {}`} {
		if requestAs(mux, "POST", "/api/events", body, owner).Code != 400 {
			t.Fatal("forged/trailing input accepted")
		}
	}
}
