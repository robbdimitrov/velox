package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	maxAnnouncementTitleLen = 200
	maxAnnouncementBodyLen  = 5000
)

var validAnnouncementSeverities = map[string]bool{
	"INFO":            true,
	"SCHEDULE_CHANGE": true,
	"CANCELLATION":    true,
}

type createAnnouncementRequest struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Severity string `json:"severity,omitempty"`
}

// handleCreateAnnouncement lets an event's organizer post a public update
// (e.g. a schedule change or cancellation notice) that anyone viewing the
// event page can read via handleEventAnnouncements. There is no live push;
// clients simply re-fetch the list.
func (s *Server) handleCreateAnnouncement(w http.ResponseWriter, r *http.Request, user User) {
	eventID := r.PathValue("eventId")
	if !s.organizerOwnsEvent(r.Context(), eventID, user) {
		writeError(w, http.StatusNotFound, "event_not_found")
		return
	}

	var req createAnnouncementRequest
	if _, ok := decodeJSONStrict(w, r, &req); !ok {
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	if req.Title == "" || len(req.Title) > maxAnnouncementTitleLen {
		writeError(w, http.StatusBadRequest, "invalid_title")
		return
	}
	if req.Body == "" || len(req.Body) > maxAnnouncementBodyLen {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.Severity == "" {
		req.Severity = "INFO"
	}
	if !validAnnouncementSeverities[req.Severity] {
		writeError(w, http.StatusBadRequest, "invalid_severity")
		return
	}

	if s.store != nil {
		announcement, err := s.store.CreateAnnouncement(r.Context(), eventID, user.OrganizerID, req.Title, req.Body, req.Severity)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		writeJSON(w, http.StatusCreated, announcement)
		return
	}

	announcement := EventAnnouncement{
		ID:        uuid.NewString(),
		EventID:   eventID,
		Title:     req.Title,
		Body:      req.Body,
		Severity:  req.Severity,
		CreatedAt: s.now(),
	}
	s.mu.Lock()
	s.announcements[eventID] = append([]EventAnnouncement{announcement}, s.announcements[eventID]...)
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, announcement)
}

// handleEventAnnouncements is a public, cacheable read: anyone viewing the
// event page can see organizer-authored updates, matching discovery-page
// endpoints like handleEvent that carry no per-user state.
func (s *Server) handleEventAnnouncements(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", discoveryCacheControl)
	eventID := r.PathValue("eventId")

	if s.store != nil {
		announcements, err := s.store.GetEventAnnouncements(r.Context(), eventID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		if announcements == nil {
			announcements = []EventAnnouncement{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"announcements": announcements})
		return
	}

	s.mu.Lock()
	announcements := append([]EventAnnouncement(nil), s.announcements[eventID]...)
	s.mu.Unlock()
	if announcements == nil {
		announcements = []EventAnnouncement{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"announcements": announcements})
}
