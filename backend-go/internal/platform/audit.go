package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
)

// AuditEntry is one append-only audit/timeline record (SRS §4.9, §4.16).
type AuditEntry struct {
	EventID     string
	ActorID     string
	Action      string
	EntityType  string
	EntityID    string
	Description string
	Metadata    map[string]any
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Audit records an entry using q, so it commits or rolls back with the surrounding transaction.
func Audit(ctx context.Context, q Querier, e AuditEntry) error {
	meta := e.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `INSERT INTO audit_logs(event_id, actor_id, action, entity_type, entity_id, description, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, nullable(e.EventID), nullable(e.ActorID), e.Action, e.EntityType, nullable(e.EntityID), e.Description, raw)
	return err
}

// SettingInt and SettingFloat read numeric platform settings (activation fee, processing fee, hold minutes).
func SettingInt(ctx context.Context, q Querier, key string, fallback int) int {
	var raw string
	if err := q.QueryRowContext(ctx, `SELECT value::text FROM platform_settings WHERE key=$1`, key).Scan(&raw); err != nil {
		return fallback
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return int(v)
	}
	return fallback
}

func SettingFloat(ctx context.Context, q Querier, key string, fallback float64) float64 {
	var raw string
	if err := q.QueryRowContext(ctx, `SELECT value::text FROM platform_settings WHERE key=$1`, key).Scan(&raw); err != nil {
		return fallback
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return v
	}
	return fallback
}

// NullString converts sql.NullString to *string for JSON output.
func NullString(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}
