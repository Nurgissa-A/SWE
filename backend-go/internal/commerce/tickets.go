package commerce

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/biletflow/backend-go/internal/platform"
)

// Admission QR payload (SRS §4.7 tamper-resistant QR): BFT1.<ticketId>.<signature>.
// The signature is an HMAC of the ticket ID with a server-only key, so a QR cannot be forged
// or edited to point at another ticket. Printed and digital copies carry the same payload,
// so they are the same single admission. Campaign QR codes are https links and are rejected.
const qrPrefix = "BFT1"

// QRSigner signs and verifies ticket payloads.
type QRSigner struct{ Key []byte }

func (s QRSigner) sign(ticketID string) string {
	mac := hmac.New(sha256.New, s.Key)
	mac.Write([]byte(qrPrefix + ":" + ticketID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))[:22]
}

func (s QRSigner) Payload(ticketID string) string {
	return qrPrefix + "." + ticketID + "." + s.sign(ticketID)
}

// ScanKind classifies a scanned code.
type ScanKind string

const (
	ScanTicket   ScanKind = "ticket"
	ScanCampaign ScanKind = "campaign"
	ScanInvalid  ScanKind = "invalid"
)

// Parse verifies a scanned payload and returns the ticket ID when it is a genuine BiletFlow ticket.
func (s QRSigner) Parse(raw string) (ScanKind, string) {
	v := strings.TrimSpace(raw)
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return ScanCampaign, ""
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 || parts[0] != qrPrefix || !platform.IsUUID(parts[1]) {
		return ScanInvalid, ""
	}
	if !hmac.Equal([]byte(parts[2]), []byte(s.sign(strings.ToLower(parts[1])))) && !hmac.Equal([]byte(parts[2]), []byte(s.sign(parts[1]))) {
		return ScanInvalid, ""
	}
	return ScanTicket, strings.ToLower(parts[1])
}

//go:embed fonts/DejaVuSans.ttf fonts/DejaVuSans-Bold.ttf
var fontFiles embed.FS

// PrintableTicket is the data printed on one ticket. It never includes payment details.
type PrintableTicket struct {
	ID, Code, EventTitle, TicketType, AttendeeName, OrderNumber string
	Venue, City, Timezone                                       string
	StartsAt                                                    time.Time
	Status                                                      string
}

// LoadPrintable loads printable tickets for an order (or one ticket when ticketID is set).
func LoadPrintable(ctx context.Context, q platform.Querier, orderID, ticketID string) ([]PrintableTicket, error) {
	query := `SELECT t.id, t.code, e.title, tt.name, t.attendee_name, o.number, coalesce(e.venue_name,''), coalesce(e.city,''),
		e.timezone, e.starts_at, t.status
		FROM tickets t JOIN orders o ON o.id=t.order_id JOIN events e ON e.id=t.event_id JOIN ticket_types tt ON tt.id=t.ticket_type_id
		WHERE t.order_id=$1`
	args := []any{orderID}
	if ticketID != "" {
		query += ` AND t.id=$2`
		args = append(args, ticketID)
	}
	rows, err := q.QueryContext(ctx, query+` ORDER BY t.issued_at, t.code`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrintableTicket
	for rows.Next() {
		var t PrintableTicket
		if err := rows.Scan(&t.ID, &t.Code, &t.EventTitle, &t.TicketType, &t.AttendeeName, &t.OrderNumber, &t.Venue, &t.City, &t.Timezone, &t.StartsAt, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RenderPDF produces a print-optimised A4 PDF with one ticket per page: black on white,
// a 7 cm QR code with high error correction, readable and scannable in grayscale (NFR-12).
func (s QRSigner) RenderPDF(tickets []PrintableTicket) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	regular, _ := fontFiles.ReadFile("fonts/DejaVuSans.ttf")
	bold, _ := fontFiles.ReadFile("fonts/DejaVuSans-Bold.ttf")
	pdf.AddUTF8FontFromBytes("dejavu", "", regular)
	pdf.AddUTF8FontFromBytes("dejavu", "B", bold)
	pdf.SetTitle("BiletFlow tickets", true)
	pdf.SetCreator("BiletFlow", true)
	for _, t := range tickets {
		pdf.AddPage()
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetLineWidth(0.4)
		pdf.Rect(15, 15, 180, 150, "D")

		pdf.SetFont("dejavu", "B", 11)
		pdf.SetXY(22, 22)
		pdf.CellFormat(100, 6, "BILETFLOW · ADMISSION TICKET", "", 0, "L", false, 0, "")

		pdf.SetFont("dejavu", "B", 20)
		pdf.SetXY(22, 32)
		pdf.MultiCell(95, 9, t.EventTitle, "", "L", false)

		loc, err := time.LoadLocation(t.Timezone)
		if err != nil {
			loc = time.UTC
		}
		y := pdf.GetY() + 4
		line := func(label, value string) {
			pdf.SetFont("dejavu", "", 9)
			pdf.SetXY(22, y)
			pdf.CellFormat(95, 5, strings.ToUpper(label), "", 0, "L", false, 0, "")
			pdf.SetFont("dejavu", "B", 12)
			pdf.SetXY(22, y+5)
			pdf.MultiCell(95, 6, value, "", "L", false)
			y = pdf.GetY() + 3
		}
		line("Date / Күні / Дата", t.StartsAt.In(loc).Format("Mon 02 Jan 2006, 15:04")+" ("+t.Timezone+")")
		venue := t.Venue
		if t.City != "" {
			venue += ", " + t.City
		}
		line("Venue / Орны / Место", venue)
		line("Ticket / Билет", t.TicketType)
		line("Attendee / Қатысушы / Участник", t.AttendeeName)
		line("Order / Тапсырыс / Заказ", t.OrderNumber)

		png, err := qrcode.Encode(s.Payload(t.ID), qrcode.High, 1024)
		if err != nil {
			return nil, err
		}
		name := "qr-" + t.ID
		pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
		pdf.ImageOptions(name, 120, 30, 70, 70, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		pdf.SetFont("dejavu", "B", 14)
		pdf.SetXY(120, 102)
		pdf.CellFormat(70, 7, t.Code, "", 0, "C", false, 0, "")
		if t.Status != "valid" && t.Status != "checked_in" {
			pdf.SetFont("dejavu", "B", 16)
			pdf.SetXY(120, 112)
			pdf.CellFormat(70, 8, "NOT VALID ("+strings.ToUpper(t.Status)+")", "1", 0, "C", false, 0, "")
		}

		pdf.SetFont("dejavu", "", 9)
		pdf.SetXY(22, 140)
		pdf.MultiCell(166, 4.5, "One admission. Printed and phone copies are the same ticket: the first scan admits, later scans are refused. "+
			"Бір рет кіру: басылған және телефондағы көшірме — бір билет. Один вход: печатная и электронная копии — один билет.", "", "L", false)
	}
	if len(tickets) == 0 {
		return nil, fmt.Errorf("no tickets to render")
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
