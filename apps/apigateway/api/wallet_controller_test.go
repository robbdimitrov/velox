package api

import (
	"testing"
	"time"
)

func TestSignQRTokenIncludesTicketUserAndEventClaims(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	server.now = func() time.Time { return now }

	ticket := WalletTicket{
		TicketID: "tkt_1",
		EventID:  "evt_1",
	}
	user := User{ID: "usr_1"}

	token, expiresAt, err := server.signQRToken(ticket, user)
	if err != nil {
		t.Fatalf("signQRToken: %v", err)
	}
	if !expiresAt.Equal(now.Add(qrTokenTTL)) {
		t.Fatalf("expiresAt = %s, want %s", expiresAt, now.Add(qrTokenTTL))
	}

	payload, err := verifyHMAC(server.secret, token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if payload["ticket_id"] != ticket.TicketID {
		t.Fatalf("ticket_id = %v, want %s", payload["ticket_id"], ticket.TicketID)
	}
	if payload["user_id"] != user.ID {
		t.Fatalf("user_id = %v, want %s", payload["user_id"], user.ID)
	}
	if payload["event_id"] != ticket.EventID {
		t.Fatalf("event_id = %v, want %s", payload["event_id"], ticket.EventID)
	}
	if payload["purpose"] != "qr_ticket" {
		t.Fatalf("purpose = %v, want qr_ticket", payload["purpose"])
	}
	if int64(payload["exp"].(float64)) != expiresAt.Unix() {
		t.Fatalf("exp = %v, want %d", payload["exp"], expiresAt.Unix())
	}
}
