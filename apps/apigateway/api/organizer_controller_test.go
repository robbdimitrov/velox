package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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
		"id":        "evt_created_canonical",
		"venue_id":  "ven_northstar",
		"name":      "Test Event",
		"starts_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	body, _ := json.Marshal(eventPayload)
	req = httptest.NewRequest(http.MethodPost, "/organizer/events", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/organizer/events", nil)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var out struct {
		Events []Event `json:"events"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(out.Events) != 1 || out.Events[0].ID != "evt_created_canonical" {
		t.Fatalf("created event not visible to organizer: %+v", out.Events)
	}
}

func TestOrganizerCreateEventStoreBackedOwnedVenueSucceeds(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")
	server.store = &DatabaseStore{db: db}

	startsAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	saleStartsAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	mock.ExpectQuery("SELECT id, email, password_hash, role, created_at").
		WithArgs("usr_organizer_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "role", "created_at"}).
			AddRow("usr_organizer_1", "organizer@velox.local", "unused-hash", RoleOrganizer, time.Now()))
	mock.ExpectQuery(`SELECT v\.id, v\.name, v\.city, v\.address, v\.capacity\s+FROM catalog\.venues v\s+JOIN catalog\.user_venues uv ON v\.id = uv\.venue_id\s+WHERE uv\.user_id = \$1`).
		WithArgs("usr_organizer_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "city", "address", "capacity"}).
			AddRow("ven_velox_arena", "Velox Arena", "Chicago", "100 Arena Way", 10000))
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO catalog\.events .*VALUES`).
		WithArgs("evt_store_created", "ven_velox_arena", "Store Created", "Details", "Concerts", startsAt, saleStartsAt, "event-final-whistle", "America/Chicago", EventStatusPublished).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO catalog\.event_sections .*FROM catalog\.venue_sections`).
		WithArgs("evt_store_created", "ven_velox_arena").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)SELECT vs\.section_id, vs\.seat_id, vs\.row_label, vs\.seat_number, vs\.x, vs\.y,\s+vs\.accessibility, COALESCE\(es\.price_amount_minor, 5000\).*FROM catalog\.venue_seats vs`).
		WithArgs("ven_velox_arena", "evt_store_created").
		WillReturnRows(sqlmock.NewRows([]string{"section_id", "seat_id", "row_label", "seat_number", "x", "y", "accessibility", "price_amount_minor"}).
			AddRow("A", "A-01", "A", 1, 44, 42, true, 8650))
	mock.ExpectExec(`INSERT INTO inventory\.event_streams`).
		WithArgs("seat:evt_store_created:A:A-01", "evt_store_created", "A", "A-01").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO projection\.seat_snapshots .*VALUES`).
		WithArgs("evt_store_created", "A", "A-01", 8650, "A", 1, 44, 42, true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	body, _ := json.Marshal(map[string]any{
		"id":             "evt_store_created",
		"venue_id":       "ven_velox_arena",
		"name":           "Store Created",
		"description":    "Details",
		"category":       "Concerts",
		"starts_at":      startsAt.Format(time.RFC3339),
		"sale_starts_at": saleStartsAt.Format(time.RFC3339),
		"image_key":      "event-final-whistle",
		"timezone":       "America/Chicago",
	})
	req := httptest.NewRequest(http.MethodPost, "/organizer/events", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	var got Event
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "evt_store_created" || got.Category != "Concerts" || got.ImageKey != "event-final-whistle" {
		t.Fatalf("unexpected event response: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestOrganizerCreateEventRejectsInvalidDateOrdering(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	startsAt := time.Now().Add(2 * time.Hour).UTC()
	body, _ := json.Marshal(map[string]any{
		"venue_id":       "ven_northstar",
		"name":           "Bad Dates",
		"starts_at":      startsAt.Format(time.RFC3339),
		"sale_starts_at": startsAt.Add(time.Hour).Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/organizer/events", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrganizerCreateEventRejectsUnknownFields(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	body, _ := json.Marshal(map[string]any{
		"venue_id":   "ven_northstar",
		"name":       "Unknown Field Event",
		"starts_at":  time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"unexpected": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/organizer/events", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrganizerCreateVenueRejectsUnknownFields(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	body, _ := json.Marshal(map[string]any{
		"name":       "Unknown Field Venue",
		"city":       "Chicago",
		"address":    "1 Unknown Way",
		"capacity":   100,
		"unexpected": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/organizer/venues", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrganizerCreateVenueAcceptsSectionTemplates(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	body, _ := json.Marshal(map[string]any{
		"id":       "ven_custom",
		"name":     "Custom Hall",
		"city":     "Chicago",
		"address":  "1 Custom Way",
		"capacity": 120,
		"sections": []map[string]any{{
			"section_id":            "vip",
			"name":                  "VIP Floor",
			"row_count":             3,
			"seats_per_row":         12,
			"price_cents":           12500,
			"accessible_edge_seats": true,
		}},
	})
	req := httptest.NewRequest(http.MethodPost, "/organizer/venues", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	var got Venue
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "ven_custom" || got.Name != "Custom Hall" {
		t.Fatalf("unexpected venue response: %+v", got)
	}
}

func TestOrganizerCreateVenueRejectsInvalidSectionTemplate(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	body, _ := json.Marshal(map[string]any{
		"name":     "Bad Template",
		"city":     "Chicago",
		"address":  "1 Bad Way",
		"capacity": 120,
		"sections": []map[string]any{{
			"section_id":    "A",
			"row_count":     27,
			"seats_per_row": 12,
		}},
	})
	req := httptest.NewRequest(http.MethodPost, "/organizer/venues", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrganizerCreateEventReturnsNotFoundForUnownedVenue(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")
	server.store = &DatabaseStore{db: db}

	mock.ExpectQuery("SELECT id, email, password_hash, role, created_at").
		WithArgs("usr_organizer_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "role", "created_at"}).
			AddRow("usr_organizer_1", "organizer@velox.local", "unused-hash", RoleOrganizer, time.Now()))
	mock.ExpectQuery(`SELECT v\.id, v\.name, v\.city, v\.address, v\.capacity\s+FROM catalog\.venues v\s+JOIN catalog\.user_venues uv ON v\.id = uv\.venue_id\s+WHERE uv\.user_id = \$1`).
		WithArgs("usr_organizer_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "city", "address", "capacity"}).
			AddRow("ven_other", "Other Hall", "Austin", "1 Other Way", 1000))

	body, _ := json.Marshal(map[string]any{
		"venue_id":  "ven_velox_arena",
		"name":      "Unowned Venue",
		"starts_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/organizer/events", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestOrganizerRoutesRejectReserver(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")

	req := httptest.NewRequest(http.MethodGet, "/organizer/venues", nil)
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

// TestCancelEventReturns503WithoutCommittingWhenOrderServiceUnset keeps event
// cancellation fail-closed before the catalog write when orderservice is absent.
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

	req := httptest.NewRequest(http.MethodGet, "/organizer/venues", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code == http.StatusForbidden || rr.Code == http.StatusUnauthorized {
		t.Fatalf("should be authorized, got %d", rr.Code)
	}
}

func TestOrganizerCreateVenueCanonicalRoute(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	body, _ := json.Marshal(map[string]any{
		"id":       "ven_created_canonical",
		"name":     "Created Hall",
		"city":     "Chicago",
		"address":  "1 Created Way",
		"capacity": 10,
	})
	req := httptest.NewRequest(http.MethodPost, "/organizer/venues", bytes.NewReader(body))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestLegacyAPIRoutesRemainForOrganizerCompatibility(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "organizer@velox.local", "organizer")

	req := httptest.NewRequest(http.MethodGet, "/api/organizer/venues", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("legacy venue route status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}
