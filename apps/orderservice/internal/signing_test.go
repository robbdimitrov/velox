package internal

import (
	"encoding/json"
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
