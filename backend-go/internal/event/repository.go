package event

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/platform"
)

var ErrNotFound = errors.New("event not found")

// Repository is the PostgreSQL persistence boundary for events, ticket types and staff.
type Repository struct{ DB *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{DB: db} }

const eventColumns = `e.id, e.organizer_id, e.title, e.description, e.category, e.image_url, e.starts_at, e.ends_at, e.timezone,
	e.venue_id, e.venue_name, e.venue_address, e.city, e.visibility, e.capacity, e.status, e.registration_opens_at,
	e.registration_closes_at, e.refund_policy, e.paid_sales_status, e.suspended_at IS NOT NULL, e.suspension_reason,
	e.duplicated_from, e.published_at, e.cancelled_at, e.created_at, e.updated_at, e.access_token,
	(SELECT min(tt.price_kzt) FROM ticket_types tt WHERE tt.event_id = e.id AND NOT tt.hidden),
	(SELECT sum(GREATEST(s.quantity - s.reserved - s.sold, 0)) FROM ticket_type_stock s JOIN ticket_types tt ON tt.id = s.ticket_type_id
		WHERE s.event_id = e.id AND NOT tt.hidden),
	(SELECT coalesce(sum(s.reserved + s.sold),0) FROM ticket_type_stock s WHERE s.event_id = e.id)`

type rowScanner interface{ Scan(dest ...any) error }

func scanEvent(row rowScanner) (*Event, error) {
	var e Event
	var desc, cat, img, venueID, vName, vAddr, city, reason, dup sql.NullString
	var ends, opens, closes, published, cancelled sql.NullTime
	var capacity, priceFrom, typeLeft sql.NullInt64
	var used int
	var visibility, status string
	err := row.Scan(&e.ID, &e.OrganizerID, &e.Title, &desc, &cat, &img, &e.StartsAt, &ends, &e.Timezone,
		&venueID, &vName, &vAddr, &city, &visibility, &capacity, &status, &opens, &closes, &e.RefundPolicy,
		&e.PaidSalesStatus, &e.Suspended, &reason, &dup, &published, &cancelled, &e.CreatedAt, &e.UpdatedAt, &e.AccessToken,
		&priceFrom, &typeLeft, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.Visibility, e.Status = Visibility(visibility), Status(status)
	e.Description, e.Category, e.ImageURL = platform.NullString(desc), platform.NullString(cat), platform.NullString(img)
	e.VenueID, e.VenueName, e.VenueAddress, e.City = platform.NullString(venueID), platform.NullString(vName), platform.NullString(vAddr), platform.NullString(city)
	e.SuspensionReason, e.DuplicatedFrom = platform.NullString(reason), platform.NullString(dup)
	e.EndsAt, e.RegistrationOpensAt, e.RegistrationClosesAt = nullTime(ends), nullTime(opens), nullTime(closes)
	e.PublishedAt, e.CancelledAt = nullTime(published), nullTime(cancelled)
	if capacity.Valid {
		c := int(capacity.Int64)
		e.Capacity = &c
	}
	if priceFrom.Valid {
		p := int(priceFrom.Int64)
		e.PriceFromKzt = &p
	}
	if typeLeft.Valid {
		left := int(typeLeft.Int64)
		if e.Capacity != nil && *e.Capacity-used < left {
			left = max(*e.Capacity-used, 0)
		}
		e.TicketsLeft = &left
	}
	e.Lifecycle = Lifecycle(&e, time.Now())
	return &e, nil
}

func nullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// Lifecycle classifies an event for dashboards (SRS §4.16).
func Lifecycle(e *Event, now time.Time) string {
	switch e.Status {
	case StatusDraft:
		return "draft"
	case StatusCancelled:
		return "cancelled"
	}
	end := e.StartsAt.Add(4 * time.Hour)
	if e.EndsAt != nil {
		end = *e.EndsAt
	}
	switch {
	case now.Before(e.StartsAt):
		return "upcoming"
	case now.Before(end):
		return "active"
	default:
		return "completed"
	}
}

func (r *Repository) Create(ctx context.Context, e *Event) error {
	row := r.DB.QueryRowContext(ctx, `INSERT INTO events(organizer_id, title, description, category, image_url, starts_at, ends_at,
		timezone, venue_id, venue_name, venue_address, city, visibility, capacity, registration_opens_at, registration_closes_at,
		refund_policy, duplicated_from)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING id`,
		e.OrganizerID, e.Title, e.Description, e.Category, e.ImageURL, e.StartsAt, e.EndsAt, e.Timezone, e.VenueID,
		e.VenueName, e.VenueAddress, e.City, string(e.Visibility), e.Capacity, e.RegistrationOpensAt, e.RegistrationClosesAt,
		e.RefundPolicy, e.DuplicatedFrom)
	return row.Scan(&e.ID)
}

func (r *Repository) Get(ctx context.Context, id string) (*Event, error) {
	if !platform.IsUUID(id) {
		return nil, ErrNotFound
	}
	return scanEvent(r.DB.QueryRowContext(ctx, `SELECT `+eventColumns+` FROM events e WHERE e.id=$1`, id))
}

// List returns events matching the filter. Visibility rules are applied by the service.
func (r *Repository) List(ctx context.Context, f ListFilter, organizerID string, publicOnly bool) ([]*Event, error) {
	var where []string
	var args []any
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if f.Status != nil {
		where = append(where, "e.status = "+arg(string(*f.Status)))
	}
	switch {
	case f.Mine:
		where = append(where, "e.organizer_id = "+arg(organizerID))
	case publicOnly && organizerID != "":
		where = append(where, "(e.organizer_id = "+arg(organizerID)+" OR (e.status='published' AND e.visibility='public' AND e.suspended_at IS NULL))")
	case publicOnly:
		where = append(where, "e.status='published' AND e.visibility='public' AND e.suspended_at IS NULL")
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		p := arg("%" + strings.ToLower(q) + "%")
		where = append(where, "(lower(e.title) LIKE "+p+" OR lower(coalesce(e.category,'')) LIKE "+p+" OR lower(coalesce(e.city,'')) LIKE "+p+" OR lower(coalesce(e.venue_name,'')) LIKE "+p+")")
	}
	if f.Category != "" && f.Category != "all" {
		where = append(where, "lower(coalesce(e.category,'')) = lower("+arg(f.Category)+")")
	}
	if f.City != "" {
		where = append(where, "lower(coalesce(e.city,'')) = lower("+arg(f.City)+")")
	}
	if f.From != nil {
		where = append(where, "coalesce(e.ends_at, e.starts_at) >= "+arg(*f.From))
	}
	if f.To != nil {
		where = append(where, "e.starts_at <= "+arg(*f.To))
	}
	query := `SELECT ` + eventColumns + ` FROM events e`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY e.starts_at ASC LIMIT 200"
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Update writes every editable column of e.
func (r *Repository) Update(ctx context.Context, q platform.Querier, e *Event) error {
	res, err := q.ExecContext(ctx, `UPDATE events SET title=$2, description=$3, category=$4, image_url=$5, starts_at=$6, ends_at=$7,
		timezone=$8, venue_name=$9, venue_address=$10, city=$11, visibility=$12, capacity=$13, registration_opens_at=$14,
		registration_closes_at=$15, refund_policy=$16, updated_at=now() WHERE id=$1`,
		e.ID, e.Title, e.Description, e.Category, e.ImageURL, e.StartsAt, e.EndsAt, e.Timezone, e.VenueName, e.VenueAddress,
		e.City, string(e.Visibility), e.Capacity, e.RegistrationOpensAt, e.RegistrationClosesAt, e.RefundPolicy)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) SetStatus(ctx context.Context, q platform.Querier, id string, status Status) error {
	_, err := q.ExecContext(ctx, `UPDATE events SET status=$2, updated_at=now(),
		published_at = CASE WHEN $2='published' THEN coalesce(published_at, now()) ELSE published_at END
		WHERE id=$1`, id, string(status))
	return err
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM events WHERE id=$1`, id)
	return err
}

func (r *Repository) CountOrders(ctx context.Context, eventID string, statuses ...string) (int, error) {
	var n int
	err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM orders WHERE event_id=$1 AND status = ANY(string_to_array($2, ','))`,
		eventID, strings.Join(statuses, ",")).Scan(&n)
	return n, err
}

// TicketHolders returns the users holding valid tickets for an event (for change notices).
func (r *Repository) TicketHolders(ctx context.Context, q platform.Querier, eventID string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT user_id FROM tickets WHERE event_id=$1 AND status IN ('valid','checked_in')`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ───────────── Ticket types ─────────────

const ticketTypeColumns = `tt.id, tt.event_id, tt.name, tt.description, tt.price_kzt, tt.quantity, tt.sales_start_at, tt.sales_end_at,
	tt.max_per_order, tt.hidden, tt.sort_order, s.reserved, s.sold, s.refunded, s.checked_in`

func (r *Repository) TicketTypes(ctx context.Context, eventID string) ([]*TicketType, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT `+ticketTypeColumns+` FROM ticket_types tt JOIN ticket_type_stock s ON s.ticket_type_id = tt.id
		WHERE tt.event_id=$1 ORDER BY tt.sort_order, tt.price_kzt, tt.created_at`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*TicketType{}
	for rows.Next() {
		var t TicketType
		var start, end sql.NullTime
		var c Counts
		if err := rows.Scan(&t.ID, &t.EventID, &t.Name, &t.Description, &t.PriceKzt, &t.Quantity, &start, &end, &t.MaxPerOrder,
			&t.Hidden, &t.SortOrder, &c.Reserved, &c.Sold, &c.Refunded, &c.CheckedIn); err != nil {
			return nil, err
		}
		t.SalesStartAt, t.SalesEndAt = nullTime(start), nullTime(end)
		t.Available = max(t.Quantity-c.Reserved-c.Sold, 0)
		t.Counts = &c
		out = append(out, &t)
	}
	return out, rows.Err()
}

func (r *Repository) CreateTicketType(ctx context.Context, q platform.Querier, t *TicketType) error {
	return q.QueryRowContext(ctx, `INSERT INTO ticket_types(event_id, name, description, price_kzt, quantity, sales_start_at, sales_end_at,
		max_per_order, hidden, sort_order) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		t.EventID, t.Name, t.Description, t.PriceKzt, t.Quantity, t.SalesStartAt, t.SalesEndAt, t.MaxPerOrder, t.Hidden, t.SortOrder).Scan(&t.ID)
}

func (r *Repository) UpdateTicketType(ctx context.Context, t *TicketType) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE ticket_types SET name=$2, description=$3, price_kzt=$4, quantity=$5, sales_start_at=$6,
		sales_end_at=$7, max_per_order=$8, hidden=$9, sort_order=$10 WHERE id=$1`,
		t.ID, t.Name, t.Description, t.PriceKzt, t.Quantity, t.SalesStartAt, t.SalesEndAt, t.MaxPerOrder, t.Hidden, t.SortOrder)
	return err
}

func (r *Repository) DeleteTicketType(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM ticket_types WHERE id=$1`, id)
	return err
}

func (r *Repository) TicketTypeOrdered(ctx context.Context, id string) (bool, error) {
	var used bool
	err := r.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM order_items WHERE ticket_type_id=$1)`, id).Scan(&used)
	return used, err
}

// ───────────── Staff ─────────────

func (r *Repository) Staff(ctx context.Context, eventID string) ([]StaffMember, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT s.id, s.user_id, coalesce(d.email,''), d.display_name, s.can_undo, s.created_at
		FROM staff_assignments s LEFT JOIN user_directory d ON d.user_id = s.user_id WHERE s.event_id=$1 ORDER BY s.created_at`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StaffMember{}
	for rows.Next() {
		var m StaffMember
		var display sql.NullString
		if err := rows.Scan(&m.ID, &m.UserID, &m.Email, &display, &m.CanUndo, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.DisplayName = platform.NullString(display)
		out = append(out, m)
	}
	return out, rows.Err()
}
