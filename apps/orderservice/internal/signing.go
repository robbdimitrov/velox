package internal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
)

const orderEventSigningKeyEnv = "ORDER_EVENT_SIGNING_KEY"

// devOrderEventSigningKey keeps local/dev environments working without extra
// setup; seatservice's signing::order_event_signing_key shares this literal.
const devOrderEventSigningKey = "velox-dev-order-signing-key"

func orderEventSigningKey() []byte {
	if key := os.Getenv(orderEventSigningKeyEnv); key != "" {
		return []byte(key)
	}
	return []byte(devOrderEventSigningKey)
}

// signOrderEvent is HMAC-SHA256 over event_type + "|" + payload, hex-encoded;
// seatservice's signing::verify_order_event must reproduce it exactly.
func signOrderEvent(key []byte, eventType string, payload []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(eventType))
	mac.Write([]byte("|"))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// signedOrderEnvelope embeds order verbatim (via json.RawMessage) so the
// signed bytes are byte-identical to what seatservice receives under "Order".
func signedOrderEnvelope(eventType string, order map[string]any) ([]byte, error) {
	orderBytes, err := json.Marshal(order)
	if err != nil {
		return nil, err
	}
	envelope := map[string]any{
		"Type":      eventType,
		"Order":     json.RawMessage(orderBytes),
		"Signature": signOrderEvent(orderEventSigningKey(), eventType, orderBytes),
	}
	return json.Marshal(envelope)
}
