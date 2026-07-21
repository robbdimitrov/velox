package internal

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// capturedString records a bind value so later matchers can assert against
// generated IDs without predicting them.
type capturedString struct{ dest *string }

func (c capturedString) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	*c.dest = s
	return true
}

// capturedBytes records a JSON bind value for later payload assertions.
type capturedBytes struct{ dest *[]byte }

func (c capturedBytes) Match(v driver.Value) bool {
	b, ok := v.([]byte)
	if !ok {
		return false
	}
	*c.dest = append([]byte(nil), b...)
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
	mock.ExpectQuery("SELECT status FROM catalog.events").
		WithArgs(req.EventID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("PUBLISHED"))
	mock.ExpectExec("INSERT INTO orders.orders").
		WithArgs(capturedString{&orderID}, req.UserID, req.IdempotencyKey, sqlmock.AnyArg(), resPrefixed{&orderID}).
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

// TestCreateOrder_LocksEventRowForShare verifies the status check uses a row
// lock, since plain SELECT would not conflict with apigateway's CancelEvent.
func TestCreateOrder_LocksEventRowForShare(t *testing.T) {
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

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT request_hash, response_ref").
		WithArgs(req.UserID, req.IdempotencyKey).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO orders.idempotency_keys").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT status FROM catalog.events WHERE id = \\$1 FOR SHARE").
		WithArgs(req.EventID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("PUBLISHED"))
	mock.ExpectExec("INSERT INTO orders.orders").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO orders.order_seats").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO orders.outbox_events").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE orders.idempotency_keys").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if _, err := s.CreateOrder(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestCreateOrder_RejectsWhenEventNotPublished(t *testing.T) {
	for _, eventStatus := range []string{"CANCELLED", "DRAFT"} {
		t.Run(eventStatus, func(t *testing.T) {
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

			mock.ExpectBegin()
			mock.ExpectQuery("SELECT request_hash, response_ref").
				WithArgs(req.UserID, req.IdempotencyKey).
				WillReturnError(sql.ErrNoRows)
			mock.ExpectExec("INSERT INTO orders.idempotency_keys").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT status FROM catalog.events").
				WithArgs(req.EventID).
				WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(eventStatus))
			mock.ExpectRollback()

			_, err = s.CreateOrder(context.Background(), req)
			if !errors.Is(err, ErrEventNotBookable) {
				t.Fatalf("err = %v, want ErrEventNotBookable", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %s", err)
			}
		})
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
	mock.ExpectQuery("SELECT event_id").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}).AddRow("evt-1"))
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
			mock.ExpectQuery("SELECT event_id").
				WithArgs(orderID).
				WillReturnRows(sqlmock.NewRows([]string{"event_id"}).AddRow("evt-1"))
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

// TestCancelOrdersForEvent_TransitionsPendingHeldAndConfirmed verifies the
// single-transaction bulk cancel and one outbox row per returned order.
func TestCancelOrdersForEvent_TransitionsPendingHeldAndConfirmed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &Store{db: db}

	eventID := "evt-1"
	orders := []struct {
		id string
	}{
		{"ord-1"},
		{"ord-2"},
		{"ord-3"},
	}

	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"id"})
	for _, o := range orders {
		rows.AddRow(o.id)
	}
	mock.ExpectQuery("UPDATE orders.orders").
		WithArgs(eventID).
		WillReturnRows(rows)
	for _, o := range orders {
		mock.ExpectExec("INSERT INTO orders.outbox_events").
			WithArgs(sqlmock.AnyArg(), o.id, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec("INSERT INTO orders.outbox_events").
		WithArgs(sqlmock.AnyArg(), eventID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	count, err := s.CancelOrdersForEvent(context.Background(), eventID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

// TestCancelOrdersForEvent_SkipsAlreadyCancelled ensures already-cancelled
// orders are not returned, re-cancelled, or double-counted.
func TestCancelOrdersForEvent_SkipsAlreadyCancelled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &Store{db: db}

	eventID := "evt-1"

	mock.ExpectBegin()
	// ord-1 is already CANCELLED, so it doesn't match the WHERE clause and is
	// never returned; only ord-2 transitions and is counted.
	mock.ExpectQuery("UPDATE orders.orders").
		WithArgs(eventID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ord-2"))
	mock.ExpectExec("INSERT INTO orders.outbox_events").
		WithArgs(sqlmock.AnyArg(), "ord-2", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO orders.outbox_events").
		WithArgs(sqlmock.AnyArg(), eventID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	count, err := s.CancelOrdersForEvent(context.Background(), eventID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestCancelOrdersForEvent_NoOrdersForEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &Store{db: db}

	eventID := "evt-empty"

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE orders.orders").
		WithArgs(eventID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("INSERT INTO orders.outbox_events").
		WithArgs(sqlmock.AnyArg(), eventID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	count, err := s.CancelOrdersForEvent(context.Background(), eventID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

// TestWriteEventCancelledOutboxTx_DeterministicDedupID keeps retried event
// cancellation dedup stable while row primary keys remain unique.
func TestWriteEventCancelledOutboxTx_DeterministicDedupID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()

	eventID := "evt-77"
	headersBytes, _ := json.Marshal(map[string]string{})

	var dedupIDs []string
	var rowIDs []string
	for range 2 {
		mock.ExpectBegin()
		var rowID string
		var payload []byte
		mock.ExpectExec("INSERT INTO orders.outbox_events").
			WithArgs(capturedString{&rowID}, eventID, capturedBytes{&payload}, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("failed to begin tx: %v", err)
		}
		if err := writeEventCancelledOutboxTx(context.Background(), tx, eventID, headersBytes); err != nil {
			t.Fatalf("writeEventCancelledOutboxTx failed: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit failed: %v", err)
		}

		var envelope struct {
			Order struct {
				OutboxEventID string `json:"outbox_event_id"`
			} `json:"Order"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatalf("failed to unmarshal payload: %v", err)
		}
		dedupIDs = append(dedupIDs, envelope.Order.OutboxEventID)
		rowIDs = append(rowIDs, rowID)
	}

	if dedupIDs[0] != dedupIDs[1] {
		t.Fatalf("payload outbox_event_id differed across retries: %q vs %q", dedupIDs[0], dedupIDs[1])
	}
	if dedupIDs[0] != eventCancelledDedupID(eventID) {
		t.Fatalf("payload outbox_event_id = %q, want %q", dedupIDs[0], eventCancelledDedupID(eventID))
	}
	// viewservice dedups order events in a uuid-typed column, so EventCancelled
	// outbox IDs must remain syntactically valid UUIDs.
	if _, err := uuid.Parse(dedupIDs[0]); err != nil {
		t.Fatalf("payload outbox_event_id %q is not a valid UUID: %v", dedupIDs[0], err)
	}
	if rowIDs[0] == rowIDs[1] {
		t.Fatalf("orders.outbox_events.id should be a fresh UUID per row, got same value %q twice", rowIDs[0])
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
