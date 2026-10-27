package event

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository bounds every operation, including row-lock acquisition.
type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const columns = `id::text,organizer_id::text,title,description,category,starts_at,ends_at,venue_id::text,visibility,capacity,status,created_at`

func scan(row pgx.Row) (*Event, error) {
	e := &Event{}
	err := row.Scan(&e.ID, &e.OrganizerID, &e.Title, &e.Description, &e.Category, &e.StartsAt, &e.EndsAt, &e.VenueID, &e.Visibility, &e.Capacity, &e.Status, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}
func (r *PostgresRepository) Create(ctx context.Context, e *Event) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.pool.Exec(ctx, `INSERT INTO events.events (id,organizer_id,title,description,category,starts_at,ends_at,venue_id,visibility,capacity,status,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, e.ID, e.OrganizerID, e.Title, e.Description, e.Category, e.StartsAt, e.EndsAt, e.VenueID, e.Visibility, e.Capacity, e.Status, e.CreatedAt)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" && pg.ConstraintName == "events_organizer_id_fkey" {
		return ErrForbidden
	}
	return err
}
func (r *PostgresRepository) Get(ctx context.Context, id string) (*Event, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return scan(r.pool.QueryRow(ctx, "SELECT "+columns+" FROM events.events WHERE id=$1", id))
}
func (r *PostgresRepository) List(ctx context.Context, owner string, status *Status) ([]*Event, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var args []any
	query := "SELECT " + columns + " FROM events.events WHERE status='published' AND visibility='public'"
	// UUID parameter for organizer index use, avoiding casts on indexed columns.
	if owner != "" {
		query = "SELECT " + columns + " FROM events.events WHERE (organizer_id=$1::uuid OR (status='published' AND visibility='public'))"
		args = []any{owner}
	}
	if status != nil {
		if len(args) == 0 {
			query += " AND status=$1"
		} else {
			query += " AND status=$2"
		}
		args = append(args, string(*status))
	}
	query += " ORDER BY created_at DESC,id"
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*Event, 0)
	for rows.Next() {
		e, err := scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
func (r *PostgresRepository) Mutate(ctx context.Context, id string, change func(*Event) error) (*Event, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	e, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM events.events WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return nil, err
	}
	if err = change(e); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE events.events SET title=$2,description=$3,category=$4,starts_at=$5,ends_at=$6,venue_id=$7,visibility=$8,capacity=$9,status=$10 WHERE id=$1`, e.ID, e.Title, e.Description, e.Category, e.StartsAt, e.EndsAt, e.VenueID, e.Visibility, e.Capacity, e.Status)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return e, nil
}
