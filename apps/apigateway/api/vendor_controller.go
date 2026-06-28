package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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
		counts, _, err := s.store.GetVendorInventory(r.Context(), eventID)
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

func (s *Server) handleVendorMetricsStream(w http.ResponseWriter, r *http.Request, user User) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported")
		return
	}

	// For simplicity, we listen to all events for this vendor or a specific event if eventId is query param?
	// The frontend connects to `/vendor/metrics/stream` (no event ID). We'll assume the first event for the vendor.
	
	s.mu.Lock()
	var eventID string
	for _, event := range s.events {
		if event.VendorID == user.VendorID {
			eventID = event.ID
			break
		}
	}
	s.mu.Unlock()

	if eventID == "" {
		writeError(w, http.StatusNotFound, "event_not_found")
		return
	}

	sendMetrics := func() {
		if s.store != nil {
			metrics, err := s.store.GetVendorMetrics(r.Context(), eventID)
			if err == nil {
				payload, _ := json.Marshal(metrics)
				fmt.Fprintf(w, "data: %s\n\n", payload)
				flusher.Flush()
			}
		}
	}

	sendMetrics()

	ch := make(chan string, 10)
	s.mu.Lock()
	if s.vendorClients[eventID] == nil {
		s.vendorClients[eventID] = make(map[chan string]struct{})
	}
	s.vendorClients[eventID][ch] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.vendorClients[eventID], ch)
		s.mu.Unlock()
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			sendMetrics()
		case <-ticker.C:
			// Heartbeat can just re-send metrics
			sendMetrics()
		}
	}
}

func (s *Server) handleListVenues(w http.ResponseWriter, r *http.Request, user User) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"venues": []Venue{}})
		return
	}
	venues, err := s.store.GetVendorVenues(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if venues == nil {
		venues = []Venue{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"venues": venues})
}

func (s *Server) handleListVenueStaff(w http.ResponseWriter, r *http.Request, user User) {
	venueID := r.PathValue("id")
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"staff": []User{}})
		return
	}
	staff, err := s.store.GetVenueStaff(r.Context(), venueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if staff == nil {
		staff = []User{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"staff": staff})
}

func (s *Server) handleCreateEvent(w http.ResponseWriter, r *http.Request, user User) {
	var req Event
	if !decodeJSON(w, r, &req) {
		return
	}
	
	req.Status = "PUBLISHED"
	if req.ID == "" {
		req.ID = "evt_" + time.Now().Format("20060102150405")
	}

	if s.store != nil {
		venues, err := s.store.GetVendorVenues(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		ownsVenue := false
		for _, v := range venues {
			if v.ID == req.VenueID {
				ownsVenue = true
				break
			}
		}
		if !ownsVenue {
			writeError(w, http.StatusForbidden, "not_venue_owner")
			return
		}

		if err := s.store.CreateEvent(r.Context(), req); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
	} else {
		s.mu.Lock()
		s.events[req.ID] = req
		s.mu.Unlock()
	}

	writeJSON(w, http.StatusCreated, req)
}
