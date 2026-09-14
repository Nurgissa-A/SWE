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
// TicketTypeIDs is a placeholder for the future Ticket Type relation
// (SRS 4.3); ticket types aren't modeled yet, this just reserves the
// shape so Week 3-4 doesn't require restructuring Event.
type Event struct {
	ID            string     `json:"id"`
	OrganizerID   string     `json:"organizerId"`
	Title         string     `json:"title"`
	Description   *string    `json:"description,omitempty"`
	Category      *string    `json:"category,omitempty"`
	StartsAt      time.Time  `json:"startsAt"`
	EndsAt        *time.Time `json:"endsAt,omitempty"`
	VenueID       *string    `json:"venueId,omitempty"`
	Visibility    Visibility `json:"visibility"`
	Capacity      *int       `json:"capacity,omitempty"`
	Status        Status     `json:"status"`
	CreatedAt     time.Time  `json:"createdAt"`
	TicketTypeIDs []string   `json:"-"` // reserved for future Ticket Type relation, not serialized yet
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
