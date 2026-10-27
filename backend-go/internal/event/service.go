package event

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
)

var ErrValidation = errors.New("validation failed")

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}
func validID(id string) bool {
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return false
	}
	compact := strings.ReplaceAll(id, "-", "")
	if len(compact) != 32 {
		return false
	}
	_, err := hex.DecodeString(compact)
	return err == nil && id != "00000000-0000-0000-0000-000000000000"
}
func invalid(message string) error { return fmt.Errorf("%w: %s", ErrValidation, message) }
func organizer(ctx context.Context) (string, error) {
	p, ok := auth.Current(ctx)
	if !ok || p.ID == "" {
		return "", ErrUnauthorized
	}
	if p.Role != "organizer" || !p.EmailVerified || !validID(p.ID) {
		return "", ErrForbidden
	}
	return p.ID, nil
}
func owner(ctx context.Context, e *Event) bool {
	id, err := organizer(ctx)
	return err == nil && strings.EqualFold(id, e.OrganizerID)
}
func validate(e *Event) error {
	if strings.TrimSpace(e.Title) == "" {
		return invalid("title must not be blank")
	}
	if e.StartsAt.IsZero() {
		return invalid("startsAt is required")
	}
	if e.EndsAt != nil && !e.EndsAt.After(e.StartsAt) {
		return invalid("endsAt must be after startsAt")
	}
	if e.Capacity != nil && (*e.Capacity <= 0 || *e.Capacity > 2147483647) {
		return invalid("capacity must be between 1 and 2147483647")
	}
	if e.VenueID != nil && !validID(*e.VenueID) {
		return invalid("venueId must be a nonzero UUID")
	}
	switch e.Visibility {
	case VisibilityPublic, VisibilityUnlisted, VisibilityPrivate:
	default:
		return invalid("invalid visibility")
	}
	return nil
}

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) Create(ctx context.Context, in CreateEventInput) (*Event, error) {
	id, err := organizer(ctx)
	if err != nil {
		return nil, err
	}
	e := &Event{
		OrganizerID: strings.ToLower(id), Title: strings.TrimSpace(in.Title),
		Description: in.Description, Category: in.Category,
		StartsAt: in.StartsAt, EndsAt: in.EndsAt, VenueID: in.VenueID,
		Visibility: in.Visibility, Capacity: in.Capacity,
		Status: StatusDraft, CreatedAt: time.Now().UTC(),
	}
	if e.Visibility == "" {
		e.Visibility = VisibilityPublic
	}
	if err = validate(e); err != nil {
		return nil, err
	}
	e.ID, err = newID()
	if err != nil {
		return nil, err
	}
	if err = s.repo.Create(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}
func (s *Service) Get(ctx context.Context, id string) (*Event, error) {
	if !validID(id) {
		return nil, invalid("eventId must be a nonzero UUID")
	}
	e, err := s.repo.Get(ctx, strings.ToLower(id))
	if err != nil {
		return nil, err
	}
	if !owner(ctx, e) && !(e.Status == StatusPublished && (e.Visibility == VisibilityPublic || e.Visibility == VisibilityUnlisted)) {
		return nil, ErrNotFound
	}
	return e, nil
}
func (s *Service) List(ctx context.Context, status *Status) ([]*Event, error) {
	p, ok := auth.Current(ctx)
	if !ok || !validID(p.ID) || !p.EmailVerified {
		return nil, ErrUnauthorized
	}
	if status != nil && *status != StatusDraft && *status != StatusPublished && *status != StatusCancelled {
		return nil, invalid("invalid status")
	}
	id, _ := organizer(ctx)
	return s.repo.List(ctx, strings.ToLower(id), status)
}
func (s *Service) mutate(ctx context.Context, id string, change func(*Event) error) (*Event, error) {
	if _, err := organizer(ctx); err != nil {
		return nil, err
	}
	if !validID(id) {
		return nil, invalid("eventId must be a nonzero UUID")
	}
	return s.repo.Mutate(ctx, strings.ToLower(id), func(e *Event) error {
		if !owner(ctx, e) {
			return ErrForbidden
		}
		if e.Status != StatusDraft {
			return ErrConflict
		}
		return change(e)
	})
}
func (s *Service) Update(ctx context.Context, id string, in UpdateEventInput) (*Event, error) {
	return s.mutate(ctx, id, func(e *Event) error {
		if in.Title == nil && in.Description == nil && in.Category == nil && in.StartsAt == nil && in.EndsAt == nil && in.VenueID == nil && in.Visibility == nil && in.Capacity == nil {
			return invalid("provide at least one field")
		}
		if in.Title != nil {
			e.Title = strings.TrimSpace(*in.Title)
		}
		if in.Description != nil {
			e.Description = in.Description
		}
		if in.Category != nil {
			e.Category = in.Category
		}
		if in.StartsAt != nil {
			e.StartsAt = *in.StartsAt
		}
		if in.EndsAt != nil {
			e.EndsAt = in.EndsAt
		}
		if in.VenueID != nil {
			e.VenueID = in.VenueID
		}
		if in.Visibility != nil {
			e.Visibility = *in.Visibility
		}
		if in.Capacity != nil {
			e.Capacity = in.Capacity
		}
		return validate(e)
	})
}
func (s *Service) Publish(ctx context.Context, id string) (*Event, error) {
	return s.mutate(ctx, id, func(e *Event) error {
		if err := validate(e); err != nil {
			return err
		}
		if e.VenueID == nil {
			return invalid("venueId is required to publish a physical event")
		}
		e.Status = StatusPublished
		return nil
	})
}
