// Package platform holds the shared plumbing for the Go business service:
// PostgreSQL access and migrations, JSON/HTTP helpers, the audit log,
// platform settings and the identity directory cache.
package platform

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Querier is satisfied by *sql.DB and *sql.Tx so helpers work inside or outside a transaction.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Open connects to PostgreSQL and verifies the connection.
func Open(url string) (*sql.DB, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Migrate applies embedded migrations in file-name order exactly once each.
// A transaction-scoped advisory lock prevents two Go instances migrating at the same time.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS go_schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(strings.TrimPrefix(name, "migrations/"), ".sql")
		body, err := migrationFiles.ReadFile(name)
		if err != nil {
			return err
		}
		err = InTx(ctx, db, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(727401)`); err != nil {
				return err
			}
			var done bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM go_schema_migrations WHERE version=$1)`, version).Scan(&done); err != nil || done {
				return err
			}
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return fmt.Errorf("migration %s: %w", version, err)
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO go_schema_migrations(version) VALUES ($1)`, version)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// InTx runs fn in a transaction, committing on success and rolling back on error or panic.
func InTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
