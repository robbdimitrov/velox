package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestPostgresReservationSmoke(t *testing.T) {
	databaseURL := os.Getenv("VELOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set VELOX_TEST_DATABASE_URL to run PostgreSQL smoke test")
	}
	store, err := OpenPostgresStore(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	server := NewServerWithStore("test", store)
	client := newTestClient(server)
	cookie := client.login(t, "buyer@velox.local", "buyer")
	order := client.reserve(t, cookie, "pg-smoke-reserve", []string{"A-09"}, http.StatusCreated)

	req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("confirm status = %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("orders status = %d body=%s", rr.Code, rr.Body.String())
	}
}
