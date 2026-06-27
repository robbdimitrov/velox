package internal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	RoleReserver = "reserver"
	RoleVendor = "vendor"

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

type Server struct {
	mu          sync.Mutex
	secret      []byte
	now         func() time.Time
	holdTTL     time.Duration
	users       map[string]User
	events      map[string]Event
	seats       map[string]map[string]map[string]*Seat
	orders      map[string]*Order
	idempotency map[string]idempotencyRecord
	loginFails  map[string]loginFailure
	store       *PostgresStore
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

func NewServer(secret string) *Server {
	return NewServerWithStore(secret, nil)
}

func NewServerWithStore(secret string, store *PostgresStore) *Server {
	s := &Server{
		secret:      []byte(secret),
		now:         time.Now,
		holdTTL:     5 * time.Minute,
		users:       map[string]User{},
		events:      map[string]Event{},
		seats:       map[string]map[string]map[string]*Seat{},
		orders:      map[string]*Order{},
		idempotency: map[string]idempotencyRecord{},
		loginFails:  map[string]loginFailure{},
		store:       store,
	}
	s.seed()
	return s
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /sessions", s.handleCreateSession)
	mux.HandleFunc("DELETE /sessions", s.handleDeleteSession)
	mux.HandleFunc("GET /events", s.handleEvents)
	mux.HandleFunc("GET /events/{eventId}", s.handleEvent)
	mux.HandleFunc("GET /events/{eventId}/sections/{sectionId}/seats", s.handleSeats)
	mux.HandleFunc("GET /events/{eventId}/stream", s.handleSeatStream)
	mux.HandleFunc("POST /reservations", s.requireRole(RoleReserver, s.handleCreateReservation))
	mux.HandleFunc("POST /reservations/{reservationId}/confirm", s.requireRole(RoleReserver, s.handleConfirmReservation))
	mux.HandleFunc("GET /orders", s.requireRole(RoleReserver, s.handleOrders))
	mux.HandleFunc("GET /orders/{orderId}", s.requireRole(RoleReserver, s.handleOrder))
	mux.HandleFunc("GET /vendor/events", s.requireRole(RoleVendor, s.handleVendorEvents))
	mux.HandleFunc("GET /vendor/events/{eventId}/orders", s.requireRole(RoleVendor, s.handleVendorOrders))
	mux.HandleFunc("GET /vendor/events/{eventId}/inventory", s.requireRole(RoleVendor, s.handleVendorInventory))
	return limitBody(mux, 1<<20)
}

func (s *Server) seed() {
	s.users["reserver@velox.local"] = User{ID: "usr_reserver_1", Email: "reserver@velox.local", Password: "reserver", Role: RoleReserver}
	s.users["vendor@velox.local"] = User{ID: "usr_vendor_1", Email: "vendor@velox.local", Password: "vendor", Role: RoleVendor, VendorID: "ven_northstar"}
	event := Event{
		ID:          "evt_neon_riot",
		VendorID:    "ven_northstar",
		Name:        "Neon Riot Live",
		Venue:       "Velox Arena",
		City:        "Chicago",
		StartsAt:    time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC),
		SectionIDs:  []string{"A", "B"},
		DemandScore: 94,
	}
	s.events[event.ID] = event
	s.seats[event.ID] = map[string]map[string]*Seat{}
	for _, sectionID := range event.SectionIDs {
		s.seats[event.ID][sectionID] = map[string]*Seat{}
		for row := 'A'; row <= 'D'; row++ {
			for n := 1; n <= 10; n++ {
				id := fmt.Sprintf("%c-%02d", row, n)
				s.seats[event.ID][sectionID][id] = &Seat{
					EventID: event.ID, SectionID: sectionID, ID: id,
					Row: string(row), Number: n, PriceCents: 8500 + n*150,
					Status: StatusAvailable, Version: 1,
				}
				event.SeatsTotal++
				event.SeatsOpen++
			}
		}
	}
	s.events[event.ID] = event
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email := strings.ToLower(req.Email)
	attemptKey := email + "|" + clientIP(r)
	s.mu.Lock()
	now := s.now()
	if failure := s.loginFails[attemptKey]; failure.LockedUntil.After(now) {
		s.mu.Unlock()
		writeError(w, http.StatusTooManyRequests, "too_many_login_attempts")
		return
	}
	user, ok := s.users[email]
	s.mu.Unlock()
	if !ok || !constantTimeStringEqual(user.Password, req.Password) {
		s.recordLoginFailure(attemptKey, now)
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	s.clearLoginFailure(attemptKey)
	token, err := s.sign(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_signing_failed")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: s.now().Add(12 * time.Hour)})
	writeJSON(w, http.StatusOK, map[string]any{"user": publicUser(user)})
}

func (s *Server) recordLoginFailure(key string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	failure := s.loginFails[key]
	failure.Count++
	if failure.Count >= 5 {
		failure.LockedUntil = now.Add(5 * time.Minute)
	}
	s.loginFails[key] = failure
}

func (s *Server) clearLoginFailure(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.loginFails, key)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		event.SeatsOpen = s.openSeatsLocked(event.ID)
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].StartsAt.Before(events[j].StartsAt) })
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "projection_lag_ms": 0})
}

func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("eventId")
	s.mu.Lock()
	event, ok := s.events[eventID]
	if ok {
		event.SeatsOpen = s.openSeatsLocked(event.ID)
	}
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "event_not_found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"event": event, "projection_lag_ms": 0})
}

func (s *Server) handleSeats(w http.ResponseWriter, r *http.Request) {
	eventID, sectionID := r.PathValue("eventId"), r.PathValue("sectionId")
	if s.store != nil {
		seats, err := s.store.ListSeats(r.Context(), eventID, sectionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "seat_snapshot_unavailable")
			return
		}
		if len(seats) > 0 {
			writeJSON(w, http.StatusOK, map[string]any{"seats": seats, "snapshot_age_ms": 0})
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	section, ok := s.seats[eventID][sectionID]
	if !ok {
		writeError(w, http.StatusNotFound, "section_not_found")
		return
	}
	seats := make([]Seat, 0, len(section))
	for _, seat := range section {
		s.expireSeatIfNeededLocked(seat)
		seats = append(seats, *seat)
	}
	sort.Slice(seats, func(i, j int) bool { return seats[i].ID < seats[j].ID })
	writeJSON(w, http.StatusOK, map[string]any{"seats": seats, "snapshot_age_ms": 0})
}

func (s *Server) handleSeatStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintf(w, "event: heartbeat\ndata: {\"event_id\":%q}\n\n", r.PathValue("eventId"))
}

func (s *Server) handleCreateReservation(w http.ResponseWriter, r *http.Request, user User) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing_idempotency_key")
		return
	}
	var req struct {
		EventID   string   `json:"event_id"`
		SectionID string   `json:"section_id"`
		SeatIDs   []string `json:"seat_ids"`
	}
	body, ok := decodeJSONBytes(w, r, &req)
	if !ok {
		return
	}
	if len(req.SeatIDs) == 0 || len(req.SeatIDs) > 8 {
		writeError(w, http.StatusBadRequest, "invalid_seat_count")
		return
	}
	hash := requestHash(body)
	idemKey := "reserve:" + user.ID + ":" + key

	s.mu.Lock()
	if rec, exists := s.idempotency[idemKey]; exists {
		s.mu.Unlock()
		if rec.Hash != hash {
			writeError(w, http.StatusConflict, "idempotency_key_conflict")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"order": rec.Response})
		return
	}
	section, ok := s.seats[req.EventID][req.SectionID]
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusNotFound, "section_not_found")
		return
	}
	selectedSeats := make([]Seat, 0, len(req.SeatIDs))
	for _, seatID := range req.SeatIDs {
		seat, ok := section[seatID]
		if !ok {
			s.mu.Unlock()
			writeError(w, http.StatusNotFound, "seat_not_found")
			return
		}
		s.expireSeatIfNeededLocked(seat)
		if s.store == nil && seat.Status != StatusAvailable {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, "seat_not_available")
			return
		}
		selectedSeats = append(selectedSeats, *seat)
	}
	if s.store != nil {
		s.mu.Unlock()
		order, idemHit, err := s.store.CreateReservation(
			r.Context(),
			user,
			key,
			hash,
			ReservationRequest{EventID: req.EventID, SectionID: req.SectionID, SeatIDs: append([]string(nil), req.SeatIDs...)},
			selectedSeats,
			s.now(),
			s.holdTTL,
		)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		s.applyOrderHold(order)
		if idemHit {
			writeJSON(w, http.StatusOK, map[string]any{"order": order})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"order": order})
		return
	}
	now := s.now()
	orderID := fmt.Sprintf("ord_%d", now.UnixNano())
	expiresAt := now.Add(s.holdTTL).UnixMilli()
	total := 0
	for _, seatID := range req.SeatIDs {
		seat := section[seatID]
		seat.Status = StatusHeld
		seat.Version++
		seat.HeldByOrderID = orderID
		seat.ExpiresAtServerMS = expiresAt
		total += seat.PriceCents
	}
	order := &Order{
		ID: orderID, ReservationID: "res_" + orderID, UserID: user.ID, EventID: req.EventID,
		SectionID: req.SectionID, SeatIDs: append([]string(nil), req.SeatIDs...), Status: OrderPending,
		TotalCents: total, ExpiresAtServerMS: expiresAt, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli(),
	}
	s.orders[order.ID] = order
	s.idempotency[idemKey] = idempotencyRecord{Hash: hash, Response: *order}
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"order": order})
}

func (s *Server) handleConfirmReservation(w http.ResponseWriter, r *http.Request, user User) {
	reservationID := r.PathValue("reservationId")
	if s.store != nil {
		order, err := s.store.ConfirmReservation(r.Context(), user, reservationID, s.now())
		if err != nil {
			writeStoreError(w, err)
			return
		}
		s.applyOrderSold(order)
		writeJSON(w, http.StatusOK, map[string]any{"order": order})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orderByReservationLocked(reservationID)
	if !ok || order.UserID != user.ID {
		writeError(w, http.StatusNotFound, "reservation_not_found")
		return
	}
	if order.Status == OrderConfirmed {
		writeJSON(w, http.StatusOK, map[string]any{"order": order})
		return
	}
	if s.now().UnixMilli() >= order.ExpiresAtServerMS {
		s.expireOrderLocked(order)
		writeError(w, http.StatusConflict, "reservation_expired")
		return
	}
	for _, seatID := range order.SeatIDs {
		seat := s.seats[order.EventID][order.SectionID][seatID]
		if seat.Status != StatusHeld || seat.HeldByOrderID != order.ID {
			writeError(w, http.StatusConflict, "reservation_not_held")
			return
		}
	}
	for _, seatID := range order.SeatIDs {
		seat := s.seats[order.EventID][order.SectionID][seatID]
		seat.Status = StatusSold
		seat.Version++
		seat.ExpiresAtServerMS = 0
	}
	order.Status = OrderConfirmed
	order.ExpiresAtServerMS = 0
	order.UpdatedAt = s.now().UnixMilli()
	writeJSON(w, http.StatusOK, map[string]any{"order": order})
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request, user User) {
	if s.store != nil {
		orders, err := s.store.ListOrders(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "orders_unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	orders := make([]Order, 0)
	for _, order := range s.orders {
		if order.UserID == user.ID {
			if s.now().UnixMilli() >= order.ExpiresAtServerMS && order.Status == OrderPending {
				s.expireOrderLocked(order)
			}
			orders = append(orders, *order)
		}
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].CreatedAt > orders[j].CreatedAt })
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (s *Server) handleOrder(w http.ResponseWriter, r *http.Request, user User) {
	if s.store != nil {
		order, err := s.store.GetOrder(r.Context(), user, r.PathValue("orderId"))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"order": order})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[r.PathValue("orderId")]
	if !ok || order.UserID != user.ID {
		writeError(w, http.StatusNotFound, "order_not_found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": order})
}

func (s *Server) handleVendorEvents(w http.ResponseWriter, r *http.Request, user User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var events []Event
	for _, event := range s.events {
		if event.VendorID == user.VendorID {
			event.SeatsOpen = s.openSeatsLocked(event.ID)
			events = append(events, event)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) handleVendorOrders(w http.ResponseWriter, r *http.Request, user User) {
	if !s.vendorOwnsEvent(r.PathValue("eventId"), user) {
		writeError(w, http.StatusNotFound, "event_not_found")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var orders []Order
	for _, order := range s.orders {
		if order.EventID == r.PathValue("eventId") {
			orders = append(orders, *order)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (s *Server) handleVendorInventory(w http.ResponseWriter, r *http.Request, user User) {
	eventID := r.PathValue("eventId")
	if !s.vendorOwnsEvent(eventID, user) {
		writeError(w, http.StatusNotFound, "event_not_found")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[string]int{StatusAvailable: 0, StatusHeld: 0, StatusSold: 0}
	for _, section := range s.seats[eventID] {
		for _, seat := range section {
			s.expireSeatIfNeededLocked(seat)
			counts[seat.Status]++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"inventory": counts, "active_holds": counts[StatusHeld]})
}

func (s *Server) requireRole(role string, next func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.authenticate(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication_required")
			return
		}
		if user.Role != role {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r, user)
	}
}

func (s *Server) authenticate(r *http.Request) (User, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return User{}, err
	}
	userID, err := s.verify(cookie.Value)
	if err != nil {
		return User{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, user := range s.users {
		if user.ID == userID {
			return user, nil
		}
	}
	return User{}, errors.New("unknown user")
}

func (s *Server) sign(user User) (string, error) {
	payload, err := json.Marshal(map[string]any{"sub": user.ID, "role": user.Role, "exp": s.now().Add(12 * time.Hour).Unix()})
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Server) verify(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", errors.New("bad token")
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, actual) {
		return "", errors.New("bad signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	var payload struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", err
	}
	if payload.Exp <= s.now().Unix() {
		return "", errors.New("expired token")
	}
	return payload.Sub, nil
}

func (s *Server) vendorOwnsEvent(eventID string, user User) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[eventID]
	return ok && event.VendorID == user.VendorID
}

func (s *Server) orderByReservationLocked(reservationID string) (*Order, bool) {
	for _, order := range s.orders {
		if order.ReservationID == reservationID {
			return order, true
		}
	}
	return nil, false
}

func (s *Server) expireSeatIfNeededLocked(seat *Seat) {
	if seat.Status == StatusHeld && seat.ExpiresAtServerMS > 0 && s.now().UnixMilli() >= seat.ExpiresAtServerMS {
		seat.Status = StatusAvailable
		seat.Version++
		seat.HeldByOrderID = ""
		seat.ExpiresAtServerMS = 0
	}
}

func (s *Server) expireOrderLocked(order *Order) {
	if order.Status != OrderPending {
		return
	}
	for _, seatID := range order.SeatIDs {
		seat := s.seats[order.EventID][order.SectionID][seatID]
		if seat.Status == StatusHeld && seat.HeldByOrderID == order.ID {
			seat.Status = StatusAvailable
			seat.Version++
			seat.HeldByOrderID = ""
			seat.ExpiresAtServerMS = 0
		}
	}
	order.Status = OrderExpired
	order.UpdatedAt = s.now().UnixMilli()
}

func (s *Server) openSeatsLocked(eventID string) int {
	open := 0
	for _, section := range s.seats[eventID] {
		for _, seat := range section {
			s.expireSeatIfNeededLocked(seat)
			if seat.Status == StatusAvailable {
				open++
			}
		}
	}
	return open
}

func publicUser(user User) map[string]string {
	out := map[string]string{"id": user.ID, "email": user.Email, "role": user.Role}
	if user.VendorID != "" {
		out["vendor_id"] = user.VendorID
	}
	return out
}

func requestHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func constantTimeStringEqual(expected, actual string) bool {
	expectedHash := sha256.Sum256([]byte(expected))
	actualHash := sha256.Sum256([]byte(actual))
	return hmac.Equal(expectedHash[:], actualHash[:])
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func decodeJSONBytes(w http.ResponseWriter, r *http.Request, dst any) ([]byte, bool) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return nil, false
	}
	if len(raw) == 0 {
		writeError(w, http.StatusBadRequest, "empty_json")
		return nil, false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_schema")
		return nil, false
	}
	return raw, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	_, ok := decodeJSONBytes(w, r, dst)
	return ok
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrStoreIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_key_conflict")
	case errors.Is(err, ErrStoreConflict):
		writeError(w, http.StatusConflict, "seat_not_available")
	case errors.Is(err, ErrStoreExpired):
		writeError(w, http.StatusConflict, "reservation_expired")
	case errors.Is(err, ErrStoreNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		writeError(w, http.StatusInternalServerError, "store_unavailable")
	}
}

func (s *Server) applyOrderHold(order Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	section := s.seats[order.EventID][order.SectionID]
	for _, seatID := range order.SeatIDs {
		if seat := section[seatID]; seat != nil {
			seat.Status = StatusHeld
			seat.Version++
			seat.HeldByOrderID = order.ID
			seat.ExpiresAtServerMS = order.ExpiresAtServerMS
		}
	}
	copied := order
	s.orders[order.ID] = &copied
}

func (s *Server) applyOrderSold(order Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	section := s.seats[order.EventID][order.SectionID]
	for _, seatID := range order.SeatIDs {
		if seat := section[seatID]; seat != nil {
			seat.Status = StatusSold
			seat.Version++
			seat.ExpiresAtServerMS = 0
		}
	}
	copied := order
	s.orders[order.ID] = &copied
}

func limitBody(next http.Handler, n int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, n)
		next.ServeHTTP(w, r)
	})
}
