package api

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestGetEventAnnouncementsCapsResultSet confirms GetEventAnnouncements' query
// carries a fixed LIMIT so the public, unauthenticated announcements endpoint
// can never be made to return an unbounded result set (AGENTS.md's Secure
// Engineering section requires bounding collection sizes at the point untrusted
// requests read them). The regex requires "LIMIT 100" to appear after the
// ORDER BY clause, so this fails if the cap is ever dropped.
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

// TestGetEventVenueIDReturnsVenueWithoutInventoryQuery confirms the lean
// ownership-check path queries only catalog.events for venue_id and never
// touches GetOrganizerInventory's seat-count aggregation over
// projection.seat_snapshots, which GetEvent wastefully runs and discards when
// only venue ownership is being checked.
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

// TestGetEventStatusReturnsStatusWithoutInventoryQuery confirms
// handleCreateReservation's booking-race gate queries only catalog.events for
// status and never touches GetOrganizerInventory's seat-count aggregation
// over projection.seat_snapshots, which GetEvent wastefully runs and discards
// when only PUBLISHED-vs-not is being checked on apigateway's hottest write
// path.
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
