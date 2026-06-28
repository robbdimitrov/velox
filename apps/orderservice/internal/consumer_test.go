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

func TestHandleSeatReserved_IdempotentCheck(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()

	orderID := "ord-123"
	
	mock.ExpectBegin()

	// Return a status of 'CONFIRMED' (not 'PENDING'), so it should rollback and exit
	rows := sqlmock.NewRows([]string{"total_amount_minor", "event_id", "status"}).
		AddRow(15000, "evt-1", "CONFIRMED")
	mock.ExpectQuery("SELECT o.total_amount_minor, s.event_id, o.status").
		WithArgs(orderID).
		WillReturnRows(rows)

	// Since it's not PENDING, it should rollback
	mock.ExpectRollback()

	handleSeatReserved(context.Background(), db, orderID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}
