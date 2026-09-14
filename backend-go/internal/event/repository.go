package event

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("event not found")

// Repository is the persistence boundary for Event. The Postgres
// implementation lands once Aliar's DB setup task is done; until then
// InMemoryRepository lets the service and handler be built and tested
// against a real interface.
type Repository interface {
	Create(e *Event) error
	Get(id string) (*Event, error)
	List(status *Status) ([]*Event, error)
	Update(e *Event) error
	Delete(id string) error
}

type InMemoryRepository struct {
	mu     sync.Mutex
	events map[string]*Event
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{events: make(map[string]*Event)}
}

func (r *InMemoryRepository) Create(e *Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[e.ID] = e
	return nil
}

func (r *InMemoryRepository) Get(id string) (*Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.events[id]
	if !ok {
		return nil, ErrNotFound
	}
	return e, nil
}

func (r *InMemoryRepository) List(status *Status) ([]*Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]*Event, 0, len(r.events))
	for _, e := range r.events {
		if status == nil || e.Status == *status {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *InMemoryRepository) Update(e *Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.events[e.ID]; !ok {
		return ErrNotFound
	}
	r.events[e.ID] = e
	return nil
}

func (r *InMemoryRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.events[id]; !ok {
		return ErrNotFound
	}
	delete(r.events, id)
	return nil
}
