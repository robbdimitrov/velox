package api

import (
	"fmt"
	"net/http"
	"sort"
	"time"
)

// discoveryCacheControl is only for query-only, unauthenticated reads; command
// and per-user endpoints must not share this CDN policy.
const discoveryCacheControl = "public, max-age=1, stale-while-revalidate=5"

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", discoveryCacheControl)
	if s.store != nil {
		events, err := s.store.GetEvents(r.Context())
		if err == nil {
			lagMS, lagErr := s.store.GetGlobalProjectionLagMS(r.Context())
			if lagErr != nil {
				lagMS = 0
			}
			writeJSON(w, http.StatusOK, map[string]any{"events": events, "projection_lag_ms": lagMS})
			return
		}
	}
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
	w.Header().Set("Cache-Control", discoveryCacheControl)
	eventID := r.PathValue("eventId")
	if s.store != nil {
		event, err := s.store.GetEvent(r.Context(), eventID)
		if err == nil {
			lagMS, lagErr := s.store.GetProjectionLagMS(r.Context(), eventID)
			if lagErr != nil {
				lagMS = 0
			}
			writeJSON(w, http.StatusOK, map[string]any{"event": event, "projection_lag_ms": lagMS})
			return
		}
	}
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

// dynamicSeatCacheControl uses the shortest HTTP max-age while Redis
// single-flight absorbs stampedes behind it.
const dynamicSeatCacheControl = "public, max-age=1, stale-while-revalidate=1"

func (s *Server) handleSeats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", dynamicSeatCacheControl)
	eventID, sectionID := r.PathValue("eventId"), r.PathValue("sectionId")
	if s.store != nil {
		seats, snapshotAgeMS, err := s.getSeatsCached(r.Context(), eventID, sectionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "seat_snapshot_unavailable")
			return
		}

		if seats == nil {
			seats = []Seat{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"seats": seats, "snapshot_age_ms": snapshotAgeMS})
		return
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
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported")
		return
	}

	eventID := r.PathValue("eventId")
	fmt.Fprintf(w, "event: heartbeat\ndata: {\"event_id\":%q}\n\n", eventID)
	flusher.Flush()

	ch := make(chan string, 10)
	s.mu.Lock()
	if s.seatClients[eventID] == nil {
		s.seatClients[eventID] = make(map[chan string]struct{})
	}
	s.seatClients[eventID][ch] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.seatClients[eventID], ch)
		s.mu.Unlock()
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case payload := <-ch:
			fmt.Fprintf(w, "event: update\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"event_id\":%q}\n\n", eventID)
			flusher.Flush()
		}
	}
}
