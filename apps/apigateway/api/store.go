package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrStoreNotFound = errors.New("store not found")
)

// listResultLimit bounds unpaginated list reads until real pagination ships;
// see docs/api.md's collection pagination note.
const listResultLimit = 100

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
		LIMIT $2
	`, user.ID, listResultLimit)
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

func (s *DatabaseStore) GetOrganizerOrders(ctx context.Context, eventID string) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT o.id::text
		FROM orders.orders o
		JOIN orders.order_seats os ON os.order_id = o.id
		WHERE os.event_id = $1
		ORDER BY o.id::text
		LIMIT $2
	`, eventID, listResultLimit)
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
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		order, err := loadOrderTx(ctx, tx, orderID)
		rollback(tx)
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
		SELECT seat_id, row_label, seat_number, x, y, accessibility, status, aggregate_version,
			COALESCE(extract(epoch from expires_at) * 1000, 0)::bigint,
			COALESCE(extract(epoch from (now() - updated_at)) * 1000, 0)::bigint
		FROM projection.seat_snapshots
		WHERE event_id = $1 AND section_id = $2
		ORDER BY row_label, seat_number, seat_id
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
		if err := rows.Scan(&seat.ID, &seat.Row, &seat.Number, &seat.X, &seat.Y, &seat.Accessibility, &seat.Status, &seat.Version, &seat.ExpiresAtServerMS, &ageMS); err != nil {
			return nil, 0, err
		}
		if seat.Row == "" || seat.Number == 0 {
			seat.Row, seat.Number = splitSeatLabel(seat.ID)
		}
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
		LIMIT $2
	`, userID, listResultLimit)
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
		if t.Status == "CANCELLED" {
			t.TransferStatus = "LOCKED"
		}
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

func (s *DatabaseStore) GetWalletTicketIDsForOrder(ctx context.Context, userID, orderID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ticket_id
		FROM projection.wallet_tickets
		WHERE user_id = $1 AND order_id = $2
		ORDER BY ticket_id
	`, userID, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ticketIDs []string
	for rows.Next() {
		var ticketID string
		if err := rows.Scan(&ticketID); err != nil {
			return nil, err
		}
		ticketIDs = append(ticketIDs, ticketID)
	}
	if ticketIDs == nil {
		ticketIDs = []string{}
	}
	return ticketIDs, rows.Err()
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
	counts := map[string]int{StatusAvailable: 0, StatusHeld: 0, StatusReserved: 0}
	sectionCounts := make(map[string]map[string]int)
	for rows.Next() {
		var sectionID, status string
		var count int
		if err := rows.Scan(&sectionID, &status, &count); err != nil {
			return nil, nil, err
		}
		counts[status] += count
		if sectionCounts[sectionID] == nil {
			sectionCounts[sectionID] = map[string]int{StatusAvailable: 0, StatusHeld: 0, StatusReserved: 0}
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
		SELECT id::text, COALESCE(reservation_id, ''), user_id, status, reservation_expires_at, created_at, updated_at
		FROM orders.orders
		WHERE id = $1
	`, orderID).Scan(&order.ID, &order.ReservationID, &order.UserID, &order.Status, &expiresAt, &createdAt, &updatedAt)
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
		SELECT event_id, section_id, seat_id
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
		if err := rows.Scan(&seat.EventID, &seat.SectionID, &seat.ID); err != nil {
			return nil, err
		}
		seat.Row, seat.Number = splitSeatLabel(seat.ID)
		seats = append(seats, seat)
	}
	return seats, rows.Err()
}

// demandScore weighs confirmed reservations above active holds so a fully
// held-but-unconfirmed event doesn't outrank one with real bookings.
func demandScore(reserved, held, totalSeats int) int {
	if totalSeats <= 0 {
		return 0
	}
	return ((reserved * 100) + (held * 50)) / totalSeats
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

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
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
	reservedSeats := counts[StatusReserved]
	totalSeats := metrics.SeatsRemaining + metrics.ActiveHolds + reservedSeats

	metrics.SectionAvailability = make(map[string]int)
	for sec, sc := range sectionCounts {
		total := sc[StatusAvailable] + sc[StatusHeld] + sc[StatusReserved]
		if total > 0 {
			metrics.SectionAvailability[sec] = (sc[StatusAvailable] * 100) / total
		} else {
			metrics.SectionAvailability[sec] = 0
		}
	}

	metrics.DemandScore = demandScore(reservedSeats, metrics.ActiveHolds, totalSeats)
	metrics.ProjectionLagMs, err = s.GetProjectionLagMS(ctx, eventID)
	if err != nil {
		return metrics, err
	}

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

func (s *DatabaseStore) CreateVenue(ctx context.Context, userID string, venue Venue, sections []VenueSectionTemplate) (Venue, error) {
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

	if len(sections) == 0 {
		err = createDefaultVenueTemplateTx(ctx, tx, venue.ID)
	} else {
		err = createVenueTemplateTx(ctx, tx, venue.ID, sections)
	}
	if err != nil {
		return Venue{}, err
	}

	if err := tx.Commit(); err != nil {
		return Venue{}, err
	}
	return venue, nil
}

func createVenueTemplateTx(ctx context.Context, tx *sql.Tx, venueID string, sections []VenueSectionTemplate) error {
	for i, section := range sections {
		width := 44 + section.SeatsPerRow*42
		height := 36 + section.RowCount*42
		_, err := tx.ExecContext(ctx, `
			INSERT INTO catalog.venue_sections (
				venue_id, section_id, name, display_order, width, height
			)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (venue_id, section_id) DO UPDATE SET
				name = EXCLUDED.name,
				display_order = EXCLUDED.display_order,
				width = EXCLUDED.width,
				height = EXCLUDED.height
		`, venueID, section.SectionID, section.Name, i+1, width, height)
		if err != nil {
			return err
		}

		for rowIndex := 0; rowIndex < section.RowCount; rowIndex++ {
			rowLabel := string(rune('A' + rowIndex))
			for seatNumber := 1; seatNumber <= section.SeatsPerRow; seatNumber++ {
				seatID := fmt.Sprintf("%s-%02d", rowLabel, seatNumber)
				x := 44 + (seatNumber-1)*42
				y := 42 + rowIndex*42
				accessible := section.AccessibleEdgeSeats && (seatNumber == 1 || seatNumber == section.SeatsPerRow)
				_, err := tx.ExecContext(ctx, `
					INSERT INTO catalog.venue_seats (
						venue_id, section_id, seat_id, row_label, seat_number, x, y,
						accessibility
					)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
					ON CONFLICT (venue_id, section_id, seat_id) DO UPDATE SET
						row_label = EXCLUDED.row_label,
						seat_number = EXCLUDED.seat_number,
						x = EXCLUDED.x,
						y = EXCLUDED.y,
						accessibility = EXCLUDED.accessibility
				`, venueID, section.SectionID, seatID, rowLabel, seatNumber, x, y, accessible)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func createDefaultVenueTemplateTx(ctx context.Context, tx *sql.Tx, venueID string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO catalog.venue_sections (
			venue_id, section_id, name, display_order, width, height
		)
		SELECT $1, section_id, section_id || ' Section', display_order, 464, 204
		FROM (VALUES ('A', 1), ('B', 2)) AS sections(section_id, display_order)
		ON CONFLICT (venue_id, section_id) DO NOTHING
	`, venueID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		WITH generated_seats AS (
			SELECT
				section_id,
				row_label,
				seat_number,
				row_label || '-' || lpad(seat_number::text, 2, '0') AS seat_id,
				44 + (seat_number - 1) * 42 AS x,
				42 + (ascii(row_label) - ascii('A')) * 42 AS y,
				seat_number IN (1, 10) AS accessibility
			FROM (VALUES ('A'), ('B')) AS sections(section_id)
			CROSS JOIN unnest(ARRAY['A', 'B', 'C', 'D']) AS row_label
			CROSS JOIN generate_series(1, 10) AS seat_number
		)
		INSERT INTO catalog.venue_seats (
			venue_id, section_id, seat_id, row_label, seat_number, x, y,
			accessibility
		)
		SELECT $1, section_id, seat_id, row_label, seat_number, x, y,
		       accessibility
		FROM generated_seats
		ON CONFLICT (venue_id, section_id, seat_id) DO NOTHING
	`, venueID)
	return err
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
		INSERT INTO catalog.events (
			id, venue_id, name, description, category, starts_at, sale_starts_at, timezone, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, now(), $7, $8)
	`, event.ID, event.VenueID, event.Name, event.Description, event.Category, event.StartsAt, event.Timezone, event.Status)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO catalog.event_sections (
			event_id, section_id, name, display_order, width, height
		)
		SELECT $1, section_id, name, display_order, width, height
		FROM catalog.venue_sections
		WHERE venue_id = $2
	`, event.ID, event.VenueID)
	if err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT vs.section_id, vs.seat_id, vs.row_label, vs.seat_number, vs.x, vs.y,
		       vs.accessibility
		FROM catalog.venue_seats vs
		LEFT JOIN catalog.event_sections es
			ON es.event_id = $2 AND es.section_id = vs.section_id
		WHERE vs.venue_id = $1
		ORDER BY vs.section_id, vs.row_label, vs.seat_number, vs.seat_id
	`, event.VenueID, event.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var seats []struct {
		SectionID     string
		SeatID        string
		Row           string
		Number        int
		X             int
		Y             int
		Accessibility bool
	}
	for rows.Next() {
		var st struct {
			SectionID     string
			SeatID        string
			Row           string
			Number        int
			X             int
			Y             int
			Accessibility bool
		}
		if err := rows.Scan(&st.SectionID, &st.SeatID, &st.Row, &st.Number, &st.X, &st.Y, &st.Accessibility); err != nil {
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
			INSERT INTO projection.seat_snapshots (
				event_id, section_id, seat_id, status, aggregate_version,
				row_label, seat_number, x, y, accessibility
			)
			VALUES ($1, $2, $3, 'AVAILABLE', 0, $4, $5, $6, $7, $8)
		`, event.ID, st.SectionID, st.SeatID, st.Row, st.Number, st.X, st.Y, st.Accessibility)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *DatabaseStore) GetEvents(ctx context.Context) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.venue_id, e.name, e.description, e.category,
		       e.starts_at, e.timezone, e.status, v.name, v.city,
		       COALESCE(sec.section_ids, ''),
		       COALESCE(inv.available, 0), COALESCE(inv.held, 0), COALESCE(inv.reserved, 0)
		FROM catalog.events e
		JOIN catalog.venues v ON v.id = e.venue_id
		LEFT JOIN (
			SELECT event_id, string_agg(section_id, ',' ORDER BY display_order, section_id) AS section_ids
			FROM catalog.event_sections
			GROUP BY event_id
		) sec ON sec.event_id = e.id
		LEFT JOIN (
			SELECT event_id,
			       COUNT(*) FILTER (WHERE status = 'AVAILABLE') AS available,
			       COUNT(*) FILTER (WHERE status = 'HELD') AS held,
			       COUNT(*) FILTER (WHERE status = 'RESERVED') AS reserved
			FROM projection.seat_snapshots
			GROUP BY event_id
		) inv ON inv.event_id = e.id
		ORDER BY e.starts_at ASC
		LIMIT $1
	`, listResultLimit)
	if err != nil {
		return nil, err
	}
	return scanEventRows(rows)
}

// scanEventRows shares the column layout and derived-field logic between
// GetEvents and GetOrganizerEvents, whose queries differ only by ownership join.
func scanEventRows(rows *sql.Rows) ([]Event, error) {
	defer rows.Close()
	var events []Event
	for rows.Next() {
		var e Event
		var sectionIDs string
		var held, reserved int
		if err := rows.Scan(&e.ID, &e.VenueID, &e.Name, &e.Description, &e.Category, &e.StartsAt, &e.Timezone, &e.Status, &e.Venue, &e.City, &sectionIDs, &e.SeatsOpen, &held, &reserved); err != nil {
			return nil, err
		}
		e.SectionIDs = splitCSV(sectionIDs)
		e.SeatsTotal = e.SeatsOpen + held + reserved
		e.DemandScore = demandScore(reserved, held, e.SeatsTotal)

		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *DatabaseStore) GetOrganizerEvents(ctx context.Context, userID string) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.venue_id, e.name, e.description, e.category,
		       e.starts_at, e.timezone, e.status, v.name, v.city,
		       COALESCE(sec.section_ids, ''),
		       COALESCE(inv.available, 0), COALESCE(inv.held, 0), COALESCE(inv.reserved, 0)
		FROM catalog.events e
		JOIN catalog.venues v ON v.id = e.venue_id
		JOIN catalog.user_venues uv ON uv.venue_id = e.venue_id AND uv.user_id = $1
		LEFT JOIN (
			SELECT event_id, string_agg(section_id, ',' ORDER BY display_order, section_id) AS section_ids
			FROM catalog.event_sections
			GROUP BY event_id
		) sec ON sec.event_id = e.id
		LEFT JOIN (
			SELECT event_id,
			       COUNT(*) FILTER (WHERE status = 'AVAILABLE') AS available,
			       COUNT(*) FILTER (WHERE status = 'HELD') AS held,
			       COUNT(*) FILTER (WHERE status = 'RESERVED') AS reserved
			FROM projection.seat_snapshots
			GROUP BY event_id
		) inv ON inv.event_id = e.id
		ORDER BY e.starts_at ASC
		LIMIT $2
	`, userID, listResultLimit)
	if err != nil {
		return nil, err
	}
	return scanEventRows(rows)
}

// Keep GetEvent's event filter in sync with GetEventVenueID so public reads
// and ownership checks do not silently diverge.
func (s *DatabaseStore) GetEvent(ctx context.Context, id string) (Event, error) {
	var e Event
	var sectionIDs string
	var held, reserved int
	err := s.db.QueryRowContext(ctx, `
		SELECT e.id, e.venue_id, e.name, e.description, e.category,
		       e.starts_at, e.timezone, e.status, v.name, v.city,
		       COALESCE(sec.section_ids, ''),
		       COALESCE(inv.available, 0), COALESCE(inv.held, 0), COALESCE(inv.reserved, 0)
		FROM catalog.events e
		JOIN catalog.venues v ON v.id = e.venue_id
		LEFT JOIN (
			SELECT event_id, string_agg(section_id, ',' ORDER BY display_order, section_id) AS section_ids
			FROM catalog.event_sections
			WHERE event_id = $1
			GROUP BY event_id
		) sec ON sec.event_id = e.id
		LEFT JOIN (
			SELECT event_id,
			       COUNT(*) FILTER (WHERE status = 'AVAILABLE') AS available,
			       COUNT(*) FILTER (WHERE status = 'HELD') AS held,
			       COUNT(*) FILTER (WHERE status = 'RESERVED') AS reserved
			FROM projection.seat_snapshots
			WHERE event_id = $1
			GROUP BY event_id
		) inv ON inv.event_id = e.id
		WHERE e.id = $1
	`, id).Scan(&e.ID, &e.VenueID, &e.Name, &e.Description, &e.Category, &e.StartsAt, &e.Timezone, &e.Status, &e.Venue, &e.City, &sectionIDs, &e.SeatsOpen, &held, &reserved)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrStoreNotFound
	}
	if err != nil {
		return Event{}, err
	}
	e.SectionIDs = splitCSV(sectionIDs)
	e.SeatsTotal = e.SeatsOpen + held + reserved
	e.DemandScore = demandScore(reserved, held, e.SeatsTotal)
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
		LIMIT $2
	`, eventID, listResultLimit)
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
