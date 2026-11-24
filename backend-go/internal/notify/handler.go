package notify

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/biletflow/backend-go/internal/platform"
)

type Handler struct{ DB *sql.DB }

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/notifications", platform.Handler(h.list))
	mux.Handle("POST /api/notifications/read-all", platform.Handler(h.readAll))
	mux.Handle("POST /api/notifications/{id}/read", platform.Handler(h.read))
}

type item struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Params    map[string]any `json:"params"`
	Link      *string        `json:"link"`
	Read      bool           `json:"read"`
	CreatedAt time.Time      `json:"createdAt"`
}

// RequestLocale picks the interface language: X-Locale header, then the user's saved preference.
func RequestLocale(r *http.Request, saved string) string {
	if l := r.Header.Get("X-Locale"); l == "kk" || l == "ru" || l == "en" {
		return l
	}
	return platform.NormalizeLocale(saved)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	locale := RequestLocale(r, p.Locale)
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id, type, params, link, read_at IS NOT NULL, created_at
		FROM notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []item{}
	unread := 0
	for rows.Next() {
		var it item
		var raw []byte
		var link sql.NullString
		if err := rows.Scan(&it.ID, &it.Type, &raw, &link, &it.Read, &it.CreatedAt); err != nil {
			return err
		}
		_ = json.Unmarshal(raw, &it.Params)
		it.Link = platform.NullString(link)
		it.Title, it.Body = Render(it.Type, locale, it.Params)
		if !it.Read {
			unread++
		}
		out = append(out, it)
	}
	platform.WriteJSON(w, http.StatusOK, map[string]any{"items": out, "unread": unread})
	return rows.Err()
}

func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	if _, err := h.DB.ExecContext(r.Context(), `UPDATE notifications SET read_at=now() WHERE user_id=$1 AND read_at IS NULL`, p.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	id, err := platform.PathID(r, "id", "Notification")
	if err != nil {
		return err
	}
	if _, err := h.DB.ExecContext(r.Context(), `UPDATE notifications SET read_at=coalesce(read_at, now()) WHERE id=$1 AND user_id=$2`, id, p.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
