// Package analytics computes organizer dashboards and the event history timeline from
// BiletFlow's own records (SRS §4.15, §4.16). Reads never lock rows, so they never block checkout.
package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/event"
	"github.com/biletflow/backend-go/internal/platform"
)

type Handler struct {
	DB     *sql.DB
	Events *event.Service
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/events/{eventId}/analytics", platform.Handler(h.analytics))
	mux.Handle("GET /api/events/{eventId}/timeline", platform.Handler(h.timeline))
	mux.Handle("GET /api/organizer/overview", platform.Handler(h.overview))
}

type Summary struct {
	Capacity       *int    `json:"capacity"`
	TicketsTotal   int     `json:"ticketsTotal"`
	Sold           int     `json:"sold"`
	Remaining      int     `json:"remaining"`
	PercentSold    float64 `json:"percentSold"`
	CheckedIn      int     `json:"checkedIn"`
	AttendanceRate float64 `json:"attendanceRate"`
	Orders         int     `json:"orders"`
	GrossKzt       int     `json:"grossKzt"`
	DiscountsKzt   int     `json:"discountsKzt"`
	RefundsKzt     int     `json:"refundsKzt"`
	NetKzt         int     `json:"netKzt"`
	FeesKzt        int     `json:"feesKzt"`
	EstPayoutKzt   int     `json:"estimatedPayoutKzt"`
}

type DayPoint struct {
	Day        string `json:"day"`
	Tickets    int    `json:"tickets"`
	RevenueKzt int    `json:"revenueKzt"`
}

type TypeRow struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceKzt   int    `json:"priceKzt"`
	Quantity   int    `json:"quantity"`
	Sold       int    `json:"sold"`
	Reserved   int    `json:"reserved"`
	Refunded   int    `json:"refunded"`
	CheckedIn  int    `json:"checkedIn"`
	RevenueKzt int    `json:"revenueKzt"`
}

type CampaignRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Visits      int    `json:"visits"`
	Orders      int    `json:"orders"`
	Tickets     int    `json:"tickets"`
	RevenueKzt  int    `json:"revenueKzt"`
	DiscountKzt int    `json:"discountKzt"`
}

type Report struct {
	EventID   string        `json:"eventId"`
	Timezone  string        `json:"timezone"`
	Summary   Summary       `json:"summary"`
	Daily     []DayPoint    `json:"daily"`
	ByType    []TypeRow     `json:"byTicketType"`
	Campaigns []CampaignRow `json:"campaigns"`
	Filters   any           `json:"filters"`
	Simulated bool          `json:"isSimulated"`
}

// analytics answers GET /api/events/{id}/analytics?from=YYYY-MM-DD&to=YYYY-MM-DD&ticketTypeId=...
// Date filters use the payment day in the event's time zone (decision D-13).
func (h *Handler) analytics(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	e, err := h.Events.Managed(r.Context(), p, r.PathValue("eventId"))
	if err != nil {
		return err
	}
	q := r.URL.Query()
	from, to, typeID := q.Get("from"), q.Get("to"), q.Get("ticketTypeId")
	for _, d := range []string{from, to} {
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return platform.BadRequest("INVALID_DATE", "Dates must be YYYY-MM-DD")
			}
		}
	}
	if typeID != "" && !platform.IsUUID(typeID) {
		return platform.BadRequest("BAD_TICKET_TYPE", "Ticket type is not available")
	}
	rep, err := Build(r.Context(), h.DB, e, from, to, typeID)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, rep)
	return nil
}

// Build computes the report; it is exported so the admin export can reuse it.
func Build(ctx context.Context, db *sql.DB, e *event.Event, from, to, typeID string) (*Report, error) {
	rep := &Report{EventID: e.ID, Timezone: e.Timezone, Simulated: true,
		Filters: map[string]string{"from": from, "to": to, "ticketTypeId": typeID}}
	// Item-level filter: payment day (in the event time zone) and ticket type.
	args := []any{e.ID, e.Timezone}
	// $2 (the time zone) is always referenced so the parameter count matches with or without date filters.
	cond := `o.event_id=$1 AND $2::text <> ''`
	if from != "" {
		args = append(args, from)
		cond += fmt.Sprintf(` AND (o.paid_at AT TIME ZONE $2)::date >= $%d::date`, len(args))
	}
	if to != "" {
		args = append(args, to)
		cond += fmt.Sprintf(` AND (o.paid_at AT TIME ZONE $2)::date <= $%d::date`, len(args))
	}
	typeCond := ""
	if typeID != "" {
		args = append(args, typeID)
		typeCond = fmt.Sprintf(` AND oi.ticket_type_id=$%d`, len(args))
	}

	var s Summary
	s.Capacity = e.Capacity
	err := db.QueryRowContext(ctx, `SELECT
		count(DISTINCT o.id) FILTER (WHERE o.status IN ('completed','refunded')),
		coalesce(sum(oi.quantity * oi.unit_price_kzt) FILTER (WHERE o.status IN ('completed','refunded')),0),
		coalesce(sum(oi.discount_kzt) FILTER (WHERE o.status IN ('completed','refunded')),0),
		coalesce(sum(oi.quantity * oi.unit_price_kzt - oi.discount_kzt) FILTER (WHERE o.status='refunded'),0)
		FROM orders o JOIN order_items oi ON oi.order_id=o.id WHERE `+cond+typeCond+` AND o.paid_at IS NOT NULL`, args...).
		Scan(&s.Orders, &s.GrossKzt, &s.DiscountsKzt, &s.RefundsKzt)
	if err != nil {
		return nil, err
	}
	s.NetKzt = s.GrossKzt - s.DiscountsKzt - s.RefundsKzt
	feePct := platform.SettingFloat(ctx, db, "processing_fee_percent", 2.5)
	s.FeesKzt = int(float64(s.NetKzt)*feePct/100 + 0.5)
	s.EstPayoutKzt = s.NetKzt - s.FeesKzt

	// Inventory (not date-filtered: it describes the event now).
	invArgs := []any{e.ID}
	invCond := ``
	if typeID != "" {
		invArgs = append(invArgs, typeID)
		invCond = ` AND s.ticket_type_id=$2`
	}
	if err := db.QueryRowContext(ctx, `SELECT coalesce(sum(s.quantity),0), coalesce(sum(s.sold),0), coalesce(sum(s.checked_in),0), coalesce(sum(s.reserved),0)
		FROM ticket_type_stock s WHERE s.event_id=$1`+invCond, invArgs...).Scan(&s.TicketsTotal, &s.Sold, &s.CheckedIn, new(int)); err != nil {
		return nil, err
	}
	total := s.TicketsTotal
	if e.Capacity != nil && typeID == "" && *e.Capacity < total {
		total = *e.Capacity
	}
	s.Remaining = max(total-s.Sold, 0)
	if total > 0 {
		s.PercentSold = round1(float64(s.Sold) * 100 / float64(total))
	}
	if s.Sold > 0 {
		s.AttendanceRate = round1(float64(s.CheckedIn) * 100 / float64(s.Sold))
	}
	rep.Summary = s

	rows, err := db.QueryContext(ctx, `SELECT to_char((o.paid_at AT TIME ZONE $2)::date, 'YYYY-MM-DD'), sum(oi.quantity), sum(oi.quantity*oi.unit_price_kzt - oi.discount_kzt)
		FROM orders o JOIN order_items oi ON oi.order_id=o.id WHERE `+cond+typeCond+` AND o.status='completed' AND o.paid_at IS NOT NULL
		GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		return nil, err
	}
	rep.Daily = []DayPoint{}
	for rows.Next() {
		var d DayPoint
		if err := rows.Scan(&d.Day, &d.Tickets, &d.RevenueKzt); err != nil {
			rows.Close()
			return nil, err
		}
		rep.Daily = append(rep.Daily, d)
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `SELECT tt.id, tt.name, tt.price_kzt, tt.quantity, s.sold, s.reserved, s.refunded, s.checked_in,
		(SELECT coalesce(sum(oi.quantity*oi.unit_price_kzt - oi.discount_kzt),0) FROM order_items oi JOIN orders o ON o.id=oi.order_id
			WHERE oi.ticket_type_id=tt.id AND o.status='completed')
		FROM ticket_types tt JOIN ticket_type_stock s ON s.ticket_type_id=tt.id WHERE tt.event_id=$1 ORDER BY tt.sort_order, tt.price_kzt`, e.ID)
	if err != nil {
		return nil, err
	}
	rep.ByType = []TypeRow{}
	for rows.Next() {
		var t TypeRow
		if err := rows.Scan(&t.ID, &t.Name, &t.PriceKzt, &t.Quantity, &t.Sold, &t.Reserved, &t.Refunded, &t.CheckedIn, &t.RevenueKzt); err != nil {
			rows.Close()
			return nil, err
		}
		rep.ByType = append(rep.ByType, t)
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `SELECT c.id, c.name, c.code, c.link_visits,
		(SELECT count(*) FROM orders o WHERE o.promo_campaign_id=c.id AND o.status='completed'),
		(SELECT count(*) FROM tickets t JOIN orders o ON o.id=t.order_id WHERE o.promo_campaign_id=c.id AND t.status IN ('valid','checked_in')),
		(SELECT coalesce(sum(o.total_kzt),0) FROM orders o WHERE o.promo_campaign_id=c.id AND o.status='completed'),
		(SELECT coalesce(sum(o.discount_kzt),0) FROM orders o WHERE o.promo_campaign_id=c.id AND o.status='completed')
		FROM promo_campaigns c WHERE c.event_id=$1 ORDER BY c.created_at`, e.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rep.Campaigns = []CampaignRow{}
	for rows.Next() {
		var c CampaignRow
		if err := rows.Scan(&c.ID, &c.Name, &c.Code, &c.Visits, &c.Orders, &c.Tickets, &c.RevenueKzt, &c.DiscountKzt); err != nil {
			return nil, err
		}
		rep.Campaigns = append(rep.Campaigns, c)
	}
	return rep, rows.Err()
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }

type TimelineEntry struct {
	ID          int64          `json:"id"`
	Action      string         `json:"action"`
	EntityType  string         `json:"entityType"`
	EntityID    *string        `json:"entityId"`
	Description string         `json:"description"`
	Actor       *string        `json:"actor"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"createdAt"`
}

// timeline returns the append-only activity history of one event (SRS §4.16), filterable by
// action prefix (event, order, ticket_type, campaign, staff, activation, checkin) and date.
func (h *Handler) timeline(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	e, err := h.Events.Managed(r.Context(), p, r.PathValue("eventId"))
	if err != nil {
		return err
	}
	q := r.URL.Query()
	args := []any{e.ID}
	where := `a.event_id=$1`
	if kind := q.Get("type"); kind != "" {
		if strings.ContainsAny(kind, "%_\\") || len(kind) > 30 {
			return platform.BadRequest("INVALID_FILTER", "Unknown filter")
		}
		args = append(args, kind+".%")
		where += fmt.Sprintf(` AND a.action LIKE $%d`, len(args))
	}
	for key, op := range map[string]string{"from": ">=", "to": "<="} {
		if v := q.Get(key); v != "" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				return platform.BadRequest("INVALID_DATE", "Dates must be YYYY-MM-DD")
			}
			args = append(args, v, e.Timezone)
			where += fmt.Sprintf(` AND (a.created_at AT TIME ZONE $%d)::date %s $%d::date`, len(args), op, len(args)-1)
		}
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT a.id, a.action, a.entity_type, a.entity_id, a.description, coalesce(d.display_name, d.email), a.metadata, a.created_at
		FROM audit_logs a LEFT JOIN user_directory d ON d.user_id=a.actor_id WHERE `+where+` ORDER BY a.created_at DESC, a.id DESC LIMIT 500`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []TimelineEntry{}
	for rows.Next() {
		var t TimelineEntry
		var entity, actor sql.NullString
		var meta []byte
		if err := rows.Scan(&t.ID, &t.Action, &t.EntityType, &entity, &t.Description, &actor, &meta, &t.CreatedAt); err != nil {
			return err
		}
		t.EntityID, t.Actor = platform.NullString(entity), platform.NullString(actor)
		t.Metadata = map[string]any{}
		_ = json.Unmarshal(meta, &t.Metadata)
		out = append(out, t)
	}
	platform.WriteJSON(w, http.StatusOK, out)
	return rows.Err()
}

type OverviewRow struct {
	*event.Event
	Sold       int `json:"sold"`
	CheckedIn  int `json:"checkedIn"`
	RevenueKzt int `json:"revenueKzt"`
	OpenCases  int `json:"openCases"`
}

// overview lists the organizer's events grouped by lifecycle with headline numbers (SRS §4.16).
func (h *Handler) overview(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	if p.Role != "organizer" {
		return platform.Forbidden("ORGANIZER_REQUIRED", "Organizer role required")
	}
	events, err := h.Events.List(r.Context(), p, event.ListFilter{Mine: true})
	if err != nil {
		return err
	}
	out := []OverviewRow{}
	for _, e := range events {
		row := OverviewRow{Event: e}
		if err := h.DB.QueryRowContext(r.Context(), `SELECT
			(SELECT count(*) FROM tickets WHERE event_id=$1 AND status IN ('valid','checked_in')),
			(SELECT count(*) FROM tickets WHERE event_id=$1 AND status='checked_in'),
			(SELECT coalesce(sum(total_kzt),0) FROM orders WHERE event_id=$1 AND status='completed'),
			(SELECT count(*) FROM support_cases WHERE event_id=$1 AND kind='attendee' AND status <> 'resolved')`, e.ID).
			Scan(&row.Sold, &row.CheckedIn, &row.RevenueKzt, &row.OpenCases); err != nil {
			return err
		}
		out = append(out, row)
	}
	platform.WriteJSON(w, http.StatusOK, out)
	return nil
}
