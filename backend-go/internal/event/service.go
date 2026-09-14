package event

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// newID generates a random hex ID. Swap for a proper UUID library
// (e.g. github.com/google/uuid) once module access is set up, or for
// whatever ID strategy Postgres ends up using.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var ErrValidation = errors.New("validation failed")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates input and stores a new draft event owned by organizerID.
// organizerID will come from the JWT claims once Aliar's auth is wired in
// (Week 3-4, per the "authentication between Python and Go services" task);
// for now callers pass it in directly.
func (s *Service) Create(organizerID string, in CreateEventInput) (*Event, error) {
	if in.Title == "" {
		return nil, ErrValidation
	}
	if in.StartsAt.IsZero() {
		return nil, ErrValidation
	}

	visibility := in.Visibility
	if visibility == "" {
		visibility = VisibilityPublic
	}

	e := &Event{
		ID:          newID(),
		OrganizerID: organizerID,
		Title:       in.Title,
		Description: in.Description,
		Category:    in.Category,
		StartsAt:    in.StartsAt,
		EndsAt:      in.EndsAt,
		VenueID:     in.VenueID,
		Visibility:  visibility,
		Capacity:    in.Capacity,
		Status:      StatusDraft,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.repo.Create(e); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *Service) Get(id string) (*Event, error) {
	return s.repo.Get(id)
}

func (s *Service) List(status *Status) ([]*Event, error) {
	return s.repo.List(status)
}

// Publish moves a draft event to published, enforcing the SRS 4.2 rule
// that an event needs its required fields before it can go live.
func (s *Service) Publish(id string) (*Event, error) {
	e, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	if e.Title == "" || e.StartsAt.IsZero() {
		return nil, ErrValidation
	}
	e.Status = StatusPublished
	if err := s.repo.Update(e); err != nil {
		return nil, err
	}
	return e, nil
}
