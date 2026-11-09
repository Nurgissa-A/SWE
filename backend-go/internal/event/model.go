package event

import "time"

// Visibility mirrors the `visibility` enum in docs/openapi.yaml.
type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPrivate  Visibility = "private"
)

// Status mirrors the `status` enum in docs/openapi.yaml.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusCancelled Status = "cancelled"
)

// Event matches the Event schema in docs/openapi.yaml.
//
// OrganizerID references a User with role=organizer, owned by Aliar's
// service — we only store the ID here, not the full user record.
//
// Future ticket types will reference Event.ID; no ticket storage exists in Weeks 1–4.
type Event struct {
	ID          string     `json:"id"`
	OrganizerID string     `json:"organizerId"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Category    *string    `json:"category,omitempty"`
	StartsAt    time.Time  `json:"startsAt"`
	EndsAt      *time.Time `json:"endsAt,omitempty"`
	VenueID     *string    `json:"venueId,omitempty"`
	Visibility  Visibility `json:"visibility"`
	Capacity    *int       `json:"capacity,omitempty"`
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// CreateEventInput matches the request body of POST /api/events.
type CreateEventInput struct {
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Category    *string    `json:"category,omitempty"`
	StartsAt    time.Time  `json:"startsAt"`
	EndsAt      *time.Time `json:"endsAt,omitempty"`
	VenueID     *string    `json:"venueId,omitempty"`
	Visibility  Visibility `json:"visibility,omitempty"`
	Capacity    *int       `json:"capacity,omitempty"`
}

// UpdateEventInput preserves omitted fields. JSON null is rejected at the HTTP
// boundary; empty strings can clear optional text, as defined by the contract.
type UpdateEventInput struct {
	Title       *string     `json:"title,omitempty"`
	Description *string     `json:"description,omitempty"`
	Category    *string     `json:"category,omitempty"`
	StartsAt    *time.Time  `json:"startsAt,omitempty"`
	EndsAt      *time.Time  `json:"endsAt,omitempty"`
	VenueID     *string     `json:"venueId,omitempty"`
	Visibility  *Visibility `json:"visibility,omitempty"`
	Capacity    *int        `json:"capacity,omitempty"`
}
