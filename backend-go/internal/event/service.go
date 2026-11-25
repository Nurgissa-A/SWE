package event

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/notify"
	"github.com/biletflow/backend-go/internal/platform"
)

// ErrValidation is kept for callers that only need to know input was rejected.
var ErrValidation = errors.New("validation failed")

// IdentityLookup resolves a verified user by e-mail through the Python identity service,
// authorised with the caller's own bearer token.
type IdentityLookup func(ctx context.Context, bearer, email string) (platform.DirectoryUser, error)

// ErrUserNotFound is returned by IdentityLookup when no verified account has that e-mail.
var ErrUserNotFound = errors.New("user not found")

type Service struct {
	repo   *Repository
	lookup IdentityLookup
}

func NewService(repo *Repository, lookup IdentityLookup) *Service {
	return &Service{repo: repo, lookup: lookup}
}

func (s *Service) Repo() *Repository { return s.repo }

func trimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return &v
}

func validTimezone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil && tz != "" && tz != "Local"
}

func validate(e *Event) error {
	switch {
	case strings.TrimSpace(e.Title) == "" || len(e.Title) > 200:
		return platform.BadRequest("TITLE_REQUIRED", "title and startsAt are required")
	case e.StartsAt.IsZero():
		return platform.BadRequest("TITLE_REQUIRED", "title and startsAt are required")
	case e.EndsAt != nil && !e.EndsAt.After(e.StartsAt):
		return platform.BadRequest("END_BEFORE_START", "The event must end after it starts")
	case e.Visibility != VisibilityPublic && e.Visibility != VisibilityUnlisted && e.Visibility != VisibilityPrivate:
		return platform.BadRequest("INVALID_VISIBILITY", "Visibility must be public, unlisted or private")
	case e.Capacity != nil && *e.Capacity <= 0:
		return platform.BadRequest("INVALID_CAPACITY", "Capacity must be a positive number")
	case !validTimezone(e.Timezone):
		return platform.BadRequest("INVALID_TIMEZONE", "Unknown time zone")
	case e.RegistrationOpensAt != nil && e.RegistrationClosesAt != nil && !e.RegistrationClosesAt.After(*e.RegistrationOpensAt):
		return platform.BadRequest("REGISTRATION_WINDOW", "Registration must close after it opens")
	case e.ImageURL != nil && !strings.HasPrefix(*e.ImageURL, "https://") && !strings.HasPrefix(*e.ImageURL, "/"):
		return platform.BadRequest("INVALID_IMAGE_URL", "The image must be an https:// address")
	}
	return nil
}

func requireOrganizer(p auth.Principal) error {
	if p.ID == "" {
		return platform.ErrUnauthenticated
	}
	if p.Role != "organizer" {
		return platform.Forbidden("ORGANIZER_REQUIRED", "Organizer role required")
	}
	return nil
}

// Create validates input and stores a new draft event owned by the authenticated organizer.
func (s *Service) Create(ctx context.Context, p auth.Principal, in CreateEventInput) (*Event, error) {
	if err := requireOrganizer(p); err != nil {
		return nil, err
	}
	e := &Event{OrganizerID: p.ID, Title: strings.TrimSpace(in.Title), Description: trimPtr(in.Description), Category: trimPtr(in.Category),
		ImageURL: trimPtr(in.ImageURL), StartsAt: in.StartsAt.UTC(), EndsAt: in.EndsAt, Timezone: "Asia/Almaty", VenueID: in.VenueID,
		VenueName: trimPtr(in.VenueName), VenueAddress: trimPtr(in.VenueAddress), City: trimPtr(in.City), Visibility: in.Visibility,
		Capacity: in.Capacity, RegistrationOpensAt: in.RegistrationOpensAt, RegistrationClosesAt: in.RegistrationClosesAt,
		RefundPolicy: "No refunds unless the event is cancelled.", Status: StatusDraft}
	if in.Timezone != nil {
		e.Timezone = *in.Timezone
	}
	if e.Visibility == "" {
		e.Visibility = VisibilityPublic
	}
	if v := trimPtr(in.RefundPolicy); v != nil {
		e.RefundPolicy = *v
	}
	if e.VenueID != nil && !platform.IsUUID(*e.VenueID) {
		return nil, platform.BadRequest("INVALID_VENUE", "venueId must be a UUID")
	}
	if err := validate(e); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, e); err != nil {
		return nil, err
	}
	if err := platform.Audit(ctx, s.repo.DB, platform.AuditEntry{EventID: e.ID, ActorID: p.ID, Action: "event.created",
		EntityType: "event", EntityID: e.ID, Description: "Event created as a draft"}); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, e.ID)
}

// CanView applies SRS visibility rules: owners and platform admins see everything; others see
// published, unsuspended public/unlisted events, or a private one with its access key.
func CanView(p auth.Principal, e *Event, key string) bool {
	if (p.Role == "organizer" && p.ID == e.OrganizerID) || p.Role == "platform_admin" {
		return true
	}
	if e.Status == StatusDraft || e.Suspended {
		return false
	}
	switch e.Visibility {
	case VisibilityPublic, VisibilityUnlisted:
		return true
	default:
		return key != "" && key == e.AccessToken
	}
}

// Public strips owner-only fields.
func Public(p auth.Principal, e *Event) *Event {
	if p.ID == e.OrganizerID || p.Role == "platform_admin" {
		return e
	}
	c := *e
	c.AccessToken = ""
	return &c
}

func (s *Service) Get(ctx context.Context, p auth.Principal, id, key string) (*Event, error) {
	e, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && !CanView(p, e, key)) {
		return nil, platform.NotFound("Event not found")
	}
	if err != nil {
		return nil, err
	}
	return Public(p, e), nil
}

func (s *Service) List(ctx context.Context, p auth.Principal, f ListFilter) ([]*Event, error) {
	if f.Mine && p.Role != "organizer" {
		return []*Event{}, nil
	}
	ownerID := ""
	if p.Role == "organizer" {
		ownerID = p.ID
	}
	// mine=true lists the organizer's own events; otherwise the public catalogue (plus, for
	// organizers, their own events, as before). Platform admins use the admin endpoints.
	events, err := s.repo.List(ctx, f, ownerID, !f.Mine)
	if err != nil {
		return nil, err
	}
	out := make([]*Event, 0, len(events))
	for _, e := range events {
		out = append(out, Public(p, e))
	}
	return out, nil
}

// Managed loads an event the caller may manage. Non-organizers get 403 before any lookup,
// unknown events 404 and other organizers' events 403.
func (s *Service) Managed(ctx context.Context, p auth.Principal, id string) (*Event, error) {
	if err := requireOrganizer(p); err != nil {
		return nil, err
	}
	e, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, platform.NotFound("Event not found")
	}
	if err != nil {
		return nil, err
	}
	if e.OrganizerID != p.ID {
		return nil, platform.Forbidden("NOT_EVENT_OWNER", "Event owner required")
	}
	return e, nil
}

func (s *Service) Update(ctx context.Context, p auth.Principal, id string, in UpdateEventInput) (*Event, error) {
	e, err := s.Managed(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if e.Status == StatusCancelled {
		return nil, platform.Conflict("EVENT_CANCELLED", "A cancelled event cannot be edited")
	}
	before := *e
	set := func(dst **string, v *string) {
		if v != nil {
			*dst = trimPtr(v)
		}
	}
	if in.Title != nil {
		e.Title = strings.TrimSpace(*in.Title)
	}
	set(&e.Description, in.Description)
	set(&e.Category, in.Category)
	set(&e.ImageURL, in.ImageURL)
	set(&e.VenueName, in.VenueName)
	set(&e.VenueAddress, in.VenueAddress)
	set(&e.City, in.City)
	if in.StartsAt != nil {
		e.StartsAt = in.StartsAt.UTC()
	}
	if in.EndsAt != nil {
		e.EndsAt = in.EndsAt
	}
	if in.Timezone != nil {
		e.Timezone = *in.Timezone
	}
	if in.Visibility != nil {
		e.Visibility = *in.Visibility
	}
	if in.Capacity != nil {
		e.Capacity = in.Capacity
	}
	if in.RegistrationOpensAt != nil {
		e.RegistrationOpensAt = in.RegistrationOpensAt
	}
	if in.RegistrationClosesAt != nil {
		e.RegistrationClosesAt = in.RegistrationClosesAt
	}
	if in.RefundPolicy != nil && trimPtr(in.RefundPolicy) != nil {
		e.RefundPolicy = *trimPtr(in.RefundPolicy)
	}
	if err := validate(e); err != nil {
		return nil, err
	}
	if e.Capacity != nil {
		var used int
		if err := s.repo.DB.QueryRowContext(ctx, `SELECT coalesce(sum(reserved + sold),0) FROM ticket_type_stock WHERE event_id=$1`, id).Scan(&used); err != nil {
			return nil, err
		}
		if *e.Capacity < used {
			return nil, platform.Conflict("CAPACITY_BELOW_SOLD", "Capacity cannot be lower than the {n} tickets already sold or reserved", "n", used)
		}
	}
	changed := !before.StartsAt.Equal(e.StartsAt) || !sameTime(before.EndsAt, e.EndsAt) || deref(before.VenueName) != deref(e.VenueName) ||
		deref(before.VenueAddress) != deref(e.VenueAddress) || deref(before.City) != deref(e.City)
	err = platform.InTx(ctx, s.repo.DB, func(tx *sql.Tx) error {
		if err := s.repo.Update(ctx, tx, e); err != nil {
			return err
		}
		desc := "Event details updated"
		if changed {
			desc = "Date, time or venue changed"
		}
		if err := platform.Audit(ctx, tx, platform.AuditEntry{EventID: id, ActorID: p.ID, Action: "event.updated", EntityType: "event", EntityID: id, Description: desc}); err != nil {
			return err
		}
		if changed && e.Status == StatusPublished {
			holders, err := s.repo.TicketHolders(ctx, tx, id)
			if err != nil {
				return err
			}
			for _, u := range holders {
				if err := notify.Queue(ctx, tx, notify.Message{UserID: u, Type: "event_updated", Params: map[string]any{"event": e.Title}, Link: "/events/" + id, Email: true}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Publish requires a verified organizer, a venue and at least one ticket type (SRS §4.2).
func (s *Service) Publish(ctx context.Context, p auth.Principal, id string) (*Event, error) {
	e, err := s.Managed(ctx, p, id)
	if err != nil {
		return nil, err
	}
	switch {
	case e.Status == StatusCancelled:
		return nil, platform.Conflict("EVENT_CANCELLED", "A cancelled event cannot be published")
	case e.Status == StatusPublished:
		return e, nil
	case !p.EmailVerified:
		return nil, platform.Forbidden("EMAIL_NOT_VERIFIED", "Verify your e-mail address before publishing")
	case e.VenueName == nil || e.City == nil:
		return nil, platform.BadRequest("VENUE_REQUIRED", "Add the venue name and city before publishing")
	case e.Suspended:
		return nil, platform.Forbidden("EVENT_SUSPENDED", "This event was suspended by BiletFlow moderation")
	}
	types, err := s.repo.TicketTypes(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(types) == 0 {
		return nil, platform.BadRequest("TICKET_TYPE_REQUIRED", "Add at least one ticket type before publishing")
	}
	err = platform.InTx(ctx, s.repo.DB, func(tx *sql.Tx) error {
		if err := s.repo.SetStatus(ctx, tx, id, StatusPublished); err != nil {
			return err
		}
		return platform.Audit(ctx, tx, platform.AuditEntry{EventID: id, ActorID: p.ID, Action: "event.published", EntityType: "event", EntityID: id, Description: "Event published"})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

// Unpublish returns a published event to draft; once tickets exist the organizer must cancel instead.
func (s *Service) Unpublish(ctx context.Context, p auth.Principal, id string) (*Event, error) {
	e, err := s.Managed(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if e.Status != StatusPublished {
		return nil, platform.Conflict("NOT_PUBLISHED", "Only a published event can be unpublished")
	}
	n, err := s.repo.CountOrders(ctx, id, "pending", "completed")
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, platform.Conflict("HAS_ORDERS", "This event already has orders; cancel it instead so attendees are refunded and notified")
	}
	err = platform.InTx(ctx, s.repo.DB, func(tx *sql.Tx) error {
		if err := s.repo.SetStatus(ctx, tx, id, StatusDraft); err != nil {
			return err
		}
		return platform.Audit(ctx, tx, platform.AuditEntry{EventID: id, ActorID: p.ID, Action: "event.unpublished", EntityType: "event", EntityID: id, Description: "Event unpublished"})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

// Delete removes a draft that never had orders.
func (s *Service) Delete(ctx context.Context, p auth.Principal, id string) error {
	e, err := s.Managed(ctx, p, id)
	if err != nil {
		return err
	}
	if e.Status != StatusDraft {
		return platform.Conflict("NOT_DRAFT", "Only drafts can be deleted; cancel a published event instead")
	}
	if n, err := s.repo.CountOrders(ctx, id, "pending", "completed", "failed", "expired", "cancelled", "refunded"); err != nil || n > 0 {
		if err != nil {
			return err
		}
		return platform.Conflict("HAS_ORDERS", "An event with order history cannot be deleted")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return platform.Audit(ctx, s.repo.DB, platform.AuditEntry{EventID: id, ActorID: p.ID, Action: "event.deleted", EntityType: "event", EntityID: id, Description: "Draft deleted: " + e.Title})
}

// Duplicate copies event details and ticket types into a new draft, without orders,
// tickets, campaigns or history (SRS §4.16).
func (s *Service) Duplicate(ctx context.Context, p auth.Principal, id string) (*Event, error) {
	src, err := s.Managed(ctx, p, id)
	if err != nil {
		return nil, err
	}
	types, err := s.repo.TicketTypes(ctx, id)
	if err != nil {
		return nil, err
	}
	c := *src
	c.Title = src.Title + " (copy)"
	c.DuplicatedFrom = &src.ID
	c.StartsAt = src.StartsAt.AddDate(0, 0, 7)
	if src.EndsAt != nil {
		end := src.EndsAt.AddDate(0, 0, 7)
		c.EndsAt = &end
	}
	c.RegistrationOpensAt, c.RegistrationClosesAt = nil, nil
	if err := s.repo.Create(ctx, &c); err != nil {
		return nil, err
	}
	err = platform.InTx(ctx, s.repo.DB, func(tx *sql.Tx) error {
		for _, t := range types {
			copyType := *t
			copyType.EventID = c.ID
			copyType.SalesStartAt, copyType.SalesEndAt = nil, nil
			if err := s.repo.CreateTicketType(ctx, tx, &copyType); err != nil {
				return err
			}
		}
		return platform.Audit(ctx, tx, platform.AuditEntry{EventID: c.ID, ActorID: p.ID, Action: "event.duplicated", EntityType: "event", EntityID: c.ID,
			Description: "Created as a copy of " + src.Title, Metadata: map[string]any{"sourceEventId": src.ID}})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, c.ID)
}

// ───────────── Ticket types ─────────────

// TicketTypes returns the owner's full view (with counts) or the public, purchasable view.
func (s *Service) TicketTypes(ctx context.Context, p auth.Principal, id, key string) ([]*TicketType, error) {
	e, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && !CanView(p, e, key)) {
		return nil, platform.NotFound("Event not found")
	}
	if err != nil {
		return nil, err
	}
	types, err := s.repo.TicketTypes(ctx, id)
	if err != nil {
		return nil, err
	}
	var used int
	for _, t := range types {
		used += t.Counts.Reserved + t.Counts.Sold
	}
	owner := p.ID == e.OrganizerID || p.Role == "platform_admin"
	out := make([]*TicketType, 0, len(types))
	for _, t := range types {
		if e.Capacity != nil {
			t.Available = min(t.Available, max(*e.Capacity-used, 0))
		}
		if !owner {
			if t.Hidden {
				continue
			}
			t.Counts = nil
		}
		out = append(out, t)
	}
	return out, nil
}

func applyTicketInput(t *TicketType, in TicketTypeInput) error {
	if in.Name != nil {
		t.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		t.Description = strings.TrimSpace(*in.Description)
	}
	if in.PriceKzt != nil {
		t.PriceKzt = *in.PriceKzt
	}
	if in.Quantity != nil {
		t.Quantity = *in.Quantity
	}
	if in.SalesStartAt != nil {
		t.SalesStartAt = in.SalesStartAt
	}
	if in.SalesEndAt != nil {
		t.SalesEndAt = in.SalesEndAt
	}
	if in.MaxPerOrder != nil {
		t.MaxPerOrder = *in.MaxPerOrder
	}
	if in.Hidden != nil {
		t.Hidden = *in.Hidden
	}
	if in.SortOrder != nil {
		t.SortOrder = *in.SortOrder
	}
	switch {
	case t.Name == "" || len(t.Name) > 100:
		return platform.BadRequest("TICKET_NAME_REQUIRED", "Ticket type name is required")
	case t.PriceKzt < 0 || t.PriceKzt > 10_000_000:
		return platform.BadRequest("INVALID_PRICE", "Price must be between 0 and 10,000,000 KZT")
	case t.Quantity < 0 || t.Quantity > 1_000_000:
		return platform.BadRequest("INVALID_QUANTITY", "Quantity must be zero or more")
	case t.MaxPerOrder < 1 || t.MaxPerOrder > 50:
		return platform.BadRequest("INVALID_MAX_PER_ORDER", "Maximum per order must be between 1 and 50")
	case t.SalesStartAt != nil && t.SalesEndAt != nil && !t.SalesEndAt.After(*t.SalesStartAt):
		return platform.BadRequest("SALES_WINDOW", "Sales must end after they start")
	}
	return nil
}

func (s *Service) CreateTicketType(ctx context.Context, p auth.Principal, eventID string, in TicketTypeInput) (*TicketType, error) {
	e, err := s.Managed(ctx, p, eventID)
	if err != nil {
		return nil, err
	}
	if e.Status == StatusCancelled {
		return nil, platform.Conflict("EVENT_CANCELLED", "A cancelled event cannot be edited")
	}
	t := &TicketType{EventID: eventID, MaxPerOrder: 10}
	if err := applyTicketInput(t, in); err != nil {
		return nil, err
	}
	err = platform.InTx(ctx, s.repo.DB, func(tx *sql.Tx) error {
		if err := s.repo.CreateTicketType(ctx, tx, t); err != nil {
			return err
		}
		return platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "ticket_type.created", EntityType: "ticket_type", EntityID: t.ID,
			Description: "Ticket type added: " + t.Name, Metadata: map[string]any{"priceKzt": t.PriceKzt, "quantity": t.Quantity}})
	})
	if err != nil {
		return nil, err
	}
	return s.findType(ctx, eventID, t.ID)
}

func (s *Service) findType(ctx context.Context, eventID, typeID string) (*TicketType, error) {
	types, err := s.repo.TicketTypes(ctx, eventID)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		if t.ID == typeID {
			return t, nil
		}
	}
	return nil, platform.NotFound("Ticket type not found")
}

func (s *Service) UpdateTicketType(ctx context.Context, p auth.Principal, eventID, typeID string, in TicketTypeInput) (*TicketType, error) {
	if _, err := s.Managed(ctx, p, eventID); err != nil {
		return nil, err
	}
	t, err := s.findType(ctx, eventID, typeID)
	if err != nil {
		return nil, err
	}
	used, err := s.repo.TicketTypeOrdered(ctx, typeID)
	if err != nil {
		return nil, err
	}
	if used && in.PriceKzt != nil && *in.PriceKzt != t.PriceKzt {
		return nil, platform.Conflict("PRICE_LOCKED", "The price cannot change after tickets of this type were ordered; add a new ticket type instead")
	}
	if err := applyTicketInput(t, in); err != nil {
		return nil, err
	}
	if t.Quantity < t.Counts.Sold+t.Counts.Reserved {
		return nil, platform.Conflict("QUANTITY_BELOW_SOLD", "Quantity cannot be lower than the {n} tickets already sold or reserved", "n", t.Counts.Sold+t.Counts.Reserved)
	}
	if err := s.repo.UpdateTicketType(ctx, t); err != nil {
		return nil, err
	}
	if err := platform.Audit(ctx, s.repo.DB, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "ticket_type.updated", EntityType: "ticket_type", EntityID: typeID,
		Description: "Ticket type updated: " + t.Name}); err != nil {
		return nil, err
	}
	return s.findType(ctx, eventID, typeID)
}

func (s *Service) DeleteTicketType(ctx context.Context, p auth.Principal, eventID, typeID string) error {
	if _, err := s.Managed(ctx, p, eventID); err != nil {
		return err
	}
	t, err := s.findType(ctx, eventID, typeID)
	if err != nil {
		return err
	}
	used, err := s.repo.TicketTypeOrdered(ctx, typeID)
	if err != nil {
		return err
	}
	if used {
		return platform.Conflict("TICKET_TYPE_ORDERED", "This ticket type has orders; hide it instead of deleting it")
	}
	if err := s.repo.DeleteTicketType(ctx, typeID); err != nil {
		return err
	}
	return platform.Audit(ctx, s.repo.DB, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "ticket_type.deleted", EntityType: "ticket_type", EntityID: typeID,
		Description: "Ticket type removed: " + t.Name})
}

// ───────────── Staff ─────────────

func (s *Service) Staff(ctx context.Context, p auth.Principal, eventID string) ([]StaffMember, error) {
	if _, err := s.Managed(ctx, p, eventID); err != nil {
		return nil, err
	}
	return s.repo.Staff(ctx, eventID)
}

// AddStaff assigns an existing verified account as Event Admin for one event.
func (s *Service) AddStaff(ctx context.Context, r *http.Request, p auth.Principal, eventID, email string, canUndo bool) (*StaffMember, error) {
	e, err := s.Managed(ctx, p, eventID)
	if err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || len(email) > 254 {
		return nil, platform.BadRequest("INVALID_EMAIL", "Enter a valid e-mail address")
	}
	// The local directory can still contain accounts that identity has suspended.
	if s.lookup == nil {
		return nil, platform.Err(http.StatusServiceUnavailable, "IDENTITY_UNAVAILABLE", "Identity service unavailable")
	}
	user, err := s.lookup(ctx, r.Header.Get("Authorization"), email)
	if errors.Is(err, ErrUserNotFound) {
		return nil, platform.NotFound("No verified BiletFlow account uses this e-mail. Ask them to sign up first.")
	}
	if err != nil {
		return nil, platform.Err(http.StatusServiceUnavailable, "IDENTITY_UNAVAILABLE", "Identity service unavailable")
	}
	if user.ID == e.OrganizerID {
		return nil, platform.BadRequest("OWNER_IS_STAFF", "You can already check in guests for your own event")
	}
	// Preserve the role recorded by authentication when an account is already known.
	if _, err := s.repo.DB.ExecContext(ctx, `INSERT INTO user_directory (user_id, email, display_name, locale, role)
		VALUES ($1, $2, $3, $4, 'attendee')
		ON CONFLICT (user_id) DO UPDATE SET email=EXCLUDED.email, display_name=EXCLUDED.display_name,
			locale=EXCLUDED.locale, updated_at=now()`,
		user.ID, user.Email, user.DisplayName, user.Locale); err != nil {
		return nil, err
	}
	var m StaffMember
	err = platform.InTx(ctx, s.repo.DB, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `INSERT INTO staff_assignments(event_id, user_id, can_undo, created_by) VALUES ($1,$2,$3,$4)
			ON CONFLICT (event_id, user_id) DO UPDATE SET can_undo=EXCLUDED.can_undo RETURNING id, created_at`,
			eventID, user.ID, canUndo, p.ID).Scan(&m.ID, &m.CreatedAt)
		if err != nil {
			return err
		}
		if err := platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "staff.assigned", EntityType: "staff", EntityID: user.ID,
			Description: "Event Admin assigned: " + user.Email}); err != nil {
			return err
		}
		return notify.Queue(ctx, tx, notify.Message{UserID: user.ID, Type: "staff_assigned", Params: map[string]any{"event": e.Title, "organizer": nameOf(p)}, Email: true})
	})
	if err != nil {
		return nil, err
	}
	m.UserID, m.Email, m.DisplayName, m.CanUndo = user.ID, user.Email, user.DisplayName, canUndo
	return &m, nil
}

func nameOf(p auth.Principal) string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return p.Email
}

func (s *Service) RemoveStaff(ctx context.Context, p auth.Principal, eventID, staffID string) error {
	if _, err := s.Managed(ctx, p, eventID); err != nil {
		return err
	}
	var userID string
	err := s.repo.DB.QueryRowContext(ctx, `DELETE FROM staff_assignments WHERE id=$1 AND event_id=$2 RETURNING user_id`, staffID, eventID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.NotFound("Staff member not found")
	}
	if err != nil {
		return err
	}
	return platform.Audit(ctx, s.repo.DB, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "staff.removed", EntityType: "staff", EntityID: userID, Description: "Event Admin removed"})
}
