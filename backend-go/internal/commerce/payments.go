// Package commerce covers paid-sales activation, checkout, tickets, orders and refunds
// (SRS §4.4–§4.7, §4.9).
package commerce

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// ChargeResult is a payment provider's answer.
type ChargeResult struct {
	OK            bool
	ProviderRef   string
	CardLast4     string
	FailureReason string
}

// PaymentProvider is the seam where a real Kazakhstan acquirer would plug in (Strategy pattern).
type PaymentProvider interface {
	Name() string
	Charge(amountKzt int, card string) ChargeResult
	Refund(providerRef string, amountKzt int) (string, error)
}

// Simulator is a clearly labelled SIMULATION: no money moves and no card data is stored
// (SRS §3.2, §9). Test cards:
//
//	4242 4242 4242 4242 → success
//	4000 0000 0000 0002 → declined
//	4000 0000 0000 9995 → insufficient funds
type Simulator struct{}

func (Simulator) Name() string { return "biletflow-simulator" }

func randomRef(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + strings.ToUpper(hex.EncodeToString(b))
}

func (Simulator) Charge(amountKzt int, card string) ChargeResult {
	var digits strings.Builder
	for _, r := range card {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	last4 := d
	if len(d) > 4 {
		last4 = d[len(d)-4:]
	}
	res := ChargeResult{ProviderRef: randomRef("SIM-"), CardLast4: last4}
	switch {
	case amountKzt <= 0:
		res.OK = true
	case d == "4000000000000002":
		res.FailureReason = "CARD_DECLINED"
	case d == "4000000000009995":
		res.FailureReason = "INSUFFICIENT_FUNDS"
	case len(d) < 12 || len(d) > 19 || !luhn(d):
		res.FailureReason = "INVALID_CARD"
	default:
		res.OK = true
	}
	return res
}

func (Simulator) Refund(string, int) (string, error) { return randomRef("SIMR-"), nil }

func luhn(num string) bool {
	sum, double := 0, false
	for i := len(num) - 1; i >= 0; i-- {
		d := int(num[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// FailureMessage is the English text for a simulator failure code.
func FailureMessage(code string) string {
	switch code {
	case "CARD_DECLINED":
		return "Card declined (simulated)"
	case "INSUFFICIENT_FUNDS":
		return "Insufficient funds (simulated)"
	default:
		return "Invalid card number"
	}
}

// ProcessingFee is the organizer-paid fee, rounded to whole tenge (decision D-5).
func ProcessingFee(amountKzt int, percent float64) int {
	return int(float64(amountKzt)*percent/100 + 0.5)
}
