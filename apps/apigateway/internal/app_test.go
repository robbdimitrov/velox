package internal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReserverCanReserveAndConfirmSeat(t *testing.T) {
	server := NewServer("test")
	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")

	order := client.reserve(t, cookie, "idem-1", []string{"A-01"}, http.StatusCreated)
	if order.Status != OrderPending {
		t.Fatalf("status = %s, want %s", order.Status, OrderPending)
	}
	req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("confirm status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRejectsDuplicateReservationForHeldSeat(t *testing.T) {
	server := NewServer("test")
	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	client.reserve(t, cookie, "idem-1", []string{"A-02"}, http.StatusCreated)
	client.reserve(t, cookie, "idem-2", []string{"A-02"}, http.StatusConflict)
}

func TestIdempotencyKeyConflict(t *testing.T) {
	server := NewServer("test")
	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	client.reserve(t, cookie, "same-key", []string{"A-03"}, http.StatusCreated)
	client.reserve(t, cookie, "same-key", []string{"A-04"}, http.StatusConflict)
}

func TestRoleChecks(t *testing.T) {
	server := NewServer("test")
	client := newTestClient(server)
	reserverCookie := client.login(t, "reserver@velox.local", "reserver")
	vendorCookie := client.login(t, "vendor@velox.local", "vendor")

	req := httptest.NewRequest(http.MethodGet, "/vendor/events", nil)
	req.AddCookie(reserverCookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("reserver vendor status = %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewReader([]byte(`{"event_id":"evt_neon_riot","section_id":"A","seat_ids":["A-05"]}`)))
	req.Header.Set("Idempotency-Key", "vendor-reserve")
	req.AddCookie(vendorCookie)
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("vendor reserve status = %d", rr.Code)
	}
}

func TestLoginAttemptsAreRateLimited(t *testing.T) {
	server := NewServer("test")
	client := newTestClient(server)
	for range 5 {
		client.loginStatus(t, "reserver@velox.local", "wrong", http.StatusUnauthorized)
	}
	client.loginStatus(t, "reserver@velox.local", "reserver", http.StatusTooManyRequests)
}

func TestExpiredHoldReleasesSeat(t *testing.T) {
	server := NewServer("test")
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	server.now = func() time.Time { return now }
	client := newTestClient(server)
	cookie := client.login(t, "reserver@velox.local", "reserver")
	client.reserve(t, cookie, "idem-1", []string{"A-06"}, http.StatusCreated)
	now = now.Add(server.holdTTL + time.Second)
	client.reserve(t, cookie, "idem-2", []string{"A-06"}, http.StatusCreated)
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
	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
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
