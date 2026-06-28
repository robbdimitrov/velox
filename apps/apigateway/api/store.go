package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrStoreNotFound = errors.New("store not found")
)

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

func (s *PostgresStore) GetVendorInventory(ctx context.Context, eventID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM projection.seat_snapshots
		WHERE event_id = $1
		GROUP BY status
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{StatusAvailable: 0, StatusHeld: 0, StatusSold: 0}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

func loadOrderTx(ctx context.Context, tx *sql.Tx, orderID string) (Order, error) {
	var order Order
	var expiresAt sql.NullTime
	var createdAt, updatedAt time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT id::text, COALESCE(reservation_id, ''), user_id, status, COALESCE(total_amount_minor, 0), reservation_expires_at, created_at, updated_at
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

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

func splitSeatLabel(seatID string) (string, int) {
	var row string
	var number int
	if _, err := fmt.Sscanf(seatID, "%1s-%d", &row, &number); err != nil {
		return "", 0
	}
	return row, number
}

func (s *PostgresStore) ListenSeatUpdates(ctx context.Context, handler func(payload string)) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		pgxConn := driverConn.(*stdlib.Conn).Conn()
		_, err := pgxConn.Exec(ctx, "LISTEN seat_updates")
		if err != nil {
			return err
		}
		for {
			notification, err := pgxConn.WaitForNotification(ctx)
			if err != nil {
				return err
			}
			handler(notification.Payload)
		}
	})
}

type VendorMetrics struct {
	TotalRevenueCents int64 `json:"totalRevenueCents"`
	ActiveHolds       int   `json:"activeHolds"`
	SeatsRemaining    int   `json:"seatsRemaining"`
	DemandScore       int   `json:"demandScore"`
	ProjectionLagMs   int64 `json:"projectionLagMs"`
}

func (s *PostgresStore) GetVendorMetrics(ctx context.Context, eventID string) (VendorMetrics, error) {
	var metrics VendorMetrics
	
	// Get total revenue from order_summaries
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_amount_minor), 0)
		FROM projection.order_summaries
		WHERE event_id = $1 AND status = 'CONFIRMED'
	`, eventID).Scan(&metrics.TotalRevenueCents)
	if err != nil {
		return metrics, err
	}

	// Get inventory counts
	counts, err := s.GetVendorInventory(ctx, eventID)
	if err != nil {
		return metrics, err
	}
	
	metrics.ActiveHolds = counts[StatusHeld]
	metrics.SeatsRemaining = counts[StatusAvailable]
	
	// Compute fake demand score and lag for demo
	metrics.DemandScore = 98
	metrics.ProjectionLagMs = 12

	return metrics, nil
}

func (s *PostgresStore) ListenVendorUpdates(ctx context.Context, handler func(payload string)) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		pgxConn := driverConn.(*stdlib.Conn).Conn()
		_, err := pgxConn.Exec(ctx, "LISTEN vendor_updates")
		if err != nil {
			return err
		}
		for {
			notification, err := pgxConn.WaitForNotification(ctx)
			if err != nil {
				return err
			}
			handler(notification.Payload)
		}
	})
}
