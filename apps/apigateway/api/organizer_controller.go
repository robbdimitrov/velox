package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	eventNameMaxLength        = 120
	eventDescriptionMaxLength = 5000
	defaultEventCategory      = "Concerts"
	defaultEventTimezone      = "UTC"
	venueNameMaxLength        = 120
	venueAddressMaxLength     = 240
	venueCityMaxLength        = 80
	maxVenueCapacity          = 250000
	maxVenueSections          = 8
	maxVenueRowsPerSection    = 26
	maxVenueSeatsPerRow       = 50
)

var allowedEventCategories = map[string]struct{}{
	"Concerts":  {},
	"Sports":    {},
	"Theatre":   {},
	"Festivals": {},
}

type createEventRequest struct {
	ID          string    `json:"id"`
	VenueID     string    `json:"venue_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	StartsAt    time.Time `json:"starts_at"`
	Timezone    string    `json:"timezone"`
}

type createVenueRequest struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	City     string                 `json:"city"`
	Address  string                 `json:"address"`
	Capacity int                    `json:"capacity"`
	Sections []VenueSectionTemplate `json:"sections"`
}

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
			if len(orders) >= listResultLimit {
				break
			}
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
	counts := map[string]int{StatusAvailable: 0, StatusHeld: 0, StatusReserved: 0}
	for _, section := range s.seats[eventID] {
		for _, seat := range section {
			s.expireSeatIfNeededLocked(seat)
			counts[seat.Status]++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"inventory": counts, "active_holds": counts[StatusHeld]})
}

// organizerOwnsEvent reports whether user organizes eventID. Store-backed
// ownership is venue-derived; demo mode checks s.events directly.
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

	eventID := r.PathValue("eventId")
	if eventID != "" {
		if !s.organizerOwnsEvent(r.Context(), eventID, user) {
			writeError(w, http.StatusNotFound, "event_not_found")
			return
		}
	} else {
		s.mu.Lock()
		for _, event := range s.events {
			if event.OrganizerID == user.OrganizerID {
				eventID = event.ID
				break
			}
		}
		s.mu.Unlock()
	}

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
	var req createVenueRequest
	if _, ok := decodeJSONStrict(w, r, &req); !ok {
		return
	}

	venue, sections, ok := s.normalizeCreateVenueRequest(w, req)
	if !ok {
		return
	}

	if s.store != nil {
		venue, err := s.store.CreateVenue(r.Context(), user.ID, venue, sections)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		writeJSON(w, http.StatusCreated, venue)
		return
	}

	writeJSON(w, http.StatusCreated, venue)
}

func (s *Server) normalizeCreateVenueRequest(w http.ResponseWriter, req createVenueRequest) (Venue, []VenueSectionTemplate, bool) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > venueNameMaxLength {
		writeError(w, http.StatusBadRequest, "invalid_venue_name")
		return Venue{}, nil, false
	}
	city := strings.TrimSpace(req.City)
	if city == "" || len(city) > venueCityMaxLength {
		writeError(w, http.StatusBadRequest, "invalid_venue_city")
		return Venue{}, nil, false
	}
	address := strings.TrimSpace(req.Address)
	if address == "" || len(address) > venueAddressMaxLength {
		writeError(w, http.StatusBadRequest, "invalid_venue_address")
		return Venue{}, nil, false
	}
	if req.Capacity <= 0 || req.Capacity > maxVenueCapacity {
		writeError(w, http.StatusBadRequest, "invalid_venue_capacity")
		return Venue{}, nil, false
	}
	venueID := strings.TrimSpace(req.ID)
	if venueID == "" {
		venueID = "ven_" + s.now().Format("20060102150405")
	}

	sections := make([]VenueSectionTemplate, 0, len(req.Sections))
	if len(req.Sections) > maxVenueSections {
		writeError(w, http.StatusBadRequest, "invalid_venue_sections")
		return Venue{}, nil, false
	}
	seen := map[string]struct{}{}
	for _, section := range req.Sections {
		normalized, ok := normalizeVenueSectionTemplate(w, section, seen)
		if !ok {
			return Venue{}, nil, false
		}
		sections = append(sections, normalized)
	}

	return Venue{
		ID:       venueID,
		Name:     name,
		City:     city,
		Address:  address,
		Capacity: req.Capacity,
	}, sections, true
}

func normalizeVenueSectionTemplate(w http.ResponseWriter, section VenueSectionTemplate, seen map[string]struct{}) (VenueSectionTemplate, bool) {
	sectionID := strings.ToUpper(strings.TrimSpace(section.SectionID))
	if sectionID == "" || len(sectionID) > 12 {
		writeError(w, http.StatusBadRequest, "invalid_venue_section")
		return VenueSectionTemplate{}, false
	}
	if _, exists := seen[sectionID]; exists {
		writeError(w, http.StatusBadRequest, "duplicate_venue_section")
		return VenueSectionTemplate{}, false
	}
	seen[sectionID] = struct{}{}
	name := strings.TrimSpace(section.Name)
	if name == "" {
		name = sectionID + " Section"
	}
	if len(name) > venueNameMaxLength {
		writeError(w, http.StatusBadRequest, "invalid_venue_section")
		return VenueSectionTemplate{}, false
	}
	if section.RowCount <= 0 || section.RowCount > maxVenueRowsPerSection ||
		section.SeatsPerRow <= 0 || section.SeatsPerRow > maxVenueSeatsPerRow {
		writeError(w, http.StatusBadRequest, "invalid_venue_section")
		return VenueSectionTemplate{}, false
	}
	section.SectionID = sectionID
	section.Name = name
	return section, true
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
	venueID := r.PathValue("venueId")
	if venueID == "" {
		venueID = r.PathValue("id")
	}
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

// handleCancelEvent marks the catalog event cancelled and asks orderservice to
// bulk-cancel matching orders. Both steps are idempotent for safe retries.
func (s *Server) handleCancelEvent(w http.ResponseWriter, r *http.Request, user User) {
	eventID := r.PathValue("eventId")
	if !s.organizerOwnsEvent(r.Context(), eventID, user) {
		writeError(w, http.StatusNotFound, "event_not_found")
		return
	}

	// Fail before the catalog write; otherwise an unconfigured orderservice
	// would leave the event cancelled while outstanding orders stay active.
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
	var req createEventRequest
	if _, ok := decodeJSONStrict(w, r, &req); !ok {
		return
	}

	event, ok := s.normalizeCreateEventRequest(w, req, user)
	if !ok {
		return
	}

	if s.store != nil {
		venues, err := s.store.GetOrganizerVenues(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		ownsVenue := false
		for _, v := range venues {
			if v.ID == event.VenueID {
				ownsVenue = true
				break
			}
		}
		if !ownsVenue {
			writeError(w, http.StatusNotFound, "venue_not_found")
			return
		}

		if err := s.store.CreateEvent(r.Context(), event); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
	} else {
		s.mu.Lock()
		s.events[event.ID] = event
		s.mu.Unlock()
	}

	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) normalizeCreateEventRequest(w http.ResponseWriter, req createEventRequest, user User) (Event, bool) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > eventNameMaxLength {
		writeError(w, http.StatusBadRequest, "invalid_event_name")
		return Event{}, false
	}
	description := strings.TrimSpace(req.Description)
	if len(description) > eventDescriptionMaxLength {
		writeError(w, http.StatusBadRequest, "invalid_event_description")
		return Event{}, false
	}
	venueID := strings.TrimSpace(req.VenueID)
	if venueID == "" {
		writeError(w, http.StatusBadRequest, "invalid_venue")
		return Event{}, false
	}
	if req.StartsAt.IsZero() {
		writeError(w, http.StatusBadRequest, "invalid_starts_at")
		return Event{}, false
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = defaultEventCategory
	}
	if _, ok := allowedEventCategories[category]; !ok {
		writeError(w, http.StatusBadRequest, "invalid_event_category")
		return Event{}, false
	}
	timezone := strings.TrimSpace(req.Timezone)
	if timezone == "" {
		timezone = defaultEventTimezone
	}
	eventID := strings.TrimSpace(req.ID)
	if eventID == "" {
		eventID = "evt_" + time.Now().Format("20060102150405")
	}

	return Event{
		ID:          eventID,
		VenueID:     venueID,
		Status:      EventStatusPublished,
		OrganizerID: user.OrganizerID,
		Name:        name,
		Category:    category,
		Description: description,
		StartsAt:    req.StartsAt,
		Timezone:    timezone,
	}, true
}
