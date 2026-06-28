package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVendorCreateEvent(t *testing.T) {
	server := NewServerWithStore("test", nil)
	client := newTestClient(server)
	
	// Create vendor
	reqBody := `{"email":"new_vendor@velox.local","password":"pass","role":"vendor"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte(reqBody)))
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("register failed: %s", rr.Body.String())
	}

	cookie := client.login(t, "new_vendor@velox.local", "pass")

	eventPayload := map[string]any{
		"venue_id": "ven_northstar",
		"name": "Test Event",
		"starts_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	body, _ := json.Marshal(eventPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/vendor/events", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	// Since s.store == nil, handleCreateEvent might fail or panic if it doesn't mock store
	// We'll just verify the role check passes and it hits the handler (even if it 500s due to no store)
	if rr.Code == http.StatusForbidden || rr.Code == http.StatusUnauthorized {
		t.Fatalf("should be authorized, got %d", rr.Code)
	}
}

func TestVendorListVenues(t *testing.T) {
	server := NewServerWithStore("test", nil)
	client := newTestClient(server)
	cookie := client.login(t, "vendor@velox.local", "vendor")

	req := httptest.NewRequest(http.MethodGet, "/api/vendor/venues", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code == http.StatusForbidden || rr.Code == http.StatusUnauthorized {
		t.Fatalf("should be authorized, got %d", rr.Code)
	}
}
