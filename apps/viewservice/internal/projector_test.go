package internal

import (
	"errors"
	"testing"
)

func TestDropsDuplicateEventID(t *testing.T) {
	p := NewProjector()
	event := Event{EventID: "evt1", AggregateID: "seat1", AggregateVersion: 1, Type: "SeatReservationHeld", Seat: Seat{EventID: "event", SectionID: "A", SeatID: "A-01", Status: "HELD"}}
	if err := p.Apply(event); err != nil {
		t.Fatal(err)
	}
	event.Seat.Status = "SOLD"
	if err := p.Apply(event); err != nil {
		t.Fatal(err)
	}
	if got := p.Seats["event:A:A-01"].Status; got != "HELD" {
		t.Fatalf("status = %s", got)
	}
}

func TestRejectsLowerAggregateVersion(t *testing.T) {
	p := NewProjector()
	if err := p.Apply(Event{EventID: "evt1", AggregateID: "seat1", AggregateVersion: 2, Type: "SeatReservationHeld"}); err != nil {
		t.Fatal(err)
	}
	err := p.Apply(Event{EventID: "evt2", AggregateID: "seat1", AggregateVersion: 1, Type: "SeatReservationExpired"})
	if !errors.Is(err, ErrStaleAggregateVersion) {
		t.Fatalf("err = %v", err)
	}
}

func TestOrderEvents(t *testing.T) {
	p := NewProjector()
	order := Order{OrderID: "order1", UserID: "user1", EventID: "event1", Status: "PENDING"}
	event := Event{EventID: "evt1", AggregateID: "order1", AggregateVersion: 1, Type: "OrderCreated", Order: order}

	if err := p.Apply(event); err != nil {
		t.Fatal(err)
	}

	if got := p.Orders["order1"].Status; got != "PENDING" {
		t.Fatalf("expected status PENDING, got %s", got)
	}

	if len(p.VendorOrderIDs["event1"]) != 1 || p.VendorOrderIDs["event1"][0] != "order1" {
		t.Fatalf("expected vendor order ID to be recorded")
	}
}

func TestAllowsZeroAggregateVersion(t *testing.T) {
	p := NewProjector()
	// Set an initial version
	if err := p.Apply(Event{EventID: "evt1", AggregateID: "seat1", AggregateVersion: 2, Type: "SeatReservationHeld"}); err != nil {
		t.Fatal(err)
	}
	// Try applying an event with AggregateVersion 0 (e.g. from a legacy system or external event)
	err := p.Apply(Event{EventID: "evt2", AggregateID: "seat1", AggregateVersion: 0, Type: "SeatReservationExpired"})
	if err != nil {
		t.Fatalf("expected nil error for zero aggregate version, got %v", err)
	}
}
