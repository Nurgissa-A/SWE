package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/config"
	"github.com/biletflow/backend-go/internal/database"
	"github.com/biletflow/backend-go/internal/event"
	"github.com/biletflow/backend-go/internal/httpapi"
)

func run() error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stop := context.WithTimeout(ctx, 10*time.Second)
	pool, err := database.Open(startup, c.DatabaseURL, c.MaxConns)
	stop()
	if err != nil {
		return err
	}
	defer pool.Close()
	key, err := os.ReadFile(c.KeyFile)
	if err != nil {
		return errors.New("JWT public key is unreadable")
	}
	verifier, err := auth.NewVerifier(key, c.Issuer, c.Audience, c.KeyID, c.IdentityURL)
	if err != nil {
		return err
	}
	events := http.NewServeMux()
	event.NewHandler(event.NewService(event.NewPostgresRepository(pool))).RegisterRoutes(events)
	mux := http.NewServeMux()
	mux.Handle("/api/", verifier.Middleware(events))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		probe, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		var version int
		if err := pool.QueryRow(probe, "SELECT version FROM events.schema_migrations WHERE version=1 AND to_regclass('events.events') IS NOT NULL").Scan(&version); err != nil {
			auth.WriteError(w, 503, "event storage unavailable")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	server := &http.Server{Addr: c.Address, Handler: httpapi.Middleware(mux, c.Origins), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	slog.Info("Go event service listening", "address", c.Address)
	select {
	case err = <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP listener failed")
	case <-ctx.Done():
		shutdown, finish := context.WithTimeout(context.Background(), 10*time.Second)
		defer finish()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return errors.New("HTTP shutdown timed out")
		}
		return nil
	}
}
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err.Error())
		os.Exit(1)
	}
}
