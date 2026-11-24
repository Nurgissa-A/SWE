package support_test

import (
	"testing"

	"github.com/biletflow/backend-go/internal/testutil"
)

type supportCase struct {
	ID, Status, Kind string
	Number           int
	Escalated        bool
	CanManage        bool
	CanEscalate      bool
	OrderNumber      *string
	Messages         []struct{ Role, Body string }
}

func TestAttendeeCaseFlowWithEscalation(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	guest := env.User("attendee", "Guest")
	stranger := env.User("attendee", "Stranger")
	admin := env.User("platform_admin", "BiletFlow Admin")
	env.Must(200, "GET", "/api/notifications", nil, admin) // admin appears in the directory
	id, types := env.PublishedEvent(org, "Concert", testutil.EventOptions{})
	order := env.Buy(guest, id, map[string]int{types[0]: 1}, nil)

	if code := env.Do("POST", "/api/support/cases", map[string]any{"category": "ticket_delivery", "subject": "Where is it", "message": "?", "orderId": order["id"]}, stranger).ErrorCode(); code != "CONTEXT_NOT_YOURS" {
		t.Fatalf("attached someone else's order: %s", code)
	}
	if code := env.Do("POST", "/api/support/cases", map[string]any{"category": "payment", "subject": "No context", "message": "hi"}, guest).ErrorCode(); code != "EVENT_REQUIRED" {
		t.Fatalf("attendee case without event: %s", code)
	}
	var c supportCase
	env.Must(201, "POST", "/api/support/cases", map[string]any{"category": "ticket_delivery", "subject": "PDF missing", "message": "I did not get the PDF", "orderId": order["id"]}, guest).JSON(&c)
	if c.Status != "open" || c.OrderNumber == nil || c.Kind != "attendee" {
		t.Fatalf("case: %+v", c)
	}
	env.Must(404, "GET", "/api/support/cases/"+c.ID, nil, stranger)
	env.Must(404, "GET", "/api/support/cases/"+c.ID, nil, admin) // not escalated yet

	env.Must(200, "GET", "/api/support/cases/"+c.ID, nil, org).JSON(&c)
	if !c.CanManage || !c.CanEscalate {
		t.Fatalf("organizer permissions: %+v", c)
	}
	env.Must(201, "POST", "/api/support/cases/"+c.ID+"/messages", map[string]any{"body": "Resent, please check spam"}, org).JSON(&c)
	if c.Status != "in_progress" || len(c.Messages) != 2 || c.Messages[1].Role != "organizer" {
		t.Fatalf("reply: %+v", c)
	}
	env.Must(200, "PATCH", "/api/support/cases/"+c.ID, map[string]any{"status": "waiting_for_customer"}, org)
	if env.Do("PATCH", "/api/support/cases/"+c.ID, map[string]any{"status": "in_progress"}, guest).Code != 403 {
		t.Fatal("requester changed status")
	}
	env.Must(201, "POST", "/api/support/cases/"+c.ID+"/messages", map[string]any{"body": "Still nothing"}, guest).JSON(&c)
	if c.Status != "open" {
		t.Fatalf("requester reply should reopen: %s", c.Status)
	}
	env.Must(200, "PATCH", "/api/support/cases/"+c.ID, map[string]any{"escalate": true}, org)
	env.Must(200, "GET", "/api/support/cases/"+c.ID, nil, admin).JSON(&c)
	if !c.Escalated || !c.CanManage {
		t.Fatalf("escalation: %+v", c)
	}
	env.Must(200, "PATCH", "/api/support/cases/"+c.ID, map[string]any{"assignToMe": true, "status": "resolved"}, admin)
	var notes struct {
		Items []struct{ Type, Title string }
	}
	env.Must(200, "GET", "/api/notifications", nil, guest).JSON(&notes)
	if notes.Items[0].Type != "support_status" || notes.Items[0].Title == "" {
		t.Fatalf("status notification: %+v", notes.Items)
	}
	var mine []supportCase
	env.Must(200, "GET", "/api/support/cases", nil, stranger).JSON(&mine)
	if len(mine) != 0 {
		t.Fatal("stranger sees cases")
	}
}

func TestOrganizerCaseGoesToPlatformAdmins(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	other := env.User("organizer", "Other Org")
	admin := env.User("platform_admin", "Admin")
	env.Must(200, "GET", "/api/notifications", nil, admin)
	if env.Do("POST", "/api/support/cases", map[string]any{"kind": "organizer", "category": "activation", "subject": "Fee", "message": "help"}, env.User("attendee", "A")).Code != 403 {
		t.Fatal("attendee opened an organizer case")
	}
	var c supportCase
	env.Must(201, "POST", "/api/support/cases", map[string]any{"kind": "organizer", "category": "activation", "subject": "Fee charged twice?", "message": "help"}, org).JSON(&c)
	env.Must(404, "GET", "/api/support/cases/"+c.ID, nil, other)
	env.Must(200, "GET", "/api/support/cases/"+c.ID, nil, admin)
	var notes struct{ Items []struct{ Type string } }
	env.Must(200, "GET", "/api/notifications", nil, admin).JSON(&notes)
	if len(notes.Items) != 1 || notes.Items[0].Type != "support_new_case" {
		t.Fatalf("admin not notified: %+v", notes)
	}
}
