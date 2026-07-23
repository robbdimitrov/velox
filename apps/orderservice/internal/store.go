package internal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrIdempotencyConflict = errors.New("idempotency key conflict")
var ErrOrderNotFound = errors.New("order not found")
var ErrOrderNotConfirmable = errors.New("order not confirmable")
var ErrOrderNotCancellable = errors.New("order not cancellable")
var ErrEventNotBookable = errors.New("event not bookable")

const EventStatusPublished = "PUBLISHED"

type Store struct {
	db *sql.DB
}

func NewStore(dbURL string) (*Store, error) {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) CreateOrder(ctx context.Context, req OrderRequest) (string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	seatIDs := append([]string(nil), req.SeatIDs...)
	sort.Strings(seatIDs)
	req.SeatIDs = seatIDs
	reqBytes, _ := json.Marshal(req)
	hash := sha256.Sum256(reqBytes)

	var existingHash []byte
	var responseRef sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT request_hash, response_ref 
		FROM orders.idempotency_keys 
		WHERE service = 'orderservice' AND user_id = $1 AND idempotency_key = $2
	`, req.UserID, req.IdempotencyKey).Scan(&existingHash, &responseRef)

	if err == nil {
		if !bytes.Equal(existingHash, hash[:]) {
			return "", ErrIdempotencyConflict
		}
		if responseRef.Valid {
			return responseRef.String, nil
		}
		return "", ErrIdempotencyConflict
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.idempotency_keys (service, user_id, idempotency_key, request_hash, expires_at)
		VALUES ('orderservice', $1, $2, $3, $4)
	`, req.UserID, req.IdempotencyKey, hash[:], time.Now().Add(24*time.Hour))
	if err != nil {
		return "", err
	}

	// Re-check catalog status under a FOR SHARE row lock. Serializable alone
	// would not conflict with apigateway's non-transactional CancelEvent update.
	var eventStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM catalog.events WHERE id = $1 FOR SHARE`, req.EventID).Scan(&eventStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrEventNotBookable
	} else if err != nil {
		return "", err
	}
	if eventStatus != EventStatusPublished {
		return "", ErrEventNotBookable
	}

	orderID := uuid.New().String()

	type seatInfo struct {
		SeatID string
	}
	seats := make([]seatInfo, 0, len(seatIDs))
	for _, seatID := range seatIDs {
		seats = append(seats, seatInfo{SeatID: seatID})
	}

	// Keep reservation_id reversible for apigateway forwardOrderAction, even
	// when clients read the order via GET instead of the create response.
	reservationID := "res_" + orderID

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.orders (id, user_id, status, idempotency_key, request_hash, reservation_id)
		VALUES ($1, $2, 'PENDING', $3, $4, $5)
	`, orderID, req.UserID, req.IdempotencyKey, hash[:], reservationID)
	if err != nil {
		return "", err
	}

	for _, seat := range seats {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO orders.order_seats (order_id, event_id, section_id, seat_id, price_amount_minor, currency)
			VALUES ($1, $2, $3, $4, 0, 'USD')
		`, orderID, req.EventID, req.SectionID, seat.SeatID)
		if err != nil {
			return "", err
		}
	}

	eventID := uuid.New().String()
	orderPayload := map[string]any{
		"outbox_event_id": eventID,
		"order_id":        orderID,
		"user_id":         req.UserID,
		"event_id":        req.EventID,
		"section_id":      req.SectionID,
		"seat_ids":        req.SeatIDs,
		"reservation_id":  reservationID,
		"status":          "PENDING",
		"created_at":      time.Now(),
	}
	payloadBytes, err := signedOrderEnvelope("OrderCreated", orderPayload)
	if err != nil {
		return "", err
	}

	headers := map[string]string{}
	// Propagate request IDs from the gateway context into outbox headers.
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		headers["X-Request-ID"] = reqID
	}
	headersBytes, _ := json.Marshal(headers)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
		VALUES ($1, 'order', $2, 'OrderCreated', $3, $4)
	`, eventID, orderID, payloadBytes, headersBytes)
	if err != nil {
		return "", err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE orders.idempotency_keys 
		SET response_ref = $1 
		WHERE service = 'orderservice' AND user_id = $2 AND idempotency_key = $3
	`, orderID, req.UserID, req.IdempotencyKey)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return orderID, nil
}

// ConfirmOrder is idempotent: confirming an already CONFIRMED order returns
// its current status so retried confirms do not fail.
func (s *Store) ConfirmOrder(ctx context.Context, orderID string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM orders.orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrOrderNotFound
	} else if err != nil {
		return "", err
	}

	if status == "CONFIRMED" {
		return status, tx.Commit()
	}
	if status != "HELD" {
		return "", ErrOrderNotConfirmable
	}

	var eventIDStr string
	if err := tx.QueryRowContext(ctx, `
		SELECT event_id
		FROM orders.order_seats
		WHERE order_id = $1 LIMIT 1
	`, orderID).Scan(&eventIDStr); err != nil {
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE orders.orders SET status = 'CONFIRMED', updated_at = now() WHERE id = $1`, orderID); err != nil {
		return "", err
	}

	eventID := uuid.New().String()
	orderPayload := map[string]any{
		"outbox_event_id": eventID,
		"order_id":        orderID,
		"event_id":        eventIDStr,
		"status":          "CONFIRMED",
	}
	payloadBytes, err := signedOrderEnvelope("OrderConfirmed", orderPayload)
	if err != nil {
		return "", err
	}

	headers := map[string]string{}
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		headers["X-Request-ID"] = reqID
	}
	headersBytes, _ := json.Marshal(headers)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
		VALUES ($1, 'order', $2, 'OrderConfirmed', $3, $4)
	`, eventID, orderID, payloadBytes, headersBytes); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return "CONFIRMED", nil
}

// CancelOrder transitions an order from PENDING or HELD to CANCELLED in
// response to an explicit user cancellation. It is idempotent: cancelling an
// already CANCELLED order returns success rather than an error.
func (s *Store) CancelOrder(ctx context.Context, orderID string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM orders.orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrOrderNotFound
	} else if err != nil {
		return "", err
	}

	if status == "CANCELLED" {
		return status, tx.Commit()
	}
	if status != "PENDING" && status != "HELD" {
		return "", ErrOrderNotCancellable
	}

	if err := cancelOrderTx(ctx, tx, orderID, ""); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return "CANCELLED", nil
}

// buildOrderCancelledEnvelope is the shared payload builder for single-order
// and event-wide cancellation paths; reason is recorded for observability.
func buildOrderCancelledEnvelope(orderID, eventID string, reason string) (outboxEventID string, payloadBytes []byte, err error) {
	outboxEventID = uuid.New().String()
	orderPayload := map[string]any{
		"outbox_event_id": outboxEventID,
		"order_id":        orderID,
		"event_id":        eventID,
		"status":          "CANCELLED",
		"reason":          reason,
	}
	payloadBytes, err = signedOrderEnvelope("OrderCancelled", orderPayload)
	return outboxEventID, payloadBytes, err
}

// cancelOrderTx writes the CANCELLED state and outbox row inside tx. The caller
// must already hold the row lock and have verified the status is cancellable.
func cancelOrderTx(ctx context.Context, tx *sql.Tx, orderID, reason string) error {
	var eventIDStr string
	if err := tx.QueryRowContext(ctx, `
		SELECT event_id
		FROM orders.order_seats
		WHERE order_id = $1 LIMIT 1
	`, orderID).Scan(&eventIDStr); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE orders.orders SET status = 'CANCELLED', updated_at = now() WHERE id = $1`, orderID); err != nil {
		return err
	}

	outboxEventID, payloadBytes, err := buildOrderCancelledEnvelope(orderID, eventIDStr, reason)
	if err != nil {
		return err
	}

	headers := map[string]string{}
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		headers["X-Request-ID"] = reqID
	}
	headersBytes, _ := json.Marshal(headers)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
		VALUES ($1, 'order', $2, 'OrderCancelled', $3, $4)
	`, outboxEventID, orderID, payloadBytes, headersBytes)
	return err
}

// CancelOrdersForEvent cancels all PENDING, HELD, or CONFIRMED orders for an
// organizer-cancelled event in one transaction. It is retry-safe: only still
// cancellable orders match, and EventCancelled gets a deterministic dedup ID.
func (s *Store) CancelOrdersForEvent(ctx context.Context, eventID string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		UPDATE orders.orders
		SET status = 'CANCELLED', updated_at = now()
		WHERE id IN (SELECT DISTINCT order_id FROM orders.order_seats WHERE event_id = $1)
		  AND status IN ('PENDING', 'HELD', 'CONFIRMED')
		RETURNING id
	`, eventID)
	if err != nil {
		return 0, err
	}
	type cancelledOrder struct {
		id string
	}
	var cancelledOrders []cancelledOrder
	for rows.Next() {
		var o cancelledOrder
		if err := rows.Scan(&o.id); err != nil {
			rows.Close()
			return 0, err
		}
		cancelledOrders = append(cancelledOrders, o)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	headers := map[string]string{}
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		headers["X-Request-ID"] = reqID
	}
	headersBytes, _ := json.Marshal(headers)

	for _, o := range cancelledOrders {
		outboxEventID, payloadBytes, err := buildOrderCancelledEnvelope(o.id, eventID, "EVENT_CANCELLED")
		if err != nil {
			return 0, err
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
			VALUES ($1, 'order', $2, 'OrderCancelled', $3, $4)
		`, outboxEventID, o.id, payloadBytes, headersBytes); err != nil {
			return 0, err
		}
	}

	if err := writeEventCancelledOutboxTx(ctx, tx, eventID, headersBytes); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return len(cancelledOrders), nil
}

// eventCancelledDedupNamespace seeds deterministic EventCancelled UUIDv5 IDs.
var eventCancelledDedupNamespace = uuid.MustParse("6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b1e")

// eventCancelledDedupID returns a valid, deterministic UUID for both
// seatservice TEXT dedup and viewservice uuid-typed processed_events dedup.
func eventCancelledDedupID(eventID string) string {
	return uuid.NewSHA1(eventCancelledDedupNamespace, []byte(eventID)).String()
}

// writeEventCancelledOutboxTx writes one EventCancelled row. The row ID is
// fresh, but the payload outbox_event_id is stable so retries dedupe.
func writeEventCancelledOutboxTx(ctx context.Context, tx *sql.Tx, eventID string, headersBytes []byte) error {
	outboxRowID := uuid.New().String()
	orderPayload := map[string]any{
		"outbox_event_id": eventCancelledDedupID(eventID),
		"event_id":        eventID,
	}
	payloadBytes, err := signedOrderEnvelope("EventCancelled", orderPayload)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
		VALUES ($1, 'catalog_event', $2, 'EventCancelled', $3, $4)
	`, outboxRowID, eventID, payloadBytes, headersBytes)
	return err
}
