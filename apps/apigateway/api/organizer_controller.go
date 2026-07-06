package api

import (
	"context"
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
	if !s.organizerOwnsEvent(r.Context(), r.PathValue("eventId"), user) {
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
	if !s.organizerOwnsEvent(r.Context(), eventID, user) {
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

// organizerOwnsEvent reports whether user organizes eventID. In store-backed
// mode catalog.events has no organizer column directly - ownership is
// transitive through the event's venue - so it defers to organizerOwnsVenue
// the same way handleCreateEvent already establishes venue ownership. The
// in-memory s.events map (populated only in demo/no-store mode) is checked
// otherwise.
func (s *Server) organizerOwnsEvent(ctx context.Context, eventID string, user User) bool {
	if s.store != nil {
		venueID, err := s.store.GetEventVenueID(ctx, eventID)
		if err != nil {
			return false
		}
		return s.organizerOwnsVenue(ctx, user.ID, venueID)
	}
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
	if !s.organizerOwnsVenue(r.Context(), user.ID, venueID) {
		writeError(w, http.StatusNotFound, "venue_not_found")
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

// organizerOwnsVenue reports whether venueID belongs to organizerID, so
// venue-scoped endpoints never leak or mutate another organizer's venue.
func (s *Server) organizerOwnsVenue(ctx context.Context, organizerID, venueID string) bool {
	venues, err := s.store.GetOrganizerVenues(ctx, organizerID)
	if err != nil {
		return false
	}
	for _, v := range venues {
		if v.ID == venueID {
			return true
		}
	}
	return false
}

// handleCancelEvent cancels an entire event: it marks the event cancelled in
// the catalog, tells orderservice to bulk-cancel every order tied to it, and
// relies on handleCreateReservation's PUBLISHED-status gate to reject any new
// reservation racing in after the catalog update — so cancellation can't be
// bypassed by a booking that lands between these two steps. The catalog
// update and the orderservice call are each independently idempotent, so a
// client retry after a partial failure (e.g. the orderservice call failing)
// is always safe to repeat in full.
func (s *Server) handleCancelEvent(w http.ResponseWriter, r *http.Request, user User) {
	eventID := r.PathValue("eventId")
	if !s.organizerOwnsEvent(r.Context(), eventID, user) {
		writeError(w, http.StatusNotFound, "event_not_found")
		return
	}

	// Checked before the catalog write below: if orderservice is unconfigured,
	// there is no point committing catalog.events.status = 'CANCELLED' only to
	// fail afterward, since that would permanently mark the event cancelled
	// while every outstanding order stays untouched, with a client retry
	// hitting the same 503 forever (the catalog write is a no-op on retry).
	if s.orderSvcBaseURL == "" {
		writeError(w, http.StatusServiceUnavailable, "order_service_unavailable")
		return
	}

	if s.store != nil {
		if err := s.store.CancelEvent(r.Context(), eventID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
	} else {
		s.mu.Lock()
		if event, ok := s.events[eventID]; ok {
			event.Status = "CANCELLED"
			s.events[eventID] = event
		}
		s.mu.Unlock()
	}

	resp, err := s.doOrderServiceRequest(r.Context(), "/events/"+eventID+"/cancel", nil)
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
		EventID         string `json:"event_id"`
		CancelledOrders int    `json:"cancelled_orders"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"event_id":         eventID,
		"status":           "CANCELLED",
		"cancelled_orders": upstream.CancelledOrders,
	})
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
