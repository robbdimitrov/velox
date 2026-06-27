package api

import (
	"net/http"
)

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
	if s.store != nil {
		counts, err := s.store.GetVendorInventory(r.Context(), eventID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "inventory_unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"inventory": counts, "active_holds": counts[StatusHeld]})
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

func (s *Server) vendorOwnsEvent(eventID string, user User) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[eventID]
	return ok && event.VendorID == user.VendorID
}
