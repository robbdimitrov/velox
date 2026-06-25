package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
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
	resetPostgresTestDB(t, store)

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

func TestPostgresConfirmReservationIsIdempotent(t *testing.T) {
	databaseURL := os.Getenv("VELOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set VELOX_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	store, err := OpenPostgresStore(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	resetPostgresTestDB(t, store)

	server := NewServerWithStore("test", store)
	client := newTestClient(server)
	cookie := client.login(t, "buyer@velox.local", "buyer")
	order := client.reserve(t, cookie, "pg-confirm-idem", []string{"A-10"}, http.StatusCreated)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/reservations/"+order.ReservationID+"/confirm", nil)
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		server.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("confirm attempt %d status = %d body=%s", i+1, rr.Code, rr.Body.String())
		}
	}
}

func TestPostgresExpiredHoldUpdatesOrderAndInventory(t *testing.T) {
	databaseURL := os.Getenv("VELOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set VELOX_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	store, err := OpenPostgresStore(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	resetPostgresTestDB(t, store)

	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	server := NewServerWithStore("test", store)
	server.now = func() time.Time { return now }
	client := newTestClient(server)
	cookie := client.login(t, "buyer@velox.local", "buyer")
	expiredOrder := client.reserve(t, cookie, "pg-expire-old", []string{"A-08"}, http.StatusCreated)

	now = now.Add(server.holdTTL + time.Second)
	client.reserve(t, cookie, "pg-expire-new", []string{"A-08"}, http.StatusCreated)

	var orderStatus string
	if err := store.db.QueryRowContext(context.Background(), `
		SELECT status
		FROM orders.orders
		WHERE id = $1
	`, expiredOrder.ID).Scan(&orderStatus); err != nil {
		t.Fatal(err)
	}
	if orderStatus != OrderExpired {
		t.Fatalf("expired order status = %s, want %s", orderStatus, OrderExpired)
	}

	var reservationStatus string
	if err := store.db.QueryRowContext(context.Background(), `
		SELECT status
		FROM inventory.reservations
		WHERE reservation_id = $1
	`, expiredOrder.ReservationID).Scan(&reservationStatus); err != nil {
		t.Fatal(err)
	}
	if reservationStatus != "EXPIRED" {
		t.Fatalf("expired inventory status = %s, want EXPIRED", reservationStatus)
	}
}

func resetPostgresTestDB(t *testing.T, store *PostgresStore) {
	t.Helper()
	if _, err := store.db.ExecContext(context.Background(), `
		TRUNCATE
			orders.outbox_events,
			orders.order_seats,
			orders.orders,
			orders.idempotency_keys,
			inventory.reservations,
			projection.seat_snapshots
		RESTART IDENTITY
	`); err != nil {
		t.Fatal(err)
	}
}
