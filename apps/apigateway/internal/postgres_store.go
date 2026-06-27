package internal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrStoreConflict            = errors.New("store conflict")
	ErrStoreIdempotencyConflict = errors.New("store idempotency conflict")
	ErrStoreNotFound            = errors.New("store not found")
	ErrStoreExpired             = errors.New("store expired")
)

type ReservationRequest struct {
	EventID   string
	SectionID string
	SeatIDs   []string
}

type PostgresStore struct {
	db *sql.DB
}

func OpenPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) CreateReservation(ctx context.Context, user User, idempotencyKey, requestHash string, req ReservationRequest, seats []Seat, now time.Time, ttl time.Duration) (Order, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Order{}, false, err
	}
	defer rollback(tx)

	hashBytes, err := hex.DecodeString(requestHash)
	if err != nil {
		return Order{}, false, err
	}

	var responseRef sql.NullString
	var existingHash []byte
	err = tx.QueryRowContext(ctx, `
		SELECT request_hash, response_ref
		FROM orders.idempotency_keys
		WHERE service = 'apigateway.reserve' AND user_id = $1 AND idempotency_key = $2
		FOR UPDATE
	`, user.ID, idempotencyKey).Scan(&existingHash, &responseRef)
	if err == nil {
		if hex.EncodeToString(existingHash) != requestHash {
			return Order{}, false, ErrStoreIdempotencyConflict
		}
		if responseRef.Valid {
			order, err := loadOrderAndCommit(ctx, tx, responseRef.String)
			return order, true, err
		}
		return Order{}, false, ErrStoreConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Order{}, false, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO orders.idempotency_keys (service, user_id, idempotency_key, request_hash, expires_at)
		VALUES ('apigateway.reserve', $1, $2, $3, $4)
	`, user.ID, idempotencyKey, hashBytes, now.Add(24*time.Hour)); err != nil {
		return Order{}, false, err
	}

	seatByID := make(map[string]Seat, len(seats))
	for _, seat := range seats {
		seatByID[seat.ID] = seat
	}
	seatIDs := append([]string(nil), req.SeatIDs...)
	sort.Strings(seatIDs)
	orderID, err := newUUID()
	if err != nil {
		return Order{}, false, err
	}
	reservationID := "res_" + orderID
	expiresAt := now.Add(ttl)
	total := 0

	for _, seatID := range seatIDs {
		seat, ok := seatByID[seatID]
		if !ok {
			return Order{}, false, ErrStoreNotFound
		}
		if err := ensureSeatSnapshot(ctx, tx, seat); err != nil {
			return Order{}, false, err
		}
		status, version, expiresAtDB, err := lockSeatSnapshot(ctx, tx, seat.EventID, seat.SectionID, seat.ID)
		if err != nil {
			return Order{}, false, err
		}
		if status == StatusHeld && expiresAtDB.Valid && !expiresAtDB.Time.After(now) {
			reservationID, err := seatReservationIDTx(ctx, tx, seat.EventID, seat.SectionID, seat.ID)
			if err != nil {
				return Order{}, false, err
			}
			if reservationID != "" {
				if err := expireReservationTx(ctx, tx, reservationID, now); err != nil {
					return Order{}, false, err
				}
			} else {
				version++
				if _, err := tx.ExecContext(ctx, `
					UPDATE projection.seat_snapshots
					SET status = 'AVAILABLE', aggregate_version = $4, reservation_id = NULL, held_by_user_id = NULL, expires_at = NULL, updated_at = $5
					WHERE event_id = $1 AND section_id = $2 AND seat_id = $3
				`, seat.EventID, seat.SectionID, seat.ID, version, now); err != nil {
					return Order{}, false, err
				}
			}
			status = StatusAvailable
		}
		if status != StatusAvailable {
			return Order{}, false, ErrStoreConflict
		}
		total += seat.PriceCents
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO orders.orders (
			id, user_id, status, idempotency_key, request_hash, reservation_id,
			reservation_expires_at, total_amount_minor, created_at, updated_at
		)
		VALUES ($1, $2, 'PENDING', $3, $4, $5, $6, $7, $8, $8)
	`, orderID, user.ID, idempotencyKey, hashBytes, reservationID, expiresAt, total, now); err != nil {
		return Order{}, false, err
	}

	for _, seatID := range seatIDs {
		seat := seatByID[seatID]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO orders.order_seats (order_id, event_id, section_id, seat_id, price_amount_minor)
			VALUES ($1, $2, $3, $4, $5)
		`, orderID, seat.EventID, seat.SectionID, seat.ID, seat.PriceCents); err != nil {
			return Order{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE projection.seat_snapshots
			SET status = 'HELD',
				aggregate_version = aggregate_version + 1,
				reservation_id = $4,
				held_by_user_id = $5,
				expires_at = $6,
				updated_at = $7
			WHERE event_id = $1 AND section_id = $2 AND seat_id = $3
		`, seat.EventID, seat.SectionID, seat.ID, reservationID, user.ID, expiresAt, now); err != nil {
			return Order{}, false, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO inventory.reservations (reservation_id, order_id, user_id, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'HELD', $4, $5, $5)
	`, reservationID, orderID, user.ID, expiresAt, now); err != nil {
		return Order{}, false, err
	}

	order := Order{
		ID: orderID, ReservationID: reservationID, UserID: user.ID, EventID: req.EventID,
		SectionID: req.SectionID, SeatIDs: append([]string(nil), seatIDs...), Status: OrderPending,
		TotalCents: total, ExpiresAtServerMS: expiresAt.UnixMilli(), CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli(),
	}
	if err := insertOutbox(ctx, tx, orderID, "OrderCreated", order, now); err != nil {
		return Order{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE orders.idempotency_keys
		SET response_ref = $4
		WHERE service = 'apigateway.reserve' AND user_id = $1 AND idempotency_key = $2 AND request_hash = $3
	`, user.ID, idempotencyKey, hashBytes, orderID); err != nil {
		return Order{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, false, err
	}
	return order, false, nil
}

func (s *PostgresStore) ConfirmReservation(ctx context.Context, user User, reservationID string, now time.Time) (Order, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Order{}, err
	}
	defer rollback(tx)

	var orderID string
	var expiresAt sql.NullTime
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT id::text, status, reservation_expires_at
		FROM orders.orders
		WHERE reservation_id = $1 AND user_id = $2
		FOR UPDATE
	`, reservationID, user.ID).Scan(&orderID, &status, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrStoreNotFound
	}
	if err != nil {
		return Order{}, err
	}
	if status == OrderConfirmed {
		return loadOrderAndCommit(ctx, tx, orderID)
	}
	if !expiresAt.Valid || !expiresAt.Time.After(now) {
		if err := expireReservationTx(ctx, tx, reservationID, now); err != nil {
			return Order{}, err
		}
		if err := tx.Commit(); err != nil {
			return Order{}, err
		}
		return Order{}, ErrStoreExpired
	}
	seats, err := loadOrderSeatsTx(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	for _, seat := range seats {
		status, _, _, err := lockSeatSnapshot(ctx, tx, seat.EventID, seat.SectionID, seat.ID)
		if err != nil {
			return Order{}, err
		}
		if status != StatusHeld {
			return Order{}, ErrStoreConflict
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE projection.seat_snapshots
			SET status = 'SOLD', aggregate_version = aggregate_version + 1, expires_at = NULL, updated_at = $4
			WHERE event_id = $1 AND section_id = $2 AND seat_id = $3 AND reservation_id = $5
		`, seat.EventID, seat.SectionID, seat.ID, now, reservationID)
		if err != nil {
			return Order{}, err
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return Order{}, err
		}
		if updated != 1 {
			return Order{}, ErrStoreConflict
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE orders.orders
		SET status = 'CONFIRMED', reservation_expires_at = NULL, updated_at = $2
		WHERE id = $1
	`, orderID, now); err != nil {
		return Order{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE inventory.reservations
		SET status = 'CONFIRMED', updated_at = $2
		WHERE reservation_id = $1
	`, reservationID, now); err != nil {
		return Order{}, err
	}
	order, err := loadOrderTx(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if err := insertOutbox(ctx, tx, orderID, "ReservationConfirmed", order, now); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (s *PostgresStore) ListOrders(ctx context.Context, user User) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id::text
		FROM orders.orders
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, user.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []Order
	for rows.Next() {
		var orderID string
		if err := rows.Scan(&orderID); err != nil {
			return nil, err
		}
		order, err := s.GetOrder(ctx, user, orderID)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *PostgresStore) GetOrder(ctx context.Context, user User, orderID string) (Order, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer rollback(tx)
	order, err := loadOrderTx(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if order.UserID != user.ID {
		return Order{}, ErrStoreNotFound
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (s *PostgresStore) ListSeats(ctx context.Context, eventID, sectionID string) ([]Seat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT seat_id, status, aggregate_version, COALESCE(extract(epoch from expires_at) * 1000, 0)::bigint, price_amount_minor
		FROM projection.seat_snapshots
		WHERE event_id = $1 AND section_id = $2
		ORDER BY seat_id
	`, eventID, sectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var seats []Seat
	for rows.Next() {
		var seat Seat
		seat.EventID = eventID
		seat.SectionID = sectionID
		if err := rows.Scan(&seat.ID, &seat.Status, &seat.Version, &seat.ExpiresAtServerMS, &seat.PriceCents); err != nil {
			return nil, err
		}
		seat.Row, seat.Number = splitSeatLabel(seat.ID)
		seats = append(seats, seat)
	}
	return seats, rows.Err()
}

func loadOrderAndCommit(ctx context.Context, tx *sql.Tx, orderID string) (Order, error) {
	order, err := loadOrderTx(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return order, nil
}

func loadOrderTx(ctx context.Context, tx *sql.Tx, orderID string) (Order, error) {
	var order Order
	var expiresAt sql.NullTime
	var createdAt, updatedAt time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT id::text, reservation_id, user_id, status, COALESCE(total_amount_minor, 0), reservation_expires_at, created_at, updated_at
		FROM orders.orders
		WHERE id = $1
	`, orderID).Scan(&order.ID, &order.ReservationID, &order.UserID, &order.Status, &order.TotalCents, &expiresAt, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrStoreNotFound
	}
	if err != nil {
		return Order{}, err
	}
	seats, err := loadOrderSeatsTx(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if len(seats) > 0 {
		order.EventID = seats[0].EventID
		order.SectionID = seats[0].SectionID
	}
	for _, seat := range seats {
		order.SeatIDs = append(order.SeatIDs, seat.ID)
	}
	if expiresAt.Valid {
		order.ExpiresAtServerMS = expiresAt.Time.UnixMilli()
	}
	order.CreatedAt = createdAt.UnixMilli()
	order.UpdatedAt = updatedAt.UnixMilli()
	return order, nil
}

func loadOrderSeatsTx(ctx context.Context, tx *sql.Tx, orderID string) ([]Seat, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT event_id, section_id, seat_id, price_amount_minor
		FROM orders.order_seats
		WHERE order_id = $1
		ORDER BY seat_id
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var seats []Seat
	for rows.Next() {
		var seat Seat
		if err := rows.Scan(&seat.EventID, &seat.SectionID, &seat.ID, &seat.PriceCents); err != nil {
			return nil, err
		}
		seat.Row, seat.Number = splitSeatLabel(seat.ID)
		seats = append(seats, seat)
	}
	return seats, rows.Err()
}

func ensureSeatSnapshot(ctx context.Context, tx *sql.Tx, seat Seat) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO projection.seat_snapshots (event_id, section_id, seat_id, status, aggregate_version, price_amount_minor)
		VALUES ($1, $2, $3, 'AVAILABLE', 0, $4)
		ON CONFLICT (event_id, section_id, seat_id) DO UPDATE
		SET price_amount_minor = EXCLUDED.price_amount_minor
		WHERE projection.seat_snapshots.price_amount_minor = 0
	`, seat.EventID, seat.SectionID, seat.ID, seat.PriceCents)
	return err
}

func lockSeatSnapshot(ctx context.Context, tx *sql.Tx, eventID, sectionID, seatID string) (string, int64, sql.NullTime, error) {
	var status string
	var version int64
	var expiresAt sql.NullTime
	err := tx.QueryRowContext(ctx, `
		SELECT status, aggregate_version, expires_at
		FROM projection.seat_snapshots
		WHERE event_id = $1 AND section_id = $2 AND seat_id = $3
		FOR UPDATE
	`, eventID, sectionID, seatID).Scan(&status, &version, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, sql.NullTime{}, ErrStoreNotFound
	}
	return status, version, expiresAt, err
}

func seatReservationIDTx(ctx context.Context, tx *sql.Tx, eventID, sectionID, seatID string) (string, error) {
	var reservationID sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT reservation_id
		FROM projection.seat_snapshots
		WHERE event_id = $1 AND section_id = $2 AND seat_id = $3
	`, eventID, sectionID, seatID).Scan(&reservationID)
	if err != nil {
		return "", err
	}
	if !reservationID.Valid {
		return "", nil
	}
	return reservationID.String, nil
}

func expireReservationTx(ctx context.Context, tx *sql.Tx, reservationID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE orders.orders
		SET status = 'EXPIRED', updated_at = $2
		WHERE reservation_id = $1 AND status IN ('PENDING', 'AWAITING_PAYMENT')
	`, reservationID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE inventory.reservations
		SET status = 'EXPIRED', updated_at = $2
		WHERE reservation_id = $1 AND status = 'HELD'
	`, reservationID, now); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE projection.seat_snapshots
		SET status = 'AVAILABLE',
			aggregate_version = aggregate_version + 1,
			reservation_id = NULL,
			held_by_user_id = NULL,
			expires_at = NULL,
			updated_at = $2
		WHERE reservation_id = $1 AND status = 'HELD'
	`, reservationID, now)
	return err
}

func insertOutbox(ctx context.Context, tx *sql.Tx, aggregateID, eventType string, payload any, now time.Time) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	eventID, err := newUUID()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, created_at)
		VALUES ($1, 'order', $2, $3, $4::jsonb, $5)
	`, eventID, aggregateID, eventType, body, now)
	return err
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func splitSeatLabel(seatID string) (string, int) {
	var row string
	var number int
	if _, err := fmt.Sscanf(seatID, "%1s-%d", &row, &number); err != nil {
		return "", 0
	}
	return row, number
}
