package admin_test

import (
	"strings"
	"testing"

	"github.com/biletflow/backend-go/internal/testutil"
)

func TestModerationAndPaidSalesControl(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	admin := env.User("platform_admin", "Admin")
	guest := env.User("attendee", "Guest")
	id, types := env.PublishedEvent(org, "Festival", testutil.EventOptions{PaidActive: true, Types: []map[string]any{
		{"name": "Paid", "priceKzt": 5000, "quantity": 10}, {"name": "Free", "priceKzt": 0, "quantity": 10}}})
	for _, path := range []string{"/api/admin/summary", "/api/admin/events", "/api/admin/payments", "/api/admin/settings"} {
		if env.Do("GET", path, nil, org).Code != 403 {
			t.Fatalf("organizer reached %s", path)
		}
	}
	env.Must(201, "POST", "/api/events/"+id+"/report", map[string]any{"reason": "Looks like a scam"}, guest)
	var reports []map[string]any
	env.Must(200, "GET", "/api/admin/reports", nil, admin).JSON(&reports)
	if len(reports) != 1 || reports[0]["status"] != "open" {
		t.Fatalf("reports: %v", reports)
	}

	// Suspending paid sales keeps free registration open (SRS §4.5).
	env.Must(204, "POST", "/api/admin/events/"+id+"/paid-sales", map[string]any{"action": "suspend"}, admin)
	body := func(typeID string) map[string]any {
		return map[string]any{"eventId": id, "items": []map[string]any{{"ticketTypeId": typeID, "quantity": 1}}, "buyer": map[string]any{"name": "Guest", "email": guest.Email}}
	}
	if code := env.Do("POST", "/api/checkout/reserve", body(types[0]), guest).ErrorCode(); code != "PAID_SALES_INACTIVE" {
		t.Fatalf("paid sale while suspended: %s", code)
	}
	env.Must(201, "POST", "/api/checkout/reserve", body(types[1]), guest)
	if code := env.Do("POST", "/api/events/"+id+"/activation/activate", nil, org).ErrorCode(); code != "PAID_SALES_SUSPENDED" {
		t.Fatalf("organizer reactivated suspended sales: %s", code)
	}
	env.Must(204, "POST", "/api/admin/events/"+id+"/paid-sales", map[string]any{"action": "resume"}, admin)
	env.Buy(guest, id, map[string]int{types[0]: 1}, nil)

	// Suspending the whole event hides it and stops all sales.
	env.Must(204, "POST", "/api/admin/events/"+id+"/suspend", map[string]any{"reason": "Fraud report confirmed"}, admin)
	env.Must(404, "GET", "/api/events/"+id, nil, guest)
	if code := env.Do("POST", "/api/checkout/reserve", body(types[1]), guest).ErrorCode(); code != "EVENT_SUSPENDED" {
		t.Fatalf("sale on suspended event: %s", code)
	}
	var notes struct{ Items []struct{ Type string } }
	env.Must(200, "GET", "/api/notifications", nil, org).JSON(&notes)
	if notes.Items[0].Type != "event_suspended" {
		t.Fatalf("organizer not told: %+v", notes.Items)
	}
	env.Must(204, "POST", "/api/admin/events/"+id+"/unsuspend", nil, admin)
	env.Must(200, "GET", "/api/events/"+id, nil, guest)

	var payments []map[string]any
	env.Must(200, "GET", "/api/admin/payments", nil, admin).JSON(&payments)
	if len(payments) != 2 || payments[0]["isSimulated"] != true {
		t.Fatalf("payments: %v", payments)
	}
	csv := env.Must(200, "GET", "/api/admin/export/orders.csv", nil, admin).Body.String()
	if !strings.HasPrefix(csv, "number,status,title") || strings.Count(csv, "\n") != 3 {
		t.Fatalf("csv:\n%s", csv)
	}
}

func TestSettingsChangeTheActivationFee(t *testing.T) {
	env := testutil.New(t)
	admin := env.User("platform_admin", "Admin")
	org := env.User("organizer", "Org")
	if code := env.Do("PUT", "/api/admin/settings", map[string]any{"hold_minutes": 500}, admin).ErrorCode(); code != "INVALID_SETTING" {
		t.Fatalf("out-of-range setting accepted: %s", code)
	}
	env.Must(200, "PUT", "/api/admin/settings", map[string]any{"activation_fee_kzt": 7000}, admin)
	id, _ := env.PublishedEvent(org, "Paid gig", testutil.EventOptions{Types: []map[string]any{{"name": "Paid", "priceKzt": 1000, "quantity": 5}}})
	var c struct{ FeeKzt int }
	env.Must(200, "GET", "/api/events/"+id+"/activation", nil, org).JSON(&c)
	if c.FeeKzt != 7000 {
		t.Fatalf("fee %d", c.FeeKzt)
	}
}
