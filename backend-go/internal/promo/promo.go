// Package promo implements promotional campaigns, codes and Campaign QR links (SRS §4.14).
// All discounts are calculated on the server; the redemption limit is enforced atomically.
package promo

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/biletflow/backend-go/internal/platform"
)

type Campaign struct {
	ID               string     `json:"id"`
	EventID          string     `json:"eventId"`
	Name             string     `json:"name"`
	Code             string     `json:"code"`
	LinkToken        string     `json:"linkToken,omitempty"`
	DiscountType     string     `json:"discountType"`
	DiscountValue    int        `json:"discountValue"`
	ValidFrom        *time.Time `json:"validFrom,omitempty"`
	ValidUntil       *time.Time `json:"validUntil,omitempty"`
	MaxRedemptions   *int       `json:"maxRedemptions,omitempty"`
	RedemptionsCount int        `json:"redemptionsCount"`
	TicketTypeIDs    []string   `json:"ticketTypeIds"`
	Status           string     `json:"status"`
	LinkVisits       int        `json:"linkVisits"`
	CreatedAt        time.Time  `json:"createdAt"`
	// Results, filled for the organizer's campaign list.
	Results *Results `json:"results,omitempty"`
}

type Results struct {
	Orders      int `json:"orders"`
	Tickets     int `json:"tickets"`
	RevenueKzt  int `json:"revenueKzt"`
	DiscountKzt int `json:"discountKzt"`
}

const campaignColumns = `c.id, c.event_id, c.name, c.code, c.link_token, c.discount_type, c.discount_value, c.valid_from, c.valid_until,
	c.max_redemptions, c.redemptions_count, coalesce(c.ticket_type_ids, '{}'), c.status, c.link_visits, c.created_at`

func scan(row interface{ Scan(...any) error }) (*Campaign, error) {
	var c Campaign
	var from, until sql.NullTime
	var maxR sql.NullInt64
	var types pq.StringArray
	if err := row.Scan(&c.ID, &c.EventID, &c.Name, &c.Code, &c.LinkToken, &c.DiscountType, &c.DiscountValue, &from, &until,
		&maxR, &c.RedemptionsCount, &types, &c.Status, &c.LinkVisits, &c.CreatedAt); err != nil {
		return nil, err
	}
	if from.Valid {
		c.ValidFrom = &from.Time
	}
	if until.Valid {
		c.ValidUntil = &until.Time
	}
	if maxR.Valid {
		m := int(maxR.Int64)
		c.MaxRedemptions = &m
	}
	c.TicketTypeIDs = []string(types)
	return &c, nil
}

// FindByCode looks up an event's campaign by code, case-insensitively.
func FindByCode(ctx context.Context, q platform.Querier, eventID, code string) (*Campaign, error) {
	c, err := scan(q.QueryRowContext(ctx, `SELECT `+campaignColumns+` FROM promo_campaigns c WHERE c.event_id=$1 AND upper(c.code)=upper($2)`, eventID, strings.TrimSpace(code)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, platform.BadRequest("PROMO_INVALID", "This promo code does not exist for this event")
	}
	return c, err
}

// Problem explains why a campaign cannot be used right now, or returns nil.
func (c *Campaign) Problem(now time.Time) *platform.APIError {
	switch {
	case c.Status != "active":
		return platform.BadRequest("PROMO_DISABLED", "This promo code has been disabled by the organizer")
	case c.ValidFrom != nil && now.Before(*c.ValidFrom):
		return platform.BadRequest("PROMO_NOT_STARTED", "This promo code is not active yet")
	case c.ValidUntil != nil && now.After(*c.ValidUntil):
		return platform.BadRequest("PROMO_EXPIRED", "This promo code has expired")
	case c.MaxRedemptions != nil && c.RedemptionsCount >= *c.MaxRedemptions:
		return platform.BadRequest("PROMO_EXHAUSTED", "This promo code has reached its redemption limit")
	}
	return nil
}

// Line is one ticket type in a basket.
type Line struct {
	TicketTypeID string
	Quantity     int
	UnitPriceKzt int
}

// Discount computes the discount per ticket type and in total. Percent discounts round to
// whole tenge per ticket; fixed discounts are capped at the ticket price.
func (c *Campaign) Discount(lines []Line) (int, map[string]int) {
	per := map[string]int{}
	total := 0
	for _, l := range lines {
		applies := len(c.TicketTypeIDs) == 0 || slices.Contains(c.TicketTypeIDs, l.TicketTypeID)
		if !applies || l.UnitPriceKzt == 0 {
			continue
		}
		each := min(c.DiscountValue, l.UnitPriceKzt)
		if c.DiscountType == "percent" {
			each = (l.UnitPriceKzt*c.DiscountValue + 50) / 100
		}
		per[l.TicketTypeID] += each * l.Quantity
		total += each * l.Quantity
	}
	return total, per
}

// Reserve takes one redemption. The conditional UPDATE only succeeds while under the limit,
// so concurrent checkouts can never exceed max_redemptions.
func Reserve(ctx context.Context, q platform.Querier, campaignID string) error {
	res, err := q.ExecContext(ctx, `UPDATE promo_campaigns SET redemptions_count = redemptions_count + 1
		WHERE id=$1 AND status='active' AND (max_redemptions IS NULL OR redemptions_count < max_redemptions)`, campaignID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return platform.BadRequest("PROMO_EXHAUSTED", "This promo code has reached its redemption limit")
	}
	return nil
}

// ReleaseForOrders gives back reserved (not yet confirmed) redemptions of failed, expired or abandoned orders.
func ReleaseForOrders(ctx context.Context, q platform.Querier, orderIDs []string) error {
	if len(orderIDs) == 0 {
		return nil
	}
	_, err := q.ExecContext(ctx, `WITH released AS (
			UPDATE promo_redemptions SET status='released' WHERE order_id = ANY($1) AND status='reserved' RETURNING campaign_id)
		UPDATE promo_campaigns c SET redemptions_count = GREATEST(c.redemptions_count - r.n, 0)
		FROM (SELECT campaign_id, count(*) AS n FROM released GROUP BY campaign_id) r WHERE c.id = r.campaign_id`, pq.Array(orderIDs))
	return err
}

// ParseLines validates a basket used for a promo preview.
func ParseLines(ctx context.Context, q platform.Querier, eventID string, items []struct {
	TicketTypeID string `json:"ticketTypeId"`
	Quantity     int    `json:"quantity"`
}) ([]Line, error) {
	var lines []Line
	for _, it := range items {
		if !platform.IsUUID(it.TicketTypeID) || it.Quantity < 1 || it.Quantity > 50 {
			return nil, platform.BadRequest("BAD_TICKET_TYPE", "Ticket type is not available")
		}
		var price int
		err := q.QueryRowContext(ctx, `SELECT price_kzt FROM ticket_types WHERE id=$1 AND event_id=$2 AND NOT hidden`, it.TicketTypeID, eventID).Scan(&price)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, platform.BadRequest("BAD_TICKET_TYPE", "Ticket type is not available")
		}
		if err != nil {
			return nil, err
		}
		lines = append(lines, Line{TicketTypeID: it.TicketTypeID, Quantity: it.Quantity, UnitPriceKzt: price})
	}
	return lines, nil
}
