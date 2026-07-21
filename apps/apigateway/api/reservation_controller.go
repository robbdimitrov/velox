package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

const reservationTokenPurpose = "reservation"

var (
	errReservationTokenInvalid = errors.New("reservation token invalid")
	errReservationTokenExpired = errors.New("reservation token expired")
)

type reservationTokenClaims struct {
	ReservationID     string
	OrderID           string
	UserID            string
	EventID           string
	SectionID         string
	SeatIDs           []string
	ExpiresAtServerMS int64
	IssuedAtServerMS  int64
}

// validateSeatsAvailable does an early projection check before orderservice;
// seatservice's expected-version append remains authoritative.
// Empty sections are unknown; missing IDs within known sections are bad seats.
func (s *Server) validateSeatsAvailable(w http.ResponseWriter, ctx context.Context, eventID, sectionID string, seatIDs []string) bool {
	statuses, err := s.store.GetSeatStatusMap(ctx, eventID, sectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return false
	}
	if len(statuses) == 0 {
		writeError(w, http.StatusNotFound, "section_not_found")
		return false
	}
	for _, seatID := range seatIDs {
		status, exists := statuses[seatID]
		if !exists {
			writeError(w, http.StatusNotFound, "seat_not_found")
			return false
		}
		if status != StatusAvailable {
			writeError(w, http.StatusConflict, "seat_not_available")
			return false
		}
	}
	return true
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
	body, ok := decodeJSONStrict(w, r, &req)
	if !ok {
		return
	}
	if len(req.SeatIDs) == 0 || len(req.SeatIDs) > 8 {
		writeError(w, http.StatusBadRequest, "invalid_seat_count")
		return
	}
	hash := requestHash(body)
	idemKey := "reserve:" + user.ID + ":" + key
	if s.orderSvcBaseURL == "" {
		writeError(w, http.StatusServiceUnavailable, "order_service_unavailable")
		return
	}

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

	if s.store != nil {
		s.mu.Unlock()
		// Rejects reservations after catalog cancellation, before any hold.
		// GetEventStatus avoids GetEvent's unused inventory aggregation here.
		status, err := s.store.GetEventStatus(r.Context(), req.EventID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if status != EventStatusPublished {
			writeError(w, http.StatusConflict, "event_not_bookable")
			return
		}
		if !s.validateSeatsAvailable(w, r.Context(), req.EventID, req.SectionID, req.SeatIDs) {
			return
		}
	} else {
		section, ok := s.seats[req.EventID][req.SectionID]
		if !ok {
			s.mu.Unlock()
			writeError(w, http.StatusNotFound, "section_not_found")
			return
		}
		for _, seatID := range req.SeatIDs {
			seat, ok := section[seatID]
			if !ok {
				s.mu.Unlock()
				writeError(w, http.StatusNotFound, "seat_not_found")
				return
			}
			s.expireSeatIfNeededLocked(seat)
			if seat.Status != StatusAvailable {
				s.mu.Unlock()
				writeError(w, http.StatusConflict, "seat_not_available")
				return
			}
		}
		for _, seatID := range req.SeatIDs {
			seat := section[seatID]
			seat.Status = StatusHeld
			seat.HeldByOrderID = idemKey
		}
		s.mu.Unlock()
	}

	orderReq := map[string]any{
		"event_id":        req.EventID,
		"section_id":      req.SectionID,
		"seat_ids":        req.SeatIDs,
		"idempotency_key": key,
		"user_id":         user.ID,
	}
	bodyBytes, _ := json.Marshal(orderReq)

	resp, err := s.doOrderServiceRequest(r.Context(), "/orders", bodyBytes)
	if err != nil {
		s.releasePendingReservation(req.EventID, req.SectionID, req.SeatIDs, idemKey)
		writeError(w, http.StatusBadGateway, "upstream_error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		s.releasePendingReservation(req.EventID, req.SectionID, req.SeatIDs, idemKey)
		writeUpstreamError(w, resp)
		return
	}

	var upstreamOrder struct {
		OrderID string `json:"order_id"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstreamOrder); err != nil {
		s.releasePendingReservation(req.EventID, req.SectionID, req.SeatIDs, idemKey)
		writeError(w, http.StatusBadGateway, "upstream_error")
		return
	}
	if upstreamOrder.OrderID == "" || upstreamOrder.Status == "" {
		s.releasePendingReservation(req.EventID, req.SectionID, req.SeatIDs, idemKey)
		writeError(w, http.StatusBadGateway, "upstream_error")
		return
	}

	order, err := s.completePendingReservation(r.Context(), user.ID, req.EventID, req.SectionID, req.SeatIDs, idemKey, hash, upstreamOrder.OrderID, upstreamOrder.Status)
	if err != nil {
		s.releasePendingReservation(req.EventID, req.SectionID, req.SeatIDs, idemKey)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, resp.StatusCode, map[string]any{"order": order})
}

func (s *Server) handleConfirmReservation(w http.ResponseWriter, r *http.Request, user User) {
	s.forwardOrderAction(w, r, user, "confirm")
}

func (s *Server) handleCancelReservation(w http.ResponseWriter, r *http.Request, user User) {
	s.forwardOrderAction(w, r, user, "cancel")
}

// forwardOrderAction verifies reservation ownership, then proxies confirm or
// cancel to orderservice's internal /orders/{id}/{action} endpoint.
func (s *Server) forwardOrderAction(w http.ResponseWriter, r *http.Request, user User, action string) {
	reservationID := r.PathValue("reservationId")
	token := r.Header.Get("Reservation-Token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "reservation_token_required")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing_idempotency_key")
		return
	}

	claims, err := s.verifyReservationToken(token, user, reservationID)
	if err != nil {
		if errors.Is(err, errReservationTokenExpired) {
			writeError(w, http.StatusConflict, "reservation_token_expired")
			return
		}
		writeError(w, http.StatusUnauthorized, "reservation_token_invalid")
		return
	}
	orderID := strings.TrimPrefix(reservationID, "res_")
	if !constantTimeStringEqual(claims.OrderID, orderID) {
		writeError(w, http.StatusUnauthorized, "reservation_token_invalid")
		return
	}
	hash := requestHash([]byte(action + ":" + reservationID + ":" + token))
	idemKey := "reservation_action:" + action + ":" + user.ID + ":" + key
	s.mu.Lock()
	if rec, exists := s.idempotency[idemKey]; exists {
		s.mu.Unlock()
		if rec.Hash != hash {
			writeError(w, http.StatusConflict, "idempotency_key_conflict")
			return
		}
		writeJSON(w, http.StatusOK, rec.Response)
		return
	}
	s.mu.Unlock()

	if s.store != nil {
		order, err := s.store.GetOrder(r.Context(), user, orderID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if !reservationTokenMatchesOrder(claims, order) {
			writeError(w, http.StatusUnauthorized, "reservation_token_invalid")
			return
		}
	} else {
		s.mu.Lock()
		order, ok := s.orderByReservationLocked(reservationID)
		s.mu.Unlock()
		if !ok || order.UserID != user.ID {
			writeError(w, http.StatusNotFound, "order_not_found")
			return
		}
		if !reservationTokenMatchesOrder(claims, *order) {
			writeError(w, http.StatusUnauthorized, "reservation_token_invalid")
			return
		}
	}

	if s.orderSvcBaseURL == "" {
		writeError(w, http.StatusServiceUnavailable, "order_service_unavailable")
		return
	}

	resp, err := s.doOrderServiceRequest(r.Context(), "/orders/"+orderID+"/"+action, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		writeUpstreamError(w, resp)
		return
	}

	var upstream struct {
		OrderID string `json:"order_id"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error")
		return
	}
	out := map[string]any{
		"order_id":          upstream.OrderID,
		"status":            upstream.Status,
		"wallet_ticket_ids": []string{},
	}
	if action == "confirm" && s.store != nil {
		ticketIDs, err := s.store.GetWalletTicketIDsForOrder(r.Context(), user.ID, upstream.OrderID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		out["wallet_ticket_ids"] = ticketIDs
	}
	s.mu.Lock()
	s.idempotency[idemKey] = idempotencyRecord{Hash: hash, Response: out}
	s.mu.Unlock()
	writeJSON(w, resp.StatusCode, out)
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

func (s *Server) releasePendingReservation(eventID, sectionID string, seatIDs []string, pendingID string) {
	if s.store != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	section, ok := s.seats[eventID][sectionID]
	if !ok {
		return
	}
	for _, seatID := range seatIDs {
		seat, ok := section[seatID]
		if !ok || seat.Status != StatusHeld || seat.HeldByOrderID != pendingID {
			continue
		}
		seat.Status = StatusAvailable
		seat.HeldByOrderID = ""
		seat.ExpiresAtServerMS = 0
	}
}

func (s *Server) completePendingReservation(ctx context.Context, userID, eventID, sectionID string, seatIDs []string, pendingID, hash, orderID, status string) (Order, error) {
	reservationID := "res_" + orderID
	now := s.now()
	selectedSeats, err := s.selectedReservationSeats(ctx, eventID, sectionID, seatIDs)
	if err != nil {
		return Order{}, err
	}
	order := Order{
		ID:                orderID,
		ReservationID:     reservationID,
		UserID:            userID,
		EventID:           eventID,
		SectionID:         sectionID,
		SeatIDs:           append([]string(nil), seatIDs...),
		Seats:             selectedSeats,
		Status:            status,
		ExpiresAtServerMS: now.Add(s.holdTTL).UnixMilli(),
		ServerTimeMS:      now.UnixMilli(),
		CreatedAt:         now.UnixMilli(),
		UpdatedAt:         now.UnixMilli(),
	}
	token, err := s.signReservationToken(order, now)
	if err != nil {
		return Order{}, err
	}
	order.ReservationToken = token
	if s.store != nil {
		return order, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	section, ok := s.seats[eventID][sectionID]
	if ok {
		for _, seatID := range seatIDs {
			seat, ok := section[seatID]
			if !ok || seat.Status != StatusHeld || seat.HeldByOrderID != pendingID {
				continue
			}
			seat.HeldByOrderID = orderID
			seat.ExpiresAtServerMS = order.ExpiresAtServerMS
		}
	}
	s.orders[orderID] = &order
	s.idempotency[pendingID] = idempotencyRecord{Hash: hash, Response: order}
	return order, nil
}

func (s *Server) selectedReservationSeats(ctx context.Context, eventID, sectionID string, seatIDs []string) ([]Seat, error) {
	wanted := make(map[string]struct{}, len(seatIDs))
	for _, seatID := range seatIDs {
		wanted[seatID] = struct{}{}
	}
	selected := make([]Seat, 0, len(seatIDs))
	if s.store != nil {
		seats, _, err := s.store.ListSeats(ctx, eventID, sectionID)
		if err != nil {
			return nil, err
		}
		for _, seat := range seats {
			if _, ok := wanted[seat.ID]; !ok {
				continue
			}
			selected = append(selected, seat)
		}
		sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
		return selected, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	section := s.seats[eventID][sectionID]
	for _, seatID := range seatIDs {
		seat := section[seatID]
		selected = append(selected, *seat)
	}
	return selected, nil
}

func (s *Server) signReservationToken(order Order, issuedAt time.Time) (string, error) {
	return signHMAC(s.secret, map[string]any{
		"purpose":              reservationTokenPurpose,
		"iss":                  s.tokenIssuer,
		"aud":                  s.tokenAudience,
		"reservation_id":       order.ReservationID,
		"order_id":             order.ID,
		"user_id":              order.UserID,
		"event_id":             order.EventID,
		"section_id":           order.SectionID,
		"seat_ids":             order.SeatIDs,
		"expires_at":           time.UnixMilli(order.ExpiresAtServerMS).UTC().Format(time.RFC3339),
		"expires_at_server_ms": order.ExpiresAtServerMS,
		"issued_at":            issuedAt.UTC().Format(time.RFC3339),
		"issued_at_server_ms":  issuedAt.UnixMilli(),
	})
}

func (s *Server) verifyReservationToken(token string, user User, reservationID string) (reservationTokenClaims, error) {
	payload, err := verifyHMAC(s.secret, token)
	if err != nil {
		return reservationTokenClaims{}, errReservationTokenInvalid
	}
	if stringClaim(payload, "purpose") != reservationTokenPurpose ||
		stringClaim(payload, "iss") != s.tokenIssuer ||
		stringClaim(payload, "aud") != s.tokenAudience {
		return reservationTokenClaims{}, errReservationTokenInvalid
	}
	claims := reservationTokenClaims{
		ReservationID:     stringClaim(payload, "reservation_id"),
		OrderID:           stringClaim(payload, "order_id"),
		UserID:            stringClaim(payload, "user_id"),
		EventID:           stringClaim(payload, "event_id"),
		SectionID:         stringClaim(payload, "section_id"),
		ExpiresAtServerMS: int64NumberClaim(payload, "expires_at_server_ms"),
		IssuedAtServerMS:  int64NumberClaim(payload, "issued_at_server_ms"),
	}
	if claims.ReservationID == "" || claims.OrderID == "" || claims.UserID == "" ||
		claims.EventID == "" || claims.SectionID == "" || claims.ExpiresAtServerMS == 0 {
		return reservationTokenClaims{}, errReservationTokenInvalid
	}
	seatIDs, ok := stringSliceClaim(payload, "seat_ids")
	if !ok || len(seatIDs) == 0 {
		return reservationTokenClaims{}, errReservationTokenInvalid
	}
	claims.SeatIDs = seatIDs
	if !constantTimeStringEqual(claims.ReservationID, reservationID) ||
		!constantTimeStringEqual(claims.UserID, user.ID) {
		return reservationTokenClaims{}, errReservationTokenInvalid
	}
	if s.now().UnixMilli() >= claims.ExpiresAtServerMS {
		return reservationTokenClaims{}, errReservationTokenExpired
	}
	return claims, nil
}

func stringClaim(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

func int64NumberClaim(payload map[string]any, key string) int64 {
	switch value := payload[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}

func stringSliceClaim(payload map[string]any, key string) ([]string, bool) {
	values, ok := payload[key].([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok || text == "" {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

func reservationTokenMatchesOrder(claims reservationTokenClaims, order Order) bool {
	if !constantTimeStringEqual(claims.EventID, order.EventID) ||
		!constantTimeStringEqual(claims.SectionID, order.SectionID) {
		return false
	}
	if len(claims.SeatIDs) != len(order.SeatIDs) {
		return false
	}
	tokenSeats := append([]string(nil), claims.SeatIDs...)
	orderSeats := append([]string(nil), order.SeatIDs...)
	sort.Strings(tokenSeats)
	sort.Strings(orderSeats)
	for i := range tokenSeats {
		if !constantTimeStringEqual(tokenSeats[i], orderSeats[i]) {
			return false
		}
	}
	return true
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
