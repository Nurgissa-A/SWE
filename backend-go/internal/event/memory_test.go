package event

import (
	"context"
	"sort"
	"sync"
)

// InMemoryRepository is test-only. Copies prevent aliasing across concurrent calls.
type InMemoryRepository struct {
	mu     sync.Mutex
	events map[string]Event
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{events: map[string]Event{}}
}
func clone(e Event) *Event {
	if e.Description != nil {
		v := *e.Description
		e.Description = &v
	}
	if e.Category != nil {
		v := *e.Category
		e.Category = &v
	}
	if e.VenueID != nil {
		v := *e.VenueID
		e.VenueID = &v
	}
	if e.Capacity != nil {
		v := *e.Capacity
		e.Capacity = &v
	}
	if e.EndsAt != nil {
		v := *e.EndsAt
		e.EndsAt = &v
	}
	return &e
}
func (r *InMemoryRepository) Create(ctx context.Context, e *Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	r.events[e.ID] = *clone(*e)
	return nil
}
func (r *InMemoryRepository) Get(ctx context.Context, id string) (*Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e, ok := r.events[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(e), nil
}
func (r *InMemoryRepository) List(ctx context.Context, owner string, status *Status) ([]*Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]*Event, 0)
	for _, e := range r.events {
		if (e.OrganizerID == owner || (e.Status == StatusPublished && e.Visibility == VisibilityPublic)) && (status == nil || e.Status == *status) {
			out = append(out, clone(e))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}
func (r *InMemoryRepository) Mutate(ctx context.Context, id string, change func(*Event) error) (*Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e, ok := r.events[id]
	if !ok {
		return nil, ErrNotFound
	}
	updated := clone(e)
	if err := change(updated); err != nil {
		return nil, err
	}
	r.events[id] = *clone(*updated)
	return updated, nil
}
