package commerce

import (
	"net/http"
	"strconv"

	"github.com/biletflow/backend-go/internal/platform"
)

// RegisterRoutes wires checkout, orders, refunds and activation endpoints.
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/checkout/reserve", platform.Handler(s.hReserve))
	mux.Handle("POST /api/checkout/{orderId}/pay", platform.Handler(s.hPay))
	mux.Handle("POST /api/checkout/{orderId}/abandon", platform.Handler(s.hAbandon))

	mux.Handle("GET /api/orders", platform.Handler(s.hMyOrders))
	mux.Handle("GET /api/orders/{orderId}", platform.Handler(s.hOrder))
	mux.Handle("GET /api/orders/{orderId}/tickets.pdf", platform.Handler(s.hPDF))
	mux.Handle("GET /api/orders/{orderId}/tickets/{ticketId}/pdf", platform.Handler(s.hPDF))
	mux.Handle("POST /api/orders/{orderId}/refund", platform.Handler(s.hRefund))
	mux.Handle("POST /api/orders/{orderId}/cancel", platform.Handler(s.hCancel))
	mux.Handle("GET /api/events/{eventId}/orders", platform.Handler(s.hEventOrders))
	mux.Handle("POST /api/events/{eventId}/cancel", platform.Handler(s.hCancelEvent))

	mux.Handle("GET /api/organizer/payment-setup", platform.Handler(s.hSetup))
	mux.Handle("PUT /api/organizer/payment-setup/identity", platform.Handler(s.hIdentity))
	mux.Handle("PUT /api/organizer/payment-setup/payout", platform.Handler(s.hPayout))
	mux.Handle("GET /api/events/{eventId}/activation", platform.Handler(s.hChecklist))
	mux.Handle("POST /api/events/{eventId}/activation/terms", platform.Handler(s.hTerms))
	mux.Handle("POST /api/events/{eventId}/activation/fee", platform.Handler(s.hFee))
	mux.Handle("POST /api/events/{eventId}/activation/activate", platform.Handler(s.hActivate))
}

func (s *Service) respondOrder(w http.ResponseWriter, r *http.Request, status int, orderID string) error {
	p, _ := platform.Viewer(r)
	o, err := s.LoadOrder(r.Context(), s.DB, orderID, p)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, status, o)
	return nil
}

func (s *Service) hReserve(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	if !s.Limits.Checkout.Allow(platform.ClientKey(r)) {
		return platform.ErrRateLimited
	}
	var in ReserveInput
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	id, err := s.Reserve(r.Context(), p, in)
	if err != nil {
		return err
	}
	return s.respondOrder(w, r, http.StatusCreated, id)
}

func (s *Service) hPay(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	if !s.Limits.Pay.Allow(platform.ClientKey(r)) {
		return platform.ErrRateLimited
	}
	var in struct {
		CardNumber string     `json:"cardNumber"`
		Attendees  []Attendee `json:"attendees,omitempty"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	id, err := platform.PathID(r, "orderId", "Order")
	if err != nil {
		return err
	}
	if err := s.Pay(r.Context(), p, id, in.CardNumber, in.Attendees); err != nil {
		return err
	}
	return s.respondOrder(w, r, http.StatusOK, id)
}

func (s *Service) hAbandon(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	id, err := platform.PathID(r, "orderId", "Order")
	if err != nil {
		return err
	}
	if err := s.Abandon(r.Context(), p, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) hMyOrders(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	list, err := s.MyOrders(r.Context(), p)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, list)
	return nil
}

func (s *Service) hOrder(w http.ResponseWriter, r *http.Request) error {
	if _, err := platform.Viewer(r); err != nil {
		return err
	}
	return s.respondOrder(w, r, http.StatusOK, r.PathValue("orderId"))
}

func (s *Service) hPDF(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	pdf, name, err := s.TicketPDF(r.Context(), p, r.PathValue("orderId"), r.PathValue("ticketId"))
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.Header().Set("Cache-Control", "no-store")
	_, err = w.Write(pdf)
	return err
}

func (s *Service) hRefund(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	o, err := s.Refund(r.Context(), p, r.PathValue("orderId"), in.Reason)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, o)
	return nil
}

func (s *Service) hCancel(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	o, err := s.CancelRegistration(r.Context(), p, r.PathValue("orderId"))
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, o)
	return nil
}

func (s *Service) hEventOrders(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	list, err := s.EventOrders(r.Context(), p, r.PathValue("eventId"), r.URL.Query().Get("q"), r.URL.Query().Get("status"))
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, list)
	return nil
}

func (s *Service) hCancelEvent(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	if err := s.CancelEvent(r.Context(), p, r.PathValue("eventId"), in.Reason); err != nil {
		return err
	}
	e, err := s.Events.Get(r.Context(), p, r.PathValue("eventId"), "")
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, e)
	return nil
}

func (s *Service) hSetup(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	if err := requireOrganizer(p); err != nil {
		return err
	}
	setup, err := s.Setup(r.Context(), p.ID)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, setup)
	return nil
}

func (s *Service) hIdentity(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	var in struct {
		LegalName string `json:"legalName"`
		IIN       string `json:"iin"`
		Consent   bool   `json:"consent"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	setup, err := s.VerifyIdentity(r.Context(), p, in.LegalName, in.IIN, in.Consent)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, setup)
	return nil
}

func (s *Service) hPayout(w http.ResponseWriter, r *http.Request) error {
	p, err := platform.Viewer(r)
	if err != nil {
		return err
	}
	var in struct {
		BankName      string `json:"bankName"`
		AccountHolder string `json:"accountHolder"`
		IBAN          string `json:"iban"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	setup, err := s.SetPayout(r.Context(), p, in.BankName, in.AccountHolder, in.IBAN)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, http.StatusOK, setup)
	return nil
}

func (s *Service) checklistHandler(fn func(*Service, *http.Request) (*Checklist, error)) platform.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if _, err := platform.Viewer(r); err != nil {
			return err
		}
		c, err := fn(s, r)
		if err != nil {
			return err
		}
		platform.WriteJSON(w, http.StatusOK, c)
		return nil
	}
}

func (s *Service) hChecklist(w http.ResponseWriter, r *http.Request) error {
	return s.checklistHandler(func(s *Service, r *http.Request) (*Checklist, error) {
		return s.Checklist(r.Context(), platform.OptionalViewer(r), r.PathValue("eventId"))
	})(w, r)
}

func (s *Service) hTerms(w http.ResponseWriter, r *http.Request) error {
	return s.checklistHandler(func(s *Service, r *http.Request) (*Checklist, error) {
		return s.AcceptTerms(r.Context(), platform.OptionalViewer(r), r.PathValue("eventId"))
	})(w, r)
}

func (s *Service) hFee(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CardNumber string `json:"cardNumber"`
	}
	if err := platform.Decode(r, &in); err != nil {
		return err
	}
	return s.checklistHandler(func(s *Service, r *http.Request) (*Checklist, error) {
		return s.PayFee(r.Context(), platform.OptionalViewer(r), r.PathValue("eventId"), in.CardNumber)
	})(w, r)
}

func (s *Service) hActivate(w http.ResponseWriter, r *http.Request) error {
	return s.checklistHandler(func(s *Service, r *http.Request) (*Checklist, error) {
		return s.Activate(r.Context(), platform.OptionalViewer(r), r.PathValue("eventId"))
	})(w, r)
}
