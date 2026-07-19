package api

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestGetEventAnnouncementsCapsResultSet keeps the public announcements read
// bounded; LIMIT 100 must remain after ORDER BY.
func TestGetEventAnnouncementsCapsResultSet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, event_id, title, body, severity, created_at\s+FROM catalog\.event_announcements\s+WHERE event_id = \$1\s+ORDER BY created_at DESC\s+LIMIT 100`).
		WithArgs("evt_neon_riot").
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "title", "body", "severity", "created_at"}).
			AddRow("ann_1", "evt_neon_riot", "Delay", "Doors pushed back.", "INFO", time.Now()))

	store := &DatabaseStore{db: db}
	announcements, err := store.GetEventAnnouncements(context.Background(), "evt_neon_riot")
	if err != nil {
		t.Fatalf("GetEventAnnouncements: %v", err)
	}
	if len(announcements) != 1 {
		t.Fatalf("expected 1 announcement, got %d", len(announcements))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations (query likely missing LIMIT 100): %v", err)
	}
}

func TestCreateEventCopiesGeometryToInventoryAndProjectionRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	startsAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	saleStartsAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	event := Event{
		ID:           "evt_geometry",
		VenueID:      "ven_geometry",
		Name:         "Geometry Event",
		Description:  "Geometry-backed seats",
		Category:     "Theatre",
		ImageKey:     "event-zero-hour",
		StartsAt:     startsAt,
		SaleStartsAt: saleStartsAt,
		Timezone:     "America/New_York",
		Status:       EventStatusPublished,
	}

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO catalog\.events .*VALUES`).
		WithArgs(event.ID, event.VenueID, event.Name, event.Description, event.Category, event.StartsAt, event.SaleStartsAt, event.ImageKey, event.Timezone, event.Status).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO catalog\.event_sections .*FROM catalog\.venue_sections`).
		WithArgs(event.ID, event.VenueID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`(?s)SELECT vs\.section_id, vs\.seat_id, vs\.row_label, vs\.seat_number, vs\.x, vs\.y,\s+vs\.accessibility, COALESCE\(es\.price_amount_minor, 5000\).*FROM catalog\.venue_seats vs`).
		WithArgs(event.VenueID, event.ID).
		WillReturnRows(sqlmock.NewRows([]string{"section_id", "seat_id", "row_label", "seat_number", "x", "y", "accessibility", "price_amount_minor"}).
			AddRow("A", "A-01", "A", 1, 44, 42, true, 8650).
			AddRow("A", "A-02", "A", 2, 86, 42, false, 8650))
	mock.ExpectExec(`INSERT INTO inventory\.event_streams`).
		WithArgs("seat:evt_geometry:A:A-01", event.ID, "A", "A-01").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO projection\.seat_snapshots .*VALUES`).
		WithArgs(event.ID, "A", "A-01", 8650, "A", 1, 44, 42, true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO inventory\.event_streams`).
		WithArgs("seat:evt_geometry:A:A-02", event.ID, "A", "A-02").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO projection\.seat_snapshots .*VALUES`).
		WithArgs(event.ID, "A", "A-02", 8650, "A", 2, 86, 42, false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	store := &DatabaseStore{db: db}
	if err := store.CreateEvent(context.Background(), event); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestCreateVenueCreatesDefaultSeatTemplate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	venue := Venue{
		ID:       "ven_template",
		Name:     "Template Hall",
		City:     "Chicago",
		Address:  "1 Template Way",
		Capacity: 80,
	}

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO catalog\.venues`).
		WithArgs(venue.ID, venue.Name, venue.City, venue.Address, venue.Capacity).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO catalog\.user_venues`).
		WithArgs("usr_organizer", venue.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO catalog\.venue_sections .*VALUES \('A', 1\), \('B', 2\)`).
		WithArgs(venue.ID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`(?s)WITH generated_seats AS .*generate_series\(1, 10\).*INSERT INTO catalog\.venue_seats`).
		WithArgs(venue.ID).
		WillReturnResult(sqlmock.NewResult(0, 80))
	mock.ExpectCommit()

	store := &DatabaseStore{db: db}
	created, err := store.CreateVenue(context.Background(), "usr_organizer", venue)
	if err != nil {
		t.Fatalf("CreateVenue: %v", err)
	}
	if created.ID != venue.ID {
		t.Fatalf("created venue = %+v, want %+v", created, venue)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestListSeatsReturnsProjectionGeometry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`(?s)SELECT seat_id, row_label, seat_number, x, y, accessibility, status, aggregate_version,.*FROM projection\.seat_snapshots\s+WHERE event_id = \$1 AND section_id = \$2`).
		WithArgs("evt_geometry", "A").
		WillReturnRows(sqlmock.NewRows([]string{
			"seat_id", "row_label", "seat_number", "x", "y", "accessibility",
			"status", "aggregate_version", "expires_at_server_ms", "price_amount_minor", "age_ms",
		}).AddRow("A-01", "A", 1, 44, 42, true, StatusAvailable, 0, int64(0), 8650, int64(4)))

	store := &DatabaseStore{db: db}
	seats, snapshotAgeMS, err := store.ListSeats(context.Background(), "evt_geometry", "A")
	if err != nil {
		t.Fatalf("ListSeats: %v", err)
	}
	if len(seats) != 1 {
		t.Fatalf("len(seats) = %d, want 1", len(seats))
	}
	if seats[0].X != 44 || seats[0].Y != 42 || !seats[0].Accessibility {
		t.Fatalf("seat geometry not loaded from projection: %+v", seats[0])
	}
	if snapshotAgeMS != 4 {
		t.Fatalf("snapshotAgeMS = %d, want 4", snapshotAgeMS)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

// TestGetTicketLedgerQueriesByStreamKeyNotCorrelationID ensures event-wide
// cancellation entries stay visible even though their correlation_id is event-scoped.
func TestGetTicketLedgerQueriesByStreamKeyNotCorrelationID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT event_type, occurred_at, correlation_id\s+FROM inventory\.events\s+WHERE stream_key = \$1\s+ORDER BY aggregate_version ASC`).
		WithArgs("seat:evt_neon_riot:A:A-01").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "occurred_at", "correlation_id"}).
			AddRow("SeatReservationHeld", time.Now(), "ord_1").
			AddRow("SeatReservationConfirmed", time.Now(), "ord_1").
			AddRow("SeatReservationCancelled", time.Now(), "evt_neon_riot"))

	store := &DatabaseStore{db: db}
	ledger, err := store.getTicketLedger(context.Background(), "evt_neon_riot", "A", "A-01")
	if err != nil {
		t.Fatalf("getTicketLedger: %v", err)
	}
	if len(ledger) != 3 {
		t.Fatalf("expected 3 ledger entries (including the event-scoped cancellation), got %d", len(ledger))
	}
	if ledger[2].EventType != "SeatReservationCancelled" {
		t.Fatalf("expected the cancellation entry to be present, got %+v", ledger[2])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations (query likely still keyed on correlation_id): %v", err)
	}
}

// TestGetEventVenueIDReturnsVenueWithoutInventoryQuery keeps ownership checks
// from paying for unrelated inventory aggregation.
func TestGetEventVenueIDReturnsVenueWithoutInventoryQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT venue_id FROM catalog\.events WHERE id = \$1`).
		WithArgs("evt_neon_riot").
		WillReturnRows(sqlmock.NewRows([]string{"venue_id"}).AddRow("ven_northstar"))

	store := &DatabaseStore{db: db}
	venueID, err := store.GetEventVenueID(context.Background(), "evt_neon_riot")
	if err != nil {
		t.Fatalf("GetEventVenueID: %v", err)
	}
	if venueID != "ven_northstar" {
		t.Fatalf("venueID = %q, want ven_northstar", venueID)
	}

	// No other query (e.g. GetOrganizerInventory's seat_snapshots aggregation)
	// should have been issued.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet or unexpected sqlmock expectations: %v", err)
	}
}

// TestGetEventVenueIDReturnsNotFoundForMissingEvent matches GetEvent/
// GetUserByID's sql.ErrNoRows -> ErrStoreNotFound convention used elsewhere in
// this file.
func TestGetEventVenueIDReturnsNotFoundForMissingEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT venue_id FROM catalog\.events WHERE id = \$1`).
		WithArgs("evt_does_not_exist").
		WillReturnRows(sqlmock.NewRows([]string{"venue_id"}))

	store := &DatabaseStore{db: db}
	if _, err := store.GetEventVenueID(context.Background(), "evt_does_not_exist"); err != ErrStoreNotFound {
		t.Fatalf("err = %v, want ErrStoreNotFound", err)
	}
}

// TestGetEventStatusReturnsStatusWithoutInventoryQuery keeps the reservation
// booking gate on the lean status query.
func TestGetEventStatusReturnsStatusWithoutInventoryQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT status FROM catalog\.events WHERE id = \$1`).
		WithArgs("evt_neon_riot").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("PUBLISHED"))

	store := &DatabaseStore{db: db}
	status, err := store.GetEventStatus(context.Background(), "evt_neon_riot")
	if err != nil {
		t.Fatalf("GetEventStatus: %v", err)
	}
	if status != "PUBLISHED" {
		t.Fatalf("status = %q, want PUBLISHED", status)
	}

	// No other query (e.g. GetOrganizerInventory's seat_snapshots aggregation)
	// should have been issued.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet or unexpected sqlmock expectations: %v", err)
	}
}

// TestGetEventStatusReturnsNotFoundForMissingEvent matches
// GetEventVenueID/GetEvent's sql.ErrNoRows -> ErrStoreNotFound convention.
func TestGetEventStatusReturnsNotFoundForMissingEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT status FROM catalog\.events WHERE id = \$1`).
		WithArgs("evt_does_not_exist").
		WillReturnRows(sqlmock.NewRows([]string{"status"}))

	store := &DatabaseStore{db: db}
	if _, err := store.GetEventStatus(context.Background(), "evt_does_not_exist"); err != ErrStoreNotFound {
		t.Fatalf("err = %v, want ErrStoreNotFound", err)
	}
}
