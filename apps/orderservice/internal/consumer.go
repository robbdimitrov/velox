package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

func StartConsumer(ctx context.Context, db *sql.DB, cl *kgo.Client) {
	for {
		fetches := cl.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return
		}
		fetches.EachError(func(t string, p int32, err error) {
			log.Printf("fetch error %s:%d - %v", t, p, err)
		})

		fetches.EachRecord(func(r *kgo.Record) {
			var eventType string
			for _, h := range r.Headers {
				if h.Key == "event_type" {
					eventType = string(h.Value)
				}
			}

			var payload map[string]interface{}
			if err := json.Unmarshal(r.Value, &payload); err != nil {
				log.Printf("invalid payload JSON: %v", err)
				return
			}

			orderID := ""
			if cID, ok := payload["order_id"].(string); ok {
				orderID = cID
			} else if cID, ok := payload["correlation_id"].(string); ok {
				orderID = cID
			} else if resID, ok := payload["reservation_id"].(string); ok {
				orderID = strings.TrimPrefix(resID, "res_")
			}

			if eventType == "" {
				if eType, ok := payload["event_type"].(string); ok {
					eventType = eType
				} else if eType, ok := payload["type"].(string); ok {
					eventType = eType
				}
			}

			if eventType == "" {
				s := string(r.Value)
				if strings.Contains(s, "SeatReserved") {
					eventType = "SeatReserved"
				} else if strings.Contains(s, "SeatReservationFailed") {
					eventType = "SeatReservationFailed"
				}
			}

			if orderID != "" {
				if eventType == "SeatReserved" {
					handleSeatReserved(ctx, db, orderID)
				} else if eventType == "SeatReservationFailed" {
					handleSeatReservationFailed(ctx, db, orderID)
				}
			}
		})
	}
}

func handleSeatReserved(ctx context.Context, db *sql.DB, orderID string) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("failed to begin tx: %v", err)
		return
	}
	defer tx.Rollback()

	// Simulate payment success, then update to CONFIRMED
	_, err = tx.ExecContext(ctx, `UPDATE orders.orders SET status = 'CONFIRMED', updated_at = now() WHERE id = $1`, orderID)
	if err != nil {
		log.Printf("failed to update CONFIRMED: %v", err)
		return
	}

	eventID := uuid.New().String()
	payload := map[string]string{"order_id": orderID, "status": "CONFIRMED"}
	payloadBytes, _ := json.Marshal(payload)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, 'order', $2, 'OrderConfirmed', $3)
	`, eventID, orderID, payloadBytes)
	if err != nil {
		log.Printf("failed to insert outbox event: %v", err)
		return
	}

	tx.Commit()
}

func handleSeatReservationFailed(ctx context.Context, db *sql.DB, orderID string) {
	_, err := db.ExecContext(ctx, `UPDATE orders.orders SET status = 'FAILED', updated_at = now() WHERE id = $1`, orderID)
	if err != nil {
		log.Printf("failed to update FAILED: %v", err)
	}
}
