package internal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
)

func TestSignOrderEvent_VerifiableWithSameKey(t *testing.T) {
	key := []byte("test-key")
	payload := []byte(`{"order_id":"ord-1"}`)

	sig := signOrderEvent(key, "OrderCreated", payload)

	if sig != signOrderEvent(key, "OrderCreated", payload) {
		t.Fatal("signature is not deterministic for identical inputs")
	}
	if sig == signOrderEvent(key, "OrderConfirmed", payload) {
		t.Fatal("signature must depend on event type")
	}
	if sig == signOrderEvent(key, "OrderCreated", []byte(`{"order_id":"ord-2"}`)) {
		t.Fatal("signature must depend on payload bytes")
	}
	if sig == signOrderEvent([]byte("other-key"), "OrderCreated", payload) {
		t.Fatal("signature must depend on the signing key")
	}
}

func TestOrderEventSigningKey_FallsBackToDevKeyWhenUnset(t *testing.T) {
	t.Setenv(orderEventSigningKeyEnv, "")
	if got := string(orderEventSigningKey()); got != devOrderEventSigningKey {
		t.Fatalf("orderEventSigningKey() = %q, want dev fallback %q", got, devOrderEventSigningKey)
	}
}

func TestOrderEventSigningKey_UsesEnvWhenSet(t *testing.T) {
	t.Setenv(orderEventSigningKeyEnv, "custom-key")
	if got := string(orderEventSigningKey()); got != "custom-key" {
		t.Fatalf("orderEventSigningKey() = %q, want %q", got, "custom-key")
	}
}

// TestSignedOrderEnvelope_ProducesVerifiableSignature proves a consumer can
// recompute the signature from the exact "Order" bytes it receives.
func TestSignedOrderEnvelope_ProducesVerifiableSignature(t *testing.T) {
	t.Setenv(orderEventSigningKeyEnv, "test-order-signing-key")

	order := map[string]any{
		"outbox_event_id": "evt-1",
		"order_id":        "ord-1",
		"status":          "PENDING",
	}

	raw, err := signedOrderEnvelope("OrderCreated", order)
	if err != nil {
		t.Fatalf("signedOrderEnvelope failed: %v", err)
	}

	var envelope struct {
		Type      string          `json:"Type"`
		Order     json.RawMessage `json:"Order"`
		Signature string          `json:"Signature"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}
	if envelope.Type != "OrderCreated" {
		t.Fatalf("Type = %q, want OrderCreated", envelope.Type)
	}
	if envelope.Signature == "" {
		t.Fatal("Signature must not be empty")
	}

	want := signOrderEvent(orderEventSigningKey(), "OrderCreated", envelope.Order)
	if envelope.Signature != want {
		t.Fatalf("Signature = %q, want %q (recomputed over the exact Order bytes)", envelope.Signature, want)
	}

	if !bytesEqualIgnoringWhitespace(envelope.Order, mustMarshal(t, order)) {
		t.Fatalf("Order field does not carry the original payload verbatim: got %s", envelope.Order)
	}
}

func signTestInventoryPayload(t *testing.T, key []byte, eventType, aggregateID string, aggregateVersion int64, signedPayload string) string {
	t.Helper()
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(eventType))
	mac.Write([]byte("|"))
	mac.Write([]byte(aggregateID))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(aggregateVersion, 10)))
	mac.Write([]byte("|"))
	mac.Write([]byte(signedPayload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyInventoryEventSignature_AcceptsMatchingSignature(t *testing.T) {
	key := []byte("test-inventory-key")
	payload := `{"order_id":"ord-1","event_id":"evt-1","section_id":"A","seat_id":"A-01"}`
	sig := signTestInventoryPayload(t, key, "SeatReservationHeld", "seat:evt-1:A:A-01", 3, payload)

	if !verifyInventoryEventSignature(key, "SeatReservationHeld", "seat:evt-1:A:A-01", 3, payload, sig, "ord-1") {
		t.Fatal("expected valid signature to verify")
	}
}

func TestVerifyInventoryEventSignature_RejectsMissingSignature(t *testing.T) {
	key := []byte("test-inventory-key")
	payload := `{"order_id":"ord-1"}`
	if verifyInventoryEventSignature(key, "SeatReservationHeld", "seat:evt-1:A:A-01", 3, payload, "", "ord-1") {
		t.Fatal("expected missing signature to be rejected")
	}
}

func TestVerifyInventoryEventSignature_RejectsTamperedPayload(t *testing.T) {
	key := []byte("test-inventory-key")
	signed := `{"order_id":"ord-1"}`
	sig := signTestInventoryPayload(t, key, "SeatReservationHeld", "seat:evt-1:A:A-01", 3, signed)

	tampered := `{"order_id":"ord-2"}`
	if verifyInventoryEventSignature(key, "SeatReservationHeld", "seat:evt-1:A:A-01", 3, tampered, sig, "ord-2") {
		t.Fatal("expected tampered payload to be rejected")
	}
}

func TestVerifyInventoryEventSignature_RejectsOrderIDSwap(t *testing.T) {
	key := []byte("test-inventory-key")
	payload := `{"order_id":"ord-1"}`
	sig := signTestInventoryPayload(t, key, "SeatReservationHeld", "seat:evt-1:A:A-01", 3, payload)

	if verifyInventoryEventSignature(key, "SeatReservationHeld", "seat:evt-1:A:A-01", 3, payload, sig, "ord-attacker") {
		t.Fatal("expected order_id mismatch against the signed payload to be rejected")
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return b
}

func bytesEqualIgnoringWhitespace(a, b []byte) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	aj, _ := json.Marshal(av)
	bj, _ := json.Marshal(bv)
	return string(aj) == string(bj)
}
