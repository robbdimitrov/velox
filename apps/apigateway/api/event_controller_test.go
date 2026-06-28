package api

import (
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
