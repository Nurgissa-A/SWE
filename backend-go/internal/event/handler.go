package event

import (
	"net/http"
	"time"

	"github.com/biletflow/backend-go/internal/platform"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires paths to match docs/openapi.yaml, so the contract and the
// implementation can't quietly drift apart.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/events", platform.Handler(h.create))
	mux.Handle("GET /api/events", platform.Handler(h.list))
	mux.Handle("GET /api/events/{eventId}", platform.Handler(h.get))
	mux.Handle("PATCH /api/events/{eventId}", platform.Handler(h.update))
	mux.Handle("DELETE /api/events/{eventId}", platform.Handler(h.delete))
	mux.Handle("POST /api/events/{eventId}/publish", platform.Handler(h.publish))
	mux.Handle("POST /api/events/{eventId}/unpublish", platform.Handler(h.unpublish))
	mux.Handle("POST /api/events/{eventId}/duplicate", platform.Handler(h.duplicate))
	mux.Handle("GET /api/events/{eventId}/ticket-types", platform.Handler(h.ticketTypes))
	mux.Handle("POST /api/events/{eventId}/ticket-types", platform.Handler(h.createTicketType))
	mux.Handle("PATCH /api/events/{eventId}/ticket-types/{ticketTypeId}", platform.Handler(h.updateTicketType))
	mux.Handle("DELETE /api/events/{eventId}/ticket-types/{ticketTypeId}", platform.Handler(h.deleteTicketType))
	mux.Handle("GET /api/events/{eventId}/staff", platform.Handler(h.staff))
	mux.Handle("POST /api/events/{eventId}/staff", platform.Handler(h.addStaff))
	mux.Handle("DELETE /api/events/{eventId}/staff/{staffId}", platform.Handler(h.removeStaff))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	var in CreateEventInput
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	e, err := h.service.Create(r.Context(), p, in)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusCreated, e)
	return nil
}

func parseTime(v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		if d, err2 := time.Parse("2006-01-02", v); err2 == nil {
			return &d, nil
		}
		return nil, platform.BadRequest("INVALID_DATE", "Dates must be ISO 8601")
	}
	return &t, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	f := ListFilter{Query: q.Get("q"), Category: q.Get("category"), City: q.Get("city"), Mine: q.Get("mine") == "true"}
	if s := q.Get("status"); s != "" {
		status := Status(s)
		if status != StatusDraft && status != StatusPublished && status != StatusCancelled {
			return platform.BadRequest("INVALID_STATUS", "Unknown status filter")
		}
		f.Status = &status
	}
	var err error
	if f.From, err = parseTime(q.Get("from")); err != nil {
		return err
	}
	if f.To, err = parseTime(q.Get("to")); err != nil {
		return err
	}
	events, err := h.service.List(r.Context(), platform.OptionalViewer(r), f)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, events)
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	e, err := h.service.Get(r.Context(), platform.OptionalViewer(r), r.PathValue("eventId"), r.URL.Query().Get("key"))
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, e)
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	var in UpdateEventInput
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	e, err := h.service.Update(r.Context(), p, r.PathValue("eventId"), in)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, e)
	return nil
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	if err := h.service.Delete(r.Context(), p, r.PathValue("eventId")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) lifecycle(fn func(*Service, *http.Request) (*Event, error)) platform.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if _, err := platform.Viewer(r); err != nil {
			return err
		}
		e, err := fn(h.service, r)
		if err != nil {
			return err
		}
		platform.WriteJSON(w, http.StatusOK, e)
		return nil
	}
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) error {
	return h.lifecycle(func(s *Service, r *http.Request) (*Event, error) {
		return s.Publish(r.Context(), platform.OptionalViewer(r), r.PathValue("eventId"))
	})(w, r)
}

func (h *Handler) unpublish(w http.ResponseWriter, r *http.Request) error {
	return h.lifecycle(func(s *Service, r *http.Request) (*Event, error) {
		return s.Unpublish(r.Context(), platform.OptionalViewer(r), r.PathValue("eventId"))
	})(w, r)
}

func (h *Handler) duplicate(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	e, err := h.service.Duplicate(r.Context(), p, r.PathValue("eventId"))
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusCreated, e)
	return nil
}

func (h *Handler) ticketTypes(w http.ResponseWriter, r *http.Request) error {
	types, err := h.service.TicketTypes(r.Context(), platform.OptionalViewer(r), r.PathValue("eventId"), r.URL.Query().Get("key"))
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, types)
	return nil
}

func (h *Handler) createTicketType(w http.ResponseWriter, r *http.Request) error {
	var in TicketTypeInput
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	t, err := h.service.CreateTicketType(r.Context(), p, r.PathValue("eventId"), in)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusCreated, t)
	return nil
}

func (h *Handler) updateTicketType(w http.ResponseWriter, r *http.Request) error {
	var in TicketTypeInput
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	typeID, err := platform.PathID(r, "ticketTypeId", "Ticket type")
	if err != nil {
		return err
	}
	t, err := h.service.UpdateTicketType(r.Context(), p, r.PathValue("eventId"), typeID, in)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, t)
	return nil
}

func (h *Handler) deleteTicketType(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	typeID, err := platform.PathID(r, "ticketTypeId", "Ticket type")
	if err != nil {
		return err
	}
	if err := h.service.DeleteTicketType(r.Context(), p, r.PathValue("eventId"), typeID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) staff(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	list, err := h.service.Staff(r.Context(), p, r.PathValue("eventId"))
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, list)
	return nil
}

func (h *Handler) addStaff(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email   string `json:"email"`
		CanUndo *bool  `json:"canUndo,omitempty"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	canUndo := in.CanUndo == nil || *in.CanUndo
	m, err := h.service.AddStaff(r.Context(), r, p, r.PathValue("eventId"), in.Email, canUndo)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusCreated, m)
	return nil
}

func (h *Handler) removeStaff(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	staffID, err := platform.PathID(r, "staffId", "Staff member")
	if err != nil {
		return err
	}
	if err := h.service.RemoveStaff(r.Context(), p, r.PathValue("eventId"), staffID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
