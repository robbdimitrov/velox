package internal

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// capturedString captures the driver.Value seen at its bind position so a
// later argument matcher in the same call can be checked against it, e.g.
// asserting reservation_id == "res_" + the generated order id without the
// test needing to predict the generated uuid up front.
type capturedString struct{ dest *string }

func (c capturedString) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	*c.dest = s
	return true
}

// resPrefixed matches a string equal to "res_" + the value captured by an
// earlier capturedString matcher in the same call.
type resPrefixed struct{ src *string }

func (r resPrefixed) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	return s == "res_"+*r.src
}

func TestCreateOrder_SetsReservationID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &Store{db: db}

	req := OrderRequest{
		EventID:        "evt-1",
		SectionID:      "sec-1",
		SeatIDs:        []string{"A-1"},
		IdempotencyKey: "idem-1",
		UserID:         "user-1",
	}

	var orderID string

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT request_hash, response_ref").
		WithArgs(req.UserID, req.IdempotencyKey).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO orders.idempotency_keys").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT price_amount_minor FROM projection.seat_snapshots").
		WithArgs(req.EventID, req.SectionID, "A-1").
		WillReturnRows(sqlmock.NewRows([]string{"price_amount_minor"}).AddRow(int64(5000)))
	mock.ExpectExec("INSERT INTO orders.orders").
		WithArgs(capturedString{&orderID}, req.UserID, req.IdempotencyKey, sqlmock.AnyArg(), int64(5000), resPrefixed{&orderID}).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO orders.order_seats").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO orders.outbox_events").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE orders.idempotency_keys").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	returnedID, err := s.CreateOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if returnedID != orderID {
		t.Fatalf("returned order id %s does not match inserted id %s", returnedID, orderID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestConfirmOrder_ConfirmsHeldOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &Store{db: db}

	orderID := "ord-123"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders.orders WHERE id = \\$1 FOR UPDATE").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("HELD"))
	mock.ExpectQuery("SELECT o.total_amount_minor, s.event_id").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"total_amount_minor", "event_id"}).AddRow(int64(9250), "evt-1"))
	mock.ExpectExec("UPDATE orders.orders SET status = 'CONFIRMED'").
		WithArgs(orderID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO orders.outbox_events").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	status, err := s.ConfirmOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "CONFIRMED" {
		t.Fatalf("status = %s, want CONFIRMED", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestConfirmOrder_IdempotentWhenAlreadyConfirmed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &Store{db: db}

	orderID := "ord-123"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders.orders WHERE id = \\$1 FOR UPDATE").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("CONFIRMED"))
	mock.ExpectCommit()

	status, err := s.ConfirmOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "CONFIRMED" {
		t.Fatalf("status = %s, want CONFIRMED", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestConfirmOrder_RejectsNonHeldOrder(t *testing.T) {
	cases := []string{"PENDING", "FAILED", "CANCELLED", "EXPIRED"}
	for _, initialStatus := range cases {
		t.Run(initialStatus, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to open sqlmock db: %v", err)
			}
			defer db.Close()
			s := &Store{db: db}

			orderID := "ord-123"
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT status FROM orders.orders WHERE id = \\$1 FOR UPDATE").
				WithArgs(orderID).
				WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(initialStatus))
			mock.ExpectRollback()

			_, err = s.ConfirmOrder(context.Background(), orderID)
			if !errors.Is(err, ErrOrderNotConfirmable) {
				t.Fatalf("err = %v, want ErrOrderNotConfirmable", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestConfirmOrder_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &Store{db: db}

	orderID := "missing"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders.orders WHERE id = \\$1 FOR UPDATE").
		WithArgs(orderID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = s.ConfirmOrder(context.Background(), orderID)
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("err = %v, want ErrOrderNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestCancelOrder_CancelsPendingOrHeldOrder(t *testing.T) {
	for _, initialStatus := range []string{"PENDING", "HELD"} {
		t.Run(initialStatus, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to open sqlmock db: %v", err)
			}
			defer db.Close()
			s := &Store{db: db}

			orderID := "ord-123"
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT status FROM orders.orders WHERE id = \\$1 FOR UPDATE").
				WithArgs(orderID).
				WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(initialStatus))
			mock.ExpectQuery("SELECT o.total_amount_minor, s.event_id").
				WithArgs(orderID).
				WillReturnRows(sqlmock.NewRows([]string{"total_amount_minor", "event_id"}).AddRow(int64(9250), "evt-1"))
			mock.ExpectExec("UPDATE orders.orders SET status = 'CANCELLED'").
				WithArgs(orderID).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO orders.outbox_events").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			status, err := s.CancelOrder(context.Background(), orderID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status != "CANCELLED" {
				t.Fatalf("status = %s, want CANCELLED", status)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestCancelOrder_IdempotentWhenAlreadyCancelled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &Store{db: db}

	orderID := "ord-123"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders.orders WHERE id = \\$1 FOR UPDATE").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("CANCELLED"))
	mock.ExpectCommit()

	status, err := s.CancelOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "CANCELLED" {
		t.Fatalf("status = %s, want CANCELLED", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestCancelOrder_RejectsAlreadyConfirmedOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &Store{db: db}

	orderID := "ord-123"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders.orders WHERE id = \\$1 FOR UPDATE").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("CONFIRMED"))
	mock.ExpectRollback()

	_, err = s.CancelOrder(context.Background(), orderID)
	if !errors.Is(err, ErrOrderNotCancellable) {
		t.Fatalf("err = %v, want ErrOrderNotCancellable", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}
