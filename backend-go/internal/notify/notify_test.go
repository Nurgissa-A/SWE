package notify_test

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/biletflow/backend-go/internal/notify"
	"github.com/biletflow/backend-go/internal/testutil"
)

var placeholder = regexp.MustCompile(`\{[a-z]+\}`)

// Every notification exists in Kazakh, Russian and English with the same placeholders (NFR-7).
func TestCatalogIsCompleteInThreeLanguages(t *testing.T) {
	kinds := []string{"order_confirmed", "registration_confirmed", "payment_failed", "refund_completed", "registration_cancelled", "event_cancelled",
		"event_updated", "event_suspended", "paid_sales_active", "paid_sales_suspended", "paid_sales_resumed", "staff_assigned",
		"support_new_case", "support_reply", "support_status", "support_escalated"}
	params := map[string]any{"event": "E", "number": "N", "n": 1, "total": 2, "reason": "R", "organizer": "O", "subject": "S", "preview": "P", "status": "resolved"}
	for _, kind := range kinds {
		if !notify.Known(kind) {
			t.Fatalf("%s missing", kind)
		}
		var en []string
		for _, loc := range []string{"en", "ru", "kk"} {
			title, body := notify.Render(kind, loc, params)
			if title == "" || placeholder.MatchString(title+body) {
				t.Fatalf("%s/%s not filled: %q %q", kind, loc, title, body)
			}
			raw, rawBody := notify.Render(kind, loc, map[string]any{})
			found := placeholder.FindAllString(raw+rawBody, -1)
			sort.Strings(found)
			if loc == "en" {
				en = found
			} else if strings.Join(found, ",") != strings.Join(en, ",") {
				t.Fatalf("%s/%s placeholders %v differ from English %v", kind, loc, found, en)
			}
		}
	}
	if title, _ := notify.Render("support_status", "ru", map[string]any{"number": 5, "status": "resolved"}); title != "Статус обращения №5: Решено" {
		t.Fatalf("status translated badly: %q", title)
	}
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []string
	pdfs int
}

func (f *fakeMailer) Send(to, subject, body string, files []notify.Attachment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, to+"|"+subject)
	for _, a := range files {
		if a.ContentType == "application/pdf" && strings.HasPrefix(string(a.Data), "%PDF") {
			f.pdfs++
		}
	}
	return nil
}

func TestOutboxEmailsInRecipientLanguageWithTickets(t *testing.T) {
	env := testutil.New(t)
	org := env.User("organizer", "Org")
	guest := env.User("attendee", "Guest")
	guest.Locale = "kk"
	id, types := env.PublishedEvent(org, "Той", testutil.EventOptions{})
	env.Buy(guest, id, map[string]int{types[0]: 1}, nil)
	mailer := &fakeMailer{}
	outbox := &notify.Outbox{DB: env.DB, Mailer: mailer, Attachments: env.App.Commerce.Attachments, AppURL: "https://biletflow.test"}
	n, err := outbox.Flush(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("flush: %d %v", n, err)
	}
	if mailer.sent[0] != guest.Email+"|Той: билеттеріңіз" || mailer.pdfs != 1 {
		t.Fatalf("mail: %v pdfs=%d", mailer.sent, mailer.pdfs)
	}
	if n, _ := outbox.Flush(context.Background()); n != 0 {
		t.Fatal("e-mail sent twice")
	}
}
