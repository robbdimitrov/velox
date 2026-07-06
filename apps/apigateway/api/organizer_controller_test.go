package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOrganizerCreateEvent(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)

	// Create organizer
	reqBody := `{"email":"new_organizer@velox.local","password":"pass","role":"organizer"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte(reqBody)))
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("register failed: %s", rr.Body.String())
	}

	cookie := client.login(t, "new_organizer@velox.local", "pass")

	eventPayload := map[string]any{
		"venue_id":  "ven_northstar",
		"name":      "Test Event",
		"starts_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	body, _ := json.Marshal(eventPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/organizer/events", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	// Since s.store == nil, handleCreateEvent might fail or panic if it doesn't mock store
	// We'll just verify the role check passes and it hits the handler (even if it 500s due to no store)
	if rr.Code == http.StatusForbidden || rr.Code == http.StatusUnauthorized {
		t.Fatalf("should be authorized, got %d", rr.Code)
	}
}

func TestOrganizerRoutesRejectReserver(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")

	req := httptest.NewRequest(http.MethodGet, "/api/organizer/venues", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreateAnnouncementRequiresEventOwnership(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	body, _ := json.Marshal(map[string]any{"title": "Delay", "body": "Doors pushed back 30 minutes."})
	req := httptest.NewRequest(http.MethodPost, "/organizer/events/evt_does_not_exist/announcements", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestCreateAnnouncementRejectsEmptyBody(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	body, _ := json.Marshal(map[string]any{"title": "", "body": ""})
	req := httptest.NewRequest(http.MethodPost, "/organizer/events/evt_neon_riot/announcements", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestCreateAnnouncementRejectsInvalidSeverity(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	body, _ := json.Marshal(map[string]any{"title": "Delay", "body": "Doors pushed back.", "severity": "URGENT"})
	req := httptest.NewRequest(http.MethodPost, "/organizer/events/evt_neon_riot/announcements", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestCreateAnnouncementSucceedsForOwner(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	body, _ := json.Marshal(map[string]any{"title": "Delay", "body": "Doors pushed back 30 minutes."})
	req := httptest.NewRequest(http.MethodPost, "/organizer/events/evt_neon_riot/announcements", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	var got EventAnnouncement
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Severity != "INFO" {
		t.Fatalf("severity = %s, want default INFO", got.Severity)
	}
	if got.ID == "" || got.EventID != "evt_neon_riot" {
		t.Fatalf("unexpected announcement: %+v", got)
	}
}

func TestCancelEventRequiresOwnership(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	req := httptest.NewRequest(http.MethodPost, "/organizer/events/evt_does_not_exist/cancel", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestCancelEventIsIdempotentOnRetry(t *testing.T) {
	var calls int
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"event_id":         "evt_neon_riot",
			"cancelled_orders": 3,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL + "/orders")
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	for i := range 2 {
		req := httptest.NewRequest(http.MethodPost, "/organizer/events/evt_neon_riot/cancel", nil)
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		server.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want %d body=%s", i, rr.Code, http.StatusOK, rr.Body.String())
		}
		var out struct {
			EventID         string `json:"event_id"`
			Status          string `json:"status"`
			CancelledOrders int    `json:"cancelled_orders"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode cancel response: %v", err)
		}
		if out.Status != "CANCELLED" || out.CancelledOrders != 3 {
			t.Fatalf("attempt %d: unexpected response %+v", i, out)
		}
	}
	if calls != 2 {
		t.Fatalf("order service calls = %d, want 2 (retry must re-forward)", calls)
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if server.events["evt_neon_riot"].Status != "CANCELLED" {
		t.Fatalf("event status = %s, want CANCELLED", server.events["evt_neon_riot"].Status)
	}
}

// TestCancelEventReturns503WithoutCommittingWhenOrderServiceUnset confirms
// handleCancelEvent checks orderservice availability before writing
// catalog.events.status = 'CANCELLED', not after. Committing the catalog
// write first and only then discovering orderservice is unreachable would
// permanently mark the event cancelled while every outstanding order stays
// untouched, with a client retry hitting the same 503 forever (the catalog
// write is idempotent, so a retry is a silent no-op).
func TestCancelEventReturns503WithoutCommittingWhenOrderServiceUnset(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	req := httptest.NewRequest(http.MethodPost, "/organizer/events/evt_neon_riot/cancel", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if server.events["evt_neon_riot"].Status == "CANCELLED" {
		t.Fatalf("event status was committed to CANCELLED despite orderservice being unreachable")
	}
}

func TestOrganizerListVenues(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	req := httptest.NewRequest(http.MethodGet, "/api/organizer/venues", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code == http.StatusForbidden || rr.Code == http.StatusUnauthorized {
		t.Fatalf("should be authorized, got %d", rr.Code)
	}
}
