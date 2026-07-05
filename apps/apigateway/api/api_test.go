package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReserverCanReserveAndConfirmSeat(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order-1",
			"status":   OrderPending,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")

	order := client.reserve(t, cookie, "idem-1", []string{"A-01"}, http.StatusOK)
	if order.Status != OrderPending {
		t.Fatalf("status = %s, want %s", order.Status, OrderPending)
	}
	if order.TotalCents <= 0 {
		t.Fatalf("total_cents = %d, want positive seat total", order.TotalCents)
	}

	req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("confirm status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCancelReservationCancelsOrder(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := OrderPending
		if strings.HasSuffix(r.URL.Path, "/cancel") {
			status = OrderCancelled
		}
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order-cancel",
			"status":   status,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")

	order := client.reserve(t, cookie, "idem-cancel-1", []string{"A-05"}, http.StatusOK)

	req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/cancel", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		OrderID string `json:"order_id"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	if out.Status != OrderCancelled {
		t.Fatalf("status = %s, want %s", out.Status, OrderCancelled)
	}
}

func TestConfirmReservationRejectsNonOwner(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order-owner",
			"status":   OrderPending,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	reserverCookie := client.login(t, "reserver@velox.local", "reserver")
	order := client.reserve(t, reserverCookie, "idem-owner-1", []string{"A-06"}, http.StatusOK)

	organizerCookie := client.login(t, "organizer@velox.local", "organizer")
	req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
	req.AddCookie(organizerCookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("confirm status = %d, want %d (non-owner) body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestReserveReusesIdempotencyResult(t *testing.T) {
	var calls int
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order-1",
			"status":   OrderPending,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")

	first := client.reserve(t, cookie, "idem-repeat", []string{"A-01"}, http.StatusOK)
	second := client.reserve(t, cookie, "idem-repeat", []string{"A-01"}, http.StatusOK)

	if calls != 1 {
		t.Fatalf("order service calls = %d, want 1", calls)
	}
	if second.ID != first.ID || second.ReservationID != first.ReservationID {
		t.Fatalf("idempotent response mismatch: first=%+v second=%+v", first, second)
	}
}

func TestReserveReleasesTentativeHoldOnUpstreamFailure(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(apiError{Error: "order_service_unavailable"})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	client.reserve(t, cookie, "idem-fails", []string{"A-02"}, http.StatusServiceUnavailable)

	server.mu.Lock()
	defer server.mu.Unlock()
	seat := server.seats["evt_neon_riot"]["A"]["A-02"]
	if seat.Status != StatusAvailable || seat.HeldByOrderID != "" {
		t.Fatalf("seat was not released after upstream failure: %+v", seat)
	}
}

func TestReserveRejectsMissingOrderServiceBeforeHoldingSeat(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")

	client.reserve(t, cookie, "idem-no-upstream", []string{"A-03"}, http.StatusServiceUnavailable)

	server.mu.Lock()
	defer server.mu.Unlock()
	seat := server.seats["evt_neon_riot"]["A"]["A-03"]
	if seat.Status != StatusAvailable || seat.HeldByOrderID != "" {
		t.Fatalf("seat should not be held without an order service: %+v", seat)
	}
}

func TestReserveReleasesTentativeHoldOnMalformedUpstreamResponse(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": OrderPending})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	client.reserve(t, cookie, "idem-malformed", []string{"A-04"}, http.StatusBadGateway)

	server.mu.Lock()
	defer server.mu.Unlock()
	seat := server.seats["evt_neon_riot"]["A"]["A-04"]
	if seat.Status != StatusAvailable || seat.HeldByOrderID != "" {
		t.Fatalf("seat was not released after malformed upstream response: %+v", seat)
	}
}

func TestLoginAttemptsAreRateLimited(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	for i := 0; i < 5; i++ {
		client.loginStatus(t, "reserver@velox.local", "wrong", http.StatusUnauthorized)
	}
	client.loginStatus(t, "reserver@velox.local", "reserver", http.StatusTooManyRequests)
}

type testClient struct {
	server *Server
}

func newTestClient(server *Server) testClient {
	return testClient{server: server}
}

func (c testClient) login(t *testing.T, email, password string) *http.Cookie {
	t.Helper()
	rr := c.loginStatus(t, email, password, http.StatusOK)
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == CookieName {
			return cookie
		}
	}
	t.Fatal("missing session cookie")
	return nil
}

func (c testClient) loginStatus(t *testing.T, email, password string, want int) *httptest.ResponseRecorder {
	t.Helper()
	body := []byte(`{"email":"` + email + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	c.server.Routes().ServeHTTP(rr, req)
	if rr.Code != want {
		t.Fatalf("login status = %d want %d body=%s", rr.Code, want, rr.Body.String())
	}
	return rr
}

func (c testClient) reserve(t *testing.T, cookie *http.Cookie, key string, seatIDs []string, want int) Order {
	t.Helper()
	payload := map[string]any{"event_id": "evt_neon_riot", "section_id": "A", "seat_ids": seatIDs}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", key)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	c.server.Routes().ServeHTTP(rr, req)
	if rr.Code != want {
		t.Fatalf("reserve status = %d want %d body=%s", rr.Code, want, rr.Body.String())
	}
	var out struct {
		Order Order `json:"order"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return out.Order
}
