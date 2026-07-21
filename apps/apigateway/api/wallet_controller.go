package api

import (
	"net/http"
	"time"
)

const qrTokenTTL = 90 * time.Second

func (s *Server) handleWalletTickets(w http.ResponseWriter, r *http.Request, user User) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"verification_state": "VERIFIED",
			"tickets":            []WalletTicket{},
		})
		return
	}

	tickets, err := s.store.GetWalletTickets(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "wallet_unavailable")
		return
	}
	if tickets == nil {
		tickets = []WalletTicket{}
	}

	for i := range tickets {
		token, expiresAt, err := s.signQRToken(tickets[i], user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		tickets[i].QRToken = token
		tickets[i].QRTokenExpiresAt = expiresAt.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"verification_state": "VERIFIED",
		"tickets":            tickets,
	})
}

// signQRToken mints a short-lived, purpose-scoped ticket token and returns the
// exact expiry embedded in it.
func (s *Server) signQRToken(ticket WalletTicket, user User) (string, time.Time, error) {
	expiresAt := s.now().Add(qrTokenTTL)
	token, err := signHMAC(s.secret, map[string]any{
		"ticket_id": ticket.TicketID,
		"user_id":   user.ID,
		"event_id":  ticket.EventID,
		"purpose":   "qr_ticket",
		"exp":       expiresAt.Unix(),
	})
	return token, expiresAt, err
}
