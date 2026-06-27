package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
)

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
	}
	s.mu.Unlock()

	orderReq := map[string]any{
		"event_id":        req.EventID,
		"section_id":      req.SectionID,
		"seat_ids":        req.SeatIDs,
		"idempotency_key": key,
		"user_id":         user.ID,
	}
	bodyBytes, _ := json.Marshal(orderReq)

	httpReq, err := http.NewRequestWithContext(r.Context(), "POST", s.orderSvcURL, bytes.NewReader(bodyBytes))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if reqID, ok := r.Context().Value(RequestIDKey).(string); ok {
		httpReq.Header.Set("X-Request-ID", reqID)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		writeError(w, resp.StatusCode, errResp.Error)
		return
	}

	var upstreamOrder struct {
		OrderID string `json:"order_id"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstreamOrder); err != nil {
		writeError(w, http.StatusInternalServerError, "upstream_error")
		return
	}

	writeJSON(w, resp.StatusCode, map[string]any{
		"order": map[string]string{
			"id":             upstreamOrder.OrderID,
			"status":         upstreamOrder.Status,
			"reservation_id": "res_" + upstreamOrder.OrderID,
		},
	})
}

func (s *Server) handleConfirmReservation(w http.ResponseWriter, r *http.Request, user User) {
	// API gateway just acknowledges. The order service saga runs the payment and confirmation automatically.
	writeJSON(w, http.StatusOK, map[string]any{"status": "CONFIRMING"})
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
