package api

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestGetEventAnnouncementsCapsResultSet keeps the public announcements read
// bounded; the LIMIT must remain after ORDER BY.
func TestGetEventAnnouncementsCapsResultSet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, event_id, title, body, severity, created_at\s+FROM catalog\.event_announcements\s+WHERE event_id = \$1\s+ORDER BY created_at DESC\s+LIMIT \$2`).
		WithArgs("evt_neon_riot", listResultLimit).
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
		t.Fatalf("unmet sqlmock expectations (query likely missing LIMIT): %v", err)
	}
}

func TestCreateEventCopiesGeometryToInventoryAndProjectionRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	startsAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	event := Event{
		ID:          "evt_geometry",
		VenueID:     "ven_geometry",
		Name:        "Geometry Event",
		Description: "Geometry-backed seats",
		Category:    "Theatre",
		StartsAt:    startsAt,
		Timezone:    "America/New_York",
		Status:      EventStatusPublished,
	}

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO catalog\.events .*VALUES`).
		WithArgs(event.ID, event.VenueID, event.Name, event.Description, event.Category, event.StartsAt, event.Timezone, event.Status).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO catalog\.event_sections .*FROM catalog\.venue_sections`).
		WithArgs(event.ID, event.VenueID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`(?s)SELECT vs\.section_id, vs\.seat_id, vs\.row_label, vs\.seat_number, vs\.x, vs\.y,\s+vs\.accessibility\s+FROM catalog\.venue_seats vs`).
		WithArgs(event.VenueID, event.ID).
		WillReturnRows(sqlmock.NewRows([]string{"section_id", "seat_id", "row_label", "seat_number", "x", "y", "accessibility"}).
			AddRow("A", "A-01", "A", 1, 44, 42, true).
			AddRow("A", "A-02", "A", 2, 86, 42, false))
	mock.ExpectExec(`INSERT INTO inventory\.event_streams`).
		WithArgs("seat:evt_geometry:A:A-01", event.ID, "A", "A-01").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO projection\.seat_snapshots .*VALUES`).
		WithArgs(event.ID, "A", "A-01", "A", 1, 44, 42, true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO inventory\.event_streams`).
		WithArgs("seat:evt_geometry:A:A-02", event.ID, "A", "A-02").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO projection\.seat_snapshots .*VALUES`).
		WithArgs(event.ID, "A", "A-02", "A", 2, 86, 42, false).
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
	created, err := store.CreateVenue(context.Background(), "usr_organizer", venue, nil)
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

func TestCreateVenueCreatesCustomSeatTemplate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	venue := Venue{
		ID:       "ven_custom",
		Name:     "Custom Hall",
		City:     "Chicago",
		Address:  "2 Template Way",
		Capacity: 6,
	}
	sections := []VenueSectionTemplate{{
		SectionID:           "VIP",
		Name:                "VIP Floor",
		RowCount:            2,
		SeatsPerRow:         3,
		AccessibleEdgeSeats: true,
	}}

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO catalog\.venues`).
		WithArgs(venue.ID, venue.Name, venue.City, venue.Address, venue.Capacity).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO catalog\.user_venues`).
		WithArgs("usr_organizer", venue.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO catalog\.venue_sections`).
		WithArgs(venue.ID, "VIP", "VIP Floor", 1, 170, 120).
		WillReturnResult(sqlmock.NewResult(0, 1))
	for _, args := range [][]driver.Value{
		{venue.ID, "VIP", "A-01", "A", 1, 44, 42, true},
		{venue.ID, "VIP", "A-02", "A", 2, 86, 42, false},
		{venue.ID, "VIP", "A-03", "A", 3, 128, 42, true},
		{venue.ID, "VIP", "B-01", "B", 1, 44, 84, true},
		{venue.ID, "VIP", "B-02", "B", 2, 86, 84, false},
		{venue.ID, "VIP", "B-03", "B", 3, 128, 84, true},
	} {
		mock.ExpectExec(`(?s)INSERT INTO catalog\.venue_seats`).
			WithArgs(args...).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	store := &DatabaseStore{db: db}
	created, err := store.CreateVenue(context.Background(), "usr_organizer", venue, sections)
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
			"status", "aggregate_version", "expires_at_server_ms", "age_ms",
		}).AddRow("A-01", "A", 1, 44, 42, true, StatusAvailable, 0, int64(0), int64(4)))

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

func TestGetWalletTicketsLocksCancelledTransferState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`(?s)SELECT wt\.ticket_id, wt\.order_id::text, wt\.event_id, wt\.section_id, wt\.seat_id, wt\.status,.*FROM projection\.wallet_tickets wt`).
		WithArgs("usr_1", listResultLimit).
		WillReturnRows(sqlmock.NewRows([]string{
			"ticket_id", "order_id", "event_id", "section_id", "seat_id", "status", "event", "venue",
		}).AddRow("tkt_1", "ord_1", "evt_1", "A", "A-01", "CANCELLED", "Event", "Venue"))
	mock.ExpectQuery(`SELECT event_type, occurred_at, correlation_id\s+FROM inventory\.events\s+WHERE stream_key = \$1\s+ORDER BY aggregate_version ASC`).
		WithArgs("seat:evt_1:A:A-01").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "occurred_at", "correlation_id"}))

	store := &DatabaseStore{db: db}
	tickets, err := store.GetWalletTickets(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("GetWalletTickets: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("len(tickets) = %d, want 1", len(tickets))
	}
	if tickets[0].TransferStatus != "LOCKED" {
		t.Fatalf("transfer_status = %q, want LOCKED", tickets[0].TransferStatus)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestGetOrganizerMetricsUsesProjectionCountsAndLag(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\)\s+FROM projection\.order_summaries`).
		WithArgs("evt_metrics").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))
	mock.ExpectQuery(`SELECT section_id, status, COUNT\(\*\)\s+FROM projection\.seat_snapshots`).
		WithArgs("evt_metrics").
		WillReturnRows(sqlmock.NewRows([]string{"section_id", "status", "count"}).
			AddRow("A", StatusAvailable, 5).
			AddRow("A", StatusHeld, 3).
			AddRow("A", StatusReserved, 2))
	mock.ExpectQuery(`(?s)SELECT COALESCE\(extract\(epoch from \(now\(\) - MAX\(updated_at\)\)\) \* 1000, 0\)::bigint\s+FROM projection\.seat_snapshots`).
		WithArgs("evt_metrics").
		WillReturnRows(sqlmock.NewRows([]string{"lag"}).AddRow(int64(42)))

	store := &DatabaseStore{db: db}
	metrics, err := store.GetOrganizerMetrics(context.Background(), "evt_metrics")
	if err != nil {
		t.Fatalf("GetOrganizerMetrics: %v", err)
	}
	if metrics.TotalReservations != 3 || metrics.ActiveHolds != 3 || metrics.SeatsRemaining != 5 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	if metrics.DemandScore != 35 {
		t.Fatalf("DemandScore = %d, want 35", metrics.DemandScore)
	}
	if metrics.ProjectionLagMs != 42 {
		t.Fatalf("ProjectionLagMs = %d, want 42", metrics.ProjectionLagMs)
	}
	if metrics.SectionAvailability["A"] != 50 {
		t.Fatalf("section A availability = %d, want 50", metrics.SectionAvailability["A"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

// TestGetEventsIncludesDemandScoreFromInventory guards against demand_score
// silently going to zero on the discovery list, as it did when the query's
// held/reserved counts were scanned but never fed into a score.
func TestGetEventsIncludesDemandScoreFromInventory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM catalog\.events e\s+JOIN catalog\.venues v`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "venue_id", "name", "description", "category",
			"starts_at", "timezone", "status", "venue_name", "city",
			"section_ids", "available", "held", "reserved",
		}).AddRow(
			"evt_neon_riot", "ven_velox_arena", "Neon Riot Live", "", "Concerts",
			time.Now(), "America/Chicago", "PUBLISHED", "Velox Arena", "Chicago",
			"A,B", 5, 3, 2,
		))

	store := &DatabaseStore{db: db}
	events, err := store.GetEvents(context.Background())
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].DemandScore != 35 {
		t.Fatalf("DemandScore = %d, want 35", events[0].DemandScore)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

// TestGetEventIncludesDemandScoreFromInventory is the single-event-read
// counterpart to TestGetEventsIncludesDemandScoreFromInventory.
func TestGetEventIncludesDemandScoreFromInventory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM catalog\.events e\s+JOIN catalog\.venues v`).
		WithArgs("evt_neon_riot").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "venue_id", "name", "description", "category",
			"starts_at", "timezone", "status", "venue_name", "city",
			"section_ids", "available", "held", "reserved",
		}).AddRow(
			"evt_neon_riot", "ven_velox_arena", "Neon Riot Live", "", "Concerts",
			time.Now(), "America/Chicago", "PUBLISHED", "Velox Arena", "Chicago",
			"A,B", 5, 3, 2,
		))

	store := &DatabaseStore{db: db}
	event, err := store.GetEvent(context.Background(), "evt_neon_riot")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if event.DemandScore != 35 {
		t.Fatalf("DemandScore = %d, want 35", event.DemandScore)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
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
