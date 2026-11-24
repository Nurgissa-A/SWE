// Package checkin serves the Event Admin check-in app (SRS §4.8).
package checkin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/commerce"
	"github.com/biletflow/backend-go/internal/platform"
)

type Handler struct {
	DB     *sql.DB
	Signer commerce.QRSigner
	Scans  *platform.RateLimiter
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/scanner/events", platform.Handler(h.events))
	mux.Handle("GET /api/scanner/events/{eventId}", platform.Handler(h.summary))
	mux.Handle("POST /api/scanner/events/{eventId}/scan", platform.Handler(h.scan))
	mux.Handle("POST /api/scanner/events/{eventId}/check-in", platform.Handler(h.manual))
	mux.Handle("POST /api/scanner/events/{eventId}/undo", platform.Handler(h.undo))
	mux.Handle("GET /api/scanner/events/{eventId}/attendees", platform.Handler(h.attendees))
}

// access describes what the caller may do at one event's door.
type access struct {
	title, timezone string
	startsAt        time.Time
	canUndo         bool
}

// staffAccess allows the event's organizer and Event Admins assigned to that event only.
// Anyone else gets 404, so event IDs cannot be probed.
func (h *Handler) staffAccess(ctx context.Context, p auth.Principal, eventID string) (*access, error) {
	if !platform.IsUUID(eventID) {
		return nil, platform.NotFound("Event not found")
	}
	var a access
	var organizerID, status string
	var staffUndo sql.NullBool
	err := h.DB.QueryRowContext(ctx, `SELECT e.title, e.timezone, e.starts_at, e.organizer_id, e.status, s.can_undo
		FROM events e LEFT JOIN staff_assignments s ON s.event_id=e.id AND s.user_id=$2 WHERE e.id=$1`, eventID, p.ID).
		Scan(&a.title, &a.timezone, &a.startsAt, &organizerID, &status, &staffUndo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, platform.NotFound("Event not found")
	}
	if err != nil {
		return nil, err
	}
	isOwner := p.Role == "organizer" && organizerID == p.ID
	if !isOwner && !staffUndo.Valid {
		return nil, platform.NotFound("Event not found")
	}
	if status == "draft" {
		return nil, platform.Conflict("NOT_PUBLISHED", "This event is not published")
	}
	a.canUndo = isOwner || staffUndo.Bool
	return &a, nil
}

type Counts struct {
	Sold      int `json:"sold"`
	CheckedIn int `json:"checkedIn"`
}

func (h *Handler) counts(ctx context.Context, eventID string) (Counts, error) {
	var c Counts
	err := h.DB.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE status IN ('valid','checked_in')), count(*) FILTER (WHERE status='checked_in')
		FROM tickets WHERE event_id=$1`, eventID).Scan(&c.Sold, &c.CheckedIn)
	return c, err
}

type ScannerEvent struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"startsAt"`
	Timezone string    `json:"timezone"`
	Venue    *string   `json:"venueName"`
	City     *string   `json:"city"`
	Status   string    `json:"status"`
	Role     string    `json:"role"`
	CanUndo  bool      `json:"canUndo"`
	Counts   Counts    `json:"counts"`
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT e.id, e.title, e.starts_at, e.timezone, e.venue_name, e.city, e.status,
		CASE WHEN e.organizer_id=$1 THEN 'organizer' ELSE 'event_admin' END, coalesce(s.can_undo, true),
		(SELECT count(*) FROM tickets t WHERE t.event_id=e.id AND t.status IN ('valid','checked_in')),
		(SELECT count(*) FROM tickets t WHERE t.event_id=e.id AND t.status='checked_in')
		FROM events e LEFT JOIN staff_assignments s ON s.event_id=e.id AND s.user_id=$1
		WHERE e.status <> 'draft' AND ((e.organizer_id=$1 AND $2) OR s.id IS NOT NULL)
		  AND coalesce(e.ends_at, e.starts_at + interval '6 hours') > now() - interval '2 days'
		ORDER BY e.starts_at LIMIT 100`, p.ID, p.Role == "organizer")
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []ScannerEvent{}
	for rows.Next() {
		var e ScannerEvent
		var venue, city sql.NullString
		if err := rows.Scan(&e.ID, &e.Title, &e.StartsAt, &e.Timezone, &venue, &city, &e.Status, &e.Role, &e.CanUndo, &e.Counts.Sold, &e.Counts.CheckedIn); err != nil {
			return err
		}
		e.Venue, e.City = platform.NullString(venue), platform.NullString(city)
		out = append(out, e)
	}
	platform.WriteJSON(w, http.StatusOK, out)
	return rows.Err()
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	eventID := r.PathValue("eventId")
	a, err := h.staffAccess(r.Context(), p, eventID)
	if err != nil {
		return err
	}
	c, err := h.counts(r.Context(), eventID)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, map[string]any{"id": eventID, "title": a.title, "startsAt": a.startsAt, "timezone": a.timezone, "canUndo": a.canUndo, "counts": c})
	return nil
}

// Result is what the door phone shows after a scan.
type Result struct {
	// Result is one of: admitted, already_used, cancelled, refunded, wrong_event, campaign_qr, invalid.
	Result  string      `json:"result"`
	Message string      `json:"message"`
	Ticket  *TicketInfo `json:"ticket,omitempty"`
	Counts  Counts      `json:"counts"`
}

type TicketInfo struct {
	ID           string     `json:"id"`
	Code         string     `json:"code"`
	AttendeeName string     `json:"attendeeName"`
	TicketType   string     `json:"ticketType"`
	Status       string     `json:"status"`
	CheckedInAt  *time.Time `json:"checkedInAt,omitempty"`
}

var messages = map[string]string{
	"admitted":     "Valid ticket. Let the guest in.",
	"already_used": "Already used. This ticket was checked in before.",
	"cancelled":    "Cancelled ticket. Do not admit.",
	"refunded":     "Refunded ticket. Do not admit.",
	"wrong_event":  "This ticket is for a different event.",
	"campaign_qr":  "This is a promotional QR code, not a ticket.",
	"invalid":      "Not a valid BiletFlow ticket.",
}

func (h *Handler) loadTicket(ctx context.Context, q platform.Querier, ticketID string) (*TicketInfo, string, error) {
	var t TicketInfo
	var eventID string
	var checked sql.NullTime
	err := q.QueryRowContext(ctx, `SELECT t.id, t.code, t.attendee_name, tt.name, t.status, t.checked_in_at, t.event_id
		FROM tickets t JOIN ticket_types tt ON tt.id=t.ticket_type_id WHERE t.id=$1`, ticketID).
		Scan(&t.ID, &t.Code, &t.AttendeeName, &t.TicketType, &t.Status, &checked, &eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	if checked.Valid {
		v := checked.Time.UTC()
		t.CheckedInAt = &v
	}
	return &t, eventID, nil
}

// admit checks a ticket in at most once. The conditional UPDATE is the single point of truth:
// when two devices scan the same ticket at the same moment, exactly one UPDATE matches.
func (h *Handler) admit(ctx context.Context, p auth.Principal, eventID, ticketID, method string) (*Result, error) {
	res := &Result{}
	err := platform.InTx(ctx, h.DB, func(tx *sql.Tx) error {
		var updated bool
		err := tx.QueryRowContext(ctx, `WITH u AS (UPDATE tickets SET status='checked_in', checked_in_at=now(), checked_in_by=$3
			WHERE id=$1 AND event_id=$2 AND status='valid' RETURNING id) SELECT EXISTS(SELECT 1 FROM u)`, ticketID, eventID, p.ID).Scan(&updated)
		if err != nil {
			return err
		}
		t, ticketEvent, err := h.loadTicket(ctx, tx, ticketID)
		if err != nil {
			return err
		}
		switch {
		case t == nil:
			res.Result = "invalid"
		case ticketEvent != eventID:
			res.Result, t = "wrong_event", nil
		case updated:
			res.Result = "admitted"
			if _, err := tx.ExecContext(ctx, `INSERT INTO check_in_records(ticket_id, event_id, staff_id, action, method) VALUES ($1,$2,$3,'check_in',$4)`,
				ticketID, eventID, p.ID, method); err != nil {
				return err
			}
		case t.Status == "checked_in":
			res.Result = "already_used"
		case t.Status == "cancelled" || t.Status == "refunded":
			res.Result = t.Status
		default:
			res.Result = "invalid"
		}
		res.Ticket = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	res.Message = messages[res.Result]
	res.Counts, err = h.counts(ctx, eventID)
	return res, err
}

func (h *Handler) scan(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	if !h.Scans.Allow(platform.ClientKey(r)) {
		return platform.ErrRateLimited
	}
	var in struct {
		Payload string `json:"payload"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	eventID := r.PathValue("eventId")
	if _, err := h.staffAccess(r.Context(), p, eventID); err != nil {
		return err
	}
	kind, ticketID := h.Signer.Parse(in.Payload)
	var res *Result
	switch kind {
	case commerce.ScanTicket:
		res, err = h.admit(r.Context(), p, eventID, ticketID, "qr")
		if err != nil {
			return err
		}
	default:
		// The admission endpoint never accepts a Campaign QR (SRS §4.14).
		result := "invalid"
		if kind == commerce.ScanCampaign {
			result = "campaign_qr"
		}
		c, err := h.counts(r.Context(), eventID)
		if err != nil {
			return err
		}
		res = &Result{Result: result, Message: messages[result], Counts: c}
	}
	platform.WriteJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handler) manual(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	var in struct {
		TicketID string `json:"ticketId"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	eventID := r.PathValue("eventId")
	if _, err := h.staffAccess(r.Context(), p, eventID); err != nil {
		return err
	}
	if !platform.IsUUID(in.TicketID) {
		return platform.NotFound("Ticket not found")
	}
	res, err := h.admit(r.Context(), p, eventID, in.TicketID, "manual")
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handler) undo(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	var in struct {
		TicketID string `json:"ticketId"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	eventID := r.PathValue("eventId")
	a, err := h.staffAccess(r.Context(), p, eventID)
	if err != nil {
		return err
	}
	if !a.canUndo {
		return platform.Forbidden("UNDO_NOT_ALLOWED", "You are not allowed to undo check-ins for this event")
	}
	if !platform.IsUUID(in.TicketID) {
		return platform.NotFound("Ticket not found")
	}
	var t *TicketInfo
	err = platform.InTx(r.Context(), h.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(r.Context(), `UPDATE tickets SET status='valid', checked_in_at=NULL, checked_in_by=NULL
			WHERE id=$1 AND event_id=$2 AND status='checked_in'`, in.TicketID, eventID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return platform.Conflict("NOT_CHECKED_IN", "This ticket is not checked in")
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO check_in_records(ticket_id, event_id, staff_id, action, method) VALUES ($1,$2,$3,'undo','manual')`,
			in.TicketID, eventID, p.ID); err != nil {
			return err
		}
		t, _, err = h.loadTicket(r.Context(), tx, in.TicketID)
		if err != nil {
			return err
		}
		return platform.Audit(r.Context(), tx, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "checkin.undone", EntityType: "ticket", EntityID: in.TicketID,
			Description: "Check-in undone for ticket " + t.Code})
	})
	if err != nil {
		return err
	}
	c, err := h.counts(r.Context(), eventID)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, map[string]any{"ticket": t, "counts": c})
	return nil
}

// attendees searches tickets by name, ticket code or order number. E-mail addresses are not
// returned to door staff (data minimisation, NFR-11).
func (h *Handler) attendees(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	eventID := r.PathValue("eventId")
	if _, err := h.staffAccess(r.Context(), p, eventID); err != nil {
		return err
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	rows, err := h.DB.QueryContext(r.Context(), `SELECT t.id, t.code, t.attendee_name, tt.name, t.status, t.checked_in_at
		FROM tickets t JOIN ticket_types tt ON tt.id=t.ticket_type_id JOIN orders o ON o.id=t.order_id
		WHERE t.event_id=$1 AND ($2 = '' OR lower(t.attendee_name) LIKE '%'||$2||'%' OR lower(t.code) LIKE '%'||$2||'%' OR lower(o.number) LIKE '%'||$2||'%')
		ORDER BY t.attendee_name, t.code LIMIT 50`, eventID, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []TicketInfo{}
	for rows.Next() {
		var t TicketInfo
		var checked sql.NullTime
		if err := rows.Scan(&t.ID, &t.Code, &t.AttendeeName, &t.TicketType, &t.Status, &checked); err != nil {
			return err
		}
		if checked.Valid {
			v := checked.Time.UTC()
			t.CheckedInAt = &v
		}
		out = append(out, t)
	}
	platform.WriteJSON(w, http.StatusOK, out)
	return rows.Err()
}
