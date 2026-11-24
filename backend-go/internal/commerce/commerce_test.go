package commerce_test

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/commerce"
	"github.com/biletflow/backend-go/internal/testutil"
)

func TestSimulatorCardsAndFee(t *testing.T) {
	sim := commerce.Simulator{}
	if r := sim.Charge(1000, "4242 4242 4242 4242"); !r.OK || r.CardLast4 != "4242" {
		t.Fatalf("success card: %+v", r)
	}
	for card, reason := range map[string]string{"4000000000000002": "CARD_DECLINED", "4000000000009995": "INSUFFICIENT_FUNDS", "1234": "INVALID_CARD", "4242424242424241": "INVALID_CARD"} {
		if r := sim.Charge(1000, card); r.OK || r.FailureReason != reason {
			t.Fatalf("%s: %+v", card, r)
		}
	}
	if commerce.ProcessingFee(10000, 2.5) != 250 || commerce.ProcessingFee(999, 2.5) != 25 {
		t.Fatal("processing fee rounding")
	}
}

func TestQRSignatureRejectsForgeryAndCampaignLinks(t *testing.T) {
	s := commerce.QRSigner{Key: []byte("test-qr-signing-secret-0123456789abcdef")}
	id := "4b1d6a3e-2f7c-4d8e-9a10-5c3b2a1f0e9d"
	if kind, got := s.Parse(s.Payload(id)); kind != commerce.ScanTicket || got != id {
		t.Fatalf("round trip: %s %s", kind, got)
	}
	forged := strings.Replace(s.Payload(id), "4b1d", "4b1e", 1)
	if kind, _ := s.Parse(forged); kind != commerce.ScanInvalid {
		t.Fatal("edited ticket ID accepted")
	}
	other := commerce.QRSigner{Key: []byte("another-key-0123456789abcdef012345")}
	if kind, _ := s.Parse(other.Payload(id)); kind != commerce.ScanInvalid {
		t.Fatal("QR signed with another key accepted")
	}
	if kind, _ := s.Parse("https://biletflow.test/c/abc123"); kind != commerce.ScanCampaign {
		t.Fatal("campaign link not recognised")
	}
}

func TestFreeRegistrationIssuesTicketsAndPDF(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	guest := env.User("attendee", "Dana Seitkali")
	id, types := env.PublishedEvent(org, "NU Hackathon Kickoff", testutil.EventOptions{})
	order := env.Buy(guest, id, map[string]int{types[0]: 2}, map[string]any{"attendees": []map[string]any{
		{"name": "Dana Seitkali", "email": guest.Email}, {"name": "Arman Nurlanov", "email": "arman@example.kz"}}})
	if order["status"] != "completed" || order["totalKzt"].(float64) != 0 {
		t.Fatalf("free order: %v", order)
	}
	tickets := order["tickets"].([]any)
	if len(tickets) != 2 || tickets[1].(map[string]any)["attendeeName"] != "Arman Nurlanov" || tickets[0].(map[string]any)["qrPayload"] == "" {
		t.Fatalf("tickets: %v", tickets)
	}
	orderID := order["id"].(string)
	pdf := env.Must(200, "GET", "/api/orders/"+orderID+"/tickets.pdf", nil, guest)
	if !bytes.HasPrefix(pdf.Body.Bytes(), []byte("%PDF")) || pdf.Header().Get("Content-Type") != "application/pdf" {
		t.Fatal("not a PDF")
	}
	stranger := env.User("attendee", "Stranger")
	env.Must(404, "GET", "/api/orders/"+orderID, nil, stranger)
	env.Must(404, "GET", "/api/orders/"+orderID+"/tickets.pdf", nil, stranger)
	var mine []struct{ ID string }
	env.Must(200, "GET", "/api/orders", nil, guest).JSON(&mine)
	if len(mine) != 1 || mine[0].ID != orderID {
		t.Fatalf("my orders: %+v", mine)
	}
	var notes struct{ Items []struct{ Type string } }
	env.Must(200, "GET", "/api/notifications", nil, guest).JSON(&notes)
	if len(notes.Items) != 1 || notes.Items[0].Type != "registration_confirmed" {
		t.Fatalf("confirmation: %+v", notes)
	}
}

func TestPaidSalesNeedActivation(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	buyer := env.User("attendee", "Buyer")
	id, types := env.PublishedEvent(org, "Jazz", testutil.EventOptions{Types: []map[string]any{{"name": "Standard", "priceKzt": 8000, "quantity": 50}}})
	body := map[string]any{"eventId": id, "items": []map[string]any{{"ticketTypeId": types[0], "quantity": 1}}, "buyer": map[string]any{"name": "Buyer", "email": buyer.Email}}
	if code := env.Do("POST", "/api/checkout/reserve", body, buyer).ErrorCode(); code != "PAID_SALES_INACTIVE" {
		t.Fatalf("sold before activation: %s", code)
	}
	var c struct {
		Ready, HasPaidTicketType, IdentityVerified bool
		FeeKzt                                     int
	}
	env.Must(200, "GET", "/api/events/"+id+"/activation", nil, org).JSON(&c)
	if c.Ready || !c.HasPaidTicketType || c.IdentityVerified || c.FeeKzt != 5000 {
		t.Fatalf("checklist: %+v", c)
	}
	if code := env.Do("POST", "/api/events/"+id+"/activation/activate", nil, org).ErrorCode(); code != "CHECKLIST_INCOMPLETE" {
		t.Fatalf("activated early: %s", code)
	}
	if code := env.Do("PUT", "/api/organizer/payment-setup/identity", map[string]any{"legalName": "Org", "iin": "123", "consent": true}, org).ErrorCode(); code != "INVALID_IIN" {
		t.Fatalf("bad IIN: %s", code)
	}
	if code := env.Do("POST", "/api/events/"+id+"/activation/fee", map[string]any{"cardNumber": "4000 0000 0000 0002"}, org).Code; code != 402 {
		t.Fatalf("declined fee: %d", code)
	}
	env.ActivatePaidSales(org, id)
	var setup struct {
		IbanMasked string
		IIN        *string
	}
	env.Must(200, "GET", "/api/organizer/payment-setup", nil, org).JSON(&setup)
	if !strings.HasPrefix(setup.IbanMasked, "KZ12") || strings.Contains(setup.IbanMasked, "56789012") {
		t.Fatalf("IBAN not masked: %q", setup.IbanMasked)
	}
	var stored int
	_ = env.DB.QueryRow(`SELECT count(*) FROM organizer_payment_setups s WHERE row_to_json(s)::text LIKE '%990101300123%'`).Scan(&stored)
	if stored != 0 {
		t.Fatal("the IIN was stored")
	}
	order := env.Buy(buyer, id, map[string]int{types[0]: 2}, nil)
	if order["status"] != "completed" || order["totalKzt"].(float64) != 16000 || order["isSimulated"] != true {
		t.Fatalf("paid order: %v", order)
	}
	var fee int
	_ = env.DB.QueryRow(`SELECT fee_kzt FROM payments WHERE order_id=$1`, order["id"]).Scan(&fee)
	if fee != 400 {
		t.Fatalf("processing fee recorded %d", fee)
	}
	// A second event of the same organizer is not activated by the first one (SRS §4.5).
	id2, types2 := env.PublishedEvent(org, "Second", testutil.EventOptions{Types: []map[string]any{{"name": "Paid", "priceKzt": 1000, "quantity": 5}}})
	body["eventId"], body["items"] = id2, []map[string]any{{"ticketTypeId": types2[0], "quantity": 1}}
	if code := env.Do("POST", "/api/checkout/reserve", body, buyer).ErrorCode(); code != "PAID_SALES_INACTIVE" {
		t.Fatalf("activation leaked to another event: %s", code)
	}
}

func TestDeclinedPaymentCreatesNoTicketsAndReleasesStock(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	buyer := env.User("attendee", "Buyer")
	id, types := env.PublishedEvent(org, "Jazz", testutil.EventOptions{PaidActive: true, Types: []map[string]any{{"name": "Standard", "priceKzt": 5000, "quantity": 2}}})
	body := map[string]any{"eventId": id, "items": []map[string]any{{"ticketTypeId": types[0], "quantity": 2}}, "buyer": map[string]any{"name": "Buyer", "email": buyer.Email}}
	order := env.Must(201, "POST", "/api/checkout/reserve", body, buyer).Map()
	if order["status"] != "pending" {
		t.Fatalf("not held: %v", order)
	}
	other := env.User("attendee", "Other")
	if code := env.Do("POST", "/api/checkout/reserve", body, other).ErrorCode(); code != "SOLD_OUT" {
		t.Fatalf("held stock sold twice: %s", code)
	}
	res := env.Do("POST", "/api/checkout/"+order["id"].(string)+"/pay", map[string]any{"cardNumber": "4000 0000 0000 9995"}, buyer)
	if res.Code != 402 || res.ErrorCode() != "PAYMENT_FAILED" {
		t.Fatalf("insufficient funds: %d %s", res.Code, res.Body.String())
	}
	var tickets int
	_ = env.DB.QueryRow(`SELECT count(*) FROM tickets`).Scan(&tickets)
	if tickets != 0 {
		t.Fatal("tickets issued for a failed payment")
	}
	env.Must(201, "POST", "/api/checkout/reserve", body, other)
}

func TestExpiredHoldCannotBePaid(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	buyer := env.User("attendee", "Buyer")
	id, types := env.PublishedEvent(org, "Jazz", testutil.EventOptions{PaidActive: true, Types: []map[string]any{{"name": "Standard", "priceKzt": 5000, "quantity": 1}}})
	body := map[string]any{"eventId": id, "items": []map[string]any{{"ticketTypeId": types[0], "quantity": 1}}, "buyer": map[string]any{"name": "Buyer", "email": buyer.Email}}
	order := env.Must(201, "POST", "/api/checkout/reserve", body, buyer).Map()
	if _, err := env.DB.Exec(`UPDATE orders SET expires_at = now() - interval '1 second' WHERE id=$1`, order["id"]); err != nil {
		t.Fatal(err)
	}
	if code := env.Do("POST", "/api/checkout/"+order["id"].(string)+"/pay", map[string]any{"cardNumber": "4242424242424242"}, buyer).ErrorCode(); code != "HOLD_EXPIRED" {
		t.Fatalf("expired hold paid: %s", code)
	}
	var status string
	_ = env.DB.QueryRow(`SELECT status FROM orders WHERE id=$1`, order["id"]).Scan(&status)
	if status != "expired" {
		t.Fatalf("status %s", status)
	}
	env.Must(201, "POST", "/api/checkout/reserve", body, env.User("attendee", "Next"))
}

// CONCURRENCY: 20 buyers race for 5 tickets; exactly 5 succeed and capacity is never exceeded.
func TestConcurrentCheckoutNeverOversells(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	id, types := env.PublishedEvent(org, "Tiny Gig", testutil.EventOptions{Capacity: 5, Types: []map[string]any{{"name": "Free", "quantity": 50}}})
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for i := range 20 {
		buyer := env.User("attendee", "Buyer"+string(rune('A'+i)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := env.Do("POST", "/api/checkout/reserve", map[string]any{"eventId": id, "items": []map[string]any{{"ticketTypeId": types[0], "quantity": 1}},
				"buyer": map[string]any{"name": buyer.DisplayName, "email": buyer.Email}}, buyer)
			mu.Lock()
			codes[res.Code]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	var sold int
	_ = env.DB.QueryRow(`SELECT count(*) FROM tickets WHERE event_id=$1`, id).Scan(&sold)
	if codes[201] != 5 || codes[409] != 15 || sold != 5 {
		t.Fatalf("oversold: responses %v, tickets %d", codes, sold)
	}
}

// CONCURRENCY: a code limited to 3 redemptions is used exactly 3 times by 10 simultaneous buyers.
func TestPromoLimitHoldsUnderConcurrency(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	id, types := env.PublishedEvent(org, "Jazz", testutil.EventOptions{PaidActive: true, Types: []map[string]any{{"name": "Standard", "priceKzt": 10000, "quantity": 100}}})
	env.Must(201, "POST", "/api/events/"+id+"/campaigns", map[string]any{"name": "Students", "code": "nu-25", "discountType": "percent", "discountValue": 25, "maxRedemptions": 3}, org)
	preview := env.Must(200, "POST", "/api/events/"+id+"/promo-preview", map[string]any{"code": "NU-25", "items": []map[string]any{{"ticketTypeId": types[0], "quantity": 2}}}, auth.Principal{}).Map()
	if preview["discountKzt"].(float64) != 5000 {
		t.Fatalf("preview: %v", preview)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := range 10 {
		buyer := env.User("attendee", "Student"+string(rune('A'+i)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := env.Do("POST", "/api/checkout/reserve", map[string]any{"eventId": id, "promoCode": "nu-25", "items": []map[string]any{{"ticketTypeId": types[0], "quantity": 1}},
				"buyer": map[string]any{"name": buyer.DisplayName, "email": buyer.Email}}, buyer)
			if res.Code == 201 {
				mu.Lock()
				ok++
				mu.Unlock()
				if total := res.Map()["totalKzt"].(float64); total != 7500 {
					t.Errorf("discounted total %v", total)
				}
			}
		}()
	}
	wg.Wait()
	var used int
	_ = env.DB.QueryRow(`SELECT redemptions_count FROM promo_campaigns WHERE event_id=$1`, id).Scan(&used)
	if ok != 3 || used != 3 {
		t.Fatalf("redemptions: %d accepted, counter %d", ok, used)
	}
	// Abandoning a held order gives the redemption back.
	var orderID, buyerID string
	_ = env.DB.QueryRow(`SELECT id, user_id FROM orders WHERE event_id=$1 AND status='pending' LIMIT 1`, id).Scan(&orderID, &buyerID)
	env.Must(204, "POST", "/api/checkout/"+orderID+"/abandon", nil, auth.Principal{ID: buyerID, Role: "attendee", EmailVerified: true})
	_ = env.DB.QueryRow(`SELECT redemptions_count FROM promo_campaigns WHERE event_id=$1`, id).Scan(&used)
	if used != 2 {
		t.Fatalf("abandoned order kept its redemption: %d", used)
	}
	if code := env.Do("POST", "/api/events/"+id+"/promo-preview", map[string]any{"code": "WRONG"}, auth.Principal{}).ErrorCode(); code != "PROMO_INVALID" {
		t.Fatalf("unknown code: %s", code)
	}
}

func TestRefundAndEventCancellation(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	paid := env.User("attendee", "Paid Guest")
	free := env.User("attendee", "Free Guest")
	id, types := env.PublishedEvent(org, "Festival", testutil.EventOptions{PaidActive: true, Types: []map[string]any{
		{"name": "VIP", "priceKzt": 20000, "quantity": 10}, {"name": "Free entry", "priceKzt": 0, "quantity": 10}}})
	paidOrder := env.Buy(paid, id, map[string]int{types[0]: 1}, nil)
	freeOrder := env.Buy(free, id, map[string]int{types[1]: 1}, nil)
	another := env.Buy(paid, id, map[string]int{types[0]: 1}, nil)

	if env.Do("POST", "/api/orders/"+paidOrder["id"].(string)+"/refund", map[string]any{"reason": "x"}, paid).Code != 403 {
		t.Fatal("attendee refunded their own order")
	}
	refunded := env.Must(200, "POST", "/api/orders/"+paidOrder["id"].(string)+"/refund", map[string]any{"reason": "Guest cannot attend"}, org).Map()
	if refunded["status"] != "refunded" || refunded["tickets"].([]any)[0].(map[string]any)["status"] != "refunded" {
		t.Fatalf("refund: %v", refunded)
	}
	if code := env.Do("POST", "/api/orders/"+paidOrder["id"].(string)+"/refund", map[string]any{"reason": "again"}, org).ErrorCode(); code != "NOT_REFUNDABLE" {
		t.Fatalf("double refund: %s", code)
	}

	env.Must(200, "POST", "/api/events/"+id+"/cancel", map[string]any{"reason": "Venue flooded"}, org)
	statuses := map[string]string{}
	for _, o := range []map[string]any{freeOrder, another} {
		var s string
		_ = env.DB.QueryRow(`SELECT status FROM orders WHERE id=$1`, o["id"]).Scan(&s)
		statuses[o["id"].(string)] = s
	}
	if statuses[freeOrder["id"].(string)] != "cancelled" || statuses[another["id"].(string)] != "refunded" {
		t.Fatalf("cascade: %v", statuses)
	}
	var valid int
	_ = env.DB.QueryRow(`SELECT count(*) FROM tickets WHERE event_id=$1 AND status IN ('valid','checked_in')`, id).Scan(&valid)
	if valid != 0 {
		t.Fatal("valid tickets remain after cancellation")
	}
	var notes struct{ Items []struct{ Type string } }
	env.Must(200, "GET", "/api/notifications", nil, free).JSON(&notes)
	found := false
	for _, n := range notes.Items {
		found = found || n.Type == "event_cancelled"
	}
	if !found {
		t.Fatalf("attendee not told about cancellation: %+v", notes)
	}
	if code := env.Do("PATCH", "/api/events/"+id, map[string]any{"title": "x"}, org).ErrorCode(); code != "EVENT_CANCELLED" {
		t.Fatalf("edited cancelled event: %s", code)
	}
	var auditRows int
	_ = env.DB.QueryRow(`SELECT count(*) FROM audit_logs WHERE event_id=$1 AND action='order.refunded'`, id).Scan(&auditRows)
	if auditRows != 2 {
		t.Fatalf("refund audit rows %d", auditRows)
	}
	if _, err := env.DB.Exec(`UPDATE audit_logs SET description='tampered'`); err == nil {
		t.Fatal("audit log is not append-only")
	}
}

func TestAttendeeCancelsFreeRegistration(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	guest := env.User("attendee", "Guest")
	id, types := env.PublishedEvent(org, "Meetup", testutil.EventOptions{Types: []map[string]any{{"name": "Free", "quantity": 1}}})
	order := env.Buy(guest, id, map[string]int{types[0]: 1}, nil)
	if order["canCancel"] != true {
		t.Fatal("cannot cancel own free registration")
	}
	env.Must(200, "POST", "/api/orders/"+order["id"].(string)+"/cancel", nil, guest)
	env.Buy(env.User("attendee", "Next"), id, map[string]int{types[0]: 1}, nil)
}
