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

type DatabaseStore struct {
	db *sql.DB
}

func OpenDatabaseStore(ctx context.Context, databaseURL string) (*DatabaseStore, error) {
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
	return &DatabaseStore{db: db}, nil
}

func (s *DatabaseStore) Close() error {
	return s.db.Close()
}

func (s *DatabaseStore) ListOrders(ctx context.Context, user User) ([]Order, error) {
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

func (s *DatabaseStore) GetOrder(ctx context.Context, user User, orderID string) (Order, error) {
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

func (s *DatabaseStore) ListSeats(ctx context.Context, eventID, sectionID string) ([]Seat, error) {
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

func (s *DatabaseStore) GetOrganizerInventory(ctx context.Context, eventID string) (map[string]int, map[string]map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT section_id, status, COUNT(*)
		FROM projection.seat_snapshots
		WHERE event_id = $1
		GROUP BY section_id, status
	`, eventID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	counts := map[string]int{StatusAvailable: 0, StatusHeld: 0, StatusSold: 0}
	sectionCounts := make(map[string]map[string]int)
	for rows.Next() {
		var sectionID, status string
		var count int
		if err := rows.Scan(&sectionID, &status, &count); err != nil {
			return nil, nil, err
		}
		counts[status] += count
		if sectionCounts[sectionID] == nil {
			sectionCounts[sectionID] = map[string]int{StatusAvailable: 0, StatusHeld: 0, StatusSold: 0}
		}
		sectionCounts[sectionID][status] = count
	}
	return counts, sectionCounts, rows.Err()
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

func (s *DatabaseStore) ListenSeatUpdates(ctx context.Context, handler func(payload string)) error {
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

type OrganizerMetrics struct {
	TotalReservations   int64          `json:"totalReservations"`
	ActiveHolds         int            `json:"activeHolds"`
	SeatsRemaining      int            `json:"seatsRemaining"`
	DemandScore         int            `json:"demandScore"`
	ProjectionLagMs     int64          `json:"projectionLagMs"`
	SectionAvailability map[string]int `json:"sectionAvailability"`
}

func (s *DatabaseStore) GetOrganizerMetrics(ctx context.Context, eventID string) (OrganizerMetrics, error) {
	var metrics OrganizerMetrics
	
	// Get total reservations from order_summaries
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM projection.order_summaries
		WHERE event_id = $1 AND status = 'CONFIRMED'
	`, eventID).Scan(&metrics.TotalReservations)
	if err != nil {
		return metrics, err
	}

	// Get inventory counts
	counts, sectionCounts, err := s.GetOrganizerInventory(ctx, eventID)
	if err != nil {
		return metrics, err
	}
	
	metrics.ActiveHolds = counts[StatusHeld]
	metrics.SeatsRemaining = counts[StatusAvailable]
	
	metrics.SectionAvailability = make(map[string]int)
	for sec, sc := range sectionCounts {
		total := sc[StatusAvailable] + sc[StatusHeld] + sc[StatusSold]
		if total > 0 {
			metrics.SectionAvailability[sec] = (sc[StatusAvailable] * 100) / total
		} else {
			metrics.SectionAvailability[sec] = 0
		}
	}
	
	// Compute fake demand score and lag for demo
	metrics.DemandScore = 98
	metrics.ProjectionLagMs = 12

	return metrics, nil
}

func (s *DatabaseStore) ListenOrganizerUpdates(ctx context.Context, handler func(payload string)) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		pgxConn := driverConn.(*stdlib.Conn).Conn()
		_, err := pgxConn.Exec(ctx, "LISTEN organizer_updates")
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

func (s *DatabaseStore) CreateUser(ctx context.Context, id, email, passwordHash, role string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO catalog.users (id, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, password_hash, role, created_at
	`, id, email, passwordHash, role).Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	return u, err
}

func (s *DatabaseStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, role, created_at
		FROM catalog.users
		WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrStoreNotFound
	}
	return u, err
}

func (s *DatabaseStore) GetUserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, role, created_at
		FROM catalog.users
		WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrStoreNotFound
	}
	return u, err
}

func (s *DatabaseStore) CreateVenue(ctx context.Context, userID string, venue Venue) (Venue, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Venue{}, err
	}
	defer rollback(tx)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO catalog.venues (id, name, city, address, capacity)
		VALUES ($1, $2, $3, $4, $5)
	`, venue.ID, venue.Name, venue.City, venue.Address, venue.Capacity)
	if err != nil {
		return Venue{}, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO catalog.user_venues (user_id, venue_id, venue_role)
		VALUES ($1, $2, 'owner')
	`, userID, venue.ID)
	if err != nil {
		return Venue{}, err
	}

	if err := tx.Commit(); err != nil {
		return Venue{}, err
	}
	return venue, nil
}

func (s *DatabaseStore) GetOrganizerVenues(ctx context.Context, userID string) ([]Venue, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT v.id, v.name, v.city, v.address, v.capacity
		FROM catalog.venues v
		JOIN catalog.user_venues uv ON v.id = uv.venue_id
		WHERE uv.user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var venues []Venue
	for rows.Next() {
		var v Venue
		if err := rows.Scan(&v.ID, &v.Name, &v.City, &v.Address, &v.Capacity); err != nil {
			return nil, err
		}
		venues = append(venues, v)
	}
	return venues, rows.Err()
}

func (s *DatabaseStore) GetVenueStaff(ctx context.Context, venueID string) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.email, u.role
		FROM catalog.users u
		JOIN catalog.user_venues uv ON u.id = uv.user_id
		WHERE uv.venue_id = $1
	`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var staff []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Role); err != nil {
			return nil, err
		}
		staff = append(staff, u)
	}
	return staff, rows.Err()
}

func (s *DatabaseStore) CreateEvent(ctx context.Context, event Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO catalog.events (id, venue_id, name, starts_at, status)
		VALUES ($1, $2, $3, $4, $5)
	`, event.ID, event.VenueID, event.Name, event.StartsAt, event.Status)
	if err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT section_id, seat_id FROM catalog.venue_seats WHERE venue_id = $1
	`, event.VenueID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var seats []struct{ SectionID, SeatID string }
	for rows.Next() {
		var st struct{ SectionID, SeatID string }
		if err := rows.Scan(&st.SectionID, &st.SeatID); err != nil {
			return err
		}
		seats = append(seats, st)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, st := range seats {
		streamKey := "seat-" + event.ID + "-" + st.SeatID
		_, err = tx.ExecContext(ctx, `
			INSERT INTO inventory.event_streams (stream_key, event_id, section_id, seat_id)
			VALUES ($1, $2, $3, $4)
		`, streamKey, event.ID, st.SectionID, st.SeatID)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO projection.seat_snapshots (event_id, section_id, seat_id, status, aggregate_version, price_amount_minor)
			VALUES ($1, $2, $3, 'AVAILABLE', 0, 5000)
		`, event.ID, st.SectionID, st.SeatID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *DatabaseStore) GetEvents(ctx context.Context) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, venue_id, name, starts_at, status
		FROM catalog.events
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.VenueID, &e.Name, &e.StartsAt, &e.Status); err != nil {
			return nil, err
		}
		
		counts, _, _ := s.GetOrganizerInventory(ctx, e.ID)
		e.SeatsOpen = counts[StatusAvailable]
		e.SeatsTotal = counts[StatusAvailable] + counts[StatusHeld] + counts[StatusSold]
		
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *DatabaseStore) GetEvent(ctx context.Context, id string) (Event, error) {
	var e Event
	err := s.db.QueryRowContext(ctx, `
		SELECT id, venue_id, name, starts_at, status
		FROM catalog.events
		WHERE id = $1
	`, id).Scan(&e.ID, &e.VenueID, &e.Name, &e.StartsAt, &e.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrStoreNotFound
	}
	if err != nil {
		return Event{}, err
	}
	counts, _, _ := s.GetOrganizerInventory(ctx, e.ID)
	e.SeatsOpen = counts[StatusAvailable]
	e.SeatsTotal = counts[StatusAvailable] + counts[StatusHeld] + counts[StatusSold]
	return e, nil
}
