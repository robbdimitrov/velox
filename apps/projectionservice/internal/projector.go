package internal

import (
	"errors"
	"sync"
	"time"
)

var ErrStaleAggregateVersion = errors.New("stale aggregate version")

type Event struct {
	EventID          string
	AggregateID      string
	AggregateVersion int64
	Type             string
	Seat             Seat
	Order            Order
	OccurredAt       time.Time
}

type Seat struct {
	EventID     string
	SectionID   string
	SeatID      string
	Status      string
	Version     int64
	ExpiresAtMS int64
}

type Order struct {
	OrderID string
	UserID  string
	EventID string
	Status  string
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
	if _, ok := p.processed[event.EventID]; ok {
		return nil
	}
	if last := p.aggregateVer[event.AggregateID]; last >= event.AggregateVersion {
		return ErrStaleAggregateVersion
	}
	p.processed[event.EventID] = struct{}{}
	p.aggregateVer[event.AggregateID] = event.AggregateVersion
	if !event.OccurredAt.IsZero() {
		p.ProjectionLagMS = time.Since(event.OccurredAt).Milliseconds()
	}
	switch event.Type {
	case "SeatReservationHeld", "SeatReservationExpired", "SeatTicketIssued":
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
