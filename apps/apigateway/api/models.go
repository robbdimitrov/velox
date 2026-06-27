package api

import (
	"time"
)

const (
	RoleReserver = "reserver"
	RoleVendor   = "vendor"

	StatusAvailable = "AVAILABLE"
	StatusHeld      = "HELD"
	StatusSold      = "SOLD"

	OrderPending   = "PENDING"
	OrderConfirmed = "CONFIRMED"
	OrderExpired   = "EXPIRED"

	CookieName = "velox_session"
)

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Role     string `json:"role"`
	VendorID string `json:"vendor_id,omitempty"`
}

type Event struct {
	ID          string    `json:"id"`
	VendorID    string    `json:"vendor_id"`
	Name        string    `json:"name"`
	Venue       string    `json:"venue"`
	City        string    `json:"city"`
	StartsAt    time.Time `json:"starts_at"`
	SectionIDs  []string  `json:"section_ids"`
	SeatsTotal  int       `json:"seats_total"`
	SeatsOpen   int       `json:"seats_open"`
	DemandScore int       `json:"demand_score"`
}

type Seat struct {
	EventID           string `json:"event_id"`
	SectionID         string `json:"section_id"`
	ID                string `json:"seat_id"`
	Row               string `json:"row"`
	Number            int    `json:"number"`
	PriceCents        int    `json:"price_cents"`
	Status            string `json:"status"`
	Version           int64  `json:"version"`
	HeldByOrderID     string `json:"held_by_order_id,omitempty"`
	ExpiresAtServerMS int64  `json:"expires_at_server_ms,omitempty"`
}

type Order struct {
	ID                string   `json:"id"`
	ReservationID     string   `json:"reservation_id"`
	UserID            string   `json:"user_id"`
	EventID           string   `json:"event_id"`
	SectionID         string   `json:"section_id"`
	SeatIDs           []string `json:"seat_ids"`
	Status            string   `json:"status"`
	TotalCents        int      `json:"total_cents"`
	ExpiresAtServerMS int64    `json:"expires_at_server_ms,omitempty"`
	CreatedAt         int64    `json:"created_at_server_ms"`
	UpdatedAt         int64    `json:"updated_at_server_ms"`
}

type idempotencyRecord struct {
	Hash     string
	Response Order
}

type loginFailure struct {
	Count       int
	LockedUntil time.Time
}

type apiError struct {
	Error string `json:"error"`
}
