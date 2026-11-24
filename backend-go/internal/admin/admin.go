// Package admin is the platform administration for business data (SRS §4.11):
// event moderation, paid-sales control, orders and payments monitoring, activations,
// reports, promo activity, platform settings and CSV exports. User accounts are
// administered by the Python identity service.
package admin

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/notify"
	"github.com/biletflow/backend-go/internal/platform"
)

type Handler struct{ DB *sql.DB }

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/admin/summary", h.admin(h.summary))
	mux.Handle("GET /api/admin/events", h.admin(h.events))
	mux.Handle("POST /api/admin/events/{eventId}/suspend", h.admin(h.suspend))
	mux.Handle("POST /api/admin/events/{eventId}/unsuspend", h.admin(h.unsuspend))
	mux.Handle("POST /api/admin/events/{eventId}/paid-sales", h.admin(h.paidSales))
	mux.Handle("GET /api/admin/orders", h.admin(h.orders))
	mux.Handle("GET /api/admin/payments", h.admin(h.payments))
	mux.Handle("GET /api/admin/activations", h.admin(h.activations))
	mux.Handle("GET /api/admin/campaigns", h.admin(h.campaigns))
	mux.Handle("GET /api/admin/reports", h.admin(h.reports))
	mux.Handle("PATCH /api/admin/reports/{reportId}", h.admin(h.updateReport))
	mux.Handle("GET /api/admin/settings", h.admin(h.settings))
	mux.Handle("PUT /api/admin/settings", h.admin(h.saveSettings))
	mux.Handle("GET /api/admin/export/{kind}", h.admin(h.export))
	mux.Handle("POST /api/events/{eventId}/report", platform.Handler(h.report))
}

func (h *Handler) admin(fn func(http.ResponseWriter, *http.Request, auth.Principal) error) platform.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		p, err := platform.Viewer(r)
		if err != nil {
			return err
		}
		if p.Role != "platform_admin" {
			return platform.Forbidden("ADMIN_REQUIRED", "Platform admin role required")
		}
		return fn(w, r, p)
	}
}

// rows runs a query and returns each row as a map keyed by column name (camelCase from SQL aliases).
func (h *Handler) rows(r *http.Request, query string, args ...any) ([]map[string]any, error) {
	rows, err := h.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := map[string]any{}
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				m[c] = string(b)
			} else {
				m[c] = vals[i]
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, query string, args ...any) error {
	out, err := h.rows(r, query, args...)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, out)
	return nil
}

func like(r *http.Request) string {
	return "%" + strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q"))) + "%"
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request, _ auth.Principal) error {
	out, err := h.rows(r, `SELECT
		(SELECT count(*) FROM events WHERE status='published' AND suspended_at IS NULL) AS "publishedEvents",
		(SELECT count(*) FROM events WHERE suspended_at IS NOT NULL) AS "suspendedEvents",
		(SELECT count(*) FROM orders WHERE status='completed') AS "completedOrders",
		(SELECT coalesce(sum(amount_kzt),0) FROM payments WHERE status='succeeded' AND kind='ticket') AS "ticketVolumeKzt",
		(SELECT coalesce(sum(amount_kzt),0) FROM refunds) AS "refundsKzt",
		(SELECT count(*) FROM payments WHERE status='failed' AND created_at > now() - interval '7 days') AS "failedPayments7d",
		(SELECT count(*) FROM event_reports WHERE status='open') AS "openReports",
		(SELECT count(*) FROM support_cases WHERE (kind='organizer' OR escalated) AND status <> 'resolved') AS "openCases",
		true AS "isSimulated"`)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, out[0])
	return nil
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request, _ auth.Principal) error {
	return h.list(w, r, `SELECT e.id, e.title, e.status, e.visibility, e.starts_at AS "startsAt", e.city, e.paid_sales_status AS "paidSalesStatus",
		e.suspended_at IS NOT NULL AS suspended, e.suspension_reason AS "suspensionReason", e.organizer_id AS "organizerId",
		coalesce(d.display_name, d.email) AS organizer,
		(SELECT count(*) FROM tickets t WHERE t.event_id=e.id AND t.status IN ('valid','checked_in')) AS sold,
		(SELECT count(*) FROM event_reports x WHERE x.event_id=e.id AND x.status='open') AS "openReports"
		FROM events e LEFT JOIN user_directory d ON d.user_id=e.organizer_id
		WHERE lower(e.title || ' ' || coalesce(e.city,'') || ' ' || coalesce(d.email,'')) LIKE $1
		ORDER BY e.created_at DESC LIMIT 200`, like(r))
}

func (h *Handler) eventTitle(r *http.Request) (string, string, string, error) {
	id := r.PathValue("eventId")
	if !platform.IsUUID(id) {
		return "", "", "", platform.NotFound("Event not found")
	}
	var title, organizer string
	err := h.DB.QueryRowContext(r.Context(), `SELECT title, organizer_id FROM events WHERE id=$1`, id).Scan(&title, &organizer)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", platform.NotFound("Event not found")
	}
	return id, title, organizer, err
}

func (h *Handler) suspend(w http.ResponseWriter, r *http.Request, p auth.Principal) error {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" {
		return platform.BadRequest("REASON_REQUIRED", "Give a reason for the suspension")
	}
	id, title, organizer, err := h.eventTitle(r)
	if err != nil {
		return err
	}
	err = platform.InTx(r.Context(), h.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `UPDATE events SET suspended_at=now(), suspension_reason=$2, updated_at=now() WHERE id=$1`, id, in.Reason); err != nil {
			return err
		}
		if err := platform.Audit(r.Context(), tx, platform.AuditEntry{EventID: id, ActorID: p.ID, Action: "moderation.suspended", EntityType: "event", EntityID: id,
			Description: "Suspended by BiletFlow: " + in.Reason}); err != nil {
			return err
		}
		return notify.Queue(r.Context(), tx, notify.Message{UserID: organizer, Type: "event_suspended", Params: map[string]any{"event": title, "reason": in.Reason},
			Link: "/dashboard/events/" + id, Email: true})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) unsuspend(w http.ResponseWriter, r *http.Request, p auth.Principal) error {
	id, _, _, err := h.eventTitle(r)
	if err != nil {
		return err
	}
	err = platform.InTx(r.Context(), h.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `UPDATE events SET suspended_at=NULL, suspension_reason=NULL, updated_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		return platform.Audit(r.Context(), tx, platform.AuditEntry{EventID: id, ActorID: p.ID, Action: "moderation.unsuspended", EntityType: "event", EntityID: id,
			Description: "Suspension lifted by BiletFlow"})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// paidSales suspends or resumes paid ticket sales of one event; free registration is unaffected (SRS §4.5).
func (h *Handler) paidSales(w http.ResponseWriter, r *http.Request, p auth.Principal) error {
	var in struct {
		Action string `json:"action"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	id, title, organizer, err := h.eventTitle(r)
	if err != nil {
		return err
	}
	from, to, kind := "active", "suspended", "paid_sales_suspended"
	switch in.Action {
	case "suspend":
	case "resume":
		from, to, kind = "suspended", "active", "paid_sales_resumed"
	default:
		return platform.BadRequest("INVALID_ACTION", "Action must be suspend or resume")
	}
	err = platform.InTx(r.Context(), h.DB, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(r.Context(), `UPDATE events SET paid_sales_status=$3, updated_at=now() WHERE id=$1 AND paid_sales_status=$2`, id, from, to)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return platform.Conflict("PAID_SALES_STATE", "Paid sales are not {state} for this event", "state", from)
		}
		if err := platform.Audit(r.Context(), tx, platform.AuditEntry{EventID: id, ActorID: p.ID, Action: "moderation.paid_sales_" + in.Action, EntityType: "event", EntityID: id,
			Description: "Paid sales " + to + " by BiletFlow"}); err != nil {
			return err
		}
		return notify.Queue(r.Context(), tx, notify.Message{UserID: organizer, Type: kind, Params: map[string]any{"event": title}, Link: "/dashboard/events/" + id, Email: true})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) orders(w http.ResponseWriter, r *http.Request, _ auth.Principal) error {
	return h.list(w, r, `SELECT o.id, o.number, o.status, o.buyer_name AS "buyerName", o.buyer_email AS "buyerEmail", o.total_kzt AS "totalKzt",
		o.discount_kzt AS "discountKzt", o.created_at AS "createdAt", e.id AS "eventId", e.title AS "eventTitle"
		FROM orders o JOIN events e ON e.id=o.event_id
		WHERE o.status <> 'pending' AND lower(o.number || ' ' || o.buyer_email || ' ' || o.buyer_name || ' ' || e.title) LIKE $1
		ORDER BY o.created_at DESC LIMIT 200`, like(r))
}

func (h *Handler) payments(w http.ResponseWriter, r *http.Request, _ auth.Principal) error {
	status := r.URL.Query().Get("status")
	if status != "" && status != "succeeded" && status != "failed" {
		return platform.BadRequest("INVALID_STATUS", "Unknown status")
	}
	return h.list(w, r, `SELECT p.id, p.kind, p.status, p.amount_kzt AS "amountKzt", p.fee_kzt AS "feeKzt", p.failure_reason AS "failureReason",
		p.provider, p.provider_ref AS "providerRef", p.card_last4 AS "cardLast4", p.is_simulated AS "isSimulated", p.created_at AS "createdAt",
		e.title AS "eventTitle", o.number AS "orderNumber",
		(SELECT coalesce(sum(f.amount_kzt),0) FROM refunds f WHERE f.order_id=p.order_id) AS "refundedKzt"
		FROM payments p JOIN events e ON e.id=p.event_id LEFT JOIN orders o ON o.id=p.order_id
		WHERE ($1 = '' OR p.status=$1) ORDER BY p.created_at DESC LIMIT 300`, status)
}

func (h *Handler) activations(w http.ResponseWriter, r *http.Request, _ auth.Principal) error {
	return h.list(w, r, `SELECT e.id AS "eventId", e.title, e.paid_sales_status AS "paidSalesStatus", coalesce(d.display_name, d.email) AS organizer,
		s.legal_name AS "legalName", s.identity_verified_at AS "identityVerifiedAt", s.iban_masked AS "ibanMasked",
		a.fee_kzt AS "feeKzt", a.fee_paid_at AS "feePaidAt", a.terms_accepted_at AS "termsAcceptedAt", a.activated_at AS "activatedAt"
		FROM events e LEFT JOIN event_activations a ON a.event_id=e.id LEFT JOIN organizer_payment_setups s ON s.organizer_id=e.organizer_id
		LEFT JOIN user_directory d ON d.user_id=e.organizer_id
		WHERE a.event_id IS NOT NULL OR e.paid_sales_status <> 'inactive' ORDER BY coalesce(a.activated_at, a.fee_paid_at, e.created_at) DESC LIMIT 200`)
}

func (h *Handler) campaigns(w http.ResponseWriter, r *http.Request, _ auth.Principal) error {
	return h.list(w, r, `SELECT c.id, c.name, c.code, c.status, c.discount_type AS "discountType", c.discount_value AS "discountValue",
		c.redemptions_count AS "redemptions", c.max_redemptions AS "maxRedemptions", c.link_visits AS "visits", e.title AS "eventTitle",
		(SELECT coalesce(sum(o.discount_kzt),0) FROM orders o WHERE o.promo_campaign_id=c.id AND o.status='completed') AS "discountKzt"
		FROM promo_campaigns c JOIN events e ON e.id=c.event_id ORDER BY c.created_at DESC LIMIT 200`)
}

func (h *Handler) reports(w http.ResponseWriter, r *http.Request, _ auth.Principal) error {
	return h.list(w, r, `SELECT x.id, x.reason, x.status, x.created_at AS "createdAt", e.id AS "eventId", e.title AS "eventTitle",
		e.suspended_at IS NOT NULL AS "eventSuspended", coalesce(d.email, '') AS reporter
		FROM event_reports x JOIN events e ON e.id=x.event_id LEFT JOIN user_directory d ON d.user_id=x.reporter_id
		ORDER BY (x.status='open') DESC, x.created_at DESC LIMIT 200`)
}

func (h *Handler) updateReport(w http.ResponseWriter, r *http.Request, p auth.Principal) error {
	var in struct {
		Status string `json:"status"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	if in.Status != "actioned" && in.Status != "dismissed" && in.Status != "open" {
		return platform.BadRequest("INVALID_STATUS", "Status must be open, actioned or dismissed")
	}
	id, err := platform.PathID(r, "reportId", "Report")
	if err != nil {
		return err
	}
	var eventID string
	err = h.DB.QueryRowContext(r.Context(), `UPDATE event_reports SET status=$2 WHERE id=$1 RETURNING event_id`, id, in.Status).Scan(&eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.NotFound("Report not found")
	}
	if err != nil {
		return err
	}
	if err := platform.Audit(r.Context(), h.DB, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "moderation.report_" + in.Status, EntityType: "report", EntityID: id,
		Description: "Report marked " + in.Status}); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// report lets any signed-in user flag an event to BiletFlow moderation.
func (h *Handler) report(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if len(in.Reason) < 5 || len(in.Reason) > 2000 {
		return platform.BadRequest("REASON_REQUIRED", "Describe the problem (5–2000 characters)")
	}
	id := r.PathValue("eventId")
	var ok bool
	if !platform.IsUUID(id) || h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM events WHERE id=$1 AND status <> 'draft')`, id).Scan(&ok) != nil || !ok {
		return platform.NotFound("Event not found")
	}
	if _, err := h.DB.ExecContext(r.Context(), `INSERT INTO event_reports(event_id, reporter_id, reason) VALUES ($1,$2,$3)`, id, p.ID, in.Reason); err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

var settingKeys = map[string][2]float64{"activation_fee_kzt": {0, 1_000_000}, "processing_fee_percent": {0, 20}, "hold_minutes": {2, 60}}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request, _ auth.Principal) error {
	out := map[string]float64{}
	for k := range settingKeys {
		out[k] = platform.SettingFloat(r.Context(), h.DB, k, 0)
	}
	platform.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (h *Handler) saveSettings(w http.ResponseWriter, r *http.Request, p auth.Principal) error {
	var in map[string]float64
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	err := platform.InTx(r.Context(), h.DB, func(tx *sql.Tx) error {
		for k, v := range in {
			bounds, ok := settingKeys[k]
			if !ok || v < bounds[0] || v > bounds[1] {
				return platform.BadRequest("INVALID_SETTING", "Invalid value for {key}", "key", k)
			}
			if _, err := tx.ExecContext(r.Context(), `UPDATE platform_settings SET value=to_jsonb($2::numeric) WHERE key=$1`, k, strconv.FormatFloat(v, 'f', -1, 64)); err != nil {
				return err
			}
			if err := platform.Audit(r.Context(), tx, platform.AuditEntry{ActorID: p.ID, Action: "settings.updated", EntityType: "setting", EntityID: k,
				Description: fmt.Sprintf("Setting %s changed to %v", k, v)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return h.settings(w, r, p)
}

// export streams orders or payments as CSV for offline review (SRS §4.11).
func (h *Handler) export(w http.ResponseWriter, r *http.Request, p auth.Principal) error {
	var query string
	switch r.PathValue("kind") {
	case "orders.csv":
		query = `SELECT o.number, o.status, e.title, o.buyer_name, o.buyer_email, o.subtotal_kzt, o.discount_kzt, o.total_kzt, o.fee_kzt, o.created_at, o.paid_at, o.refunded_at
			FROM orders o JOIN events e ON e.id=o.event_id WHERE o.status <> 'pending' ORDER BY o.created_at`
	case "payments.csv":
		query = `SELECT p.provider_ref, p.kind, p.status, e.title, p.amount_kzt, p.fee_kzt, p.failure_reason, p.card_last4, p.is_simulated, p.created_at
			FROM payments p JOIN events e ON e.id=p.event_id ORDER BY p.created_at`
	default:
		return platform.NotFound("Unknown export")
	}
	rows, err := h.DB.QueryContext(r.Context(), query)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="biletflow-`+r.PathValue("kind")+`"`)
	out := csv.NewWriter(w)
	_ = out.Write(cols)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		rec := make([]string, len(cols))
		for i, v := range vals {
			switch t := v.(type) {
			case nil:
			case []byte:
				rec[i] = csvSafe(string(t))
			case string:
				rec[i] = csvSafe(t)
			case time.Time:
				rec[i] = t.UTC().Format(time.RFC3339)
			default:
				rec[i] = fmt.Sprint(t)
			}
		}
		_ = out.Write(rec)
	}
	out.Flush()
	_ = platform.Audit(r.Context(), h.DB, platform.AuditEntry{ActorID: p.ID, Action: "admin.export", EntityType: "export", EntityID: r.PathValue("kind"), Description: "Exported " + r.PathValue("kind")})
	return rows.Err()
}

// csvSafe neutralises spreadsheet formula injection in exported text cells.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
