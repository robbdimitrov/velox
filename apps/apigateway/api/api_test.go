package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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

	req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
	req.Header.Set("Idempotency-Key", "confirm-1")
	req.Header.Set("Reservation-Token", order.ReservationToken)
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
	req.Header.Set("Idempotency-Key", "cancel-1")
	req.Header.Set("Reservation-Token", order.ReservationToken)
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
	req.Header.Set("Idempotency-Key", "confirm-non-owner")
	req.Header.Set("Reservation-Token", order.ReservationToken)
	req.AddCookie(organizerCookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("confirm status = %d, want %d (non-owner token) body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestGetOrderRejectsNonOwner(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order-get",
			"status":   OrderPending,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	reserverCookie := client.login(t, "reserver@velox.local", "reserver")
	order := client.reserve(t, reserverCookie, "idem-get-owner-1", []string{"A-08"}, http.StatusOK)

	organizerCookie := client.login(t, "organizer@velox.local", "organizer")
	req := httptest.NewRequest(http.MethodGet, "/orders/"+order.ID, nil)
	req.AddCookie(organizerCookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get order status = %d, want %d (non-owner) body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestReservationCreateReturnsSignedToken(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order-token",
			"status":   OrderPending,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	server.now = func() time.Time { return now }
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	order := client.reserve(t, cookie, "idem-token", []string{"A-07"}, http.StatusOK)

	if order.ReservationToken == "" {
		t.Fatal("reservation_token is empty")
	}
	if order.ReservationToken == order.ReservationID {
		t.Fatal("reservation_token must not equal reservation_id")
	}
	if order.ServerTimeMS == 0 || order.ExpiresAtServerMS <= order.ServerTimeMS {
		t.Fatalf("server/expiry timestamps are invalid: server=%d expires=%d", order.ServerTimeMS, order.ExpiresAtServerMS)
	}
	if got, want := time.Duration(order.ExpiresAtServerMS-order.ServerTimeMS)*time.Millisecond, reservationHoldDuration; got != want {
		t.Fatalf("expiry duration = %s, want %s", got, want)
	}
	if len(order.Seats) != 1 || order.Seats[0].ID != "A-07" {
		t.Fatalf("selected seat missing: %+v", order.Seats)
	}
}

func TestConfirmReservationRequiresTokenAndIdempotencyKey(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order-requires-token",
			"status":   OrderPending,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	order := client.reserve(t, cookie, "idem-requires-token", []string{"A-08"}, http.StatusOK)

	req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
	req.Header.Set("Idempotency-Key", "confirm-missing-token")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing token status = %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
	req.Header.Set("Reservation-Token", order.ReservationToken)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing idempotency status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestConfirmReservationRejectsExpiredToken(t *testing.T) {
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order-expired-token",
			"status":   OrderPending,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	server.now = func() time.Time { return now }
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	order := client.reserve(t, cookie, "idem-expired-token", []string{"A-09"}, http.StatusOK)

	now = now.Add(server.holdTTL + time.Millisecond)
	req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
	req.Header.Set("Idempotency-Key", "confirm-expired-token")
	req.Header.Set("Reservation-Token", order.ReservationToken)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expired token status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestConfirmReservationIdempotencyReplaysSameAction(t *testing.T) {
	var confirmCalls int
	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/confirm") {
			confirmCalls++
			json.NewEncoder(w).Encode(map[string]any{
				"order_id": "mock-order-idem-confirm",
				"status":   OrderConfirmed,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "mock-order-idem-confirm",
			"status":   OrderPending,
		})
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	order := client.reserve(t, cookie, "idem-confirm-create", []string{"A-10"}, http.StatusOK)

	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
		req.Header.Set("Idempotency-Key", "confirm-replay")
		req.Header.Set("Reservation-Token", order.ReservationToken)
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		server.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("confirm status = %d body=%s", rr.Code, rr.Body.String())
		}
	}
	if confirmCalls != 1 {
		t.Fatalf("confirm calls = %d, want 1", confirmCalls)
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

// TestCreateReservationRejectsCancelledEvent verifies the store-backed
// cancellation gate rejects before any seat validation or hold.
// sqlmock drives GetEventStatus because DatabaseStore always uses *sql.DB.
func TestCreateReservationRejectsCancelledEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// Once a store is attached, authenticate() must resolve the seeded user
	// through the mock.
	mock.ExpectQuery("SELECT id, email, password_hash, role, created_at").
		WithArgs("usr_reserver_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "role", "created_at"}).
			AddRow("usr_reserver_1", "reserver@velox.local", "unused-hash", RoleReserver, time.Now()))
	mock.ExpectQuery(`SELECT status FROM catalog\.events WHERE id = \$1`).
		WithArgs("evt_neon_riot").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("CANCELLED"))

	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("orderservice must not be called for a cancelled event")
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	// Log in before attaching the store so only the gate under test hits sqlmock.
	cookie := client.login(t, "reserver@velox.local", "reserver")
	server.store = &DatabaseStore{db: db}
	client.reserve(t, cookie, "idem-cancelled-event", []string{"A-01"}, http.StatusConflict)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

// TestCreateReservationReturnsNotFoundForNonexistentEvent verifies missing
// events stay 404s, not the 409 used for cancelled/unpublished events.
func TestCreateReservationReturnsNotFoundForNonexistentEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, email, password_hash, role, created_at").
		WithArgs("usr_reserver_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "role", "created_at"}).
			AddRow("usr_reserver_1", "reserver@velox.local", "unused-hash", RoleReserver, time.Now()))
	mock.ExpectQuery(`SELECT status FROM catalog\.events WHERE id = \$1`).
		WithArgs("evt_does_not_exist").
		WillReturnRows(sqlmock.NewRows([]string{"status"}))

	mockOrderSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("orderservice must not be called for a nonexistent event")
	}))
	defer mockOrderSvc.Close()

	server := NewServerWithStore("test", nil, nil)
	server.SetOrderServiceURL(mockOrderSvc.URL)
	server.SetHTTPClient(mockOrderSvc.Client())

	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	server.store = &DatabaseStore{db: db}

	payload := map[string]any{"event_id": "evt_does_not_exist", "section_id": "A", "seat_ids": []string{"A-01"}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", "idem-missing-event")
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

func TestLoginAttemptsAreRateLimited(t *testing.T) {
	server := NewServerWithStore("test", nil, nil)
	client := newTestClient(server)
	for range 5 {
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
