package analytics_test

import (
	"testing"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/testutil"
)

func TestOrganizerAnalyticsFromOwnRecords(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	id, types := env.PublishedEvent(org, "Jazz", testutil.EventOptions{Capacity: 100, PaidActive: true, Types: []map[string]any{
		{"name": "Standard", "priceKzt": 8000, "quantity": 80}, {"name": "VIP", "priceKzt": 20000, "quantity": 20}}})
	env.Must(201, "POST", "/api/events/"+id+"/campaigns", map[string]any{"name": "Radio", "code": "RADIO", "discountType": "fixed", "discountValue": 2000}, org)
	a := env.User("attendee", "Aruzhan")
	b := env.User("attendee", "Bolat")
	env.Buy(a, id, map[string]int{types[0]: 2}, nil)                                         // 16 000
	vip := env.Buy(b, id, map[string]int{types[1]: 1}, map[string]any{"promoCode": "radio"}) // 20 000 − 2 000
	env.Buy(b, id, map[string]int{types[0]: 1}, nil)                                         // 8 000, refunded below
	var last struct{ ID string }
	_ = env.DB.QueryRow(`SELECT id FROM orders WHERE event_id=$1 AND total_kzt=8000`, id).Scan(&last.ID)
	env.Must(200, "POST", "/api/orders/"+last.ID+"/refund", map[string]any{"reason": "Duplicate purchase"}, org)
	env.Must(200, "POST", "/api/scanner/events/"+id+"/scan", map[string]any{"payload": vip["tickets"].([]any)[0].(map[string]any)["qrPayload"]}, org)

	var rep struct {
		Summary struct {
			Sold, Remaining, CheckedIn, Orders                  int
			GrossKzt, DiscountsKzt, RefundsKzt, NetKzt, FeesKzt int
			PercentSold, AttendanceRate                         float64
		}
		Daily  []struct{ Tickets, RevenueKzt int }
		ByType []struct {
			Name           string
			Sold, Refunded int
		} `json:"byTicketType"`
		Campaigns []struct {
			Code                string
			Orders, DiscountKzt int
		}
	}
	env.Must(200, "GET", "/api/events/"+id+"/analytics", nil, org).JSON(&rep)
	s := rep.Summary
	if s.Sold != 3 || s.Remaining != 97 || s.PercentSold != 3 || s.CheckedIn != 1 || s.AttendanceRate != 33.3 {
		t.Fatalf("inventory: %+v", s)
	}
	if s.GrossKzt != 44000 || s.DiscountsKzt != 2000 || s.RefundsKzt != 8000 || s.NetKzt != 34000 || s.FeesKzt != 850 || s.Orders != 3 {
		t.Fatalf("money: %+v", s)
	}
	if len(rep.Daily) != 1 || rep.Daily[0].Tickets != 3 || rep.Daily[0].RevenueKzt != 34000 {
		t.Fatalf("daily: %+v", rep.Daily)
	}
	if len(rep.ByType) != 2 || rep.ByType[0].Sold != 2 || rep.ByType[0].Refunded != 1 {
		t.Fatalf("by type: %+v", rep.ByType)
	}
	if len(rep.Campaigns) != 1 || rep.Campaigns[0].Orders != 1 || rep.Campaigns[0].DiscountKzt != 2000 {
		t.Fatalf("campaigns: %+v", rep.Campaigns)
	}
	env.Must(200, "GET", "/api/events/"+id+"/analytics?ticketTypeId="+types[1], nil, org).JSON(&rep)
	if rep.Summary.Sold != 1 || rep.Summary.GrossKzt != 20000 {
		t.Fatalf("type filter: %+v", rep.Summary)
	}
	env.Must(200, "GET", "/api/events/"+id+"/analytics?from=2000-01-01&to=2000-01-02", nil, org).JSON(&rep)
	if rep.Summary.GrossKzt != 0 || len(rep.Daily) != 0 {
		t.Fatalf("date filter: %+v", rep.Summary)
	}
	other := env.User("organizer", "Rival")
	if env.Do("GET", "/api/events/"+id+"/analytics", nil, other).Code != 403 {
		t.Fatal("another organizer read the analytics")
	}
	var overview []struct {
		ID        string
		Lifecycle string
		Sold      int
	}
	env.Must(200, "GET", "/api/organizer/overview", nil, org).JSON(&overview)
	if len(overview) != 1 || overview[0].Sold != 3 || overview[0].Lifecycle != "upcoming" {
		t.Fatalf("overview: %+v", overview)
	}
	if env.Do("GET", "/api/organizer/overview", nil, auth.Principal{}).Code != 401 {
		t.Fatal("anonymous overview")
	}
}
