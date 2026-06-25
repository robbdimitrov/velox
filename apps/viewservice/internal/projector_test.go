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
