package checkin_test

import (
	"sync"
	"testing"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/commerce"
	"github.com/biletflow/backend-go/internal/testutil"
)

type scanResult struct {
	Result string
	Ticket *struct{ Code, AttendeeName, Status string }
	Counts struct{ Sold, CheckedIn int }
}

func setup(t *testing.T) (*testutil.Env, string, auth.Principal, auth.Principal, map[string]any) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	guest := env.User("attendee", "Dana Seitkali")
	id, types := env.PublishedEvent(org, "Concert", testutil.EventOptions{})
	order := env.Buy(guest, id, map[string]int{types[0]: 2}, nil)
	staff := env.User("attendee", "Door Staff")
	env.Must(201, "POST", "/api/events/"+id+"/staff", map[string]any{"email": staff.Email, "canUndo": false}, org)
	return env, id, org, staff, order
}

func TestScanAdmitsOnceAndRejectsEverythingElse(t *testing.T) {
	env, id, _, staff, order := setup(t)
	ticket := order["tickets"].([]any)[0].(map[string]any)
	payload := ticket["qrPayload"].(string)
	var r scanResult
	env.Must(200, "POST", "/api/scanner/events/"+id+"/scan", map[string]any{"payload": payload}, staff).JSON(&r)
	if r.Result != "admitted" || r.Ticket.AttendeeName != "Dana Seitkali" || r.Counts.CheckedIn != 1 || r.Counts.Sold != 2 {
		t.Fatalf("first scan: %+v", r)
	}
	env.Must(200, "POST", "/api/scanner/events/"+id+"/scan", map[string]any{"payload": payload}, staff).JSON(&r)
	if r.Result != "already_used" {
		t.Fatalf("second scan: %+v", r)
	}
	env.Must(200, "POST", "/api/scanner/events/"+id+"/scan", map[string]any{"payload": "https://biletflow.test/c/abcdef123456"}, staff).JSON(&r)
	if r.Result != "campaign_qr" {
		t.Fatalf("campaign QR: %+v", r)
	}
	env.Must(200, "POST", "/api/scanner/events/"+id+"/scan", map[string]any{"payload": "BFT1.not-a-ticket.xyz"}, staff).JSON(&r)
	if r.Result != "invalid" {
		t.Fatalf("garbage: %+v", r)
	}
	// A genuine ticket for another event is refused at this door.
	org2 := env.User("organizer", "Org Two")
	id2, types2 := env.PublishedEvent(org2, "Other", testutil.EventOptions{})
	other := env.Buy(env.User("attendee", "Xenia"), id2, map[string]int{types2[0]: 1}, nil)
	var wrong scanResult
	env.Must(200, "POST", "/api/scanner/events/"+id+"/scan", map[string]any{"payload": other["tickets"].([]any)[0].(map[string]any)["qrPayload"]}, staff).JSON(&wrong)
	if r = wrong; r.Result != "wrong_event" || r.Ticket != nil {
		t.Fatalf("wrong event: %+v", r)
	}
	// Staff assigned to one event cannot open another (no global Event Admin power).
	env.Must(404, "POST", "/api/scanner/events/"+id2+"/scan", map[string]any{"payload": payload}, staff)
	// Search hides e-mail addresses from door staff.
	res := env.Must(200, "GET", "/api/scanner/events/"+id+"/attendees?q=dana", nil, staff)
	var found []map[string]any
	res.JSON(&found)
	if len(found) != 2 || found[0]["attendeeName"] != "Dana Seitkali" {
		t.Fatalf("search: %v", found)
	}
	if _, leaked := found[0]["email"]; leaked {
		t.Fatal("door staff can see e-mail addresses")
	}
	// This staff member may not undo.
	if code := env.Do("POST", "/api/scanner/events/"+id+"/undo", map[string]any{"ticketId": ticket["id"]}, staff).ErrorCode(); code != "UNDO_NOT_ALLOWED" {
		t.Fatalf("undo without permission: %s", code)
	}
}

// CONCURRENCY: two devices scan the same ticket at the same moment; exactly one admits.
func TestTwoDevicesScanTheSameTicket(t *testing.T) {
	env, id, _, staff, order := setup(t)
	payload := order["tickets"].([]any)[1].(map[string]any)["qrPayload"].(string)
	for round := range 5 {
		_, _ = env.DB.Exec(`UPDATE tickets SET status='valid', checked_in_at=NULL WHERE event_id=$1`, id)
		var wg sync.WaitGroup
		results := make([]string, 8)
		for i := range results {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var r scanResult
				env.Must(200, "POST", "/api/scanner/events/"+id+"/scan", map[string]any{"payload": payload}, staff).JSON(&r)
				results[i] = r.Result
			}()
		}
		wg.Wait()
		admitted := 0
		for _, r := range results {
			if r == "admitted" {
				admitted++
			} else if r != "already_used" {
				t.Fatalf("unexpected result %s", r)
			}
		}
		if admitted != 1 {
			t.Fatalf("round %d: %d devices admitted the same ticket", round, admitted)
		}
	}
}

func TestOrganizerManualCheckInAndUndo(t *testing.T) {
	env, id, org, _, order := setup(t)
	ticketID := order["tickets"].([]any)[0].(map[string]any)["id"]
	var r scanResult
	env.Must(200, "POST", "/api/scanner/events/"+id+"/check-in", map[string]any{"ticketId": ticketID}, org).JSON(&r)
	if r.Result != "admitted" {
		t.Fatalf("manual: %+v", r)
	}
	env.Must(200, "POST", "/api/scanner/events/"+id+"/undo", map[string]any{"ticketId": ticketID}, org)
	env.Must(200, "POST", "/api/scanner/events/"+id+"/check-in", map[string]any{"ticketId": ticketID}, org).JSON(&r)
	if r.Result != "admitted" {
		t.Fatalf("after undo: %+v", r)
	}
	// A refunded or cancelled ticket shows its reason.
	_, _ = env.DB.Exec(`UPDATE tickets SET status='cancelled' WHERE id=$1`, order["tickets"].([]any)[1].(map[string]any)["id"])
	signer := commerce.QRSigner{Key: testutil.QRKey}
	env.Must(200, "POST", "/api/scanner/events/"+id+"/scan", map[string]any{"payload": signer.Payload(order["tickets"].([]any)[1].(map[string]any)["id"].(string))}, org).JSON(&r)
	if r.Result != "cancelled" {
		t.Fatalf("cancelled ticket: %+v", r)
	}
	// Refunded/used tickets block refunds of the order.
	if code := env.Do("POST", "/api/orders/"+order["id"].(string)+"/cancel", nil, org).ErrorCode(); code != "NOT_CANCELLABLE" {
		t.Fatalf("cancelled an order with a used ticket: %s", code)
	}
	var records int
	_ = env.DB.QueryRow(`SELECT count(*) FROM check_in_records WHERE event_id=$1`, id).Scan(&records)
	if records != 3 {
		t.Fatalf("check-in records %d", records)
	}
}
