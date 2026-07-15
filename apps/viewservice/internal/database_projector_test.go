package internal

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestApplyEvent_SeatReservationCancelled_CancelsIssuedTicket(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &DatabaseStore{db: db}

	event := Event{
		EventID:          "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b11",
		AggregateID:      "seat:evt_neon_riot:A:A-01",
		AggregateVersion: 3,
		Type:             "SeatReservationCancelled",
		Seat:             Seat{EventID: "evt_neon_riot", SectionID: "A", SeatID: "A-01", Status: "CANCELLED"},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(event.EventID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT COALESCE").
		WithArgs(event.AggregateID).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(int64(2)))
	mock.ExpectExec("INSERT INTO projection.processed_events").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO projection.seat_snapshots").
		WithArgs(event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID, event.Seat.Status, event.AggregateVersion, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE projection.wallet_tickets").
		WithArgs(event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("seat_updates").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("vendor_updates").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := s.ApplyEvent(context.Background(), event, "inventory.events.v1", 0, 1); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestApplyEvent_SeatReservationCancelled_NoIssuedTicket_NoError covers held
// seats that never issued a wallet ticket; zero-row UPDATEs must be fine.
func TestApplyEvent_SeatReservationCancelled_NoIssuedTicket_NoError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &DatabaseStore{db: db}

	event := Event{
		EventID:          "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b22",
		AggregateID:      "seat:evt_neon_riot:A:A-02",
		AggregateVersion: 1,
		Type:             "SeatReservationCancelled",
		Seat:             Seat{EventID: "evt_neon_riot", SectionID: "A", SeatID: "A-02", Status: "CANCELLED"},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(event.EventID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT COALESCE").
		WithArgs(event.AggregateID).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(int64(0)))
	mock.ExpectExec("INSERT INTO projection.processed_events").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO projection.seat_snapshots").
		WithArgs(event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID, event.Seat.Status, event.AggregateVersion, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// No ISSUED wallet_tickets row for this seat: 0 rows affected, no error.
	mock.ExpectExec("UPDATE projection.wallet_tickets").
		WithArgs(event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("seat_updates").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("vendor_updates").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := s.ApplyEvent(context.Background(), event, "inventory.events.v1", 0, 1); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestApplyEvent_SkipsNonUUIDEventID keeps malformed event IDs from
// crash-looping the UUID-typed processed_events dedup insert.
func TestApplyEvent_SkipsNonUUIDEventID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &DatabaseStore{db: db}

	event := Event{
		EventID: "event-cancel:evt_neon_riot",
		Type:    "EventCancelled",
	}

	// The transaction still opens (and is left to roll back via defer) since
	// the guard runs inside ApplyEvent after BeginTx; no other statement -
	// notably no SELECT EXISTS dedup check - should ever be attempted.
	mock.ExpectBegin()

	if err := s.ApplyEvent(context.Background(), event, "order.events.v1", 0, 1); err != nil {
		t.Fatalf("ApplyEvent returned error for a non-UUID event_id, want graceful skip: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
