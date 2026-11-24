// Package notify stores in-app notifications and delivers their e-mail copies
// through a transactional outbox (SRS §4.10).
package notify

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/platform"
)

// Message describes one notification for one user.
type Message struct {
	UserID string
	Type   string
	Params map[string]any
	Link   string
	Email  bool
}

// Queue inserts the notification using q, so it commits atomically with the change it reports.
func Queue(ctx context.Context, q platform.Querier, m Message) error {
	if !Known(m.Type) {
		return fmt.Errorf("unknown notification type %q", m.Type)
	}
	params := m.Params
	if params == nil {
		params = map[string]any{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	var link any
	if m.Link != "" {
		link = m.Link
	}
	_, err = q.ExecContext(ctx, `INSERT INTO notifications(user_id, type, params, link, email_wanted) VALUES ($1,$2,$3,$4,$5)`,
		m.UserID, m.Type, raw, link, m.Email)
	return err
}

// Attachment is a file added to an outgoing e-mail.
type Attachment struct {
	Name        string
	ContentType string
	Data        []byte
}

// AttachmentSource returns attachments for a notification (for example ticket PDFs).
type AttachmentSource func(ctx context.Context, kind string, params map[string]any, userID string) []Attachment

// Mailer sends one e-mail. SMTPMailer is the real implementation; tests use a fake.
type Mailer interface {
	Send(to, subject, body string, attachments []Attachment) error
}

// SMTPMailer delivers through a plain SMTP relay (Mailpit in development).
type SMTPMailer struct {
	Addr string // host:port
	From string
}

// envelopeSender returns the bare address for the SMTP MAIL FROM command and the encoded
// header form; "BiletFlow <no-reply@…>" is valid in the header but not in the envelope.
func envelopeSender(from string) (envelope, header string, err error) {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return "", "", fmt.Errorf("MAIL_FROM %q: %w", from, err)
	}
	return addr.Address, addr.String(), nil
}

func (m SMTPMailer) Send(to, subject, body string, attachments []Attachment) error {
	envelope, header, err := envelopeSender(m.From)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	boundary := fmt.Sprintf("bf-%d", time.Now().UnixNano())
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n", header, to, mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%s\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n", boundary, wrap(base64.StdEncoding.EncodeToString([]byte(body))))
	for _, a := range attachments {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s\r\nContent-Disposition: attachment; filename=%q\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n",
			boundary, a.ContentType, a.Name, wrap(base64.StdEncoding.EncodeToString(a.Data)))
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return smtp.SendMail(m.Addr, nil, envelope, []string{to}, b.Bytes())
}

func wrap(s string) string {
	var out strings.Builder
	for len(s) > 76 {
		out.WriteString(s[:76] + "\r\n")
		s = s[76:]
	}
	out.WriteString(s)
	return out.String()
}

// Outbox sends pending notification e-mails.
type Outbox struct {
	DB          *sql.DB
	Mailer      Mailer
	Attachments AttachmentSource
	AppURL      string
}

// Run delivers pending e-mails every interval until ctx is cancelled.
func (o *Outbox) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if n, err := o.Flush(ctx); err != nil {
			log.Printf("notification outbox: %v", err)
		} else if n > 0 {
			log.Printf("notification outbox: sent %d e-mail(s)", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Flush sends up to 50 pending e-mails; SKIP LOCKED lets several instances share the work.
func (o *Outbox) Flush(ctx context.Context) (int, error) {
	sent := 0
	err := platform.InTx(ctx, o.DB, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT n.id, n.user_id, n.type, n.params, coalesce(n.link,''), d.email, d.locale
			FROM notifications n JOIN user_directory d ON d.user_id = n.user_id
			WHERE n.email_wanted AND n.emailed_at IS NULL AND n.email_attempts < 5
			ORDER BY n.created_at LIMIT 50 FOR UPDATE OF n SKIP LOCKED`)
		if err != nil {
			return err
		}
		type pending struct {
			id, userID, kind, link, email, locale string
			params                                map[string]any
		}
		var batch []pending
		for rows.Next() {
			var p pending
			var raw []byte
			if err := rows.Scan(&p.id, &p.userID, &p.kind, &raw, &p.link, &p.email, &p.locale); err != nil {
				rows.Close()
				return err
			}
			_ = json.Unmarshal(raw, &p.params)
			batch = append(batch, p)
		}
		rows.Close()
		for _, p := range batch {
			title, body := Render(p.kind, p.locale, p.params)
			if p.link != "" && o.AppURL != "" {
				body += "\n\n" + strings.TrimRight(o.AppURL, "/") + p.link
			}
			var files []Attachment
			if o.Attachments != nil {
				files = o.Attachments(ctx, p.kind, p.params, p.userID)
			}
			if err := o.Mailer.Send(p.email, title, body, files); err != nil {
				log.Printf("e-mail for notification %s failed: %v", p.id, err)
				if _, err := tx.ExecContext(ctx, `UPDATE notifications SET email_attempts = email_attempts + 1 WHERE id=$1`, p.id); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.ExecContext(ctx, `UPDATE notifications SET emailed_at = now(), email_attempts = email_attempts + 1 WHERE id=$1`, p.id); err != nil {
				return err
			}
			sent++
		}
		return nil
	})
	return sent, err
}
