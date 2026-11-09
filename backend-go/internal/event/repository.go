package event

import (
	"context"
	"errors"
)

var (
	ErrNotFound     = errors.New("event not found")
	ErrUnauthorized = errors.New("authentication required")
	ErrForbidden    = errors.New("verified organizer and event ownership required")
	ErrConflict     = errors.New("event must be a draft")
)

// Mutate serializes read/validate/write for one event. An error from change rolls
// back the whole operation; callers must not retain or mutate its argument.
type Repository interface {
	Create(context.Context, *Event) error
	Get(context.Context, string) (*Event, error)
	List(context.Context, string, *Status) ([]*Event, error)
	Mutate(context.Context, string, func(*Event) error) (*Event, error)
}
