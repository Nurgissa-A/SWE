// Package app assembles the Go business service: repository → service → handler for each
// feature area, behind the authentication middleware.
package app

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/biletflow/backend-go/internal/admin"
	"github.com/biletflow/backend-go/internal/analytics"
	"github.com/biletflow/backend-go/internal/checkin"
	"github.com/biletflow/backend-go/internal/commerce"
	"github.com/biletflow/backend-go/internal/event"
	"github.com/biletflow/backend-go/internal/notify"
	"github.com/biletflow/backend-go/internal/platform"
	"github.com/biletflow/backend-go/internal/promo"
	"github.com/biletflow/backend-go/internal/support"
)

type Config struct {
	DB *sql.DB
	// Authenticate resolves the caller (the JWT verifier in production, a fake in tests).
	Authenticate func(http.Handler) http.Handler
	Lookup       event.IdentityLookup
	QRKey        []byte
	AppURL       string
	// RateLimitScale multiplies every limit; tests raise it so they never trip limits by accident.
	RateLimitScale int
}

// App is the assembled service plus the background jobs it needs.
type App struct {
	Handler  http.Handler
	Commerce *commerce.Service
}

func limit(n int, window time.Duration, scale int) *platform.RateLimiter {
	if scale < 1 {
		scale = 1
	}
	return platform.NewRateLimiter(n*scale, window)
}

func New(cfg Config) *App {
	scale := cfg.RateLimitScale
	signer := commerce.QRSigner{Key: cfg.QRKey}
	events := event.NewService(event.NewRepository(cfg.DB), cfg.Lookup)
	shop := &commerce.Service{DB: cfg.DB, Events: events, Provider: commerce.Simulator{}, Signer: signer}
	shop.Limits.Checkout = limit(30, time.Minute, scale)
	shop.Limits.Pay = limit(20, time.Minute, scale)

	mux := http.NewServeMux()
	event.NewHandler(events).RegisterRoutes(mux)
	shop.RegisterRoutes(mux)
	(&promo.Handler{DB: cfg.DB, Events: events, AppURL: cfg.AppURL, Preview: limit(20, time.Minute, scale)}).RegisterRoutes(mux)
	(&checkin.Handler{DB: cfg.DB, Signer: signer, Scans: limit(120, time.Minute, scale)}).RegisterRoutes(mux)
	(&support.Handler{DB: cfg.DB, Messages: limit(20, time.Minute, scale)}).RegisterRoutes(mux)
	(&analytics.Handler{DB: cfg.DB, Events: events}).RegisterRoutes(mux)
	(&admin.Handler{DB: cfg.DB}).RegisterRoutes(mux)
	(&notify.Handler{DB: cfg.DB}).RegisterRoutes(mux)

	global := limit(600, time.Minute, scale)
	api := cfg.Authenticate(platform.DirectoryMiddleware(cfg.DB, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !global.Allow("ip:" + platform.ClientIP(r)) {
			platform.WriteError(w, platform.ErrRateLimited)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		mux.ServeHTTP(w, r)
	})))

	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		platform.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	root.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := cfg.DB.PingContext(ctx); err != nil {
			platform.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
			return
		}
		platform.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	root.Handle("/api/", api)
	return &App{Handler: root, Commerce: shop}
}
