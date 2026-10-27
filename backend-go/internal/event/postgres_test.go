package event

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/database"
	"github.com/biletflow/backend-go/migrations"
	"github.com/jackc/pgx/v5/pgconn"
)

// This suite intentionally exercises up/down. Never point it at development data.
func TestPostgresIntegration(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("CI requires TEST_DATABASE_URL")
		}
		t.Skip("set TEST_DATABASE_URL to a disposable *_go_test database with Python migrations")
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.HasSuffix(u.Path, "_go_test") {
		t.Fatal("refusing database not ending _go_test")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, raw, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, down := range []bool{false, false, true, false} {
		if err = migrations.Apply(ctx, pool, down); err != nil {
			t.Fatalf("migration down=%v: %v", down, err)
		}
	}

	t.Run("migration checksum refuses drift", func(t *testing.T) {
		var checksum string
		if err := pool.QueryRow(ctx, "SELECT checksum FROM events.schema_migrations WHERE version=1").Scan(&checksum); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "UPDATE events.schema_migrations SET checksum='modified' WHERE version=1"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := pool.Exec(ctx, "UPDATE events.schema_migrations SET checksum=$1 WHERE version=1", checksum); err != nil {
				t.Error(err)
			}
		}()
		if err := migrations.Apply(ctx, pool, false); err == nil {
			t.Fatal("modified migration accepted")
		}
	})
	id, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := newID()
	otherID, _ := newID()
	_, err = pool.Exec(ctx, `INSERT INTO public.identity_users(id,email,password_hash,role,email_verified,is_active,token_version,created_at) VALUES($1,$2,'test-only-not-a-password','organizer',true,true,0,now())`, id, id+"@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO public.identity_organizer_profiles(id,user_id,contact_email) VALUES($1,$2,$3)`, profile, id, id+"@example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, "DELETE FROM events.events WHERE organizer_id=$1", id); err != nil {
			t.Error(err)
		}
		if _, err := pool.Exec(ctx, "DELETE FROM public.identity_organizer_profiles WHERE user_id=$1", id); err != nil {
			t.Error(err)
		}
		if _, err := pool.Exec(ctx, "DELETE FROM public.identity_users WHERE id=$1", id); err != nil {
			t.Error(err)
		}
	}()
	principal := auth.Principal{ID: id, Role: "organizer", EmailVerified: true}
	ownerCtx := as(principal)
	outsider := as(auth.Principal{ID: otherID, Role: "organizer", EmailVerified: true})
	repo := NewPostgresRepository(pool)
	service := NewService(repo)
	e, err := service.Create(ownerCtx, validInput())
	if err != nil {
		t.Fatal(err)
	}
	t.Run("persistent create and reopen pool", func(t *testing.T) {
		fresh, err := database.Open(ctx, raw, 2)
		if err != nil {
			t.Fatal(err)
		}
		defer fresh.Close()
		got, err := NewService(NewPostgresRepository(fresh)).Get(ownerCtx, e.ID)
		if err != nil || got.Title != e.Title || got.OrganizerID != id {
			t.Fatalf("reopened: %+v %v", got, err)
		}
	})
	t.Run("constraints and foreign key", func(t *testing.T) {
		for _, tc := range []struct {
			name, code string
			change     func(*Event)
		}{
			{"unknown organizer", "23503", func(e *Event) { e.OrganizerID = otherID }},
			{"bad visibility", "23514", func(e *Event) { e.Visibility = "invalid" }},
			{"bad status", "23514", func(e *Event) { e.Status = "complete" }},
			{"bad capacity", "23514", func(e *Event) { e.Capacity = ptr(0) }},
			{"blank title", "23514", func(e *Event) { e.Title = " " }},
			{"bad dates", "23514", func(e *Event) { e.EndsAt = ptr(e.StartsAt) }},
			{"incomplete publication", "23514", func(e *Event) { e.VenueID = nil; e.Status = StatusPublished }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				copy := *e
				copy.ID, _ = newID()
				tc.change(&copy)
				err := repo.Create(ctx, &copy)
				if tc.code == "23503" {
					if !errors.Is(err, ErrForbidden) {
						t.Fatalf("missing organizer: %v", err)
					}
					return
				}
				var pg *pgconn.PgError
				if !errors.As(err, &pg) || pg.Code != tc.code {
					t.Fatalf("constraint: %v", err)
				}
			})
		}
		if err := repo.Create(ctx, e); err == nil {
			t.Fatal("duplicate accepted")
		}
		_, err := pool.Exec(ctx, "DELETE FROM public.identity_organizer_profiles WHERE user_id=$1", id)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "23503" {
			t.Fatal("owner FK not enforced", err)
		}
	})
	t.Run("atomic partial updates and rollback", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for _, in := range []UpdateEventInput{{Title: ptr("Updated title")}, {Description: ptr("New description")}} {
			wg.Add(1)
			go func(in UpdateEventInput) {
				defer wg.Done()
				_, err := service.Update(ownerCtx, e.ID, in)
				results <- err
			}(in)
		}
		wg.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatal(err)
			}
		}
		got, err := service.Get(ownerCtx, e.ID)
		if err != nil || got.Title != "Updated title" || got.Description == nil || *got.Description != "New description" {
			t.Fatal("lost update", got, err)
		}
		if _, err = service.Update(ownerCtx, e.ID, UpdateEventInput{Title: ptr("must rollback"), Capacity: ptr(-1)}); !errors.Is(err, ErrValidation) {
			t.Fatal(err)
		}
		got, _ = service.Get(ownerCtx, e.ID)
		if got.Title != "Updated title" {
			t.Fatal("rollback failed")
		}
	})
	t.Run("ownership and scoped query", func(t *testing.T) {
		if _, err := service.Update(outsider, e.ID, UpdateEventInput{Title: ptr("forged")}); !errors.Is(err, ErrForbidden) {
			t.Fatal(err)
		}
		if _, err := service.Publish(outsider, e.ID); !errors.Is(err, ErrForbidden) {
			t.Fatal(err)
		}
		if _, err := service.Get(outsider, e.ID); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		for _, status := range []Status{StatusDraft, StatusPublished} {
			rows, err := service.List(outsider, &status)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.ID == e.ID {
					t.Fatal("draft leaked by SQL")
				}
			}
		}
	})
	t.Run("query cancellation and lock deadline", func(t *testing.T) {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := repo.Get(canceled, e.ID); err == nil {
			t.Fatal("canceled query succeeded")
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err = tx.Exec(ctx, "SELECT id FROM events.events WHERE id=$1 FOR UPDATE", e.ID); err != nil {
			t.Fatal(err)
		}
		deadline, cancel := context.WithTimeout(ownerCtx, 50*time.Millisecond)
		defer cancel()
		if _, err = service.Update(deadline, e.ID, UpdateEventInput{Title: ptr("timeout")}); err == nil {
			t.Fatal("locked update did not time out")
		}
	})
	t.Run("concurrent publication commits once", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := service.Publish(ownerCtx, e.ID); results <- err }()
		}
		wg.Wait()
		close(results)
		count := 0
		for err := range results {
			if err == nil {
				count++
			} else if !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
		}
		if count != 1 {
			t.Fatal("publication count", count)
		}
		rows, err := service.List(outsider, ptr(StatusPublished))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			if row.ID == e.ID {
				found = true
			}
		}
		if !found {
			t.Fatal("published event absent")
		}
	})
	t.Run("persisted API patch and publication", func(t *testing.T) {
		mux := http.NewServeMux()
		NewHandler(service).RegisterRoutes(mux)
		response := requestAs(mux, "POST", "/api/events", `{"title":"HTTP draft","startsAt":"2030-01-01T10:00:00Z"}`, principal)
		if response.Code != 201 {
			t.Fatal(response.Body.String())
		}
		// Read through repository to assert handlers use the same persistent boundary.
		rows, err := service.List(ownerCtx, ptr(StatusDraft))
		if err != nil {
			t.Fatal(err)
		}
		var draft *Event
		for _, row := range rows {
			if row.Title == "HTTP draft" {
				draft = row
			}
		}
		if draft == nil {
			t.Fatal("missing API draft")
		}
		path := "/api/events/" + draft.ID
		if r := requestAs(mux, "POST", path+"/publish", "", principal); r.Code != 400 {
			t.Fatal(r.Code)
		}
		if r := requestAs(mux, "PATCH", path, `{"venueId":"00000000-0000-4000-8000-000000000010","capacity":99}`, principal); r.Code != 200 {
			t.Fatal(r.Code, r.Body.String())
		}
		if r := requestAs(mux, "POST", path+"/publish", "", principal); r.Code != 200 {
			t.Fatal(r.Code, r.Body.String())
		}
		if r := requestAs(mux, "PATCH", path, `{"title":"forbidden edit"}`, principal); r.Code != 409 {
			t.Fatal(r.Code)
		}
		if r := requestAs(mux, "GET", path, "", auth.Principal{}); r.Code != 200 {
			t.Fatal(r.Code)
		}
		got, err := repo.Get(ctx, draft.ID)
		if err != nil || got.Status != StatusPublished || got.Capacity == nil || *got.Capacity != 99 {
			t.Fatal("HTTP changes not persisted", err)
		}
	})
	t.Run("closed database returns failure", func(t *testing.T) {
		closed, err := database.Open(ctx, raw, 1)
		if err != nil {
			t.Fatal(err)
		}
		closed.Close()
		mux := http.NewServeMux()
		NewHandler(NewService(NewPostgresRepository(closed))).RegisterRoutes(mux)
		if r := requestAs(mux, "GET", "/api/events/"+e.ID, "", principal); r.Code != 500 {
			t.Fatal(r.Code)
		}
	})
}
