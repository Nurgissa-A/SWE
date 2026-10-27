package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/biletflow/backend-go/internal/event"

	"github.com/biletflow/backend-go/internal/auth"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Event service wiring: repository -> service -> handler.
	// Event persistence is still in-memory and remains Madi's responsibility.
	eventRepo := event.NewInMemoryRepository()
	eventService := event.NewService(eventRepo)
	eventHandler := event.NewHandler(eventService)
	eventMux := http.NewServeMux()
	eventHandler.RegisterRoutes(eventMux)
	key, err := os.ReadFile(os.Getenv("JWT_PUBLIC_KEY_FILE"))
	if err != nil {
		log.Fatal("JWT_PUBLIC_KEY_FILE must contain a readable RSA public key")
	}
	verifier, err := auth.NewVerifier(key, os.Getenv("JWT_ISSUER"), os.Getenv("JWT_AUDIENCE"), os.Getenv("JWT_KEY_ID"), os.Getenv("IDENTITY_URL"))
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/api/", verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		eventMux.ServeHTTP(w, r)
	})))

	addr := ":8080"
	log.Printf("backend-go listening on %s", addr)
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
