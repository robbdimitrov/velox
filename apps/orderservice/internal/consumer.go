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

func StartConsumer(ctx context.Context, db *sql.DB, cl *kgo.Client, health *PipelineHealth) {
	for {
		if ctx.Err() != nil {
			return
		}
		fetches := cl.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return
		}
		hadError := false
		fetches.EachError(func(t string, p int32, err error) {
			hadError = true
			health.MarkError("consumer", err)
			slog.Error("fetch error", "topic", t, "partition", p, "error", err)
		})
		if !hadError {
			health.MarkSuccess("consumer")
		}

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
				health.MarkError("consumer", err)
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
				if strings.Contains(s, "SeatReservationHeld") {
					eventType = "SeatReservationHeld"
				} else if strings.Contains(s, "SeatReservationConfirmationFailed") {
					eventType = "SeatReservationConfirmationFailed"
				} else if strings.Contains(s, "SeatReservationFailed") {
					eventType = "SeatReservationFailed"
				} else if strings.Contains(s, "SeatReservationExpired") {
					eventType = "SeatReservationExpired"
				}
			}

			if orderID != "" {
				var err error
				switch eventType {
				case "SeatReservationHeld":
					err = handleSeatReservationHeld(recordCtx, db, orderID)
				case "SeatReservationFailed":
					err = handleSeatReservationFailed(recordCtx, db, orderID)
				case "SeatReservationExpired":
					err = handleSeatReservationExpired(recordCtx, db, orderID)
				case "SeatReservationConfirmationFailed":
					reason, _ := payload["reason"].(string)
					err = handleSeatReservationConfirmationFailed(recordCtx, db, orderID, reason)
				}
				if err != nil {
					health.MarkError("consumer", err)
				} else {
					health.MarkSuccess("consumer")
				}
			}
		})
	}
}

// handleSeatReservationHeld records a successful hold; no external event is
// emitted until confirm, cancel, or expiry.
func handleSeatReservationHeld(ctx context.Context, db *sql.DB, orderID string) error {
	res, err := db.ExecContext(ctx, `UPDATE orders.orders SET status = 'HELD', updated_at = now() WHERE id = $1 AND status = 'PENDING'`, orderID)
	if err != nil {
		slog.Error("failed to update HELD", "error", err)
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		slog.Info("order not in PENDING state or not found", "order_id", orderID)
	}
	return nil
}

func handleSeatReservationFailed(ctx context.Context, db *sql.DB, orderID string) error {
	res, err := db.ExecContext(ctx, `UPDATE orders.orders SET status = 'FAILED', updated_at = now() WHERE id = $1 AND status = 'PENDING'`, orderID)
	if err != nil {
		slog.Error("failed to update FAILED", "error", err)
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		slog.Info("order not in PENDING state or not found", "order_id", orderID)
	}
	return nil
}

// handleSeatReservationExpired records seatservice timeouts without clobbering
// a confirm/cancel that already won the race.
func handleSeatReservationExpired(ctx context.Context, db *sql.DB, orderID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin tx", "error", err)
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE orders.orders SET status = 'EXPIRED', updated_at = now() WHERE id = $1 AND status IN ('PENDING', 'HELD')`, orderID)
	if err != nil {
		slog.Error("failed to update EXPIRED", "error", err)
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		slog.Info("order not in PENDING/HELD state or not found", "order_id", orderID)
		return nil
	}

	var eventIDStr string
	if err := tx.QueryRowContext(ctx, `
		SELECT event_id
		FROM orders.order_seats
		WHERE order_id = $1 LIMIT 1
	`, orderID).Scan(&eventIDStr); err != nil {
		slog.Error("failed to get order event id", "error", err)
		return err
	}

	eventID := uuid.New().String()
	envelope := map[string]any{
		"Type": "OrderExpired",
		"Order": map[string]any{
			"outbox_event_id": eventID,
			"order_id":        orderID,
			"event_id":        eventIDStr,
			"status":          "EXPIRED",
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
		VALUES ($1, 'order', $2, 'OrderExpired', $3, $4)
	`, eventID, orderID, payloadBytes, headersBytes)
	if err != nil {
		slog.Error("failed to insert outbox event", "error", err)
		return err
	}

	return tx.Commit()
}

// handleSeatReservationConfirmationFailed self-corrects a locally CONFIRMED
// order when seatservice refused the confirm append after expiry/cancellation.
// The status guard keeps this out of normal PENDING/HELD flows.
func handleSeatReservationConfirmationFailed(ctx context.Context, db *sql.DB, orderID, reason string) error {
	eventCancelled := reason == "EVENT_CANCELLED"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin tx", "error", err)
		return err
	}
	defer tx.Rollback()

	newStatus := "EXPIRED"
	if eventCancelled {
		newStatus = "CANCELLED"
	}
	res, err := tx.ExecContext(ctx, `UPDATE orders.orders SET status = $2, updated_at = now() WHERE id = $1 AND status = 'CONFIRMED'`, orderID, newStatus)
	if err != nil {
		slog.Error("failed to update order status", "error", err, "status", newStatus)
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		slog.Info("order not in CONFIRMED state or not found", "order_id", orderID)
		return nil
	}

	var eventIDStr string
	if err := tx.QueryRowContext(ctx, `
		SELECT event_id
		FROM orders.order_seats
		WHERE order_id = $1 LIMIT 1
	`, orderID).Scan(&eventIDStr); err != nil {
		slog.Error("failed to get order event id", "error", err)
		return err
	}

	headers := map[string]string{}
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		headers["X-Request-ID"] = reqID
	}
	headersBytes, _ := json.Marshal(headers)

	if eventCancelled {
		outboxEventID, payloadBytes, err := buildOrderCancelledEnvelope(orderID, eventIDStr, reason)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
			VALUES ($1, 'order', $2, 'OrderCancelled', $3, $4)
		`, outboxEventID, orderID, payloadBytes, headersBytes); err != nil {
			slog.Error("failed to insert outbox event", "error", err)
			return err
		}
		return tx.Commit()
	}

	eventID := uuid.New().String()
	envelope := map[string]any{
		"Type": "OrderExpired",
		"Order": map[string]any{
			"outbox_event_id": eventID,
			"order_id":        orderID,
			"event_id":        eventIDStr,
			"status":          "EXPIRED",
		},
	}
	payloadBytes, _ := json.Marshal(envelope)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
		VALUES ($1, 'order', $2, 'OrderExpired', $3, $4)
	`, eventID, orderID, payloadBytes, headersBytes)
	if err != nil {
		slog.Error("failed to insert outbox event", "error", err)
		return err
	}

	return tx.Commit()
}
