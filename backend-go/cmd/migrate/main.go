package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/biletflow/backend-go/internal/database"
	"github.com/biletflow/backend-go/migrations"
)

func main() {
	down := len(os.Args) == 2 && os.Args[1] == "down"
	if len(os.Args) > 2 || (len(os.Args) == 2 && os.Args[1] != "up" && !down) {
		slog.Error("usage: migrate [up|down]")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, os.Getenv("DATABASE_URL"), 2)
	if err != nil {
		slog.Error("migration database unavailable")
		os.Exit(1)
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool, down); err != nil {
		slog.Error("Go migration failed; ensure Python migrations ran first and inspect schema version")
		os.Exit(1)
	}
	slog.Info("Go migration complete", "down", down)
}
