package commerce

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/event"
	"github.com/biletflow/backend-go/internal/notify"
	"github.com/biletflow/backend-go/internal/platform"
	"github.com/biletflow/backend-go/internal/promo"
)

// Service implements checkout, orders, refunds and activation.
type Service struct {
	DB       *sql.DB
	Events   *event.Service
	Provider PaymentProvider
	Signer   QRSigner
	Limits   struct{ Checkout, Pay *platform.RateLimiter }
}

type Attendee struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ReserveInput struct {
	EventID string `json:"eventId"`
	Items   []struct {
		TicketTypeID string `json:"ticketTypeId"`
		Quantity     int    `json:"quantity"`
	} `json:"items"`
	PromoCode *string `json:"promoCode,omitempty"`
	Buyer     struct {
		Name  string  `json:"name"`
		Email string  `json:"email"`
		Phone *string `json:"phone,omitempty"`
	} `json:"buyer"`
	Attendees []Attendee `json:"attendees,omitempty"`
	AccessKey string     `json:"accessKey,omitempty"`
}

func validEmail(s string) bool {
	at := strings.Index(s, "@")
	return at > 0 && at < len(s)-3 && strings.Contains(s[at:], ".") && len(s) <= 254 && !strings.ContainsAny(s, " \t\r\n")
}

func validName(s string) bool { n := len([]rune(strings.TrimSpace(s))); return n >= 2 && n <= 120 }

// ExpireStale marks lapsed holds as expired and gives their promo redemptions back.
// It runs lazily inside every reservation and on a timer.
func ExpireStale(ctx context.Context, q platform.Querier, eventID string) error {
	query := `UPDATE orders SET status='expired' WHERE status='pending' AND expires_at < now()`
	args := []any{}
	if eventID != "" {
		query += ` AND event_id=$1`
		args = append(args, eventID)
	}
	rows, err := q.QueryContext(ctx, query+` RETURNING id`, args...)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	return promo.ReleaseForOrders(ctx, q, ids)
}

// RunSweeper releases expired holds every interval.
func (s *Service) RunSweeper(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := platform.InTx(ctx, s.DB, func(tx *sql.Tx) error { return ExpireStale(ctx, tx, "") }); err != nil && ctx.Err() == nil {
			log.Printf("hold sweeper: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

type line struct {
	typeID, name string
	price, qty   int
}

// Reserve holds inventory for the hold period (SRS §4.6).
//
// Concurrency: the event row is locked FOR UPDATE, so every reservation for one event is
// serialised; availability is computed inside the lock from issued tickets plus live holds,
// so two buyers can never both get the last ticket and capacity is never exceeded.
func (s *Service) Reserve(ctx context.Context, p auth.Principal, in ReserveInput) (string, error) {
	if !platform.IsUUID(in.EventID) {
		return "", platform.NotFound("Event not found")
	}
	in.Buyer.Name = strings.TrimSpace(in.Buyer.Name)
	in.Buyer.Email = strings.ToLower(strings.TrimSpace(in.Buyer.Email))
	if !validName(in.Buyer.Name) || !validEmail(in.Buyer.Email) {
		return "", platform.BadRequest("BUYER_DETAILS", "Enter the buyer's name and a valid e-mail address")
	}
	if len(in.Items) == 0 || len(in.Items) > 20 {
		return "", platform.BadRequest("NO_TICKETS", "Choose at least one ticket")
	}
	for i := range in.Attendees {
		in.Attendees[i].Name = strings.TrimSpace(in.Attendees[i].Name)
		in.Attendees[i].Email = strings.ToLower(strings.TrimSpace(in.Attendees[i].Email))
		if !validName(in.Attendees[i].Name) || !validEmail(in.Attendees[i].Email) {
			return "", platform.BadRequest("ATTENDEE_DETAILS", "Enter a name and e-mail for every attendee")
		}
	}
	merged := map[string]int{}
	var order []string
	for _, it := range in.Items {
		if !platform.IsUUID(it.TicketTypeID) || it.Quantity < 1 || it.Quantity > 50 {
			return "", platform.BadRequest("BAD_TICKET_TYPE", "Ticket type is not available")
		}
		if _, seen := merged[it.TicketTypeID]; !seen {
			order = append(order, it.TicketTypeID)
		}
		merged[it.TicketTypeID] += it.Quantity
	}
	holdMinutes := platform.SettingInt(ctx, s.DB, "hold_minutes", 10)
	feePct := platform.SettingFloat(ctx, s.DB, "processing_fee_percent", 2.5)

	var orderID string
	err := platform.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		if err := ExpireStale(ctx, tx, in.EventID); err != nil {
			return err
		}
		var organizerID, status, visibility, accessToken, paidStatus string
		var suspended bool
		var capacity sql.NullInt64
		var opens, closes, ends sql.NullTime
		var starts time.Time
		err := tx.QueryRowContext(ctx, `SELECT organizer_id, status, visibility, access_token, paid_sales_status, suspended_at IS NOT NULL,
			capacity, registration_opens_at, registration_closes_at, starts_at, ends_at FROM events WHERE id=$1 FOR UPDATE`, in.EventID).
			Scan(&organizerID, &status, &visibility, &accessToken, &paidStatus, &suspended, &capacity, &opens, &closes, &starts, &ends)
		if errors.Is(err, sql.ErrNoRows) {
			return platform.NotFound("Event not found")
		}
		if err != nil {
			return err
		}
		now := time.Now()
		switch {
		case status != "published":
			return platform.Conflict("NOT_ON_SALE", "This event is not open for registration")
		case suspended:
			return platform.Conflict("EVENT_SUSPENDED", "Sales for this event are suspended")
		case visibility == "private" && organizerID != p.ID && in.AccessKey != accessToken:
			return platform.NotFound("Event not found")
		case opens.Valid && now.Before(opens.Time):
			return platform.Conflict("REGISTRATION_NOT_OPEN", "Registration has not opened yet")
		}
		closeAt := starts
		if ends.Valid {
			closeAt = ends.Time
		}
		if closes.Valid {
			closeAt = closes.Time
		}
		if now.After(closeAt) {
			return platform.Conflict("REGISTRATION_CLOSED", "Registration is closed")
		}
		eventUsed := 0
		if err := tx.QueryRowContext(ctx, `SELECT coalesce(sum(reserved + sold),0) FROM ticket_type_stock WHERE event_id=$1`, in.EventID).Scan(&eventUsed); err != nil {
			return err
		}
		var lines []line
		var promoLines []promo.Line
		totalQty, subtotal := 0, 0
		for _, typeID := range order {
			qty := merged[typeID]
			var l line
			var hidden bool
			var start, end sql.NullTime
			var maxPer, quantity, used int
			err := tx.QueryRowContext(ctx, `SELECT tt.name, tt.price_kzt, tt.hidden, tt.sales_start_at, tt.sales_end_at, tt.max_per_order, tt.quantity,
				s.reserved + s.sold FROM ticket_types tt JOIN ticket_type_stock s ON s.ticket_type_id=tt.id WHERE tt.id=$1 AND tt.event_id=$2`, typeID, in.EventID).
				Scan(&l.name, &l.price, &hidden, &start, &end, &maxPer, &quantity, &used)
			if errors.Is(err, sql.ErrNoRows) || (err == nil && hidden) {
				return platform.BadRequest("BAD_TICKET_TYPE", "Ticket type is not available")
			}
			if err != nil {
				return err
			}
			switch {
			case start.Valid && now.Before(start.Time):
				return platform.Conflict("SALES_NOT_STARTED", "Sales for \"{name}\" have not started", "name", l.name)
			case end.Valid && now.After(end.Time):
				return platform.Conflict("SALES_ENDED", "Sales for \"{name}\" have ended", "name", l.name)
			case qty > maxPer:
				return platform.BadRequest("MAX_PER_ORDER", "You can buy at most {n} \"{name}\" tickets per order", "n", maxPer, "name", l.name)
			case l.price > 0 && paidStatus != "active":
				return platform.Conflict("PAID_SALES_INACTIVE", "Paid tickets for this event are not on sale yet")
			case used+qty > quantity:
				return platform.Conflict("SOLD_OUT", "Only {n} \"{name}\" tickets left", "n", max(quantity-used, 0), "name", l.name)
			}
			l.typeID, l.qty = typeID, qty
			lines = append(lines, l)
			promoLines = append(promoLines, promo.Line{TicketTypeID: typeID, Quantity: qty, UnitPriceKzt: l.price})
			eventUsed += qty
			totalQty += qty
			subtotal += l.price * qty
		}
		if capacity.Valid && eventUsed > int(capacity.Int64) {
			return platform.Conflict("CAPACITY_REACHED", "The event has reached its capacity")
		}
		if len(in.Attendees) > 0 && len(in.Attendees) != totalQty {
			return platform.BadRequest("ATTENDEE_COUNT", "Provide details for all {n} attendees", "n", totalQty)
		}
		discount, perItem := 0, map[string]int{}
		var campaignID any
		if in.PromoCode != nil && strings.TrimSpace(*in.PromoCode) != "" {
			c, err := promo.FindByCode(ctx, tx, in.EventID, *in.PromoCode)
			if err != nil {
				return err
			}
			if problem := c.Problem(now); problem != nil {
				return problem
			}
			discount, perItem = c.Discount(promoLines)
			if discount == 0 {
				return platform.BadRequest("PROMO_NOT_APPLICABLE", "This promo code does not apply to the selected tickets")
			}
			if err := promo.Reserve(ctx, tx, c.ID); err != nil {
				return err
			}
			campaignID = c.ID
		}
		total := subtotal - discount
		attendees, _ := json.Marshal(in.Attendees)
		if in.Attendees == nil {
			attendees = []byte("[]")
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO orders(event_id, user_id, status, buyer_name, buyer_email, buyer_phone, subtotal_kzt, discount_kzt,
			total_kzt, fee_kzt, promo_campaign_id, attendee_details, expires_at)
			VALUES ($1,$2,'pending',$3,$4,$5,$6,$7,$8,$9,$10,$11, now() + make_interval(mins => $12)) RETURNING id`,
			in.EventID, p.ID, in.Buyer.Name, in.Buyer.Email, in.Buyer.Phone, subtotal, discount, total, ProcessingFee(total, feePct),
			campaignID, attendees, holdMinutes).Scan(&orderID)
		if err != nil {
			return err
		}
		for _, l := range lines {
			if _, err := tx.ExecContext(ctx, `INSERT INTO order_items(order_id, ticket_type_id, quantity, unit_price_kzt, discount_kzt) VALUES ($1,$2,$3,$4,$5)`,
				orderID, l.typeID, l.qty, l.price, perItem[l.typeID]); err != nil {
				return err
			}
		}
		if campaignID != nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO promo_redemptions(campaign_id, order_id, discount_kzt) VALUES ($1,$2,$3)`, campaignID, orderID, discount); err != nil {
				return err
			}
		}
		// Zero-total orders (free tickets or a 100% code) complete without payment (SRS §4.4).
		if total == 0 {
			return s.complete(ctx, tx, orderID, p.ID, nil, false)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return orderID, nil
}

const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func ticketCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 0, 9)
	for i, c := range b {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, codeAlphabet[int(c)%len(codeAlphabet)])
	}
	return string(out)
}

// complete marks an order paid/completed, confirms its promo redemption, issues tickets,
// records the audit entry and queues the confirmation (with PDF tickets attached by the outbox).
func (s *Service) complete(ctx context.Context, tx *sql.Tx, orderID, actorID string, attendees []Attendee, paid bool) error {
	var eventID, userID, number, buyerName, buyerEmail, title string
	var total int
	var saved []byte
	err := tx.QueryRowContext(ctx, `UPDATE orders o SET status='completed', paid_at=now(), expires_at=NULL FROM events e
		WHERE o.id=$1 AND e.id=o.event_id RETURNING o.event_id, o.user_id, o.number, o.buyer_name, o.buyer_email, o.total_kzt, o.attendee_details, e.title`, orderID).
		Scan(&eventID, &userID, &number, &buyerName, &buyerEmail, &total, &saved, &title)
	if err != nil {
		return err
	}
	if len(attendees) == 0 {
		_ = json.Unmarshal(saved, &attendees)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE promo_redemptions SET status='confirmed' WHERE order_id=$1 AND status='reserved'`, orderID); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, ticket_type_id, quantity FROM order_items WHERE order_id=$1 ORDER BY id`, orderID)
	if err != nil {
		return err
	}
	type item struct {
		id, typeID string
		qty        int
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.typeID, &it.qty); err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	rows.Close()
	n := 0
	for _, it := range items {
		for range it.qty {
			name, email := buyerName, buyerEmail
			if n < len(attendees) {
				name, email = attendees[n].Name, attendees[n].Email
			}
			n++
			if _, err := tx.ExecContext(ctx, `INSERT INTO tickets(code, order_id, order_item_id, event_id, ticket_type_id, user_id, attendee_name, attendee_email)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, ticketCode(), orderID, it.id, eventID, it.typeID, userID, name, email); err != nil {
				return err
			}
		}
	}
	desc := "Free registration " + number + " confirmed"
	action := "order.completed"
	kind := "registration_confirmed"
	if paid {
		desc = "Order " + number + " paid (simulated payment)"
		action = "order.paid"
		kind = "order_confirmed"
	}
	if err := platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: actorID, Action: action, EntityType: "order", EntityID: orderID,
		Description: desc, Metadata: map[string]any{"totalKzt": total, "tickets": n}}); err != nil {
		return err
	}
	return notify.Queue(ctx, tx, notify.Message{UserID: userID, Type: kind, Params: map[string]any{"event": title, "number": number, "n": n, "total": total, "orderId": orderID},
		Link: "/orders/" + orderID, Email: true})
}

// Pay charges a pending order through the payment provider. A failed payment never creates
// tickets and releases inventory and the promo redemption (SRS §4.6).
func (s *Service) Pay(ctx context.Context, p auth.Principal, orderID, card string, attendees []Attendee) error {
	var failure *platform.APIError
	err := platform.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		var eventID, userID, status, number, title, eventStatus, paidStatus string
		var total, fee int
		var expires sql.NullTime
		var suspended bool
		err := tx.QueryRowContext(ctx, `SELECT o.event_id, o.user_id, o.status, o.number, o.total_kzt, o.fee_kzt, o.expires_at, e.title, e.status,
			e.paid_sales_status, e.suspended_at IS NOT NULL FROM orders o JOIN events e ON e.id=o.event_id WHERE o.id=$1 FOR UPDATE OF o`, orderID).
			Scan(&eventID, &userID, &status, &number, &total, &fee, &expires, &title, &eventStatus, &paidStatus, &suspended)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && userID != p.ID) {
			return platform.NotFound("Order not found")
		}
		if err != nil {
			return err
		}
		if status != "pending" {
			return platform.Conflict("ORDER_NOT_PENDING", "This order is already {status}", "status", status)
		}
		if expires.Valid && expires.Time.Before(time.Now()) {
			if err := ExpireStale(ctx, tx, eventID); err != nil {
				return err
			}
			failure = platform.Conflict("HOLD_EXPIRED", "Your reservation expired. Please select tickets again.")
			return nil
		}
		if eventStatus != "published" || suspended || paidStatus != "active" {
			return platform.Conflict("NOT_ON_SALE", "Sales for this event are currently unavailable")
		}
		for i := range attendees {
			if !validName(attendees[i].Name) || !validEmail(strings.TrimSpace(attendees[i].Email)) {
				return platform.BadRequest("ATTENDEE_DETAILS", "Enter a name and e-mail for every attendee")
			}
		}
		charge := s.Provider.Charge(total, card)
		paymentStatus, chargedFee := "failed", 0
		var reason any
		if charge.OK {
			paymentStatus, chargedFee = "succeeded", fee
		} else {
			reason = charge.FailureReason
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO payments(kind, order_id, event_id, payer_id, amount_kzt, fee_kzt, status, failure_reason, provider, provider_ref, card_last4)
			VALUES ('ticket',$1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, orderID, eventID, p.ID, total, chargedFee, paymentStatus, reason, s.Provider.Name(), charge.ProviderRef, charge.CardLast4); err != nil {
			return err
		}
		if !charge.OK {
			if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='failed', expires_at=NULL WHERE id=$1`, orderID); err != nil {
				return err
			}
			if err := promo.ReleaseForOrders(ctx, tx, []string{orderID}); err != nil {
				return err
			}
			if err := platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "order.payment_failed", EntityType: "order", EntityID: orderID,
				Description: "Payment failed for order " + number + ": " + FailureMessage(charge.FailureReason)}); err != nil {
				return err
			}
			if err := notify.Queue(ctx, tx, notify.Message{UserID: p.ID, Type: "payment_failed", Params: map[string]any{"number": number, "event": title}, Link: "/events/" + eventID, Email: true}); err != nil {
				return err
			}
			failure = platform.Err(http.StatusPaymentRequired, "PAYMENT_FAILED", FailureMessage(charge.FailureReason), "reason", charge.FailureReason)
			return nil
		}
		return s.complete(ctx, tx, orderID, p.ID, attendees, true)
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	return nil
}

// Abandon releases a hold immediately instead of waiting for it to expire.
func (s *Service) Abandon(ctx context.Context, p auth.Principal, orderID string) error {
	return platform.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		var userID, status string
		err := tx.QueryRowContext(ctx, `SELECT user_id, status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&userID, &status)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && userID != p.ID) {
			return platform.NotFound("Order not found")
		}
		if err != nil {
			return err
		}
		if status != "pending" {
			return platform.Conflict("ORDER_NOT_PENDING", "This order is already {status}", "status", status)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='cancelled', cancelled_at=now(), expires_at=NULL WHERE id=$1`, orderID); err != nil {
			return err
		}
		return promo.ReleaseForOrders(ctx, tx, []string{orderID})
	})
}
