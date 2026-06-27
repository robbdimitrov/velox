package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
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
			slog.Error("fetch error", "topic", t, "partition", p, "error", err)
		})

		fetches.EachRecord(func(r *kgo.Record) {
			var eventType string
			var reqID string
			for _, h := range r.Headers {
				if h.Key == "event_type" {
					eventType = string(h.Value)
				}
				if h.Key == "X-Request-ID" {
					reqID = string(h.Value)
				}
			}

			recordCtx := ctx
			if reqID != "" {
				recordCtx = context.WithValue(ctx, "request_id", reqID)
			}

			var payload map[string]interface{}
			if err := json.Unmarshal(r.Value, &payload); err != nil {
				slog.Error("invalid payload JSON", "error", err)
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
				switch eventType {
				case "SeatReserved":
					handleSeatReserved(recordCtx, db, orderID)
				case "SeatReservationFailed":
					handleSeatReservationFailed(recordCtx, db, orderID)
				}
			}
		})
	}
}

func handleSeatReserved(ctx context.Context, db *sql.DB, orderID string) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin tx", "error", err)
		return
	}
	defer tx.Rollback()

	var totalAmount int64
	var eventIDStr string
	err = tx.QueryRowContext(ctx, `
		SELECT o.total_amount_minor, s.event_id 
		FROM orders.orders o 
		JOIN orders.order_seats s ON s.order_id = o.id 
		WHERE o.id = $1 LIMIT 1
	`, orderID).Scan(&totalAmount, &eventIDStr)
	if err != nil {
		slog.Error("failed to get order total amount and event id", "error", err)
		return
	}

	_, err = tx.ExecContext(ctx, `UPDATE orders.orders SET status = 'CONFIRMED', updated_at = now() WHERE id = $1`, orderID)
	if err != nil {
		slog.Error("failed to update CONFIRMED", "error", err)
		return
	}

	eventID := uuid.New().String()
	envelope := map[string]any{
		"Type": "OrderConfirmed",
		"Order": map[string]any{
			"order_id":           orderID,
			"event_id":           eventIDStr,
			"status":             "CONFIRMED",
			"total_amount_minor": totalAmount,
		},
	}
	payloadBytes, _ := json.Marshal(envelope)

	headers := map[string]string{}
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		headers["X-Request-ID"] = reqID
	}
	headersBytes, _ := json.Marshal(headers)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
		VALUES ($1, 'order', $2, 'OrderConfirmed', $3, $4)
	`, eventID, orderID, payloadBytes, headersBytes)
	if err != nil {
		slog.Error("failed to insert outbox event", "error", err)
		return
	}

	tx.Commit()
}

func handleSeatReservationFailed(ctx context.Context, db *sql.DB, orderID string) {
	_, err := db.ExecContext(ctx, `UPDATE orders.orders SET status = 'FAILED', updated_at = now() WHERE id = $1`, orderID)
	if err != nil {
		slog.Error("failed to update FAILED", "error", err)
	}
}
