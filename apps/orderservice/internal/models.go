package internal

import "time"

type OrderRequest struct {
	EventID        string   `json:"event_id"`
	SectionID      string   `json:"section_id"`
	SeatIDs        []string `json:"seat_ids"`
	IdempotencyKey string   `json:"idempotency_key"`
	UserID         string   `json:"user_id"`
}

type OrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type OrderEvent struct {
	OrderID   string    `json:"order_id"`
	UserID    string    `json:"user_id"`
	EventID   string    `json:"event_id"`
	SectionID string    `json:"section_id"`
	SeatIDs   []string  `json:"seat_ids"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
