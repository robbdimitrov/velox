package internal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
)

// signedInventoryPayload covers the fields common to every seatservice
// signed_payload; Held-only reservation_id/user_id aren't used here.
type signedInventoryPayload struct {
	OrderID     string `json:"order_id"`
	EventID     string `json:"event_id"`
	SectionID   string `json:"section_id"`
	SeatID      string `json:"seat_id"`
	ExpiresAtMS *int64 `json:"expires_at_ms"`
}

// Also cross-checks signed_payload's fields against Seat/CorrelationID so the
// wire's "seat" object can't be swapped out under an otherwise-valid signature.
func verifyInventoryEventSignature(key []byte, event Event) bool {
	if event.Signature == "" {
		return false
	}
	signatureBytes, err := hex.DecodeString(event.Signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(event.Type))
	mac.Write([]byte("|"))
	mac.Write([]byte(event.AggregateID))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(event.AggregateVersion, 10)))
	mac.Write([]byte("|"))
	mac.Write([]byte(event.SignedPayload))
	if !hmac.Equal(mac.Sum(nil), signatureBytes) {
		return false
	}

	var payload signedInventoryPayload
	if err := json.Unmarshal([]byte(event.SignedPayload), &payload); err != nil {
		return false
	}
	// SeatReservationCancelled correlates to the catalog event being cancelled,
	// not the seat's own order, so its payload.OrderID legitimately differs
	// from event.CorrelationID (see seatservice's cancelled_payload).
	if event.Type != "SeatReservationCancelled" && payload.OrderID != event.CorrelationID {
		return false
	}
	if payload.EventID != event.Seat.EventID ||
		payload.SectionID != event.Seat.SectionID ||
		payload.SeatID != event.Seat.SeatID {
		return false
	}
	if payload.ExpiresAtMS != nil && *payload.ExpiresAtMS != event.Seat.ExpiresAtMS {
		return false
	}
	return true
}
