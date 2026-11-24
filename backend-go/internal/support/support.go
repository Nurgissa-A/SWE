// Package support implements support cases with asynchronous message threads (SRS §4.13).
//
// Attendee cases belong to an event and are handled by that event's organizer, who can
// escalate them to BiletFlow; organizer cases go to platform admins. Nobody else can see a case.
package support

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/notify"
	"github.com/biletflow/backend-go/internal/platform"
)

type Handler struct {
	DB       *sql.DB
	Messages *platform.RateLimiter
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/support/cases", platform.Handler(h.list))
	mux.Handle("POST /api/support/cases", platform.Handler(h.create))
	mux.Handle("GET /api/support/cases/{caseId}", platform.Handler(h.get))
	mux.Handle("POST /api/support/cases/{caseId}/messages", platform.Handler(h.reply))
	mux.Handle("PATCH /api/support/cases/{caseId}", platform.Handler(h.update))
}

var categories = map[string]bool{"ticket_delivery": true, "payment": true, "refund": true, "event_information": true,
	"check_in": true, "account": true, "activation": true, "technical": true}
var statuses = map[string]bool{"open": true, "in_progress": true, "waiting_for_customer": true, "resolved": true}

type Case struct {
	ID          string     `json:"id"`
	Number      int        `json:"number"`
	Kind        string     `json:"kind"`
	Category    string     `json:"category"`
	Subject     string     `json:"subject"`
	Status      string     `json:"status"`
	RequesterID string     `json:"requesterId"`
	Requester   string     `json:"requester"`
	EventID     *string    `json:"eventId"`
	EventTitle  *string    `json:"eventTitle"`
	OrderID     *string    `json:"orderId"`
	OrderNumber *string    `json:"orderNumber"`
	TicketID    *string    `json:"ticketId"`
	TicketCode  *string    `json:"ticketCode"`
	AssignedTo  *string    `json:"assignedTo"`
	Assignee    *string    `json:"assignee"`
	Escalated   bool       `json:"escalated"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt"`
	Messages    []Message  `json:"messages,omitempty"`
	// Viewer permissions.
	CanManage   bool `json:"canManage"`
	CanEscalate bool `json:"canEscalate"`
	organizerID string
}

type Message struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"authorId"`
	Author    string    `json:"author"`
	Role      string    `json:"role"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

const caseColumns = `c.id, c.number, c.kind, c.category, c.subject, c.status, c.requester_id, coalesce(rd.display_name, rd.email, 'User'),
	c.event_id, e.title, coalesce(e.organizer_id::text, ''), c.order_id, o.number, c.ticket_id, t.code, c.assigned_to, coalesce(ad.display_name, ad.email),
	c.escalated, c.created_at, c.updated_at, c.resolved_at
	FROM support_cases c
	LEFT JOIN events e ON e.id=c.event_id LEFT JOIN orders o ON o.id=c.order_id LEFT JOIN tickets t ON t.id=c.ticket_id
	LEFT JOIN user_directory rd ON rd.user_id=c.requester_id LEFT JOIN user_directory ad ON ad.user_id=c.assigned_to`

func scanCase(row interface{ Scan(...any) error }) (*Case, error) {
	var c Case
	var eventID, title, orderID, orderNo, ticketID, code, assigned, assignee sql.NullString
	var resolved sql.NullTime
	err := row.Scan(&c.ID, &c.Number, &c.Kind, &c.Category, &c.Subject, &c.Status, &c.RequesterID, &c.Requester, &eventID, &title, &c.organizerID,
		&orderID, &orderNo, &ticketID, &code, &assigned, &assignee, &c.Escalated, &c.CreatedAt, &c.UpdatedAt, &resolved)
	if err != nil {
		return nil, err
	}
	c.EventID, c.EventTitle, c.OrderID, c.OrderNumber = platform.NullString(eventID), platform.NullString(title), platform.NullString(orderID), platform.NullString(orderNo)
	c.TicketID, c.TicketCode, c.AssignedTo, c.Assignee = platform.NullString(ticketID), platform.NullString(code), platform.NullString(assigned), platform.NullString(assignee)
	if resolved.Valid {
		c.ResolvedAt = &resolved.Time
	}
	return &c, nil
}

// handles reports whether p may work on the case (not just read it as requester).
func handles(p auth.Principal, c *Case) bool {
	if p.Role == "platform_admin" {
		return c.Kind == "organizer" || c.Escalated
	}
	return c.Kind == "attendee" && p.Role == "organizer" && c.organizerID == p.ID
}

func canSee(p auth.Principal, c *Case) bool { return c.RequesterID == p.ID || handles(p, c) }

func (h *Handler) load(ctx context.Context, p auth.Principal, id string) (*Case, error) {
	if !platform.IsUUID(id) {
		return nil, platform.NotFound("Support case not found")
	}
	c, err := scanCase(h.DB.QueryRowContext(ctx, `SELECT `+caseColumns+` WHERE c.id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !canSee(p, c)) {
		return nil, platform.NotFound("Support case not found")
	}
	if err != nil {
		return nil, err
	}
	c.CanManage = handles(p, c)
	c.CanEscalate = c.Kind == "attendee" && !c.Escalated && p.Role == "organizer" && c.organizerID == p.ID
	return c, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	where := `c.requester_id=$1`
	switch p.Role {
	case "organizer":
		where += ` OR (c.kind='attendee' AND e.organizer_id=$1)`
	case "platform_admin":
		where += ` OR c.kind='organizer' OR c.escalated`
	}
	args := []any{p.ID}
	if s := r.URL.Query().Get("status"); statuses[s] {
		where = `(` + where + `) AND c.status=$2`
		args = append(args, s)
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT `+caseColumns+` WHERE `+where+` ORDER BY c.updated_at DESC LIMIT 200`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []*Case{}
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return err
		}
		c.CanManage = handles(p, c)
		out = append(out, c)
	}
	platform.WriteJSON(w, http.StatusOK, out)
	return rows.Err()
}

// recipients are the people told about activity on a case, other than the actor.
func (h *Handler) recipients(ctx context.Context, q platform.Querier, c *Case, actor string) []string {
	set := map[string]bool{c.RequesterID: true}
	if c.AssignedTo != nil {
		set[*c.AssignedTo] = true
	} else if c.Kind == "attendee" && c.organizerID != "" {
		set[c.organizerID] = true
	}
	if c.Kind == "organizer" || c.Escalated {
		rows, err := q.QueryContext(ctx, `SELECT user_id FROM user_directory WHERE role='platform_admin'`)
		if err == nil {
			for rows.Next() {
				var id string
				if rows.Scan(&id) == nil && c.AssignedTo == nil {
					set[id] = true
				}
			}
			rows.Close()
		}
	}
	delete(set, actor)
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

func preview(s string) string {
	if utf8.RuneCountInString(s) <= 200 {
		return s
	}
	return string([]rune(s)[:200]) + "…"
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	if !h.Messages.Allow(platform.ClientKey(r)) {
		return platform.ErrRateLimited
	}
	var in struct {
		Kind     string  `json:"kind,omitempty"`
		Category string  `json:"category"`
		Subject  string  `json:"subject"`
		Message  string  `json:"message"`
		EventID  *string `json:"eventId,omitempty"`
		OrderID  *string `json:"orderId,omitempty"`
		TicketID *string `json:"ticketId,omitempty"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	in.Subject, in.Message = strings.TrimSpace(in.Subject), strings.TrimSpace(in.Message)
	if in.Kind == "" {
		in.Kind = "attendee"
	}
	switch {
	case in.Kind != "attendee" && in.Kind != "organizer":
		return platform.BadRequest("INVALID_KIND", "Unknown case type")
	case in.Kind == "organizer" && p.Role != "organizer":
		return platform.Forbidden("ORGANIZER_REQUIRED", "Organizer role required")
	case !categories[in.Category]:
		return platform.BadRequest("INVALID_CATEGORY", "Choose a category")
	case utf8.RuneCountInString(in.Subject) < 3 || utf8.RuneCountInString(in.Subject) > 150:
		return platform.BadRequest("SUBJECT_REQUIRED", "Write a short subject (3–150 characters)")
	case utf8.RuneCountInString(in.Message) < 1 || utf8.RuneCountInString(in.Message) > 5000:
		return platform.BadRequest("MESSAGE_REQUIRED", "Write a message (up to 5000 characters)")
	}
	ctx := r.Context()
	// Context comes from the requester's own order or ticket; anything else is rejected (SRS §4.13).
	if in.TicketID != nil {
		var orderID, eventID, owner string
		if !platform.IsUUID(*in.TicketID) || h.DB.QueryRowContext(ctx, `SELECT order_id, event_id, user_id FROM tickets WHERE id=$1`, *in.TicketID).Scan(&orderID, &eventID, &owner) != nil || owner != p.ID {
			return platform.BadRequest("CONTEXT_NOT_YOURS", "You can only attach your own orders and tickets")
		}
		in.OrderID, in.EventID = &orderID, &eventID
	}
	if in.OrderID != nil {
		var eventID, owner string
		if !platform.IsUUID(*in.OrderID) || h.DB.QueryRowContext(ctx, `SELECT event_id, user_id FROM orders WHERE id=$1`, *in.OrderID).Scan(&eventID, &owner) != nil || owner != p.ID {
			return platform.BadRequest("CONTEXT_NOT_YOURS", "You can only attach your own orders and tickets")
		}
		if in.EventID != nil && *in.EventID != eventID {
			return platform.BadRequest("CONTEXT_MISMATCH", "The order belongs to a different event")
		}
		in.EventID = &eventID
	}
	if in.EventID != nil {
		var organizer, status string
		if !platform.IsUUID(*in.EventID) || h.DB.QueryRowContext(ctx, `SELECT organizer_id, status FROM events WHERE id=$1`, *in.EventID).Scan(&organizer, &status) != nil ||
			(status == "draft" && organizer != p.ID) {
			return platform.NotFound("Event not found")
		}
		if in.Kind == "organizer" && organizer != p.ID {
			return platform.BadRequest("CONTEXT_NOT_YOURS", "Organizer cases can only reference your own events")
		}
	}
	if in.Kind == "attendee" && in.EventID == nil {
		return platform.BadRequest("EVENT_REQUIRED", "Open the case from the event, order or ticket it is about")
	}
	var id string
	err = platform.InTx(ctx, h.DB, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `INSERT INTO support_cases(kind, requester_id, category, subject, event_id, order_id, ticket_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`, in.Kind, p.ID, in.Category, in.Subject, in.EventID, in.OrderID, in.TicketID).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO support_messages(case_id, author_id, body) VALUES ($1,$2,$3)`, id, p.ID, in.Message); err != nil {
			return err
		}
		c, err := scanCase(tx.QueryRowContext(ctx, `SELECT `+caseColumns+` WHERE c.id=$1`, id))
		if err != nil {
			return err
		}
		for _, u := range h.recipients(ctx, tx, c, p.ID) {
			if err := notify.Queue(ctx, tx, notify.Message{UserID: u, Type: "support_new_case", Params: map[string]any{"number": c.Number, "subject": c.Subject, "preview": preview(in.Message)},
				Link: "/support/" + id, Email: true}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	c, err := h.load(ctx, p, id)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusCreated, c)
	return nil
}

func (h *Handler) withMessages(ctx context.Context, c *Case) error {
	rows, err := h.DB.QueryContext(ctx, `SELECT m.id, m.author_id, coalesce(d.display_name, d.email, 'User'), coalesce(d.role,''), m.body, m.created_at
		FROM support_messages m LEFT JOIN user_directory d ON d.user_id=m.author_id WHERE m.case_id=$1 ORDER BY m.created_at`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	c.Messages = []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.AuthorID, &m.Author, &m.Role, &m.Body, &m.CreatedAt); err != nil {
			return err
		}
		switch {
		case m.AuthorID == c.RequesterID:
			m.Role = "requester"
		case m.Role == "platform_admin":
			m.Role = "biletflow"
		default:
			m.Role = "organizer"
		}
		c.Messages = append(c.Messages, m)
	}
	return rows.Err()
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	c, err := h.load(r.Context(), p, r.PathValue("caseId"))
	if err != nil {
		return err
	}
	if err := h.withMessages(r.Context(), c); err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, c)
	return nil
}

func (h *Handler) reply(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	if !h.Messages.Allow(platform.ClientKey(r)) {
		return platform.ErrRateLimited
	}
	var in struct {
		Body string `json:"body"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	in.Body = strings.TrimSpace(in.Body)
	if utf8.RuneCountInString(in.Body) < 1 || utf8.RuneCountInString(in.Body) > 5000 {
		return platform.BadRequest("MESSAGE_REQUIRED", "Write a message (up to 5000 characters)")
	}
	ctx := r.Context()
	c, err := h.load(ctx, p, r.PathValue("caseId"))
	if err != nil {
		return err
	}
	err = platform.InTx(ctx, h.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO support_messages(case_id, author_id, body) VALUES ($1,$2,$3)`, c.ID, p.ID, in.Body); err != nil {
			return err
		}
		// A requester's reply reopens a resolved or waiting case; a handler's reply moves a new case into progress.
		next := c.Status
		if p.ID == c.RequesterID && (c.Status == "waiting_for_customer" || c.Status == "resolved") {
			next = "open"
		} else if p.ID != c.RequesterID && c.Status == "open" {
			next = "in_progress"
		}
		if _, err := tx.ExecContext(ctx, `UPDATE support_cases SET status=$2, updated_at=now(), resolved_at = CASE WHEN $2='resolved' THEN resolved_at ELSE NULL END WHERE id=$1`, c.ID, next); err != nil {
			return err
		}
		for _, u := range h.recipients(ctx, tx, c, p.ID) {
			if err := notify.Queue(ctx, tx, notify.Message{UserID: u, Type: "support_reply", Params: map[string]any{"number": c.Number, "preview": preview(in.Body)},
				Link: "/support/" + c.ID, Email: true}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	c, err = h.load(ctx, p, c.ID)
	if err != nil {
		return err
	}
	if err := h.withMessages(ctx, c); err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusCreated, c)
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	var in struct {
		Status     *string `json:"status,omitempty"`
		AssignToMe bool    `json:"assignToMe,omitempty"`
		Escalate   bool    `json:"escalate,omitempty"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	c, err := h.load(ctx, p, r.PathValue("caseId"))
	if err != nil {
		return err
	}
	isRequester := p.ID == c.RequesterID
	switch {
	case in.Escalate && !c.CanEscalate:
		return platform.Forbidden("CANNOT_ESCALATE", "Only the event organizer can escalate this case")
	case (in.AssignToMe || (in.Status != nil && *in.Status != "resolved")) && !c.CanManage:
		return platform.Forbidden("CANNOT_MANAGE", "You cannot change this case")
	case in.Status != nil && !statuses[*in.Status]:
		return platform.BadRequest("INVALID_STATUS", "Unknown status")
	case in.Status != nil && *in.Status == "resolved" && !c.CanManage && !isRequester:
		return platform.Forbidden("CANNOT_MANAGE", "You cannot change this case")
	}
	err = platform.InTx(ctx, h.DB, func(tx *sql.Tx) error {
		if in.Status != nil && *in.Status != c.Status {
			if _, err := tx.ExecContext(ctx, `UPDATE support_cases SET status=$2, updated_at=now(),
				resolved_at = CASE WHEN $2='resolved' THEN now() ELSE NULL END WHERE id=$1`, c.ID, *in.Status); err != nil {
				return err
			}
			if !isRequester {
				if err := notify.Queue(ctx, tx, notify.Message{UserID: c.RequesterID, Type: "support_status", Params: map[string]any{"number": c.Number, "status": *in.Status},
					Link: "/support/" + c.ID, Email: true}); err != nil {
					return err
				}
			}
		}
		if in.AssignToMe {
			if _, err := tx.ExecContext(ctx, `UPDATE support_cases SET assigned_to=$2, updated_at=now(), status = CASE WHEN status='open' THEN 'in_progress' ELSE status END WHERE id=$1`, c.ID, p.ID); err != nil {
				return err
			}
		}
		if in.Escalate {
			if _, err := tx.ExecContext(ctx, `UPDATE support_cases SET escalated=true, assigned_to=NULL, updated_at=now() WHERE id=$1`, c.ID); err != nil {
				return err
			}
			c.Escalated, c.AssignedTo = true, nil
			rows, err := tx.QueryContext(ctx, `SELECT user_id FROM user_directory WHERE role='platform_admin'`)
			if err != nil {
				return err
			}
			var admins []string
			for rows.Next() {
				var id string
				if rows.Scan(&id) == nil {
					admins = append(admins, id)
				}
			}
			rows.Close()
			for _, a := range admins {
				if err := notify.Queue(ctx, tx, notify.Message{UserID: a, Type: "support_escalated", Params: map[string]any{"number": c.Number, "subject": c.Subject},
					Link: "/support/" + c.ID, Email: true}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	c, err = h.load(ctx, p, c.ID)
	if err != nil {
		return err
	}
	if err := h.withMessages(ctx, c); err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, c)
	return nil
}
