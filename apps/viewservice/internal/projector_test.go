package internal

import (
	"encoding/json"
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

func TestSeatReservationCancelledEvent(t *testing.T) {
	p := NewProjector()
	event := Event{EventID: "evt1", AggregateID: "seat1", AggregateVersion: 1, Type: "SeatReservationCancelled", Seat: Seat{EventID: "event", SectionID: "A", SeatID: "A-01", Status: "CANCELLED"}}
	if err := p.Apply(event); err != nil {
		t.Fatal(err)
	}
	if got := p.Seats["event:A:A-01"].Status; got != "CANCELLED" {
		t.Fatalf("status = %s", got)
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

	if len(p.OrganizerOrderIDs["event1"]) != 1 || p.OrganizerOrderIDs["event1"][0] != "order1" {
		t.Fatalf("expected organizer order ID to be recorded")
	}
}

func TestOrderCancelledAndExpiredEvents(t *testing.T) {
	cases := []struct {
		eventType string
		status    string
	}{
		{eventType: "OrderCancelled", status: "CANCELLED"},
		{eventType: "OrderExpired", status: "EXPIRED"},
	}

	for _, tc := range cases {
		p := NewProjector()
		order := Order{OrderID: "order1", UserID: "user1", EventID: "event1", Status: tc.status}
		event := Event{EventID: "evt1", AggregateID: "order1", AggregateVersion: 1, Type: tc.eventType, Order: order}

		if err := p.Apply(event); err != nil {
			t.Fatal(err)
		}

		if got := p.Orders["order1"].Status; got != tc.status {
			t.Fatalf("expected status %s, got %s", tc.status, got)
		}

		if len(p.OrganizerOrderIDs["event1"]) != 1 || p.OrganizerOrderIDs["event1"][0] != "order1" {
			t.Fatalf("expected organizer order ID to be recorded")
		}
	}
}

func TestAllowsZeroAggregateVersion(t *testing.T) {
	p := NewProjector()
	if err := p.Apply(Event{EventID: "evt1", AggregateID: "seat1", AggregateVersion: 2, Type: "SeatReservationHeld"}); err != nil {
		t.Fatal(err)
	}
	err := p.Apply(Event{EventID: "evt2", AggregateID: "seat1", AggregateVersion: 0, Type: "SeatReservationExpired"})
	if err != nil {
		t.Fatalf("expected nil error for zero aggregate version, got %v", err)
	}
}

// orderservice nests IDs under Order; this fixture exercises the JSON path
// where top-level event_id and aggregate_id are absent.
const orderConfirmedEnvelope = `{
	"Type": "OrderConfirmed",
	"Order": {
		"outbox_event_id": "outbox-evt-1",
		"order_id": "order-42",
		"user_id": "user-9",
		"event_id": "evt_neon_riot",
		"status": "CONFIRMED",
		"total_amount_minor": 8500
	}
}`

func TestResolvedIDsFallBackToNestedOrderFields(t *testing.T) {
	var event Event
	if err := json.Unmarshal([]byte(orderConfirmedEnvelope), &event); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if event.EventID != "" {
		t.Fatalf("expected empty top-level EventID from orderservice's envelope, got %q", event.EventID)
	}
	if event.AggregateID != "" {
		t.Fatalf("expected empty top-level AggregateID from orderservice's envelope, got %q", event.AggregateID)
	}

	if got := event.ResolvedEventID(); got != "outbox-evt-1" {
		t.Fatalf("ResolvedEventID() = %q, want %q", got, "outbox-evt-1")
	}
	if got := event.ResolvedAggregateID(); got != "order-42" {
		t.Fatalf("ResolvedAggregateID() = %q, want %q", got, "order-42")
	}
	if got := event.Order.UserID; got != "user-9" {
		t.Fatalf("Order.UserID = %q, want %q", got, "user-9")
	}
	if got := event.Order.EventID; got != "evt_neon_riot" {
		t.Fatalf("Order.EventID = %q, want %q", got, "evt_neon_riot")
	}
}

func TestResolvedIDsPreferTopLevelFieldsWhenPresent(t *testing.T) {
	event := Event{
		EventID:     "seat-evt-1",
		AggregateID: "seat:evt_neon_riot:A:A-01",
		Order:       Order{OutboxEventID: "outbox-evt-1", OrderID: "order-42"},
	}

	if got := event.ResolvedEventID(); got != "seat-evt-1" {
		t.Fatalf("ResolvedEventID() = %q, want top-level %q", got, "seat-evt-1")
	}
	if got := event.ResolvedAggregateID(); got != "seat:evt_neon_riot:A:A-01" {
		t.Fatalf("ResolvedAggregateID() = %q, want top-level %q", got, "seat:evt_neon_riot:A:A-01")
	}
}

func TestApplyDedupesOrderEventsByResolvedEventID(t *testing.T) {
	p := NewProjector()
	var event Event
	if err := json.Unmarshal([]byte(orderConfirmedEnvelope), &event); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if err := p.Apply(event); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if got := p.Orders["order-42"].Status; got != "CONFIRMED" {
		t.Fatalf("status = %s, want CONFIRMED", got)
	}

	// Duplicate delivery (same outbox_event_id) must be dropped, not reprocessed.
	event.Order.Status = "SOMETHING_ELSE"
	if err := p.Apply(event); err != nil {
		t.Fatalf("duplicate apply: %v", err)
	}
	if got := p.Orders["order-42"].Status; got != "CONFIRMED" {
		t.Fatalf("duplicate event mutated projection: status = %s, want unchanged CONFIRMED", got)
	}
}
