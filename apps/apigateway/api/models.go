package api

import (
	"time"
)

const (
	RoleReserver  = "reserver"
	RoleOrganizer = "organizer"

	StatusAvailable = "AVAILABLE"
	StatusHeld      = "HELD"
	StatusSold      = "SOLD"

	OrderPending   = "PENDING"
	OrderHeld      = "HELD"
	OrderConfirmed = "CONFIRMED"
	OrderCancelled = "CANCELLED"
	OrderFailed    = "FAILED"
	OrderExpired   = "EXPIRED"

	CookieName = "velox_session"
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Password    string    `json:"-"`
	Role        string    `json:"role"`
	OrganizerID string    `json:"organizer_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Venue struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	City     string `json:"city"`
	Address  string `json:"address"`
	Capacity int    `json:"capacity"`
}

type VenueSeat struct {
	VenueID   string `json:"venue_id"`
	SectionID string `json:"section_id"`
	SeatID    string `json:"seat_id"`
}

type UserVenue struct {
	UserID    string `json:"user_id"`
	VenueID   string `json:"venue_id"`
	VenueRole string `json:"venue_role"`
}

type Event struct {
	ID          string    `json:"id"`
	VenueID     string    `json:"venue_id,omitempty"`
	Status      string    `json:"status,omitempty"`
	OrganizerID string    `json:"organizer_id,omitempty"`
	Name        string    `json:"name"`
	Venue       string    `json:"venue,omitempty"`
	City        string    `json:"city,omitempty"`
	StartsAt    time.Time `json:"starts_at"`
	SectionIDs  []string  `json:"section_ids,omitempty"`
	SeatsTotal  int       `json:"seats_total,omitempty"`
	SeatsOpen   int       `json:"seats_open,omitempty"`
	DemandScore int       `json:"demand_score,omitempty"`
}

type EventAnnouncement struct {
	ID        string    `json:"id"`
	EventID   string    `json:"event_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Severity  string    `json:"severity"`
	CreatedAt time.Time `json:"created_at"`
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

type WalletTicketLedgerEntry struct {
	EventType     string `json:"event_type"`
	Timestamp     string `json:"timestamp"`
	Actor         string `json:"actor"`
	CorrelationID string `json:"correlation_id"`
}

type WalletTicket struct {
	TicketID         string                    `json:"ticket_id"`
	EventID          string                    `json:"event_id"`
	Event            string                    `json:"event"`
	Venue            string                    `json:"venue"`
	SectionID        string                    `json:"section_id"`
	Seat             string                    `json:"seat"`
	Status           string                    `json:"status"`
	TransferStatus   string                    `json:"transfer_status"`
	QRToken          string                    `json:"qr_token"`
	QRTokenExpiresAt string                    `json:"qr_token_expires_at"`
	Ledger           []WalletTicketLedgerEntry `json:"ledger"`
}

type loginFailure struct {
	Count       int
	LockedUntil time.Time
}

type apiError struct {
	Error string `json:"error"`
}
