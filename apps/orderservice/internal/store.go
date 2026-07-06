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

	sort.Strings(req.SeatIDs)
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

	// Re-check the event's catalog status inside this transaction rather than
	// relying solely on apigateway's earlier, unsynchronized pre-check. Note
	// this is NOT made atomic by sql.LevelSerializable: apigateway's
	// CancelEvent updates catalog.events with a bare, non-transactional
	// ExecContext at the default READ COMMITTED level, so it never
	// participates in Serializable Snapshot Isolation's conflict graph and a
	// plain SELECT here could still read a stale pre-cancellation status.
	// Instead, FOR SHARE takes a row-level lock, which Postgres enforces
	// independently of isolation level: apigateway's UPDATE always takes an
	// exclusive lock on this row while it runs, so this SELECT either blocks
	// until that UPDATE's transaction commits and then reads the
	// freshly-committed status, or (if this SELECT locks first) forces
	// apigateway's UPDATE to wait until this transaction commits or rolls
	// back. Either ordering closes the race window.
	var eventStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM catalog.events WHERE id = $1 FOR SHARE`, req.EventID).Scan(&eventStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrEventNotBookable
	} else if err != nil {
		return "", err
	}
	if eventStatus != "PUBLISHED" {
		return "", ErrEventNotBookable
	}

	orderID := uuid.New().String()

	var total int64 = 0

	type seatInfo struct {
		SeatID     string
		PriceMinor int64
	}
	var seats []seatInfo
	for _, seatID := range req.SeatIDs {
		var price int64
		err := tx.QueryRowContext(ctx, `SELECT price_amount_minor FROM projection.seat_snapshots WHERE event_id = $1 AND section_id = $2 AND seat_id = $3`, req.EventID, req.SectionID, seatID).Scan(&price)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		seats = append(seats, seatInfo{SeatID: seatID, PriceMinor: price})
		total += price
	}

	// "res_" + orderID matches the reservation ID convention apigateway
	// already assumes everywhere else (see completePendingReservation in
	// apps/apigateway/api/reservation_controller.go), so any client reading
	// the order back via GET rather than the original create response still
	// sees a reservation_id that forwardOrderAction's TrimPrefix can recover.
	reservationID := "res_" + orderID

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.orders (id, user_id, status, idempotency_key, request_hash, total_amount_minor, reservation_id)
		VALUES ($1, $2, 'PENDING', $3, $4, $5, $6)
	`, orderID, req.UserID, req.IdempotencyKey, hash[:], total, reservationID)
	if err != nil {
		return "", err
	}

	for _, seat := range seats {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO orders.order_seats (order_id, event_id, section_id, seat_id, price_amount_minor)
			VALUES ($1, $2, $3, $4, $5)
		`, orderID, req.EventID, req.SectionID, seat.SeatID, seat.PriceMinor)
		if err != nil {
			return "", err
		}
	}

	eventID := uuid.New().String()
	envelope := map[string]any{
		"Type": "OrderCreated",
		"Order": map[string]any{
			"outbox_event_id":    eventID,
			"order_id":           orderID,
			"user_id":            req.UserID,
			"event_id":           req.EventID,
			"section_id":         req.SectionID,
			"seat_ids":           req.SeatIDs,
			"reservation_id":     orderID,
			"status":             "PENDING",
			"total_amount_minor": total,
			"created_at":         time.Now(),
		},
	}
	payloadBytes, _ := json.Marshal(envelope)

	headers := map[string]string{}
	// Since we are in orderservice package 'internal', we don't have access to 'main.RequestIDKey'
	// Wait, I will use a string key or define it in 'internal'
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

// ConfirmOrder transitions an order from HELD to CONFIRMED in response to an
// explicit user confirmation. It is idempotent: confirming an already
// CONFIRMED order returns its current status rather than an error, so a
// retried confirm request never fails.
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

	var totalAmount int64
	var eventIDStr string
	if err := tx.QueryRowContext(ctx, `
		SELECT o.total_amount_minor, s.event_id
		FROM orders.orders o
		JOIN orders.order_seats s ON s.order_id = o.id
		WHERE o.id = $1 LIMIT 1
	`, orderID).Scan(&totalAmount, &eventIDStr); err != nil {
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE orders.orders SET status = 'CONFIRMED', updated_at = now() WHERE id = $1`, orderID); err != nil {
		return "", err
	}

	eventID := uuid.New().String()
	envelope := map[string]any{
		"Type": "OrderConfirmed",
		"Order": map[string]any{
			"outbox_event_id":    eventID,
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

// buildOrderCancelledEnvelope builds the JSON payload for an OrderCancelled
// outbox row. It is the single source of truth for that envelope's shape,
// shared by cancelOrderTx's per-order transaction path and
// CancelOrdersForEvent's batched per-row loop, so the two call sites can't
// silently drift apart. reason is recorded for observability, e.g.
// "EVENT_CANCELLED" for bulk/event-triggered cancels, "" for an explicit
// single-order cancel.
func buildOrderCancelledEnvelope(orderID, eventID string, totalAmount int64, reason string) (outboxEventID string, payloadBytes []byte, err error) {
	outboxEventID = uuid.New().String()
	envelope := map[string]any{
		"Type": "OrderCancelled",
		"Order": map[string]any{
			"outbox_event_id":    outboxEventID,
			"order_id":           orderID,
			"event_id":           eventID,
			"status":             "CANCELLED",
			"total_amount_minor": totalAmount,
			"reason":             reason,
		},
	}
	payloadBytes, err = json.Marshal(envelope)
	return outboxEventID, payloadBytes, err
}

// cancelOrderTx transitions orderID's row to CANCELLED and writes the
// OrderCancelled outbox row within tx, using buildOrderCancelledEnvelope for
// the payload shape. The caller must already have verified, via its own
// `SELECT ... FOR UPDATE`, that orderID is in a status that call site allows
// to cancel; this helper performs no status check of its own so it can be
// shared by every explicit single-order cancel path (PENDING/HELD only,
// currently just CancelOrder). reason is recorded on the outbox payload for
// observability; CancelOrdersForEvent's bulk path builds the same envelope
// shape via buildOrderCancelledEnvelope directly rather than calling this
// function, since its per-order transaction flow (a single batched UPDATE
// across all matched orders, one transaction for the whole event) differs
// from this function's per-order SELECT-then-UPDATE flow.
func cancelOrderTx(ctx context.Context, tx *sql.Tx, orderID, reason string) error {
	var totalAmount int64
	var eventIDStr string
	if err := tx.QueryRowContext(ctx, `
		SELECT o.total_amount_minor, s.event_id
		FROM orders.orders o
		JOIN orders.order_seats s ON s.order_id = o.id
		WHERE o.id = $1 LIMIT 1
	`, orderID).Scan(&totalAmount, &eventIDStr); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE orders.orders SET status = 'CANCELLED', updated_at = now() WHERE id = $1`, orderID); err != nil {
		return err
	}

	outboxEventID, payloadBytes, err := buildOrderCancelledEnvelope(orderID, eventIDStr, totalAmount, reason)
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

// CancelOrdersForEvent cancels every outstanding order (PENDING, HELD, or
// CONFIRMED) holding a seat for eventID, used when an organizer cancels an
// entire event. Unlike CancelOrder, CONFIRMED orders are eligible here since
// the event itself is being called off. A single batched UPDATE ... WHERE
// status IN (...) RETURNING statement is the atomic check-and-set for every
// eligible order (equivalent to the per-order SELECT ... FOR UPDATE the
// single-order path uses), so the whole event's orders transition inside one
// transaction instead of one transaction per order. It is safe to call
// repeatedly: only orders still in a cancellable state are matched and
// counted, and the EventCancelled outbox row's payload-level
// outbox_event_id is deterministically derived from eventID so seatservice's
// inventory.processed_events dedup recognizes a retry and skips reprocessing.
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
		RETURNING id, total_amount_minor
	`, eventID)
	if err != nil {
		return 0, err
	}
	type cancelledOrder struct {
		id          string
		totalAmount int64
	}
	var cancelledOrders []cancelledOrder
	for rows.Next() {
		var o cancelledOrder
		if err := rows.Scan(&o.id, &o.totalAmount); err != nil {
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
		outboxEventID, payloadBytes, err := buildOrderCancelledEnvelope(o.id, eventID, o.totalAmount, "EVENT_CANCELLED")
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

// eventCancelledDedupID derives the payload-level outbox_event_id that
// seatservice's claim_event dedups EventCancelled processing on, via
// inventory.processed_events (a TEXT-keyed table, per
// apps/database/migrations/003_seatservice_processed_events.sql). It is
// deterministic per catalog eventID rather than a fresh uuid.New() per call,
// so a retried POST /events/{id}/cancel produces the identical value and is
// recognized as a duplicate instead of always reprocessing the full
// per-seat-stream fan-out.
func eventCancelledDedupID(eventID string) string {
	return "event-cancel:" + eventID
}

// writeEventCancelledOutboxTx writes a single EventCancelled outbox row for
// eventID within tx. It reuses the "Order" envelope wrapper key deliberately:
// it matches this codebase's existing envelope convention exactly and
// requires zero parser changes downstream. The orders.outbox_events.id
// primary key stays a fresh UUID per row (a retry is a new row; only the
// payload's outbox_event_id needs to be stable), while the payload's
// outbox_event_id is eventCancelledDedupID(eventID) so retries are
// recognized as duplicates by seatservice.
func writeEventCancelledOutboxTx(ctx context.Context, tx *sql.Tx, eventID string, headersBytes []byte) error {
	outboxRowID := uuid.New().String()
	envelope := map[string]any{
		"Type": "EventCancelled",
		"Order": map[string]any{
			"outbox_event_id": eventCancelledDedupID(eventID),
			"event_id":        eventID,
		},
	}
	payloadBytes, _ := json.Marshal(envelope)

	_, err := tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
		VALUES ($1, 'catalog_event', $2, 'EventCancelled', $3, $4)
	`, outboxRowID, eventID, payloadBytes, headersBytes)
	return err
}
