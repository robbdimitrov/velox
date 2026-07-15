package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

func (s *DatabaseStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
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

// ListSeats returns seats for a section plus the age of the staleset snapshot
// row returned, in milliseconds, so callers can surface real staleness
// (docs/infrastructure.md's snapshot_age_ms) instead of a hardcoded value.
func (s *DatabaseStore) ListSeats(ctx context.Context, eventID, sectionID string) ([]Seat, int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT seat_id, status, aggregate_version, COALESCE(extract(epoch from expires_at) * 1000, 0)::bigint, price_amount_minor,
			COALESCE(extract(epoch from (now() - updated_at)) * 1000, 0)::bigint
		FROM projection.seat_snapshots
		WHERE event_id = $1 AND section_id = $2
		ORDER BY seat_id
	`, eventID, sectionID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var seats []Seat
	var snapshotAgeMS int64
	for rows.Next() {
		var seat Seat
		var ageMS int64
		seat.EventID = eventID
		seat.SectionID = sectionID
		if err := rows.Scan(&seat.ID, &seat.Status, &seat.Version, &seat.ExpiresAtServerMS, &seat.PriceCents, &ageMS); err != nil {
			return nil, 0, err
		}
		seat.Row, seat.Number = splitSeatLabel(seat.ID)
		seats = append(seats, seat)
		if ageMS > snapshotAgeMS {
			snapshotAgeMS = ageMS
		}
	}
	return seats, snapshotAgeMS, rows.Err()
}

// GetSeatStatusMap treats projection.seat_snapshots as the seat existence
// source; an empty section is unknown, and a missing seat_id does not exist.
func (s *DatabaseStore) GetSeatStatusMap(ctx context.Context, eventID, sectionID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT seat_id, status FROM projection.seat_snapshots
		WHERE event_id = $1 AND section_id = $2
	`, eventID, sectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := map[string]string{}
	for rows.Next() {
		var seatID, status string
		if err := rows.Scan(&seatID, &status); err != nil {
			return nil, err
		}
		statuses[seatID] = status
	}
	return statuses, rows.Err()
}

// GetProjectionLagMS approximates read-model staleness from the newest seat
// snapshot timestamp; broker offset lag would require a separate poller.
func (s *DatabaseStore) GetProjectionLagMS(ctx context.Context, eventID string) (int64, error) {
	var lagMS int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(extract(epoch from (now() - MAX(updated_at))) * 1000, 0)::bigint
		FROM projection.seat_snapshots
		WHERE event_id = $1
	`, eventID).Scan(&lagMS)
	return lagMS, err
}

// GetGlobalProjectionLagMS is the same signal across all events, used for
// list endpoints that don't scope to a single event_id.
func (s *DatabaseStore) GetGlobalProjectionLagMS(ctx context.Context) (int64, error) {
	var lagMS int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(extract(epoch from (now() - MAX(updated_at))) * 1000, 0)::bigint
		FROM projection.seat_snapshots
	`).Scan(&lagMS)
	return lagMS, err
}

// GetWalletTickets returns issued projection tickets with their inventory event
// ledger. Transfer/use/upgrade producers do not exist yet.
func (s *DatabaseStore) GetWalletTickets(ctx context.Context, userID string) ([]WalletTicket, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT wt.ticket_id, wt.order_id::text, wt.event_id, wt.section_id, wt.seat_id, wt.status,
			e.name, v.name
		FROM projection.wallet_tickets wt
		JOIN catalog.events e ON e.id = wt.event_id
		JOIN catalog.venues v ON v.id = e.venue_id
		WHERE wt.user_id = $1
		ORDER BY wt.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []WalletTicket
	for rows.Next() {
		var t WalletTicket
		var orderID string
		if err := rows.Scan(&t.TicketID, &orderID, &t.EventID, &t.SectionID, &t.Seat, &t.Status, &t.Event, &t.Venue); err != nil {
			return nil, err
		}
		t.TransferStatus = "AVAILABLE"
		tickets = append(tickets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range tickets {
		ledger, err := s.getTicketLedger(ctx, tickets[i].EventID, tickets[i].SectionID, tickets[i].Seat)
		if err != nil {
			return nil, err
		}
		tickets[i].Ledger = ledger
	}
	return tickets, nil
}

// getTicketLedger queries by stream_key because event-wide cancellation uses
// the catalog event_id as correlation_id, not a single order_id.
func (s *DatabaseStore) getTicketLedger(ctx context.Context, eventID, sectionID, seatID string) ([]WalletTicketLedgerEntry, error) {
	streamKey := fmt.Sprintf("seat:%s:%s:%s", eventID, sectionID, seatID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT event_type, occurred_at, correlation_id
		FROM inventory.events
		WHERE stream_key = $1
		ORDER BY aggregate_version ASC
	`, streamKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ledger []WalletTicketLedgerEntry
	for rows.Next() {
		var entry WalletTicketLedgerEntry
		var occurredAt time.Time
		if err := rows.Scan(&entry.EventType, &occurredAt, &entry.CorrelationID); err != nil {
			return nil, err
		}
		entry.Timestamp = occurredAt.Format(time.RFC3339)
		entry.Actor = "seatservice"
		ledger = append(ledger, entry)
	}
	return ledger, rows.Err()
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
		// Must match seatservice's stream_key format; mismatches pre-create
		// orphaned rows that lazy stream creation never reuses.
		streamKey := fmt.Sprintf("seat:%s:%s:%s", event.ID, st.SectionID, st.SeatID)
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
		SELECT e.id, e.venue_id, e.name, e.starts_at, e.status, v.name, v.city
		FROM catalog.events e
		JOIN catalog.venues v ON v.id = e.venue_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.VenueID, &e.Name, &e.StartsAt, &e.Status, &e.Venue, &e.City); err != nil {
			return nil, err
		}

		counts, _, _ := s.GetOrganizerInventory(ctx, e.ID)
		e.SeatsOpen = counts[StatusAvailable]
		e.SeatsTotal = counts[StatusAvailable] + counts[StatusHeld] + counts[StatusSold]

		events = append(events, e)
	}
	return events, rows.Err()
}

// Keep GetEvent's event filter in sync with GetEventVenueID so public reads
// and ownership checks do not silently diverge.
func (s *DatabaseStore) GetEvent(ctx context.Context, id string) (Event, error) {
	var e Event
	err := s.db.QueryRowContext(ctx, `
		SELECT e.id, e.venue_id, e.name, e.starts_at, e.status, v.name, v.city
		FROM catalog.events e
		JOIN catalog.venues v ON v.id = e.venue_id
		WHERE e.id = $1
	`, id).Scan(&e.ID, &e.VenueID, &e.Name, &e.StartsAt, &e.Status, &e.Venue, &e.City)
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

// GetEventVenueID is the lean ownership-check path. Keep its event filter in
// sync with GetEvent so ownership and public reads do not diverge.
func (s *DatabaseStore) GetEventVenueID(ctx context.Context, eventID string) (string, error) {
	var venueID string
	err := s.db.QueryRowContext(ctx, `
		SELECT venue_id FROM catalog.events WHERE id = $1
	`, eventID).Scan(&venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrStoreNotFound
	}
	return venueID, err
}

// GetEventStatus is the lean booking gate path; use GetEvent when seat counts
// or other public event details are needed.
func (s *DatabaseStore) GetEventStatus(ctx context.Context, eventID string) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `
		SELECT status FROM catalog.events WHERE id = $1
	`, eventID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrStoreNotFound
	}
	return status, err
}

// CancelEvent is idempotent so handleCancelEvent can safely retry after a
// partial failure between catalog update and orderservice bulk cancellation.
func (s *DatabaseStore) CancelEvent(ctx context.Context, eventID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE catalog.events SET status = 'CANCELLED' WHERE id = $1 AND status <> 'CANCELLED'
	`, eventID)
	return err
}

func (s *DatabaseStore) CreateAnnouncement(ctx context.Context, eventID, organizerID, title, body, severity string) (EventAnnouncement, error) {
	var a EventAnnouncement
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO catalog.event_announcements (id, event_id, organizer_id, title, body, severity)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, event_id, title, body, severity, created_at
	`, uuid.New().String(), eventID, organizerID, title, body, severity).
		Scan(&a.ID, &a.EventID, &a.Title, &a.Body, &a.Severity, &a.CreatedAt)
	return a, err
}

// GetEventAnnouncements returns the newest public announcements with a fixed
// cap because feeds are small and cacheable.
func (s *DatabaseStore) GetEventAnnouncements(ctx context.Context, eventID string) ([]EventAnnouncement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, event_id, title, body, severity, created_at
		FROM catalog.event_announcements
		WHERE event_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var announcements []EventAnnouncement
	for rows.Next() {
		var a EventAnnouncement
		if err := rows.Scan(&a.ID, &a.EventID, &a.Title, &a.Body, &a.Severity, &a.CreatedAt); err != nil {
			return nil, err
		}
		announcements = append(announcements, a)
	}
	return announcements, rows.Err()
}
