package internal

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHandleSeatReservationFailed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()

	orderID := "ord-123"
	mock.ExpectExec("UPDATE orders.orders SET status = 'FAILED'").
		WithArgs(orderID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	handleSeatReservationFailed(context.Background(), db, orderID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestHandleSeatReservationHeld(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()

	orderID := "ord-123"
	mock.ExpectExec("UPDATE orders.orders SET status = 'HELD'").
		WithArgs(orderID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	handleSeatReservationHeld(context.Background(), db, orderID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestHandleSeatReservationExpired(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()

	orderID := "ord-123"

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE orders.orders SET status = 'EXPIRED'").
		WithArgs(orderID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	rows := sqlmock.NewRows([]string{"event_id", "total_amount_minor"}).AddRow("evt-1", int64(9250))
	mock.ExpectQuery("SELECT s.event_id, o.total_amount_minor").
		WithArgs(orderID).
		WillReturnRows(rows)
	mock.ExpectExec("INSERT INTO orders.outbox_events").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	handleSeatReservationExpired(context.Background(), db, orderID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestHandleSeatReservationExpired_AlreadyConfirmed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()

	orderID := "ord-123"

	mock.ExpectBegin()
	// No rows affected: order already CONFIRMED/CANCELLED, so no outbox event.
	mock.ExpectExec("UPDATE orders.orders SET status = 'EXPIRED'").
		WithArgs(orderID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	handleSeatReservationExpired(context.Background(), db, orderID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}
