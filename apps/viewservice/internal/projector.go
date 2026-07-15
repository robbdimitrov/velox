package internal

import (
	"errors"
	"sync"
	"time"
)

var ErrStaleAggregateVersion = errors.New("stale aggregate version")

// Event unifies seatservice's flat event shape with orderservice's nested
// envelope; Resolved* helpers hide the producer-specific IDs.
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

// ResolvedEventID returns seatservice's event_id or orderservice's outbox_event_id.
func (e Event) ResolvedEventID() string {
	if e.EventID != "" {
		return e.EventID
	}
	return e.Order.OutboxEventID
}

// ResolvedAggregateID returns seatservice's aggregate_id or orderservice's order_id.
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
	mu                sync.Mutex
	processed         map[string]struct{}
	aggregateVer      map[string]int64
	Seats             map[string]Seat
	Orders            map[string]Order
	OrganizerOrderIDs map[string][]string
	ProjectionLagMS   int64
}

func NewProjector() *Projector {
	return &Projector{
		processed:         map[string]struct{}{},
		aggregateVer:      map[string]int64{},
		Seats:             map[string]Seat{},
		Orders:            map[string]Order{},
		OrganizerOrderIDs: map[string][]string{},
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
	case "SeatReservationHeld", "SeatReservationExpired", "SeatReservationConfirmed", "SeatReservationCancelled", "SeatTicketIssued":
		key := event.Seat.EventID + ":" + event.Seat.SectionID + ":" + event.Seat.SeatID
		event.Seat.Version = event.AggregateVersion
		p.Seats[key] = event.Seat
	case "OrderCreated", "OrderConfirmed", "OrderCancelled", "OrderExpired":
		p.Orders[event.Order.OrderID] = event.Order
		p.OrganizerOrderIDs[event.Order.EventID] = appendUnique(p.OrganizerOrderIDs[event.Order.EventID], event.Order.OrderID)
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
