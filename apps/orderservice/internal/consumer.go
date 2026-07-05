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
					err = handleSeatReservationConfirmationFailed(recordCtx, db, orderID)
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

// handleSeatReservationHeld records that seatservice successfully held the
// requested seat(s). No outbox event is produced here: nothing external
// cares that a hold succeeded until the user explicitly confirms or cancels,
// or the hold expires.
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

// handleSeatReservationExpired records that seatservice's periodic expiry
// sweep timed out the hold before the user confirmed or cancelled. The
// status guard ensures a timeout that races with a user's confirm/cancel
// never clobbers that outcome.
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
	var totalAmount int64
	if err := tx.QueryRowContext(ctx, `
		SELECT s.event_id, o.total_amount_minor
		FROM orders.order_seats s
		JOIN orders.orders o ON o.id = s.order_id
		WHERE s.order_id = $1 LIMIT 1
	`, orderID).Scan(&eventIDStr, &totalAmount); err != nil {
		slog.Error("failed to get order event id", "error", err)
		return err
	}

	eventID := uuid.New().String()
	envelope := map[string]any{
		"Type": "OrderExpired",
		"Order": map[string]any{
			"outbox_event_id":    eventID,
			"order_id":           orderID,
			"event_id":           eventIDStr,
			"status":             "EXPIRED",
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
		VALUES ($1, 'order', $2, 'OrderExpired', $3, $4)
	`, eventID, orderID, payloadBytes, headersBytes)
	if err != nil {
		slog.Error("failed to insert outbox event", "error", err)
		return err
	}

	return tx.Commit()
}

// handleSeatReservationConfirmationFailed self-corrects an order that
// confirmed locally before orderservice consumed an earlier, independent
// SeatReservationExpired: seatservice's compare-and-append guard already
// refused to append SeatReservationConfirmed onto the expired stream, so this
// order's seat was never actually confirmed. The status guard restricts this
// to exactly that impossible state (CONFIRMED with no ticket) - it must never
// fire against a PENDING/HELD order, since this event is only ever emitted
// synchronously while seatservice handles that same order's OrderConfirmed.
func handleSeatReservationConfirmationFailed(ctx context.Context, db *sql.DB, orderID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin tx", "error", err)
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE orders.orders SET status = 'EXPIRED', updated_at = now() WHERE id = $1 AND status = 'CONFIRMED'`, orderID)
	if err != nil {
		slog.Error("failed to update EXPIRED", "error", err)
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
	var totalAmount int64
	if err := tx.QueryRowContext(ctx, `
		SELECT s.event_id, o.total_amount_minor
		FROM orders.order_seats s
		JOIN orders.orders o ON o.id = s.order_id
		WHERE s.order_id = $1 LIMIT 1
	`, orderID).Scan(&eventIDStr, &totalAmount); err != nil {
		slog.Error("failed to get order event id", "error", err)
		return err
	}

	eventID := uuid.New().String()
	envelope := map[string]any{
		"Type": "OrderExpired",
		"Order": map[string]any{
			"outbox_event_id":    eventID,
			"order_id":           orderID,
			"event_id":           eventIDStr,
			"status":             "EXPIRED",
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
		VALUES ($1, 'order', $2, 'OrderExpired', $3, $4)
	`, eventID, orderID, payloadBytes, headersBytes)
	if err != nil {
		slog.Error("failed to insert outbox event", "error", err)
		return err
	}

	return tx.Commit()
}
