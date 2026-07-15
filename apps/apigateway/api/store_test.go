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
