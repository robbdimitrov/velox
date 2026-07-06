package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

// validateSeatsAvailable checks each requested seat against the real
// event-sourced projection (projection.seat_snapshots) rather than the
// in-memory demo seed. Every seat in this codebase gets its snapshot row
// pre-created with status AVAILABLE (see GetSeatStatusMap's doc comment), so
// an empty result for the section means the section is unknown, and a
// missing seat_id within a known section means that seat doesn't exist. This
// is a cheap early rejection only — the authoritative check-and-reserve
// happens in seatservice via expected-version locking once the order reaches
// Kafka, so a race here just means the request proceeds to orderservice and
// fails downstream instead of failing fast.
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
		// Closes the cancellation race: without this, a reservation could be
		// created after handleCancelEvent's catalog update but its orderservice
		// bulk-cancel hasn't reached this order yet (or never will, since the
		// order didn't exist when it ran), leaving a live booking on a
		// cancelled event. Uses the lean GetEventStatus rather than GetEvent so
		// this hottest write path skips GetEvent's GetOrganizerInventory
		// seat-count aggregation, which this check never reads.
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

	order := s.completePendingReservation(user.ID, req.EventID, req.SectionID, req.SeatIDs, idemKey, hash, upstreamOrder.OrderID, upstreamOrder.Status)
	writeJSON(w, resp.StatusCode, map[string]any{"order": order})
}

func (s *Server) handleConfirmReservation(w http.ResponseWriter, r *http.Request, user User) {
	s.forwardOrderAction(w, r, user, "confirm")
}

func (s *Server) handleCancelReservation(w http.ResponseWriter, r *http.Request, user User) {
	s.forwardOrderAction(w, r, user, "cancel")
}

// forwardOrderAction proxies a terminal state-transition action (confirm or
// cancel) to orderservice's internal /orders/{id}/{action} endpoint.
// orderservice's internal endpoints take no user context and trust apigateway
// as the auth boundary, so ownership of the order behind the reservation ID
// must be verified here before forwarding, the same way handleOrder verifies
// ownership before returning order data.
func (s *Server) forwardOrderAction(w http.ResponseWriter, r *http.Request, user User, action string) {
	reservationID := r.PathValue("reservationId")
	orderID := strings.TrimPrefix(reservationID, "res_")

	if s.store != nil {
		if _, err := s.store.GetOrder(r.Context(), user, orderID); err != nil {
			writeStoreError(w, err)
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
	writeJSON(w, resp.StatusCode, map[string]any{"order_id": upstream.OrderID, "status": upstream.Status})
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

func (s *Server) completePendingReservation(userID, eventID, sectionID string, seatIDs []string, pendingID, hash, orderID, status string) Order {
	reservationID := "res_" + orderID
	now := s.now()
	order := Order{
		ID:                orderID,
		ReservationID:     reservationID,
		UserID:            userID,
		EventID:           eventID,
		SectionID:         sectionID,
		SeatIDs:           append([]string(nil), seatIDs...),
		Status:            status,
		ExpiresAtServerMS: now.Add(s.holdTTL).UnixMilli(),
		CreatedAt:         now.UnixMilli(),
		UpdatedAt:         now.UnixMilli(),
	}
	if s.store != nil {
		return order
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
			order.TotalCents += seat.PriceCents
			seat.HeldByOrderID = orderID
			seat.ExpiresAtServerMS = order.ExpiresAtServerMS
		}
	}
	s.orders[orderID] = &order
	s.idempotency[pendingID] = idempotencyRecord{Hash: hash, Response: order}
	return order
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
