package event

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/biletflow/backend-go/internal/auth"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/events", h.create)
	mux.HandleFunc("GET /api/events", h.list)
	mux.HandleFunc("GET /api/events/{eventId}", h.get)
	mux.HandleFunc("PATCH /api/events/{eventId}", h.update)
	mux.HandleFunc("POST /api/events/{eventId}/publish", h.publish)
}
func decode(r *http.Request, out any, patch bool) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil {
		return invalid("cannot read body")
	}
	if len(body) > 1<<20 {
		return invalid("body too large")
	}
	// Reject null and duplicate keys rather than silently accepting ambiguous input.
	d := json.NewDecoder(bytes.NewReader(body))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return invalid("body must be a JSON object")
	}
	allowed := map[string]bool{"title": true, "description": true, "category": true, "startsAt": true, "endsAt": true, "venueId": true, "visibility": true, "capacity": true}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return invalid("invalid JSON")
		}
		name, ok := key.(string)
		if !ok || seen[name] || !allowed[name] {
			return invalid("duplicate or unknown field")
		}
		seen[name] = true
		var raw json.RawMessage
		if err = d.Decode(&raw); err != nil {
			return invalid("invalid JSON")
		}
		if name == "visibility" && bytes.Equal(bytes.TrimSpace(raw), []byte(`""`)) {
			return invalid("invalid visibility")
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) && (patch || name == "title" || name == "startsAt" || name == "visibility") {
			return invalid("null is not allowed for this field")
		}
	}
	if _, err = d.Token(); err != nil {
		return invalid("invalid JSON")
	}
	if patch && len(seen) == 0 {
		return invalid("provide at least one field")
	}
	d = json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return invalid("invalid or unknown request field")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return invalid("expected one JSON object")
	}
	return nil
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateEventInput
	if _, err := organizer(r.Context()); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	if err := decode(r, &in, false); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	e, err := h.service.Create(r.Context(), in)
	respond(w, r, e, err, 201)
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	if _, err := organizer(r.Context()); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	var in UpdateEventInput
	if err := decode(r, &in, true); err != nil {
		respond(w, r, nil, err, 0)
		return
	}
	e, err := h.service.Update(r.Context(), r.PathValue("eventId"), in)
	respond(w, r, e, err, 200)
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	e, err := h.service.Get(r.Context(), r.PathValue("eventId"))
	respond(w, r, e, err, 200)
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	var status *Status
	if q, ok := r.URL.Query()["status"]; ok {
		if len(q) != 1 || q[0] == "" {
			respond(w, r, nil, invalid("invalid status"), 0)
			return
		}
		s := Status(q[0])
		status = &s
	}
	e, err := h.service.List(r.Context(), status)
	respond(w, r, e, err, 200)
}
func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	e, err := h.service.Publish(r.Context(), r.PathValue("eventId"))
	respond(w, r, e, err, 200)
}
func respond(w http.ResponseWriter, r *http.Request, v any, err error, success int) {
	if err == nil {
		writeJSON(w, success, v)
		return
	}
	status := 500
	message := "internal error"
	switch {
	case errors.Is(err, ErrValidation):
		status = 400
		message = err.Error()
	case errors.Is(err, ErrUnauthorized):
		status = 401
		message = ErrUnauthorized.Error()
	case errors.Is(err, ErrForbidden):
		status = 403
		message = ErrForbidden.Error()
	case errors.Is(err, ErrNotFound):
		status = 404
		message = ErrNotFound.Error()
	case errors.Is(err, ErrConflict):
		status = 409
		message = ErrConflict.Error()
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		status = 503
		message = "event storage unavailable"
	}
	if status >= 500 {
		slog.ErrorContext(r.Context(), "event operation failed", "method", r.Method, "route", r.Pattern, "errorType", timeOut(err))
	}
	auth.WriteError(w, status, message)
}

// Log only classification, never a driver error containing SQL/credentials/data.
func timeOut(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	return "storage"
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("response write failed")
	}
}
