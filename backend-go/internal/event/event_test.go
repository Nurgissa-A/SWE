package event_test

import (
	"testing"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/testutil"
)

// These keep the original Week 1–4 event tests (creation, publishing, filtering, ownership,
// visibility, forged input) and run them against PostgreSQL instead of the in-memory store.

func TestCreateEventSuccessAndValidation(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org One")
	var got struct{ ID, Status, OrganizerID, Lifecycle string }
	env.Must(201, "POST", "/api/events", map[string]any{"title": "Test Concert", "startsAt": "2031-01-01T10:00:00Z"}, org).JSON(&got)
	if got.Status != "draft" || got.ID == "" || got.OrganizerID != org.ID || got.Lifecycle != "draft" {
		t.Fatalf("unexpected event %+v", got)
	}
	if res := env.Do("POST", "/api/events", map[string]any{"startsAt": "2031-01-01T10:00:00Z"}, org); res.Code != 400 {
		t.Fatalf("missing title: %d", res.Code)
	}
	if res := env.Do("POST", "/api/events", map[string]any{"title": "x", "startsAt": "2031-01-01T10:00:00Z", "endsAt": "2030-01-01T10:00:00Z"}, org); res.ErrorCode() != "END_BEFORE_START" {
		t.Fatalf("end before start accepted: %s", res.Body.String())
	}
	if res := env.Do("GET", "/api/events/00000000-0000-4000-8000-00000000abcd", nil, org); res.Code != 404 {
		t.Fatalf("unknown event: %d", res.Code)
	}
	if res := env.Do("GET", "/api/events/not-a-uuid", nil, org); res.Code != 404 {
		t.Fatalf("malformed id: %d", res.Code)
	}
}

func TestPublishRequiresVenueAndTicketType(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	var ev struct{ ID string }
	env.Must(201, "POST", "/api/events", map[string]any{"title": "Draft", "startsAt": "2031-01-01T10:00:00Z"}, org).JSON(&ev)
	if code := env.Do("POST", "/api/events/"+ev.ID+"/publish", nil, org).ErrorCode(); code != "VENUE_REQUIRED" {
		t.Fatalf("published without venue: %s", code)
	}
	env.Must(200, "PATCH", "/api/events/"+ev.ID, map[string]any{"venueName": "Hall", "city": "Almaty"}, org)
	if code := env.Do("POST", "/api/events/"+ev.ID+"/publish", nil, org).ErrorCode(); code != "TICKET_TYPE_REQUIRED" {
		t.Fatalf("published without ticket type: %s", code)
	}
	env.Must(201, "POST", "/api/events/"+ev.ID+"/ticket-types", map[string]any{"name": "Free", "quantity": 10}, org)
	unverified := org
	unverified.EmailVerified = false
	if code := env.Do("POST", "/api/events/"+ev.ID+"/publish", nil, unverified).ErrorCode(); code != "EMAIL_NOT_VERIFIED" {
		t.Fatalf("unverified organizer published: %s", code)
	}
	var published struct{ Status, Lifecycle string }
	env.Must(200, "POST", "/api/events/"+ev.ID+"/publish", nil, org).JSON(&published)
	if published.Status != "published" || published.Lifecycle != "upcoming" {
		t.Fatalf("publish: %+v", published)
	}
}

func TestListFilterByStatusAndPublicCatalogue(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	other := env.User("attendee", "Guest")
	pubID, _ := env.PublishedEvent(org, "Jazz Night", testutil.EventOptions{})
	env.Must(201, "POST", "/api/events", map[string]any{"title": "Secret draft", "startsAt": "2031-01-01T10:00:00Z"}, org)

	var drafts []struct{ ID, Status string }
	env.Must(200, "GET", "/api/events?status=draft&mine=true", nil, org).JSON(&drafts)
	if len(drafts) != 1 || drafts[0].Status != "draft" {
		t.Fatalf("draft filter: %+v", drafts)
	}
	for _, viewer := range []auth.Principal{other, {}} {
		var rows []struct{ ID string }
		env.Must(200, "GET", "/api/events", nil, viewer).JSON(&rows)
		if len(rows) != 1 || rows[0].ID != pubID {
			t.Fatalf("public catalogue for %q: %+v", viewer.Role, rows)
		}
	}
	var search []struct{ ID string }
	env.Must(200, "GET", "/api/events?q=jazz&city=Astana", nil, auth.Principal{}).JSON(&search)
	if len(search) != 1 {
		t.Fatalf("search: %+v", search)
	}
	env.Must(200, "GET", "/api/events?q=opera", nil, auth.Principal{}).JSON(&search)
	if len(search) != 0 {
		t.Fatalf("search matched wrong event: %+v", search)
	}
}

func TestEventAuthorizationAndVisibility(t *testing.T) {
	env := testutil.New(t)
	owner := env.User("organizer", "Owner")
	other := env.User("organizer", "Other")
	var e struct{ ID, OrganizerID, AccessToken string }
	env.Must(201, "POST", "/api/events", map[string]any{"title": "Private draft", "startsAt": "2031-01-01T10:00:00Z", "visibility": "private",
		"venueName": "Hall", "city": "Astana"}, owner).JSON(&e)
	if e.OrganizerID != owner.ID {
		t.Fatal("organizer must be authenticated subject")
	}
	env.Must(201, "POST", "/api/events/"+e.ID+"/ticket-types", map[string]any{"name": "Free", "quantity": 10}, owner)
	if env.Do("POST", "/api/events/"+e.ID+"/publish", nil, other).Code != 403 {
		t.Fatal("non-owner publish")
	}
	if env.Do("GET", "/api/events/"+e.ID, nil, other).Code != 404 {
		t.Fatal("draft leaked")
	}
	env.Must(200, "POST", "/api/events/"+e.ID+"/publish", nil, owner)
	if env.Do("GET", "/api/events/"+e.ID, nil, auth.Principal{}).Code != 404 {
		t.Fatal("private published event leaked")
	}
	var detail struct{ AccessToken string }
	env.Must(200, "GET", "/api/events/"+e.ID+"?key="+e.AccessToken, nil, auth.Principal{}).JSON(&detail)
	if detail.AccessToken != "" {
		t.Fatal("access token shown to a visitor")
	}
	var rows []struct{ ID string }
	env.Must(200, "GET", "/api/events", nil, other).JSON(&rows)
	if len(rows) != 0 {
		t.Fatal("list leaked private event")
	}
	publicID, _ := env.PublishedEvent(owner, "Public", testutil.EventOptions{})
	env.Must(200, "GET", "/api/events/"+publicID, nil, auth.Principal{})
}

func TestNonOrganizerCannotCreateOrPublish(t *testing.T) {
	env := testutil.New(t)
	body := map[string]any{"title": "Event", "startsAt": "2031-01-01T10:00:00Z"}
	if env.Do("POST", "/api/events", body, auth.Principal{}).Code != 401 {
		t.Fatal("anonymous create")
	}
	for _, role := range []string{"attendee", "event_admin", "platform_admin"} {
		p := env.User(role, "Someone")
		if env.Do("POST", "/api/events", body, p).Code != 403 {
			t.Fatal("privileged event role inferred incorrectly")
		}
		if env.Do("POST", "/api/events/00000000-0000-4000-8000-000000000001/publish", nil, p).Code != 403 {
			t.Fatal("global publish granted")
		}
	}
}

func TestCreateRejectsSuppliedOrganizerAndTrailingJSON(t *testing.T) {
	env := testutil.New(t)
	owner := env.User("organizer", "Owner")
	for _, body := range []string{`{"title":"x","startsAt":"2030-01-01T10:00:00Z","organizerId":"forged"}`, `{"title":"x","startsAt":"2030-01-01T10:00:00Z"} {}`} {
		if env.Do("POST", "/api/events", body, owner).Code != 400 {
			t.Fatal("forged/trailing input accepted")
		}
	}
}

func TestUnpublishDuplicateAndDelete(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	buyer := env.User("attendee", "Buyer")
	id, types := env.PublishedEvent(org, "Workshop", testutil.EventOptions{Types: []map[string]any{{"name": "Seat", "quantity": 20, "maxPerOrder": 4}}})

	var copyEv struct {
		ID, Title, Status string
		DuplicatedFrom    *string
	}
	env.Must(201, "POST", "/api/events/"+id+"/duplicate", nil, org).JSON(&copyEv)
	if copyEv.Status != "draft" || copyEv.DuplicatedFrom == nil || *copyEv.DuplicatedFrom != id || copyEv.Title != "Workshop (copy)" {
		t.Fatalf("duplicate: %+v", copyEv)
	}
	var copyTypes []struct{ Name string }
	env.Must(200, "GET", "/api/events/"+copyEv.ID+"/ticket-types", nil, org).JSON(&copyTypes)
	if len(copyTypes) != 1 || copyTypes[0].Name != "Seat" {
		t.Fatalf("ticket types not copied: %+v", copyTypes)
	}
	env.Must(204, "DELETE", "/api/events/"+copyEv.ID, nil, org)
	env.Must(404, "GET", "/api/events/"+copyEv.ID, nil, org)

	env.Buy(buyer, id, map[string]int{types[0]: 1}, nil)
	if code := env.Do("POST", "/api/events/"+id+"/unpublish", nil, org).ErrorCode(); code != "HAS_ORDERS" {
		t.Fatalf("unpublished an event with orders: %s", code)
	}
	if code := env.Do("DELETE", "/api/events/"+id, nil, org).ErrorCode(); code != "NOT_DRAFT" {
		t.Fatalf("deleted a published event: %s", code)
	}
	if code := env.Do("DELETE", "/api/events/"+id+"/ticket-types/"+types[0], nil, org).ErrorCode(); code != "TICKET_TYPE_ORDERED" {
		t.Fatalf("deleted an ordered ticket type: %s", code)
	}
	if code := env.Do("PATCH", "/api/events/"+id+"/ticket-types/"+types[0], map[string]any{"priceKzt": 500}, org).ErrorCode(); code != "PRICE_LOCKED" {
		t.Fatalf("changed price after sales: %s", code)
	}
	if code := env.Do("PATCH", "/api/events/"+id+"/ticket-types/"+types[0], map[string]any{"quantity": 0}, org).ErrorCode(); code != "QUANTITY_BELOW_SOLD" {
		t.Fatalf("quantity below sold: %s", code)
	}
	env.Must(200, "PATCH", "/api/events/"+id+"/ticket-types/"+types[0], map[string]any{"hidden": true}, org)
	var visible []any
	env.Must(200, "GET", "/api/events/"+id+"/ticket-types", nil, buyer).JSON(&visible)
	if len(visible) != 0 {
		t.Fatal("hidden ticket type visible to attendees")
	}
}

func TestEditingDateNotifiesTicketHolders(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	buyer := env.User("attendee", "Holder")
	id, types := env.PublishedEvent(org, "Concert", testutil.EventOptions{})
	env.Buy(buyer, id, map[string]int{types[0]: 1}, nil)
	env.Must(200, "PATCH", "/api/events/"+id, map[string]any{"description": "Now with more music"}, org)
	var before struct{ Items []struct{ Type string } }
	env.Must(200, "GET", "/api/notifications", nil, buyer).JSON(&before)
	for _, n := range before.Items {
		if n.Type == "event_updated" {
			t.Fatal("description change should not notify")
		}
	}
	env.Must(200, "PATCH", "/api/events/"+id, map[string]any{"startsAt": "2031-06-02T14:00:00Z", "endsAt": "2031-06-02T18:00:00Z"}, org)
	var after struct {
		Items []struct{ Type, Title string }
	}
	env.Must(200, "GET", "/api/notifications", nil, buyer).JSON(&after)
	if len(after.Items) == 0 || after.Items[0].Type != "event_updated" || after.Items[0].Title != "Event updated: Concert" {
		t.Fatalf("holder not notified: %+v", after.Items)
	}
	var timeline []struct{ Action, Description string }
	env.Must(200, "GET", "/api/events/"+id+"/timeline?type=event", nil, org).JSON(&timeline)
	if len(timeline) < 3 || timeline[0].Description != "Date, time or venue changed" {
		t.Fatalf("timeline: %+v", timeline)
	}
	if env.Do("GET", "/api/events/"+id+"/timeline", nil, buyer).Code != 403 {
		t.Fatal("attendee read the organizer timeline")
	}
}

func TestStaffAssignment(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	staff := env.User("attendee", "Door Person")
	id, _ := env.PublishedEvent(org, "Gig", testutil.EventOptions{})
	if code := env.Do("POST", "/api/events/"+id+"/staff", map[string]any{"email": "nobody@example.kz"}, org).Code; code != 404 {
		t.Fatalf("unknown e-mail: %d", code)
	}
	var m struct{ UserID, Email string }
	env.Must(201, "POST", "/api/events/"+id+"/staff", map[string]any{"email": staff.Email, "canUndo": false}, org).JSON(&m)
	if m.UserID != staff.ID {
		t.Fatalf("staff: %+v", m)
	}
	delete(env.Identity, staff.Email)
	env.Must(404, "POST", "/api/events/"+id+"/staff", map[string]any{"email": staff.Email}, org)
	var list []struct{ ID, Email string }
	env.Must(200, "GET", "/api/events/"+id+"/staff", nil, org).JSON(&list)
	if len(list) != 1 || list[0].Email != staff.Email {
		t.Fatalf("staff list: %+v", list)
	}
	var events []struct{ ID, Role string }
	env.Must(200, "GET", "/api/scanner/events", nil, staff).JSON(&events)
	if len(events) != 1 || events[0].ID != id || events[0].Role != "event_admin" {
		t.Fatalf("assigned events: %+v", events)
	}
	env.Must(204, "DELETE", "/api/events/"+id+"/staff/"+list[0].ID, nil, org)
	env.Must(404, "GET", "/api/scanner/events/"+id, nil, staff)
}
