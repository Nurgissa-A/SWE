package event

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
)

func newTestHandler() *Handler {
	repo := NewInMemoryRepository()
	service := NewService(repo)
	return NewHandler(service)
}

func newTestMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	wrapped := http.NewServeMux()
	wrapped.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := auth.Principal{ID: "00000000-0000-4000-8000-000000000001", Role: "organizer", EmailVerified: true}
		mux.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	}))
	return wrapped
}

func TestCreateEvent_Success(t *testing.T) {
	mux := newTestMux(newTestHandler())

	body, _ := json.Marshal(CreateEventInput{
		Title:    "Test Concert",
		StartsAt: time.Now().Add(24 * time.Hour),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var got Event
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Status != StatusDraft {
		t.Errorf("expected new event to be draft, got %s", got.Status)
	}
	if got.ID == "" {
		t.Error("expected event to have an ID")
	}
}

func TestCreateEvent_MissingTitle(t *testing.T) {
	mux := newTestMux(newTestHandler())

	body, _ := json.Marshal(CreateEventInput{
		StartsAt: time.Now().Add(24 * time.Hour),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing title, got %d", rec.Code)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	mux := newTestMux(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/events/does-not-exist", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPublishEvent_Success(t *testing.T) {
	h := newTestHandler()
	mux := newTestMux(h)

	createBody, _ := json.Marshal(CreateEventInput{
		Title:    "Publishable Event",
		StartsAt: time.Now().Add(24 * time.Hour),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(createBody))
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	var created Event
	json.NewDecoder(createRec.Body).Decode(&created)

	publishReq := httptest.NewRequest(http.MethodPost, "/api/events/"+created.ID+"/publish", nil)
	publishRec := httptest.NewRecorder()
	mux.ServeHTTP(publishRec, publishReq)

	if publishRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", publishRec.Code, publishRec.Body.String())
	}

	var published Event
	json.NewDecoder(publishRec.Body).Decode(&published)
	if published.Status != StatusPublished {
		t.Errorf("expected status published, got %s", published.Status)
	}
}

func TestListEvents_FilterByStatus(t *testing.T) {
	h := newTestHandler()
	mux := newTestMux(h)

	body, _ := json.Marshal(CreateEventInput{
		Title:    "Draft Event",
		StartsAt: time.Now().Add(24 * time.Hour),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	listReq := httptest.NewRequest(http.MethodGet, "/api/events?status=draft", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)

	var events []Event
	json.NewDecoder(listRec.Body).Decode(&events)
	if len(events) != 1 {
		t.Fatalf("expected 1 draft event, got %d", len(events))
	}
}
