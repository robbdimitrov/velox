package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (s *Server) handleOrganizerEvents(w http.ResponseWriter, r *http.Request, user User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var events []Event
	for _, event := range s.events {
		if event.OrganizerID == user.OrganizerID {
			event.SeatsOpen = s.openSeatsLocked(event.ID)
			events = append(events, event)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) handleOrganizerOrders(w http.ResponseWriter, r *http.Request, user User) {
	if !s.organizerOwnsEvent(r.PathValue("eventId"), user) {
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

func (s *Server) handleOrganizerInventory(w http.ResponseWriter, r *http.Request, user User) {
	eventID := r.PathValue("eventId")
	if !s.organizerOwnsEvent(eventID, user) {
		writeError(w, http.StatusNotFound, "event_not_found")
		return
	}
	if s.store != nil {
		counts, _, err := s.store.GetOrganizerInventory(r.Context(), eventID)
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

func (s *Server) organizerOwnsEvent(eventID string, user User) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[eventID]
	return ok && event.OrganizerID == user.OrganizerID
}

func (s *Server) handleOrganizerMetricsStream(w http.ResponseWriter, r *http.Request, user User) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported")
		return
	}

	// For simplicity, we listen to all events for this organizer or a specific event if eventId is query param?
	// The frontend connects to `/organizer/metrics/stream` (no event ID). We'll assume the first event for the organizer.
	
	s.mu.Lock()
	var eventID string
	for _, event := range s.events {
		if event.OrganizerID == user.OrganizerID {
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
			metrics, err := s.store.GetOrganizerMetrics(r.Context(), eventID)
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
	if s.organizerClients[eventID] == nil {
		s.organizerClients[eventID] = make(map[chan string]struct{})
	}
	s.organizerClients[eventID][ch] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.organizerClients[eventID], ch)
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

func (s *Server) handleCreateVenue(w http.ResponseWriter, r *http.Request, user User) {
	var req Venue
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.ID == "" {
		req.ID = "ven_" + time.Now().Format("20060102150405")
	}

	if s.store != nil {
		venue, err := s.store.CreateVenue(r.Context(), user.ID, req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		writeJSON(w, http.StatusCreated, venue)
		return
	}

	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handleListVenues(w http.ResponseWriter, r *http.Request, user User) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"venues": []Venue{}})
		return
	}
	venues, err := s.store.GetOrganizerVenues(r.Context(), user.ID)
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
		venues, err := s.store.GetOrganizerVenues(r.Context(), user.ID)
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
