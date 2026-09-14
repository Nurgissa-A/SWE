package event

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires paths to match docs/openapi.yaml exactly, so the
// contract and the implementation can't quietly drift apart.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/events", h.create)
	mux.HandleFunc("GET /api/events", h.list)
	mux.HandleFunc("GET /api/events/{eventId}", h.get)
	mux.HandleFunc("POST /api/events/{eventId}/publish", h.publish)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateEventInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// TODO(Week 3-4): organizerID comes from the JWT claims set by
	// Aliar's auth middleware. Hardcoded until that's wired in.
	organizerID := "TODO-organizer-id"

	e, err := h.service.Create(organizerID, in)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			http.Error(w, "title and startsAt are required", http.StatusBadRequest)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, e)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	var status *Status
	if q := r.URL.Query().Get("status"); q != "" {
		s := Status(q)
		status = &s
	}

	events, err := h.service.List(status)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("eventId")
	e, err := h.service.Get(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("eventId")
	e, err := h.service.Publish(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrValidation) {
			http.Error(w, "event missing required fields for publishing", http.StatusBadRequest)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
