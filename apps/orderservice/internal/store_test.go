package internal

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

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
