package internal

import (
	"errors"
	"sync"
	"time"
)

var ErrStaleAggregateVersion = errors.New("stale aggregate version")

// Event unifies two producer shapes onto one struct: seatservice's
// SeatInventoryEvent is flat (top-level event_id/aggregate_id/aggregate_version),
// while orderservice's envelope nests everything under "Order" and has no
// top-level event_id/aggregate_id/aggregate_version at all. ResolvedEventID
// and ResolvedAggregateID paper over that difference so callers don't need to
// know which producer sent a given message.
type Event struct {
	EventID          string `json:"event_id"`
	AggregateID      string `json:"aggregate_id"`
	AggregateVersion int64  `json:"aggregate_version"`
	Type             string
	CorrelationID    string `json:"correlation_id"`
	Seat             Seat
	Order            Order
	OccurredAt       time.Time `json:"occurred_at"`
}

// ResolvedEventID is the top-level event_id for seatservice-originated
// events, or orderservice's nested outbox_event_id when the top-level field
// is absent (orderservice's envelope has no top-level event_id).
func (e Event) ResolvedEventID() string {
	if e.EventID != "" {
		return e.EventID
	}
	return e.Order.OutboxEventID
}

// ResolvedAggregateID is the top-level aggregate_id for seatservice-originated
// events, or the order_id for orderservice-originated events (which have no
// concept of a seat-stream aggregate).
func (e Event) ResolvedAggregateID() string {
	if e.AggregateID != "" {
		return e.AggregateID
	}
	return e.Order.OrderID
}

type Seat struct {
	EventID     string `json:"event_id"`
	SectionID   string `json:"section_id"`
	SeatID      string `json:"seat_id"`
	Status      string
	Version     int64
	ExpiresAtMS int64 `json:"expires_at_ms"`
}

type Order struct {
	OutboxEventID    string `json:"outbox_event_id"`
	OrderID          string `json:"order_id"`
	UserID           string `json:"user_id"`
	EventID          string `json:"event_id"`
	Status           string
	TotalAmountMinor int64 `json:"total_amount_minor"`
}

type Projector struct {
	mu              sync.Mutex
	processed       map[string]struct{}
	aggregateVer    map[string]int64
	Seats           map[string]Seat
	Orders          map[string]Order
	VendorOrderIDs  map[string][]string
	ProjectionLagMS int64
}

func NewProjector() *Projector {
	return &Projector{
		processed:      map[string]struct{}{},
		aggregateVer:   map[string]int64{},
		Seats:          map[string]Seat{},
		Orders:         map[string]Order{},
		VendorOrderIDs: map[string][]string{},
	}
}

func (p *Projector) Apply(event Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	eventID := event.ResolvedEventID()
	aggregateID := event.ResolvedAggregateID()
	if _, ok := p.processed[eventID]; ok {
		return nil
	}
	last := p.aggregateVer[aggregateID]
	if event.AggregateVersion > 0 && last >= event.AggregateVersion {
		return ErrStaleAggregateVersion
	}
	p.processed[eventID] = struct{}{}
	p.aggregateVer[aggregateID] = event.AggregateVersion
	if !event.OccurredAt.IsZero() {
		p.ProjectionLagMS = time.Since(event.OccurredAt).Milliseconds()
	}
	switch event.Type {
	case "SeatReservationHeld", "SeatReservationExpired", "SeatReservationConfirmed", "SeatTicketIssued":
		key := event.Seat.EventID + ":" + event.Seat.SectionID + ":" + event.Seat.SeatID
		event.Seat.Version = event.AggregateVersion
		p.Seats[key] = event.Seat
	case "OrderCreated", "OrderConfirmed", "OrderExpired":
		p.Orders[event.Order.OrderID] = event.Order
		p.VendorOrderIDs[event.Order.EventID] = appendUnique(p.VendorOrderIDs[event.Order.EventID], event.Order.OrderID)
	}
	return nil
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
