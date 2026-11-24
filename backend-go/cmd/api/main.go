package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// Embed the time zone database: the Alpine runtime image has none, and -trimpath removes
	// the Go installation fallback, so event time zones such as Asia/Almaty would be rejected.
	_ "time/tzdata"

	"github.com/biletflow/backend-go/internal/app"
	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/event"
	"github.com/biletflow/backend-go/internal/notify"
	"github.com/biletflow/backend-go/internal/platform"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	key, err := os.ReadFile(os.Getenv("JWT_PUBLIC_KEY_FILE"))
	if err != nil {
		log.Fatal("JWT_PUBLIC_KEY_FILE must contain a readable RSA public key")
	}
	verifier, err := auth.NewVerifier(key, os.Getenv("JWT_ISSUER"), os.Getenv("JWT_AUDIENCE"), os.Getenv("JWT_KEY_ID"), os.Getenv("IDENTITY_URL"))
	if err != nil {
		log.Fatal(err)
	}
	qrKey := []byte(os.Getenv("TICKET_QR_SECRET"))
	if len(qrKey) < 32 {
		log.Fatal("TICKET_QR_SECRET must be at least 32 characters (it signs ticket QR codes)")
	}
	db, err := platform.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("DATABASE_URL: %v", err)
	}
	defer db.Close()
	if err := platform.Migrate(ctx, db); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	platform.TrustProxy = os.Getenv("TRUST_PROXY") == "1"
	appURL := env("APP_URL", "http://localhost:3000")

	lookup := func(ctx context.Context, bearer, email string) (platform.DirectoryUser, error) {
		u, err := verifier.LookupUser(ctx, bearer, email)
		if errors.Is(err, auth.ErrNoSuchUser) {
			return platform.DirectoryUser{}, event.ErrUserNotFound
		}
		return platform.DirectoryUser{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Locale: platform.NormalizeLocale(u.Locale)}, err
	}
	service := app.New(app.Config{DB: db, Authenticate: verifier.Middleware, Lookup: lookup, QRKey: qrKey, AppURL: appURL})

	// Background jobs: release expired checkout holds; deliver notification e-mails.
	go service.Commerce.RunSweeper(ctx, 30*time.Second)
	outbox := &notify.Outbox{DB: db, Mailer: notify.SMTPMailer{Addr: env("SMTP_ADDR", "localhost:1025"), From: env("MAIL_FROM", "BiletFlow <no-reply@biletflow.local>")},
		Attachments: service.Commerce.Attachments, AppURL: appURL}
	go outbox.Run(ctx, 5*time.Second)

	addr := ":" + env("PORT", "8080")
	server := &http.Server{Addr: addr, Handler: service.Handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("backend-go listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
