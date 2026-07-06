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
		EventID:          "evt-cancel-1",
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

// TestApplyEvent_SeatReservationCancelled_NoIssuedTicket_NoError covers the
// common case where a seat was only ever HELD (never CONFIRMED), so no
// wallet_tickets row exists for it. The UPDATE ... WHERE status = 'ISSUED'
// affects zero rows in that case, which must not surface as an error.
func TestApplyEvent_SeatReservationCancelled_NoIssuedTicket_NoError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &DatabaseStore{db: db}

	event := Event{
		EventID:          "evt-cancel-2",
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
