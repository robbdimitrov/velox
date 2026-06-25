package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
	ErrOrderNotFound       = errors.New("order not found")
)

type Order struct {
	ID            string
	UserID        string
	EventID       string
	SectionID     string
	SeatIDs       []string
	Status        string
	ReservationID string
	CreatedAt     time.Time
}

type OutboxEvent struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	CreatedAt     time.Time
	PublishedAt   *time.Time
}

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	orders      map[string]Order
	idempotency map[string]idem
	outbox      []OutboxEvent
}

type idem struct {
	Hash  string
	Order Order
}

func NewService() *Service {
	return &Service{
		now:         time.Now,
		orders:      map[string]Order{},
		idempotency: map[string]idem{},
	}
}

func (s *Service) CreateReservation(userID, idempotencyKey string, req CreateReservationRequest) (Order, error) {
	if userID == "" || idempotencyKey == "" {
		return Order{}, errors.New("missing user or idempotency key")
	}
	hash, err := stableHash(req)
	if err != nil {
		return Order{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userID + ":" + idempotencyKey
	if existing, ok := s.idempotency[key]; ok {
		if existing.Hash != hash {
			return Order{}, ErrIdempotencyConflict
		}
		return existing.Order, nil
	}
	now := s.now()
	order := Order{
		ID: fmt.Sprintf("ord_%d", now.UnixNano()), UserID: userID, EventID: req.EventID,
		SectionID: req.SectionID, SeatIDs: append([]string(nil), req.SeatIDs...),
		Status: "PENDING", ReservationID: fmt.Sprintf("res_%d", now.UnixNano()), CreatedAt: now,
	}
	s.orders[order.ID] = order
	s.idempotency[key] = idem{Hash: hash, Order: order}
	s.outbox = append(s.outbox, OutboxEvent{
		ID: fmt.Sprintf("evt_%d", now.UnixNano()), AggregateType: "order", AggregateID: order.ID,
		EventType: "OrderCreated", Payload: mustJSON(order), CreatedAt: now,
	})
	return order, nil
}

func (s *Service) ConfirmReservation(orderID string) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderID]
	if !ok {
		return Order{}, ErrOrderNotFound
	}
	order.Status = "CONFIRM_REQUESTED"
	s.orders[orderID] = order
	now := s.now()
	s.outbox = append(s.outbox, OutboxEvent{
		ID: fmt.Sprintf("evt_%d_confirm", now.UnixNano()), AggregateType: "order", AggregateID: order.ID,
		EventType: "ReservationConfirmRequested", Payload: mustJSON(order), CreatedAt: now,
	})
	return order, nil
}

func (s *Service) Outbox() []OutboxEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]OutboxEvent(nil), s.outbox...)
}

type CreateReservationRequest struct {
	EventID   string   `json:"event_id"`
	SectionID string   `json:"section_id"`
	SeatIDs   []string `json:"seat_ids"`
}

func stableHash(v any) (string, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func mustJSON(v any) []byte {
	body, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return body
}
