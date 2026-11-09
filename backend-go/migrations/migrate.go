// Package migrations owns only the events schema. Identity must migrate first.
package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var files embed.FS

// Apply is transactional, checksum-verified and serialized across migrators.
// down is destructive and intended only for disposable development databases.
func Apply(ctx context.Context, pool *pgxpool.Pool, down bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(64721,1)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS events; CREATE TABLE IF NOT EXISTS events.schema_migrations (version integer PRIMARY KEY, checksum text NOT NULL)`); err != nil {
		return err
	}
	up, err := files.ReadFile("001_events.up.sql")
	if err != nil {
		return err
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(up))
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM events.schema_migrations WHERE version<>1 OR checksum<>$1", checksum).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("unsupported or modified Go migration")
	}
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM events.schema_migrations WHERE version=1").Scan(&count); err != nil {
		return err
	}
	if down && count == 1 {
		sql, err := files.ReadFile("001_events.down.sql")
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "DELETE FROM events.schema_migrations WHERE version=1"); err != nil {
			return err
		}
	} else if !down && count == 0 {
		if _, err = tx.Exec(ctx, string(up)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO events.schema_migrations VALUES (1,$1)", checksum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
