package internal

import (
	"context"
	"encoding/json"
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

func TestHandleSeatReservationConfirmationFailed(t *testing.T) {
	for _, reason := range []string{"", "EXPIRED"} {
		t.Run(reason, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to open sqlmock db: %v", err)
			}
			defer db.Close()

			orderID := "ord-123"

			mock.ExpectBegin()
			mock.ExpectExec("UPDATE orders.orders SET status = \\$2").
				WithArgs(orderID, "EXPIRED").
				WillReturnResult(sqlmock.NewResult(0, 1))
			rows := sqlmock.NewRows([]string{"event_id", "total_amount_minor"}).AddRow("evt-1", int64(9250))
			mock.ExpectQuery("SELECT s.event_id, o.total_amount_minor").
				WithArgs(orderID).
				WillReturnRows(rows)
			mock.ExpectExec("INSERT INTO orders.outbox_events").
				WithArgs(sqlmock.AnyArg(), orderID, sqlmock.AnyArg(), sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			handleSeatReservationConfirmationFailed(context.Background(), db, orderID, reason)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %s", err)
			}
		})
	}
}

// TestHandleSeatReservationConfirmationFailed_NotConfirmed asserts the guard
// clause: this handler must only ever correct an order in the impossible
// CONFIRMED-with-no-ticket state, never a PENDING/HELD order.
func TestHandleSeatReservationConfirmationFailed_NotConfirmed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()

	orderID := "ord-123"

	mock.ExpectBegin()
	// No rows affected: order isn't CONFIRMED, so no outbox event.
	mock.ExpectExec("UPDATE orders.orders SET status = \\$2").
		WithArgs(orderID, "EXPIRED").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	handleSeatReservationConfirmationFailed(context.Background(), db, orderID, "")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

// TestHandleSeatReservationConfirmationFailed_EventCancelled verifies that a
// reason of "EVENT_CANCELLED" (the concurrent-EventCancelled race case, as
// opposed to a plain hold expiry) sets the order to CANCELLED and writes an
// OrderCancelled outbox row instead of EXPIRED/OrderExpired.
func TestHandleSeatReservationConfirmationFailed_EventCancelled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()

	orderID := "ord-123"

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE orders.orders SET status = \\$2").
		WithArgs(orderID, "CANCELLED").
		WillReturnResult(sqlmock.NewResult(0, 1))
	rows := sqlmock.NewRows([]string{"event_id", "total_amount_minor"}).AddRow("evt-1", int64(9250))
	mock.ExpectQuery("SELECT s.event_id, o.total_amount_minor").
		WithArgs(orderID).
		WillReturnRows(rows)
	var payload []byte
	mock.ExpectExec("INSERT INTO orders.outbox_events").
		WithArgs(sqlmock.AnyArg(), orderID, capturedBytes{&payload}, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := handleSeatReservationConfirmationFailed(context.Background(), db, orderID, "EVENT_CANCELLED"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope struct {
		Type  string `json:"Type"`
		Order struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"Order"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if envelope.Type != "OrderCancelled" {
		t.Fatalf("envelope Type = %q, want OrderCancelled", envelope.Type)
	}
	if envelope.Order.Status != "CANCELLED" {
		t.Fatalf("envelope Order.status = %q, want CANCELLED", envelope.Order.Status)
	}
	if envelope.Order.Reason != "EVENT_CANCELLED" {
		t.Fatalf("envelope Order.reason = %q, want EVENT_CANCELLED", envelope.Order.Reason)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}
