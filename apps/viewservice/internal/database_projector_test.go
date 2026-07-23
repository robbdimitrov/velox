package internal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

var testSigningKey = []byte("test-event-signing-key")

// signInventoryTestEvent independently reproduces seatservice's canonical
// form so fixtures prove the gate accepts what the documented scheme produces.
func signInventoryTestEvent(t *testing.T, key []byte, event Event) Event {
	t.Helper()
	payload := map[string]any{
		"order_id":   event.CorrelationID,
		"event_id":   event.Seat.EventID,
		"section_id": event.Seat.SectionID,
		"seat_id":    event.Seat.SeatID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(event.Type))
	mac.Write([]byte("|"))
	mac.Write([]byte(event.AggregateID))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(event.AggregateVersion, 10)))
	mac.Write([]byte("|"))
	mac.Write(payloadBytes)
	event.SignedPayload = string(payloadBytes)
	event.Signature = hex.EncodeToString(mac.Sum(nil))
	return event
}

func TestApplyEvent_SeatReservationCancelled_CancelsIssuedTicket(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &DatabaseStore{db: db, signingKey: testSigningKey}

	event := signInventoryTestEvent(t, testSigningKey, Event{
		EventID:          "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b11",
		AggregateID:      "seat:evt_neon_riot:A:A-01",
		AggregateVersion: 3,
		Type:             "SeatReservationCancelled",
		Seat:             Seat{EventID: "evt_neon_riot", SectionID: "A", SeatID: "A-01", Status: "CANCELLED"},
	})

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
	mock.ExpectExec("organizer_updates").
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
	s := &DatabaseStore{db: db, signingKey: testSigningKey}

	event := signInventoryTestEvent(t, testSigningKey, Event{
		EventID:          "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b22",
		AggregateID:      "seat:evt_neon_riot:A:A-02",
		AggregateVersion: 1,
		Type:             "SeatReservationCancelled",
		Seat:             Seat{EventID: "evt_neon_riot", SectionID: "A", SeatID: "A-02", Status: "CANCELLED"},
	})

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
	mock.ExpectExec("organizer_updates").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := s.ApplyEvent(context.Background(), event, "inventory.events.v1", 0, 1); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestIssueOrBufferWalletTicketIssuesWhenOrderProjected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer tx.Rollback()

	event := Event{
		EventID:          "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b33",
		CorrelationID:    "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b44",
		AggregateVersion: 2,
		Seat:             Seat{EventID: "evt_1", SectionID: "A", SeatID: "A-01"},
	}

	mock.ExpectQuery("SELECT user_id FROM projection.order_summaries").
		WithArgs(event.CorrelationID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("usr_1"))
	mock.ExpectExec("INSERT INTO projection.wallet_tickets").
		WithArgs(event.EventID, "usr_1", event.CorrelationID, event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID, "ISSUED", event.AggregateVersion).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := issueOrBufferWalletTicket(context.Background(), tx, event); err != nil {
		t.Fatalf("issueOrBufferWalletTicket: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestIssueOrBufferWalletTicketBuffersWhenOrderMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer tx.Rollback()

	event := Event{
		EventID:          "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b55",
		CorrelationID:    "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b66",
		AggregateVersion: 2,
		Seat:             Seat{EventID: "evt_1", SectionID: "A", SeatID: "A-02"},
	}

	mock.ExpectQuery("SELECT user_id FROM projection.order_summaries").
		WithArgs(event.CorrelationID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
	mock.ExpectExec("INSERT INTO projection.pending_wallet_ticket_events").
		WithArgs(event.EventID, event.CorrelationID, event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID, event.AggregateVersion).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := issueOrBufferWalletTicket(context.Background(), tx, event); err != nil {
		t.Fatalf("issueOrBufferWalletTicket: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestIssuePendingWalletTicketsForOrderCreatesCancelledTicket(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer tx.Rollback()

	orderID := "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b77"
	mock.ExpectQuery("SELECT p.ticket_id, p.event_id, p.section_id, p.seat_id").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{
			"ticket_id", "event_id", "section_id", "seat_id", "aggregate_version", "user_id", "status",
		}).AddRow("tkt_1", "evt_1", "A", "A-03", int64(4), "usr_1", "CANCELLED"))
	mock.ExpectExec("INSERT INTO projection.wallet_tickets").
		WithArgs("tkt_1", "usr_1", orderID, "evt_1", "A", "A-03", "CANCELLED", int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM projection.pending_wallet_ticket_events").
		WithArgs("tkt_1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := issuePendingWalletTicketsForOrder(context.Background(), tx, orderID); err != nil {
		t.Fatalf("issuePendingWalletTicketsForOrder: %v", err)
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

// TestApplyEvent_RejectsMissingInventorySignature proves an unsigned event is
// rejected before any mutation: no sqlmock expectation is set, so any query would fail the test.
func TestApplyEvent_RejectsMissingInventorySignature(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &DatabaseStore{db: db, signingKey: testSigningKey}

	event := Event{
		EventID:          "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b88",
		AggregateID:      "seat:evt_neon_riot:A:A-04",
		AggregateVersion: 1,
		Type:             "SeatReservationCancelled",
		Seat:             Seat{EventID: "evt_neon_riot", SectionID: "A", SeatID: "A-04", Status: "CANCELLED"},
	}

	err = s.ApplyEvent(context.Background(), event, "inventory.events.v1", 0, 1)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("err = %v, want ErrInvalidSignature", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestApplyEvent_RejectsTamperedInventorySignature proves a signature that
// does not match the event's own fields is rejected without mutating state.
func TestApplyEvent_RejectsTamperedInventorySignature(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &DatabaseStore{db: db, signingKey: testSigningKey}

	event := signInventoryTestEvent(t, testSigningKey, Event{
		EventID:          "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2b99",
		AggregateID:      "seat:evt_neon_riot:A:A-05",
		AggregateVersion: 1,
		Type:             "SeatReservationCancelled",
		Seat:             Seat{EventID: "evt_neon_riot", SectionID: "A", SeatID: "A-05", Status: "CANCELLED"},
	})
	// Tamper with the seat after signing: the signature no longer matches.
	event.Seat.SeatID = "A-06"

	err = s.ApplyEvent(context.Background(), event, "inventory.events.v1", 0, 1)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("err = %v, want ErrInvalidSignature", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestApplyEvent_RejectsInventorySignatureFromWrongKey proves a signature
// produced with a different key (e.g. a compromised or stale secret) fails.
func TestApplyEvent_RejectsInventorySignatureFromWrongKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &DatabaseStore{db: db, signingKey: testSigningKey}

	event := signInventoryTestEvent(t, []byte("a-different-key"), Event{
		EventID:          "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2baa",
		AggregateID:      "seat:evt_neon_riot:A:A-07",
		AggregateVersion: 1,
		Type:             "SeatReservationCancelled",
		Seat:             Seat{EventID: "evt_neon_riot", SectionID: "A", SeatID: "A-07", Status: "CANCELLED"},
	})

	err = s.ApplyEvent(context.Background(), event, "inventory.events.v1", 0, 1)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("err = %v, want ErrInvalidSignature", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestApplyEvent_OrderEventsSkipInventorySignatureCheck proves order.events.v1
// is unaffected: seatservice verifies that direction, not viewservice.
func TestApplyEvent_OrderEventsSkipInventorySignatureCheck(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock db: %v", err)
	}
	defer db.Close()
	s := &DatabaseStore{db: db, signingKey: testSigningKey}

	event := Event{
		EventID: "6f6d0b8e-2b3d-4b7e-9b8b-2b1e9b8b2bbb",
		Type:    "OrderCreated",
		Order:   Order{OrderID: "ord-1", UserID: "usr-1", EventID: "evt-1", Status: "PENDING"},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(event.EventID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT COALESCE").
		WithArgs(event.Order.OrderID).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(int64(0)))
	mock.ExpectExec("INSERT INTO projection.processed_events").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO projection.order_summaries").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT p.ticket_id, p.event_id, p.section_id, p.seat_id").
		WithArgs(event.Order.OrderID).
		WillReturnRows(sqlmock.NewRows([]string{
			"ticket_id", "event_id", "section_id", "seat_id", "aggregate_version", "user_id", "status",
		}))
	mock.ExpectExec("organizer_updates").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := s.ApplyEvent(context.Background(), event, "order.events.v1", 0, 1); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
