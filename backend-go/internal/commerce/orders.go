package commerce

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/notify"
	"github.com/biletflow/backend-go/internal/platform"
	"github.com/biletflow/backend-go/internal/promo"
)

type OrderEvent struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	StartsAt     time.Time  `json:"startsAt"`
	EndsAt       *time.Time `json:"endsAt,omitempty"`
	Timezone     string     `json:"timezone"`
	VenueName    *string    `json:"venueName,omitempty"`
	City         *string    `json:"city,omitempty"`
	Status       string     `json:"status"`
	RefundPolicy string     `json:"refundPolicy"`
	OrganizerID  string     `json:"organizerId"`
}

type OrderItem struct {
	TicketTypeID string `json:"ticketTypeId"`
	Name         string `json:"name"`
	Quantity     int    `json:"quantity"`
	UnitPriceKzt int    `json:"unitPriceKzt"`
	DiscountKzt  int    `json:"discountKzt"`
}

type Ticket struct {
	ID           string     `json:"id"`
	Code         string     `json:"code"`
	TicketType   string     `json:"ticketType"`
	AttendeeName string     `json:"attendeeName"`
	Status       string     `json:"status"`
	CheckedInAt  *time.Time `json:"checkedInAt,omitempty"`
	QRPayload    string     `json:"qrPayload,omitempty"`
}

type Order struct {
	ID          string      `json:"id"`
	Number      string      `json:"number"`
	Status      string      `json:"status"`
	Event       OrderEvent  `json:"event"`
	BuyerName   string      `json:"buyerName"`
	BuyerEmail  string      `json:"buyerEmail"`
	BuyerPhone  *string     `json:"buyerPhone,omitempty"`
	Items       []OrderItem `json:"items"`
	SubtotalKzt int         `json:"subtotalKzt"`
	DiscountKzt int         `json:"discountKzt"`
	TotalKzt    int         `json:"totalKzt"`
	PromoCode   *string     `json:"promoCode,omitempty"`
	IsSimulated bool        `json:"isSimulated"`
	ExpiresAt   *time.Time  `json:"expiresAt,omitempty"`
	PaidAt      *time.Time  `json:"paidAt,omitempty"`
	RefundedAt  *time.Time  `json:"refundedAt,omitempty"`
	CancelledAt *time.Time  `json:"cancelledAt,omitempty"`
	CreatedAt   time.Time   `json:"createdAt"`
	Tickets     []Ticket    `json:"tickets"`
	// Permissions for the viewer, so clients can show or hide actions.
	CanRefund bool `json:"canRefund"`
	CanCancel bool `json:"canCancel"`
}

func nt(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// LoadOrder returns an order with items and tickets. QR payloads are included only for the owner.
func (s *Service) LoadOrder(ctx context.Context, q platform.Querier, orderID string, p auth.Principal) (*Order, error) {
	if !platform.IsUUID(orderID) {
		return nil, platform.NotFound("Order not found")
	}
	var o Order
	var userID string
	var phone, venue, city, code sql.NullString
	var ends, expires, paid, refunded, cancelled sql.NullTime
	err := q.QueryRowContext(ctx, `SELECT o.id, o.number, o.status, o.user_id, o.buyer_name, o.buyer_email, o.buyer_phone, o.subtotal_kzt, o.discount_kzt,
		o.total_kzt, o.expires_at, o.paid_at, o.refunded_at, o.cancelled_at, o.created_at, c.code,
		e.id, e.title, e.starts_at, e.ends_at, e.timezone, e.venue_name, e.city, e.status, e.refund_policy, e.organizer_id
		FROM orders o JOIN events e ON e.id=o.event_id LEFT JOIN promo_campaigns c ON c.id=o.promo_campaign_id WHERE o.id=$1`, orderID).
		Scan(&o.ID, &o.Number, &o.Status, &userID, &o.BuyerName, &o.BuyerEmail, &phone, &o.SubtotalKzt, &o.DiscountKzt, &o.TotalKzt,
			&expires, &paid, &refunded, &cancelled, &o.CreatedAt, &code, &o.Event.ID, &o.Event.Title, &o.Event.StartsAt, &ends,
			&o.Event.Timezone, &venue, &city, &o.Event.Status, &o.Event.RefundPolicy, &o.Event.OrganizerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, platform.NotFound("Order not found")
	}
	if err != nil {
		return nil, err
	}
	owner := userID == p.ID
	organizer := p.Role == "organizer" && p.ID == o.Event.OrganizerID
	if !owner && !organizer && p.Role != "platform_admin" {
		return nil, platform.NotFound("Order not found")
	}
	o.BuyerPhone, o.PromoCode = platform.NullString(phone), platform.NullString(code)
	o.Event.VenueName, o.Event.City, o.Event.EndsAt = platform.NullString(venue), platform.NullString(city), nt(ends)
	o.ExpiresAt, o.PaidAt, o.RefundedAt, o.CancelledAt = nt(expires), nt(paid), nt(refunded), nt(cancelled)
	o.IsSimulated = true

	rows, err := q.QueryContext(ctx, `SELECT oi.ticket_type_id, tt.name, oi.quantity, oi.unit_price_kzt, oi.discount_kzt
		FROM order_items oi JOIN ticket_types tt ON tt.id=oi.ticket_type_id WHERE oi.order_id=$1 ORDER BY tt.sort_order, tt.name`, orderID)
	if err != nil {
		return nil, err
	}
	o.Items = []OrderItem{}
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.TicketTypeID, &it.Name, &it.Quantity, &it.UnitPriceKzt, &it.DiscountKzt); err != nil {
			rows.Close()
			return nil, err
		}
		o.Items = append(o.Items, it)
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT t.id, t.code, tt.name, t.attendee_name, t.status, t.checked_in_at
		FROM tickets t JOIN ticket_types tt ON tt.id=t.ticket_type_id WHERE t.order_id=$1 ORDER BY t.issued_at, t.code`, orderID)
	if err != nil {
		return nil, err
	}
	o.Tickets = []Ticket{}
	anyUsed := false
	for rows.Next() {
		var t Ticket
		var checked sql.NullTime
		if err := rows.Scan(&t.ID, &t.Code, &t.TicketType, &t.AttendeeName, &t.Status, &checked); err != nil {
			rows.Close()
			return nil, err
		}
		t.CheckedInAt = nt(checked)
		if owner && t.Status == "valid" {
			t.QRPayload = s.Signer.Payload(t.ID)
		}
		anyUsed = anyUsed || t.Status == "checked_in"
		o.Tickets = append(o.Tickets, t)
	}
	rows.Close()
	o.CanRefund = organizer && o.Status == "completed" && o.TotalKzt > 0 && !anyUsed
	o.CanCancel = (organizer || owner) && o.Status == "completed" && o.TotalKzt == 0 && !anyUsed
	return &o, nil
}

// OrderSummary is a row in a list of orders.
type OrderSummary struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Status      string    `json:"status"`
	EventID     string    `json:"eventId"`
	EventTitle  string    `json:"eventTitle"`
	StartsAt    time.Time `json:"startsAt"`
	BuyerName   string    `json:"buyerName"`
	BuyerEmail  string    `json:"buyerEmail"`
	TotalKzt    int       `json:"totalKzt"`
	DiscountKzt int       `json:"discountKzt"`
	Tickets     int       `json:"tickets"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (s *Service) listOrders(ctx context.Context, where string, args ...any) ([]OrderSummary, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT o.id, o.number, o.status, e.id, e.title, e.starts_at, o.buyer_name, o.buyer_email, o.total_kzt, o.discount_kzt,
		(SELECT count(*) FROM tickets t WHERE t.order_id=o.id), o.created_at
		FROM orders o JOIN events e ON e.id=o.event_id WHERE `+where+` ORDER BY o.created_at DESC LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OrderSummary{}
	for rows.Next() {
		var o OrderSummary
		if err := rows.Scan(&o.ID, &o.Number, &o.Status, &o.EventID, &o.EventTitle, &o.StartsAt, &o.BuyerName, &o.BuyerEmail, &o.TotalKzt, &o.DiscountKzt, &o.Tickets, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// MyOrders lists the attendee's own orders (excluding abandoned holds).
func (s *Service) MyOrders(ctx context.Context, p auth.Principal) ([]OrderSummary, error) {
	return s.listOrders(ctx, `o.user_id=$1 AND o.status IN ('completed','refunded','cancelled','pending')`, p.ID)
}

// EventOrders lists an event's orders for its organizer, optionally filtered.
func (s *Service) EventOrders(ctx context.Context, p auth.Principal, eventID, query, status string) ([]OrderSummary, error) {
	if _, err := s.Events.Managed(ctx, p, eventID); err != nil {
		return nil, err
	}
	where := `o.event_id=$1 AND o.status IN ('completed','refunded','cancelled')`
	args := []any{eventID}
	if status != "" {
		where += ` AND o.status=$2`
		args = append(args, status)
	}
	if q := strings.ToLower(strings.TrimSpace(query)); q != "" {
		args = append(args, "%"+q+"%")
		where += fmt.Sprintf(` AND (lower(o.number) LIKE $%d OR lower(o.buyer_name) LIKE $%[1]d OR lower(o.buyer_email) LIKE $%[1]d)`, len(args))
	}
	return s.listOrders(ctx, where, args...)
}

// TicketPDF renders the owner's tickets for an order, or one ticket of it.
func (s *Service) TicketPDF(ctx context.Context, p auth.Principal, orderID, ticketID string) ([]byte, string, error) {
	o, err := s.LoadOrder(ctx, s.DB, orderID, p)
	if err != nil {
		return nil, "", err
	}
	if o.Status != "completed" {
		return nil, "", platform.Conflict("NO_VALID_TICKETS", "This order has no valid tickets")
	}
	if ticketID != "" && !platform.IsUUID(ticketID) {
		return nil, "", platform.NotFound("Ticket not found")
	}
	tickets, err := LoadPrintable(ctx, s.DB, orderID, ticketID)
	if err != nil {
		return nil, "", err
	}
	if len(tickets) == 0 {
		return nil, "", platform.NotFound("Ticket not found")
	}
	pdf, err := s.Signer.RenderPDF(tickets)
	return pdf, "biletflow-" + o.Number + ".pdf", err
}

// Attachments adds the PDF tickets to confirmation e-mails sent by the outbox.
func (s *Service) Attachments(ctx context.Context, kind string, params map[string]any, userID string) []notify.Attachment {
	if kind != "order_confirmed" && kind != "registration_confirmed" {
		return nil
	}
	orderID, _ := params["orderId"].(string)
	if orderID == "" {
		return nil
	}
	tickets, err := LoadPrintable(ctx, s.DB, orderID, "")
	if err != nil || len(tickets) == 0 {
		return nil
	}
	pdf, err := s.Signer.RenderPDF(tickets)
	if err != nil {
		return nil
	}
	return []notify.Attachment{{Name: "biletflow-tickets.pdf", ContentType: "application/pdf", Data: pdf}}
}

// refundOrder refunds one completed paid order inside tx: tickets become invalid, a simulated
// refund and an audit entry are recorded, and the attendee is notified (SRS §4.9).
func (s *Service) refundOrder(ctx context.Context, tx *sql.Tx, orderID, actorID, reason string) error {
	var eventID, userID, number, title string
	var total int
	err := tx.QueryRowContext(ctx, `UPDATE orders o SET status='refunded', refunded_at=now() FROM events e
		WHERE o.id=$1 AND e.id=o.event_id AND o.status='completed' RETURNING o.event_id, o.user_id, o.number, o.total_kzt, e.title`, orderID).
		Scan(&eventID, &userID, &number, &total, &title)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tickets SET status='refunded' WHERE order_id=$1 AND status IN ('valid','checked_in')`, orderID); err != nil {
		return err
	}
	var ref string
	_ = tx.QueryRowContext(ctx, `SELECT provider_ref FROM payments WHERE order_id=$1 AND status='succeeded' LIMIT 1`, orderID).Scan(&ref)
	if _, err := s.Provider.Refund(ref, total); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO refunds(order_id, amount_kzt, reason, initiated_by) VALUES ($1,$2,$3,$4)`, orderID, total, reason, actorID); err != nil {
		return err
	}
	if err := platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: actorID, Action: "order.refunded", EntityType: "order", EntityID: orderID,
		Description: fmt.Sprintf("Order %s refunded: %d KZT (simulated). Reason: %s", number, total, reason), Metadata: map[string]any{"amountKzt": total}}); err != nil {
		return err
	}
	return notify.Queue(ctx, tx, notify.Message{UserID: userID, Type: "refund_completed", Params: map[string]any{"number": number, "total": total, "event": title},
		Link: "/orders/" + orderID, Email: true})
}

// cancelRegistration cancels a completed free order inside tx.
func cancelRegistration(ctx context.Context, tx *sql.Tx, orderID, actorID string, notifyAttendee bool) error {
	var eventID, userID, number, title string
	err := tx.QueryRowContext(ctx, `UPDATE orders o SET status='cancelled', cancelled_at=now() FROM events e
		WHERE o.id=$1 AND e.id=o.event_id AND o.status='completed' RETURNING o.event_id, o.user_id, o.number, e.title`, orderID).Scan(&eventID, &userID, &number, &title)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tickets SET status='cancelled' WHERE order_id=$1 AND status IN ('valid','checked_in')`, orderID); err != nil {
		return err
	}
	if err := platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: actorID, Action: "order.cancelled", EntityType: "order", EntityID: orderID,
		Description: "Registration " + number + " cancelled"}); err != nil {
		return err
	}
	if !notifyAttendee {
		return nil
	}
	return notify.Queue(ctx, tx, notify.Message{UserID: userID, Type: "registration_cancelled", Params: map[string]any{"event": title, "number": number}, Link: "/orders/" + orderID, Email: true})
}

// Refund is the organizer-initiated full refund of a paid order (decision D-8): not allowed
// once any of its tickets was used.
func (s *Service) Refund(ctx context.Context, p auth.Principal, orderID, reason string) (*Order, error) {
	o, err := s.LoadOrder(ctx, s.DB, orderID, p)
	if err != nil {
		return nil, err
	}
	if p.Role != "organizer" || p.ID != o.Event.OrganizerID {
		return nil, platform.Forbidden("NOT_EVENT_OWNER", "Only the event organizer can refund orders")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, platform.BadRequest("REASON_REQUIRED", "Give a reason for the refund")
	}
	if !o.CanRefund {
		return nil, platform.Conflict("NOT_REFUNDABLE", "Only paid orders whose tickets were not used can be refunded")
	}
	err = platform.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		var locked string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&locked); err != nil {
			return err
		}
		var used bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tickets WHERE order_id=$1 AND status='checked_in')`, orderID).Scan(&used); err != nil {
			return err
		}
		if locked != "completed" || used {
			return platform.Conflict("NOT_REFUNDABLE", "Only paid orders whose tickets were not used can be refunded")
		}
		return s.refundOrder(ctx, tx, orderID, p.ID, reason)
	})
	if err != nil {
		return nil, err
	}
	return s.LoadOrder(ctx, s.DB, orderID, p)
}

// CancelRegistration cancels a free registration, by the organizer or by the attendee.
func (s *Service) CancelRegistration(ctx context.Context, p auth.Principal, orderID string) (*Order, error) {
	o, err := s.LoadOrder(ctx, s.DB, orderID, p)
	if err != nil {
		return nil, err
	}
	if !o.CanCancel {
		return nil, platform.Conflict("NOT_CANCELLABLE", "Only free registrations whose tickets were not used can be cancelled")
	}
	byOrganizer := p.ID == o.Event.OrganizerID
	err = platform.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		var used bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tickets WHERE order_id=$1 AND status='checked_in')`, orderID).Scan(&used); err != nil {
			return err
		}
		if used {
			return platform.Conflict("NOT_CANCELLABLE", "Only free registrations whose tickets were not used can be cancelled")
		}
		return cancelRegistration(ctx, tx, orderID, p.ID, byOrganizer)
	})
	if err != nil {
		return nil, err
	}
	return s.LoadOrder(ctx, s.DB, orderID, p)
}

// CancelEvent cancels an event: every paid order is refunded, every free registration is
// cancelled, open holds are released and each attendee is notified (decision D-9).
func (s *Service) CancelEvent(ctx context.Context, p auth.Principal, eventID, reason string) error {
	e, err := s.Events.Managed(ctx, p, eventID)
	if err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return platform.BadRequest("REASON_REQUIRED", "Tell attendees why the event is cancelled")
	}
	if e.Status == "cancelled" {
		return platform.Conflict("EVENT_CANCELLED", "This event is already cancelled")
	}
	return s.cancelEvent(ctx, eventID, p.ID, e.Title, reason)
}

func (s *Service) cancelEvent(ctx context.Context, eventID, actorID, title, reason string) error {
	return platform.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `SELECT 1 FROM events WHERE id=$1 FOR UPDATE`, eventID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE events SET status='cancelled', cancelled_at=now(), updated_at=now() WHERE id=$1`, eventID); err != nil {
			return err
		}
		pending, err := tx.QueryContext(ctx, `UPDATE orders SET status='cancelled', cancelled_at=now(), expires_at=NULL WHERE event_id=$1 AND status='pending' RETURNING id`, eventID)
		if err != nil {
			return err
		}
		var pendingIDs []string
		for pending.Next() {
			var id string
			_ = pending.Scan(&id)
			pendingIDs = append(pendingIDs, id)
		}
		pending.Close()
		if err := promo.ReleaseForOrders(ctx, tx, pendingIDs); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT id, user_id, total_kzt FROM orders WHERE event_id=$1 AND status='completed'`, eventID)
		if err != nil {
			return err
		}
		type ord struct {
			id, user string
			total    int
		}
		var orders []ord
		for rows.Next() {
			var o ord
			if err := rows.Scan(&o.id, &o.user, &o.total); err != nil {
				rows.Close()
				return err
			}
			orders = append(orders, o)
		}
		rows.Close()
		notified := map[string]bool{}
		for _, o := range orders {
			if o.total > 0 {
				err = s.refundOrder(ctx, tx, o.id, actorID, "Event cancelled: "+reason)
			} else {
				err = cancelRegistration(ctx, tx, o.id, actorID, false)
			}
			if err != nil {
				return err
			}
			if !notified[o.user] {
				notified[o.user] = true
				if err := notify.Queue(ctx, tx, notify.Message{UserID: o.user, Type: "event_cancelled", Params: map[string]any{"event": title, "reason": reason},
					Link: "/orders/" + o.id, Email: true}); err != nil {
					return err
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE promo_campaigns SET status='disabled' WHERE event_id=$1`, eventID); err != nil {
			return err
		}
		return platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: actorID, Action: "event.cancelled", EntityType: "event", EntityID: eventID,
			Description: "Event cancelled: " + reason, Metadata: map[string]any{"orders": len(orders)}})
	})
}
