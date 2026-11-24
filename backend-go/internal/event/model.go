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
// OrganizerID references a Python identity user; only the UUID is stored here.
type Event struct {
	ID                   string     `json:"id"`
	OrganizerID          string     `json:"organizerId"`
	Title                string     `json:"title"`
	Description          *string    `json:"description,omitempty"`
	Category             *string    `json:"category,omitempty"`
	ImageURL             *string    `json:"imageUrl,omitempty"`
	StartsAt             time.Time  `json:"startsAt"`
	EndsAt               *time.Time `json:"endsAt,omitempty"`
	Timezone             string     `json:"timezone"`
	VenueID              *string    `json:"venueId,omitempty"`
	VenueName            *string    `json:"venueName,omitempty"`
	VenueAddress         *string    `json:"venueAddress,omitempty"`
	City                 *string    `json:"city,omitempty"`
	Visibility           Visibility `json:"visibility"`
	Capacity             *int       `json:"capacity,omitempty"`
	Status               Status     `json:"status"`
	RegistrationOpensAt  *time.Time `json:"registrationOpensAt,omitempty"`
	RegistrationClosesAt *time.Time `json:"registrationClosesAt,omitempty"`
	RefundPolicy         string     `json:"refundPolicy"`
	PaidSalesStatus      string     `json:"paidSalesStatus"`
	Suspended            bool       `json:"suspended"`
	SuspensionReason     *string    `json:"suspensionReason,omitempty"`
	DuplicatedFrom       *string    `json:"duplicatedFrom,omitempty"`
	PublishedAt          *time.Time `json:"publishedAt,omitempty"`
	CancelledAt          *time.Time `json:"cancelledAt,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	// Lifecycle is derived: draft, upcoming, active, completed or cancelled (SRS §4.16).
	Lifecycle string `json:"lifecycle"`
	// Listing summary, filled for list and detail responses.
	PriceFromKzt *int `json:"priceFromKzt,omitempty"`
	TicketsLeft  *int `json:"ticketsLeft,omitempty"`
	// AccessToken unlocks a private event page; only the owner receives it.
	AccessToken string `json:"accessToken,omitempty"`
}

// CreateEventInput matches the request body of POST /api/events.
type CreateEventInput struct {
	Title                string     `json:"title"`
	Description          *string    `json:"description,omitempty"`
	Category             *string    `json:"category,omitempty"`
	ImageURL             *string    `json:"imageUrl,omitempty"`
	StartsAt             time.Time  `json:"startsAt"`
	EndsAt               *time.Time `json:"endsAt,omitempty"`
	Timezone             *string    `json:"timezone,omitempty"`
	VenueID              *string    `json:"venueId,omitempty"`
	VenueName            *string    `json:"venueName,omitempty"`
	VenueAddress         *string    `json:"venueAddress,omitempty"`
	City                 *string    `json:"city,omitempty"`
	Visibility           Visibility `json:"visibility,omitempty"`
	Capacity             *int       `json:"capacity,omitempty"`
	RegistrationOpensAt  *time.Time `json:"registrationOpensAt,omitempty"`
	RegistrationClosesAt *time.Time `json:"registrationClosesAt,omitempty"`
	RefundPolicy         *string    `json:"refundPolicy,omitempty"`
}

// UpdateEventInput is a partial update (PATCH): only supplied fields change.
type UpdateEventInput struct {
	Title                *string     `json:"title,omitempty"`
	Description          *string     `json:"description,omitempty"`
	Category             *string     `json:"category,omitempty"`
	ImageURL             *string     `json:"imageUrl,omitempty"`
	StartsAt             *time.Time  `json:"startsAt,omitempty"`
	EndsAt               *time.Time  `json:"endsAt,omitempty"`
	Timezone             *string     `json:"timezone,omitempty"`
	VenueName            *string     `json:"venueName,omitempty"`
	VenueAddress         *string     `json:"venueAddress,omitempty"`
	City                 *string     `json:"city,omitempty"`
	Visibility           *Visibility `json:"visibility,omitempty"`
	Capacity             *int        `json:"capacity,omitempty"`
	RegistrationOpensAt  *time.Time  `json:"registrationOpensAt,omitempty"`
	RegistrationClosesAt *time.Time  `json:"registrationClosesAt,omitempty"`
	RefundPolicy         *string     `json:"refundPolicy,omitempty"`
}

// TicketType is one kind of ticket for an event (SRS §4.3). Counts are only filled for the owner.
type TicketType struct {
	ID           string     `json:"id"`
	EventID      string     `json:"eventId"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	PriceKzt     int        `json:"priceKzt"`
	Quantity     int        `json:"quantity"`
	SalesStartAt *time.Time `json:"salesStartAt,omitempty"`
	SalesEndAt   *time.Time `json:"salesEndAt,omitempty"`
	MaxPerOrder  int        `json:"maxPerOrder"`
	Hidden       bool       `json:"hidden"`
	SortOrder    int        `json:"sortOrder"`
	Available    int        `json:"available"`
	Counts       *Counts    `json:"counts,omitempty"`
}

// Counts reports inventory states for one ticket type.
type Counts struct {
	Reserved  int `json:"reserved"`
	Sold      int `json:"sold"`
	Refunded  int `json:"refunded"`
	CheckedIn int `json:"checkedIn"`
}

type TicketTypeInput struct {
	Name         *string    `json:"name,omitempty"`
	Description  *string    `json:"description,omitempty"`
	PriceKzt     *int       `json:"priceKzt,omitempty"`
	Quantity     *int       `json:"quantity,omitempty"`
	SalesStartAt *time.Time `json:"salesStartAt,omitempty"`
	SalesEndAt   *time.Time `json:"salesEndAt,omitempty"`
	MaxPerOrder  *int       `json:"maxPerOrder,omitempty"`
	Hidden       *bool      `json:"hidden,omitempty"`
	SortOrder    *int       `json:"sortOrder,omitempty"`
}

// StaffMember is an Event Admin assigned to one event (SRS §2.3, §4.8).
type StaffMember struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Email       string    `json:"email"`
	DisplayName *string   `json:"displayName"`
	CanUndo     bool      `json:"canUndo"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ListFilter narrows the event listing.
type ListFilter struct {
	Status   *Status
	Query    string
	Category string
	City     string
	From     *time.Time
	To       *time.Time
	Mine     bool
}
