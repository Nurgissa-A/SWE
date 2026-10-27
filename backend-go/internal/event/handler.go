package event

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/biletflow/backend-go/internal/auth"
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
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		auth.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if decoder.Decode(&struct{}{}) != io.EOF {
		auth.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	principal, ok := auth.Current(r.Context())
	if !ok {
		auth.WriteError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if principal.Role != "organizer" {
		auth.WriteError(w, http.StatusForbidden, "Organizer role required")
		return
	}
	organizerID := principal.ID

	e, err := h.service.Create(organizerID, in)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			auth.WriteError(w, http.StatusBadRequest, "title and startsAt are required")
			return
		}
		auth.WriteError(w, http.StatusInternalServerError, "internal error")
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
		auth.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	visible := make([]*Event, 0, len(events))
	principal, _ := auth.Current(r.Context())
	for _, e := range events {
		if (principal.Role == "organizer" && principal.ID == e.OrganizerID) || (e.Status == StatusPublished && e.Visibility == VisibilityPublic) {
			visible = append(visible, e)
		}
	}
	writeJSON(w, http.StatusOK, visible)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("eventId")
	e, err := h.service.Get(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			auth.WriteError(w, http.StatusNotFound, "event not found")
			return
		}
		auth.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	principal, _ := auth.Current(r.Context())
	if !((principal.Role == "organizer" && principal.ID == e.OrganizerID) || (e.Status == StatusPublished && (e.Visibility == VisibilityPublic || e.Visibility == VisibilityUnlisted))) {
		auth.WriteError(w, http.StatusNotFound, "Event not found")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.Current(r.Context())
	if !ok {
		auth.WriteError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if principal.Role != "organizer" {
		auth.WriteError(w, http.StatusForbidden, "Organizer role required")
		return
	}
	id := r.PathValue("eventId")
	current, err := h.service.Get(id)
	if err != nil {
		auth.WriteError(w, http.StatusNotFound, "Event not found")
		return
	}
	if current.OrganizerID != principal.ID {
		auth.WriteError(w, http.StatusForbidden, "Event owner required")
		return
	}
	e, err := h.service.Publish(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			auth.WriteError(w, http.StatusNotFound, "event not found")
			return
		}
		if errors.Is(err, ErrValidation) {
			auth.WriteError(w, http.StatusBadRequest, "event missing required fields for publishing")
			return
		}
		auth.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
