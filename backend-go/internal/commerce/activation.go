package commerce

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
	"github.com/biletflow/backend-go/internal/notify"
	"github.com/biletflow/backend-go/internal/platform"
)

// PaymentSetup is the organizer's simulated identity check and payout account (SRS §3.2, §4.5).
// The IIN is validated for format only and never stored; the IBAN is stored masked.
type PaymentSetup struct {
	LegalName          *string    `json:"legalName"`
	IdentityVerifiedAt *time.Time `json:"identityVerifiedAt"`
	BankName           *string    `json:"bankName"`
	AccountHolder      *string    `json:"accountHolder"`
	IBANMasked         *string    `json:"ibanMasked"`
	PayoutVerifiedAt   *time.Time `json:"payoutVerifiedAt"`
	IsSimulated        bool       `json:"isSimulated"`
}

func (s *Service) Setup(ctx context.Context, organizerID string) (PaymentSetup, error) {
	out := PaymentSetup{IsSimulated: true}
	var legal, bank, holder, iban sql.NullString
	var idAt, payAt sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT legal_name, identity_verified_at, bank_name, account_holder, iban_masked, payout_verified_at
		FROM organizer_payment_setups WHERE organizer_id=$1`, organizerID).Scan(&legal, &idAt, &bank, &holder, &iban, &payAt)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.LegalName, out.BankName, out.AccountHolder, out.IBANMasked = platform.NullString(legal), platform.NullString(bank), platform.NullString(holder), platform.NullString(iban)
	out.IdentityVerifiedAt, out.PayoutVerifiedAt = nt(idAt), nt(payAt)
	return out, nil
}

var iinPattern = regexp.MustCompile(`^\d{12}$`)
var ibanPattern = regexp.MustCompile(`^KZ\d{2}[A-Z0-9]{16}$`)

func requireOrganizer(p auth.Principal) error {
	if p.ID == "" {
		return platform.ErrUnauthenticated
	}
	if p.Role != "organizer" {
		return platform.Forbidden("ORGANIZER_REQUIRED", "Organizer role required")
	}
	return nil
}

// VerifyIdentity performs the simulated identity check (format check plus consent).
func (s *Service) VerifyIdentity(ctx context.Context, p auth.Principal, legalName, iin string, consent bool) (PaymentSetup, error) {
	if err := requireOrganizer(p); err != nil {
		return PaymentSetup{}, err
	}
	legalName = strings.TrimSpace(legalName)
	if !validName(legalName) {
		return PaymentSetup{}, platform.BadRequest("LEGAL_NAME_REQUIRED", "Enter your full legal name")
	}
	if !iinPattern.MatchString(strings.TrimSpace(iin)) {
		return PaymentSetup{}, platform.BadRequest("INVALID_IIN", "The IIN must be 12 digits")
	}
	if !consent {
		return PaymentSetup{}, platform.BadRequest("CONSENT_REQUIRED", "Consent to the identity check is required")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO organizer_payment_setups(organizer_id, legal_name, identity_verified_at) VALUES ($1,$2,now())
		ON CONFLICT (organizer_id) DO UPDATE SET legal_name=EXCLUDED.legal_name, identity_verified_at=now(), updated_at=now()`, p.ID, legalName)
	if err != nil {
		return PaymentSetup{}, err
	}
	return s.Setup(ctx, p.ID)
}

// SetPayout stores a simulated payout account; only a masked IBAN is kept.
func (s *Service) SetPayout(ctx context.Context, p auth.Principal, bank, holder, iban string) (PaymentSetup, error) {
	if err := requireOrganizer(p); err != nil {
		return PaymentSetup{}, err
	}
	bank, holder = strings.TrimSpace(bank), strings.TrimSpace(holder)
	iban = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(iban), " ", ""))
	if bank == "" || !validName(holder) {
		return PaymentSetup{}, platform.BadRequest("PAYOUT_DETAILS", "Enter the bank and the account holder")
	}
	if !ibanPattern.MatchString(iban) {
		return PaymentSetup{}, platform.BadRequest("INVALID_IBAN", "Enter a Kazakhstan IBAN: KZ followed by 18 characters")
	}
	masked := iban[:4] + strings.Repeat("•", len(iban)-8) + iban[len(iban)-4:]
	_, err := s.DB.ExecContext(ctx, `INSERT INTO organizer_payment_setups(organizer_id, bank_name, account_holder, iban_masked, payout_verified_at) VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (organizer_id) DO UPDATE SET bank_name=EXCLUDED.bank_name, account_holder=EXCLUDED.account_holder, iban_masked=EXCLUDED.iban_masked,
		payout_verified_at=now(), updated_at=now()`, p.ID, bank, holder, masked)
	if err != nil {
		return PaymentSetup{}, err
	}
	return s.Setup(ctx, p.ID)
}

// Checklist is the per-event paid-sales activation checklist (SRS §4.5).
type Checklist struct {
	EventID          string     `json:"eventId"`
	PaidSalesStatus  string     `json:"paidSalesStatus"`
	HasPaidTicket    bool       `json:"hasPaidTicketType"`
	IdentityVerified bool       `json:"identityVerified"`
	PayoutAccount    bool       `json:"payoutAccount"`
	FeeKzt           int        `json:"feeKzt"`
	FeePaid          bool       `json:"feePaid"`
	TermsAccepted    bool       `json:"termsAccepted"`
	ActivatedAt      *time.Time `json:"activatedAt"`
	Ready            bool       `json:"ready"`
	IsSimulated      bool       `json:"isSimulated"`
}

func (s *Service) Checklist(ctx context.Context, p auth.Principal, eventID string) (*Checklist, error) {
	e, err := s.Events.Managed(ctx, p, eventID)
	if err != nil {
		return nil, err
	}
	setup, err := s.Setup(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	c := &Checklist{EventID: e.ID, PaidSalesStatus: e.PaidSalesStatus, IsSimulated: true,
		IdentityVerified: setup.IdentityVerifiedAt != nil, PayoutAccount: setup.PayoutVerifiedAt != nil}
	if err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ticket_types WHERE event_id=$1 AND price_kzt > 0)`, eventID).Scan(&c.HasPaidTicket); err != nil {
		return nil, err
	}
	var fee sql.NullInt64
	var paid, terms, activated sql.NullTime
	err = s.DB.QueryRowContext(ctx, `SELECT fee_kzt, fee_paid_at, terms_accepted_at, activated_at FROM event_activations WHERE event_id=$1`, eventID).Scan(&fee, &paid, &terms, &activated)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	c.FeeKzt = platform.SettingInt(ctx, s.DB, "activation_fee_kzt", 5000)
	if fee.Valid {
		c.FeeKzt = int(fee.Int64)
	}
	c.FeePaid, c.TermsAccepted, c.ActivatedAt = paid.Valid, terms.Valid, nt(activated)
	c.Ready = c.HasPaidTicket && c.IdentityVerified && c.PayoutAccount && c.FeePaid && c.TermsAccepted
	return c, nil
}

func (s *Service) AcceptTerms(ctx context.Context, p auth.Principal, eventID string) (*Checklist, error) {
	if _, err := s.Events.Managed(ctx, p, eventID); err != nil {
		return nil, err
	}
	err := platform.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO event_activations(event_id, terms_accepted_at) VALUES ($1, now())
			ON CONFLICT (event_id) DO UPDATE SET terms_accepted_at=coalesce(event_activations.terms_accepted_at, now())`, eventID); err != nil {
			return err
		}
		return platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "activation.terms_accepted", EntityType: "event", EntityID: eventID,
			Description: "Organizer accepted the paid-sales terms"})
	})
	if err != nil {
		return nil, err
	}
	return s.Checklist(ctx, p, eventID)
}

// PayFee charges the non-refundable per-event activation fee through the simulator (decision D-2).
func (s *Service) PayFee(ctx context.Context, p auth.Principal, eventID, card string) (*Checklist, error) {
	c, err := s.Checklist(ctx, p, eventID)
	if err != nil {
		return nil, err
	}
	if c.FeePaid {
		return c, nil
	}
	charge := s.Provider.Charge(c.FeeKzt, card)
	var failure *platform.APIError
	err = platform.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		status := "failed"
		var reason any
		if charge.OK {
			status = "succeeded"
		} else {
			reason = charge.FailureReason
		}
		var paymentID string
		if err := tx.QueryRowContext(ctx, `INSERT INTO payments(kind, event_id, payer_id, amount_kzt, status, failure_reason, provider, provider_ref, card_last4)
			VALUES ('activation_fee',$1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, eventID, p.ID, c.FeeKzt, status, reason, s.Provider.Name(), charge.ProviderRef, charge.CardLast4).Scan(&paymentID); err != nil {
			return err
		}
		if !charge.OK {
			failure = platform.Err(http.StatusPaymentRequired, "PAYMENT_FAILED", FailureMessage(charge.FailureReason), "reason", charge.FailureReason)
			return nil
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO event_activations(event_id, fee_kzt, fee_payment_id, fee_paid_at) VALUES ($1,$2,$3,now())
			ON CONFLICT (event_id) DO UPDATE SET fee_kzt=EXCLUDED.fee_kzt, fee_payment_id=EXCLUDED.fee_payment_id, fee_paid_at=now()`, eventID, c.FeeKzt, paymentID); err != nil {
			return err
		}
		return platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "activation.fee_paid", EntityType: "payment", EntityID: paymentID,
			Description: "Activation fee paid (simulated)", Metadata: map[string]any{"amountKzt": c.FeeKzt}})
	})
	if err != nil {
		return nil, err
	}
	if failure != nil {
		return nil, failure
	}
	return s.Checklist(ctx, p, eventID)
}

// Activate turns on paid sales for this one event once every checklist item is done.
func (s *Service) Activate(ctx context.Context, p auth.Principal, eventID string) (*Checklist, error) {
	c, err := s.Checklist(ctx, p, eventID)
	if err != nil {
		return nil, err
	}
	if c.PaidSalesStatus == "suspended" {
		return nil, platform.Forbidden("PAID_SALES_SUSPENDED", "Paid sales were suspended by BiletFlow; contact support")
	}
	if c.PaidSalesStatus == "active" {
		return c, nil
	}
	if !c.Ready {
		return nil, platform.Conflict("CHECKLIST_INCOMPLETE", "Complete every item of the activation checklist first")
	}
	err = platform.InTx(ctx, s.DB, func(tx *sql.Tx) error {
		var title string
		if err := tx.QueryRowContext(ctx, `UPDATE events SET paid_sales_status='active', updated_at=now() WHERE id=$1 AND paid_sales_status='inactive' RETURNING title`, eventID).Scan(&title); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE event_activations SET activated_at=now(), activated_by=$2 WHERE event_id=$1`, eventID, p.ID); err != nil {
			return err
		}
		if err := platform.Audit(ctx, tx, platform.AuditEntry{EventID: eventID, ActorID: p.ID, Action: "activation.activated", EntityType: "event", EntityID: eventID,
			Description: "Paid sales activated"}); err != nil {
			return err
		}
		return notify.Queue(ctx, tx, notify.Message{UserID: p.ID, Type: "paid_sales_active", Params: map[string]any{"event": title}, Link: "/dashboard/events/" + eventID})
	})
	if err != nil {
		return nil, err
	}
	return s.Checklist(ctx, p, eventID)
}
