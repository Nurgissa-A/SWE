package promo

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/lib/pq"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/biletflow/backend-go/internal/event"
	"github.com/biletflow/backend-go/internal/platform"
)

type Handler struct {
	DB     *sql.DB
	Events *event.Service
	// AppURL is the public web address used in Campaign QR links, e.g. https://biletflow.kz.
	AppURL string
	// Preview guards the public promo-preview endpoint against code guessing.
	Preview *platform.RateLimiter
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/events/{eventId}/campaigns", platform.Handler(h.list))
	mux.Handle("POST /api/events/{eventId}/campaigns", platform.Handler(h.create))
	mux.Handle("PATCH /api/events/{eventId}/campaigns/{campaignId}", platform.Handler(h.update))
	mux.Handle("GET /api/events/{eventId}/campaigns/{campaignId}/qr.png", platform.Handler(h.qr))
	mux.Handle("POST /api/events/{eventId}/promo-preview", platform.Handler(h.preview))
	mux.Handle("GET /api/public/campaigns/{token}", platform.Handler(h.visit))
}

// Link is the opaque https link a Campaign QR encodes. It never encodes an admission ticket.
func (h *Handler) Link(token string) string {
	return strings.TrimRight(h.AppURL, "/") + "/c/" + token
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	e, err := h.Events.Managed(r.Context(), p, r.PathValue("eventId"))
	if err != nil {
		return err
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT `+campaignColumns+`,
		(SELECT count(*) FROM orders o WHERE o.promo_campaign_id=c.id AND o.status='completed'),
		(SELECT count(*) FROM tickets t JOIN orders o ON o.id=t.order_id WHERE o.promo_campaign_id=c.id AND t.status IN ('valid','checked_in')),
		(SELECT coalesce(sum(o.total_kzt),0) FROM orders o WHERE o.promo_campaign_id=c.id AND o.status='completed'),
		(SELECT coalesce(sum(o.discount_kzt),0) FROM orders o WHERE o.promo_campaign_id=c.id AND o.status='completed')
		FROM promo_campaigns c WHERE c.event_id=$1 ORDER BY c.created_at DESC`, e.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []*Campaign{}
	for rows.Next() {
		var c Campaign
		var from, until sql.NullTime
		var maxR sql.NullInt64
		var types pq.StringArray
		res := Results{}
		if err := rows.Scan(&c.ID, &c.EventID, &c.Name, &c.Code, &c.LinkToken, &c.DiscountType, &c.DiscountValue, &from, &until,
			&maxR, &c.RedemptionsCount, &types, &c.Status, &c.LinkVisits, &c.CreatedAt,
			&res.Orders, &res.Tickets, &res.RevenueKzt, &res.DiscountKzt); err != nil {
			return err
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
		c.Results = &res
		out = append(out, &c)
	}
	platform.WriteJSON(w, http.StatusOK, out)
	return rows.Err()
}

var codePattern = regexp.MustCompile(`^[A-Z0-9_-]{3,30}$`)

type campaignInput struct {
	Name           string     `json:"name"`
	Code           string     `json:"code"`
	DiscountType   string     `json:"discountType"`
	DiscountValue  int        `json:"discountValue"`
	ValidFrom      *time.Time `json:"validFrom,omitempty"`
	ValidUntil     *time.Time `json:"validUntil,omitempty"`
	MaxRedemptions *int       `json:"maxRedemptions,omitempty"`
	TicketTypeIDs  []string   `json:"ticketTypeIds,omitempty"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	var in campaignInput
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	e, err := h.Events.Managed(r.Context(), p, r.PathValue("eventId"))
	if err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	switch {
	case in.Name == "" || len(in.Name) > 100:
		return platform.BadRequest("CAMPAIGN_NAME_REQUIRED", "Campaign name is required")
	case !codePattern.MatchString(in.Code):
		return platform.BadRequest("INVALID_CODE", "Codes use 3–30 letters, digits, - or _")
	case in.DiscountType != "percent" && in.DiscountType != "fixed":
		return platform.BadRequest("INVALID_DISCOUNT", "Discount type must be percent or fixed")
	case in.DiscountValue < 1 || (in.DiscountType == "percent" && in.DiscountValue > 100):
		return platform.BadRequest("INVALID_DISCOUNT", "Percent discounts are 1–100; fixed discounts are at least 1 KZT")
	case in.MaxRedemptions != nil && *in.MaxRedemptions < 1:
		return platform.BadRequest("INVALID_LIMIT", "The redemption limit must be at least 1")
	case in.ValidFrom != nil && in.ValidUntil != nil && !in.ValidUntil.After(*in.ValidFrom):
		return platform.BadRequest("INVALID_WINDOW", "The code must expire after it starts")
	}
	for _, id := range in.TicketTypeIDs {
		var ok bool
		if !platform.IsUUID(id) {
			return platform.BadRequest("BAD_TICKET_TYPE", "Ticket type is not available")
		}
		if err := h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM ticket_types WHERE id=$1 AND event_id=$2)`, id, e.ID).Scan(&ok); err != nil || !ok {
			return platform.BadRequest("BAD_TICKET_TYPE", "Ticket type is not available")
		}
	}
	var types any
	if len(in.TicketTypeIDs) > 0 {
		types = pq.Array(in.TicketTypeIDs)
	}
	var id string
	err = platform.InTx(r.Context(), h.DB, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(r.Context(), `INSERT INTO promo_campaigns(event_id, name, code, discount_type, discount_value, valid_from, valid_until,
			max_redemptions, ticket_type_ids, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
			e.ID, in.Name, in.Code, in.DiscountType, in.DiscountValue, in.ValidFrom, in.ValidUntil, in.MaxRedemptions, types, p.ID).Scan(&id)
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return platform.Conflict("CODE_TAKEN", "This event already has a campaign with that code")
		}
		if err != nil {
			return err
		}
		return platform.Audit(r.Context(), tx, platform.AuditEntry{EventID: e.ID, ActorID: p.ID, Action: "campaign.created", EntityType: "campaign", EntityID: id,
			Description: "Promo campaign created: " + in.Name + " (" + in.Code + ")"})
	})
	if err != nil {
		return err
	}
	c, err := scan(h.DB.QueryRowContext(r.Context(), `SELECT `+campaignColumns+` FROM promo_campaigns c WHERE c.id=$1`, id))
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusCreated, c)
	return nil
}

func (h *Handler) managedCampaign(r *http.Request) (*Campaign, string, error) {
	p, err := platform.Viewer(r)
	if err != nil {
		return nil, "", err
	}
	e, err := h.Events.Managed(r.Context(), p, r.PathValue("eventId"))
	if err != nil {
		return nil, "", err
	}
	id, err := platform.PathID(r, "campaignId", "Campaign")
	if err != nil {
		return nil, "", err
	}
	c, err := scan(h.DB.QueryRowContext(r.Context(), `SELECT `+campaignColumns+` FROM promo_campaigns c WHERE c.id=$1 AND c.event_id=$2`, id, e.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", platform.NotFound("Campaign not found")
	}
	return c, p.ID, err
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name           *string    `json:"name,omitempty"`
		Status         *string    `json:"status,omitempty"`
		ValidUntil     *time.Time `json:"validUntil,omitempty"`
		MaxRedemptions *int       `json:"maxRedemptions,omitempty"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	c, actor, err := h.managedCampaign(r)
	if err != nil {
		return err
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		c.Name = strings.TrimSpace(*in.Name)
	}
	if in.Status != nil {
		if *in.Status != "active" && *in.Status != "disabled" {
			return platform.BadRequest("INVALID_STATUS", "Status must be active or disabled")
		}
		c.Status = *in.Status
	}
	if in.ValidUntil != nil {
		c.ValidUntil = in.ValidUntil
	}
	if in.MaxRedemptions != nil {
		if *in.MaxRedemptions < c.RedemptionsCount || *in.MaxRedemptions < 1 {
			return platform.Conflict("LIMIT_BELOW_USED", "The limit cannot be lower than the {n} redemptions already used", "n", c.RedemptionsCount)
		}
		c.MaxRedemptions = in.MaxRedemptions
	}
	err = platform.InTx(r.Context(), h.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `UPDATE promo_campaigns SET name=$2, status=$3, valid_until=$4, max_redemptions=$5 WHERE id=$1`,
			c.ID, c.Name, c.Status, c.ValidUntil, c.MaxRedemptions); err != nil {
			return err
		}
		return platform.Audit(r.Context(), tx, platform.AuditEntry{EventID: c.EventID, ActorID: actor, Action: "campaign.updated", EntityType: "campaign", EntityID: c.ID,
			Description: "Promo campaign updated: " + c.Name + " (" + c.Status + ")"})
	})
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, c)
	return nil
}

func (h *Handler) qr(w http.ResponseWriter, r *http.Request) error {
	c, _, err := h.managedCampaign(r)
	if err != nil {
		return err
	}
	png, err := qrcode.Encode(h.Link(c.LinkToken), qrcode.Medium, 512)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", `inline; filename="campaign-`+c.Code+`.png"`)
	_, err = w.Write(png)
	return err
}

// preview validates a code against a basket without reserving a redemption.
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) error {
	if h.Preview != nil && !h.Preview.Allow(platform.ClientKey(r)) {
		return platform.ErrRateLimited
	}
	var in struct {
		Code  string `json:"code"`
		Items []struct {
			TicketTypeID string `json:"ticketTypeId"`
			Quantity     int    `json:"quantity"`
		} `json:"items"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	eventID := r.PathValue("eventId")
	if !platform.IsUUID(eventID) {
		return platform.NotFound("Event not found")
	}
	c, err := FindByCode(r.Context(), h.DB, eventID, in.Code)
	if err != nil {
		return err
	}
	if problem := c.Problem(time.Now()); problem != nil {
		return problem
	}
	lines, err := ParseLines(r.Context(), h.DB, eventID, in.Items)
	if err != nil {
		return err
	}
	total, per := c.Discount(lines)
	if len(lines) > 0 && total == 0 {
		return platform.BadRequest("PROMO_NOT_APPLICABLE", "This promo code does not apply to the selected tickets")
	}
	platform.WriteJSON(w, http.StatusOK, map[string]any{"code": c.Code, "name": c.Name, "discountType": c.DiscountType,
		"discountValue": c.DiscountValue, "discountKzt": total, "perTicketType": per})
	return nil
}

// visit resolves a Campaign QR link: counts the visit server-side (no third-party analytics)
// and tells the web app which event and code to open.
func (h *Handler) visit(w http.ResponseWriter, r *http.Request) error {
	token := r.PathValue("token")
	if len(token) < 8 || len(token) > 64 {
		return platform.NotFound("Campaign not found")
	}
	var eventID, code, status string
	err := h.DB.QueryRowContext(r.Context(), `UPDATE promo_campaigns c SET link_visits = link_visits + 1 FROM events e
		WHERE c.link_token=$1 AND e.id=c.event_id AND e.status='published' AND e.suspended_at IS NULL
		RETURNING c.event_id, c.code, c.status`, token).Scan(&eventID, &code, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.NotFound("Campaign not found")
	}
	if err != nil {
		return err
	}
	out := map[string]any{"eventId": eventID, "active": status == "active"}
	if status == "active" {
		out["code"] = code
	}
	platform.WriteJSON(w, http.StatusOK, out)
	return nil
}
