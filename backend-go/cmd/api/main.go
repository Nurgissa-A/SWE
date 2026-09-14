package main

import (
	"log"
	"net/http"

	"github.com/biletflow/backend-go/internal/event"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Event service wiring: repository -> service -> handler.
	// repository is nil for now (no DB yet) — service methods return
	// stub data until Postgres + Aliar's auth service are in place.
	eventRepo := event.NewInMemoryRepository()
	eventService := event.NewService(eventRepo)
	eventHandler := event.NewHandler(eventService)
	eventHandler.RegisterRoutes(mux)

	addr := ":8080"
	log.Printf("backend-go listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
