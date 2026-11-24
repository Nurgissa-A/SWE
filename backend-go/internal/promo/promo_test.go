package promo_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/promo"
	"github.com/biletflow/backend-go/internal/testutil"
)

func TestDiscountCalculation(t *testing.T) {
	pct := &promo.Campaign{DiscountType: "percent", DiscountValue: 15, Status: "active"}
	total, per := pct.Discount([]promo.Line{{TicketTypeID: "a", Quantity: 2, UnitPriceKzt: 3333}, {TicketTypeID: "free", Quantity: 1}})
	if total != 1000 || per["a"] != 1000 || per["free"] != 0 {
		t.Fatalf("percent: %d %v", total, per)
	}
	fixed := &promo.Campaign{DiscountType: "fixed", DiscountValue: 5000, TicketTypeIDs: []string{"b"}}
	total, _ = fixed.Discount([]promo.Line{{TicketTypeID: "a", Quantity: 1, UnitPriceKzt: 8000}, {TicketTypeID: "b", Quantity: 2, UnitPriceKzt: 3000}})
	if total != 6000 {
		t.Fatalf("fixed is capped at the price and limited to listed types: %d", total)
	}
	past := time.Now().Add(-time.Hour)
	expired := &promo.Campaign{Status: "active", ValidUntil: &past}
	if p := expired.Problem(time.Now()); p == nil || p.Code != "PROMO_EXPIRED" {
		t.Fatal("expired code accepted")
	}
}

func TestCampaignLinkAndQR(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	id, _ := env.PublishedEvent(org, "Jazz", testutil.EventOptions{})
	var c struct{ ID, LinkToken, Code string }
	env.Must(201, "POST", "/api/events/"+id+"/campaigns", map[string]any{"name": "Instagram", "code": "insta10", "discountType": "percent", "discountValue": 10}, org).JSON(&c)
	if c.Code != "INSTA10" || c.LinkToken == "" {
		t.Fatalf("campaign: %+v", c)
	}
	if code := env.Do("POST", "/api/events/"+id+"/campaigns", map[string]any{"name": "Dup", "code": "INSTA10", "discountType": "percent", "discountValue": 10}, org).ErrorCode(); code != "CODE_TAKEN" {
		t.Fatalf("duplicate code: %s", code)
	}
	png := env.Must(200, "GET", "/api/events/"+id+"/campaigns/"+c.ID+"/qr.png", nil, org)
	if !bytes.HasPrefix(png.Body.Bytes(), []byte("\x89PNG")) {
		t.Fatal("QR is not a PNG")
	}
	visit := env.Must(200, "GET", "/api/public/campaigns/"+c.LinkToken, nil, auth.Principal{}).Map()
	if visit["eventId"] != id || visit["code"] != "INSTA10" {
		t.Fatalf("visit: %v", visit)
	}
	env.Must(200, "PATCH", "/api/events/"+id+"/campaigns/"+c.ID, map[string]any{"status": "disabled"}, org)
	visit = env.Must(200, "GET", "/api/public/campaigns/"+c.LinkToken, nil, auth.Principal{}).Map()
	if visit["active"] != false || visit["code"] != nil {
		t.Fatalf("disabled campaign still hands out its code: %v", visit)
	}
	var list []struct {
		LinkVisits int
		Results    struct{ Orders int }
	}
	env.Must(200, "GET", "/api/events/"+id+"/campaigns", nil, org).JSON(&list)
	if len(list) != 1 || list[0].LinkVisits != 2 {
		t.Fatalf("visits: %+v", list)
	}
	if code := env.Do("POST", "/api/events/"+id+"/promo-preview", map[string]any{"code": "INSTA10"}, auth.Principal{}).ErrorCode(); code != "PROMO_DISABLED" {
		t.Fatalf("disabled code: %s", code)
	}
}
