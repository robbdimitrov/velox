package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEventsListing(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	events, ok := resp["events"].([]any)
	if !ok || len(events) == 0 {
		t.Fatalf("expected events array")
	}
}

func TestGetEventAndSeats(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)

	// Get an event that was seeded
	req := httptest.NewRequest(http.MethodGet, "/events/evt_neon_riot", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	// Get seats for the event
	req = httptest.NewRequest(http.MethodGet, "/events/evt_neon_riot/sections/A/seats", nil)
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	seats, ok := resp["seats"].([]any)
	if !ok || len(seats) == 0 {
		t.Fatalf("expected seats array")
	}
}

func TestEventAnnouncementsIsPublicAndCached(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/events/evt_neon_riot/announcements", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); got != discoveryCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, discoveryCacheControl)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	announcements, ok := resp["announcements"].([]any)
	if !ok {
		t.Fatalf("expected announcements array, got %v", resp["announcements"])
	}
	if len(announcements) != 0 {
		t.Fatalf("expected no announcements yet, got %d", len(announcements))
	}
}

func TestEventAnnouncementsOrderedNewestFirst(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	for _, title := range []string{"First update", "Second update", "Third update"} {
		body, _ := json.Marshal(map[string]any{"title": title, "body": "details"})
		req := httptest.NewRequest(http.MethodPost, "/organizer/events/evt_neon_riot/announcements", bytes.NewReader(body))
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		server.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create announcement %q: status = %d body=%s", title, rr.Code, rr.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/events/evt_neon_riot/announcements", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Announcements []EventAnnouncement `json:"announcements"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Announcements) != 3 {
		t.Fatalf("expected 3 announcements, got %d", len(resp.Announcements))
	}
	if resp.Announcements[0].Title != "Third update" || resp.Announcements[2].Title != "First update" {
		t.Fatalf("announcements not ordered newest first: %+v", resp.Announcements)
	}
}
