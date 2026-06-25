package internal

import (
	"errors"
	"testing"
	"time"
)

func TestCreateReservationIdempotency(t *testing.T) {
	service := NewService()
	service.now = func() time.Time { return time.Unix(100, 0) }
	req := CreateReservationRequest{EventID: "evt", SectionID: "A", SeatIDs: []string{"A-01"}}
	first, err := service.CreateReservation("usr", "key", req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateReservation("usr", "key", req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent ID mismatch: %s != %s", first.ID, second.ID)
	}
	if len(service.Outbox()) != 1 {
		t.Fatalf("outbox rows = %d, want 1", len(service.Outbox()))
	}
}

func TestRejectsConflictingIdempotencyBody(t *testing.T) {
	service := NewService()
	_, err := service.CreateReservation("usr", "key", CreateReservationRequest{EventID: "evt", SectionID: "A", SeatIDs: []string{"A-01"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateReservation("usr", "key", CreateReservationRequest{EventID: "evt", SectionID: "A", SeatIDs: []string{"A-02"}})
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("err = %v, want %v", err, ErrIdempotencyConflict)
	}
}

func TestConfirmReservationWritesOutbox(t *testing.T) {
	service := NewService()
	order, err := service.CreateReservation("usr", "key", CreateReservationRequest{EventID: "evt", SectionID: "A", SeatIDs: []string{"A-01"}})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.ConfirmReservation(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Status != "CONFIRM_REQUESTED" {
		t.Fatalf("status = %s", confirmed.Status)
	}
	if got := service.Outbox()[1].EventType; got != "ReservationConfirmRequested" {
		t.Fatalf("event type = %s", got)
	}
}
