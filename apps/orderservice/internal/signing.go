package internal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
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

const eventSigningKeyEnv = "EVENT_SIGNING_KEY"

// devEventSigningKey mirrors seatservice's signing::signing_key dev fallback.
const devEventSigningKey = "velox-dev-signing-key"

func eventSigningKey() []byte {
	if key := os.Getenv(eventSigningKeyEnv); key != "" {
		return []byte(key)
	}
	return []byte(devEventSigningKey)
}

// verifyInventoryEventSignature mirrors seatservice's signing::verify.
func verifyInventoryEventSignature(key []byte, eventType, aggregateID string, aggregateVersion int64, signedPayload, signature, expectedOrderID string) bool {
	if signature == "" {
		return false
	}
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(eventType))
	mac.Write([]byte("|"))
	mac.Write([]byte(aggregateID))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(aggregateVersion, 10)))
	mac.Write([]byte("|"))
	mac.Write([]byte(signedPayload))
	if !hmac.Equal(mac.Sum(nil), signatureBytes) {
		return false
	}
	var payload struct {
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal([]byte(signedPayload), &payload); err != nil {
		return false
	}
	return payload.OrderID == expectedOrderID
}
