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
		token, expiresAt, err := s.signQRToken(tickets[i].TicketID)
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

// signQRToken mints a short-lived signed token identifying a ticket, per
// docs/frontend.md: "QR payloads must be short-lived signed tokens, never raw
// ticket IDs alone." The "purpose" claim domain-separates this from session
// tokens signed with the same secret, so one can't be replayed as the other.
// It also returns the expiry it embedded in the token, so callers display
// the same expiry the token itself actually carries rather than computing a
// second, potentially-drifting value.
func (s *Server) signQRToken(ticketID string) (string, time.Time, error) {
	expiresAt := s.now().Add(qrTokenTTL)
	token, err := signHMAC(s.secret, map[string]any{
		"ticket_id": ticketID,
		"purpose":   "qr_ticket",
		"exp":       expiresAt.Unix(),
	})
	return token, expiresAt, err
}
