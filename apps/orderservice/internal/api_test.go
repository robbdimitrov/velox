package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type FakeStore struct {
	CreateOrderFunc          func(ctx context.Context, req OrderRequest) (string, error)
	ConfirmOrderFunc         func(ctx context.Context, orderID string) (string, error)
	CancelOrderFunc          func(ctx context.Context, orderID string) (string, error)
	CancelOrdersForEventFunc func(ctx context.Context, eventID string) (int, error)
}

func (f *FakeStore) CreateOrder(ctx context.Context, req OrderRequest) (string, error) {
	return f.CreateOrderFunc(ctx, req)
}

func (f *FakeStore) ConfirmOrder(ctx context.Context, orderID string) (string, error) {
	return f.ConfirmOrderFunc(ctx, orderID)
}

func (f *FakeStore) CancelOrder(ctx context.Context, orderID string) (string, error) {
	return f.CancelOrderFunc(ctx, orderID)
}

func (f *FakeStore) CancelOrdersForEvent(ctx context.Context, eventID string) (int, error) {
	return f.CancelOrdersForEventFunc(ctx, eventID)
}

func TestHandleCreateOrder_Idempotency(t *testing.T) {
	fakeStore := &FakeStore{
		CreateOrderFunc: func(ctx context.Context, req OrderRequest) (string, error) {
			if req.IdempotencyKey == "conflict-key" {
				return "", errors.New("conflict: request in progress or hash mismatch")
			}
			if req.IdempotencyKey == "ok-key" {
				return "order-123", nil
			}
			return "", errors.New("unknown error")
		},
	}
	api := &API{Store: fakeStore}

	t.Run("conflict", func(t *testing.T) {
		reqBody := `{"event_id":"e1","section_id":"s1","seat_ids":["seat1"],"idempotency_key":"conflict-key","user_id":"u1"}`
		req := httptest.NewRequest("POST", "/orders", bytes.NewReader([]byte(reqBody)))
		rr := httptest.NewRecorder()
		api.HandleCreateOrder(rr, req)
		if rr.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d", rr.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		reqBody := `{"event_id":"e1","section_id":"s1","seat_ids":["seat1"],"idempotency_key":"ok-key","user_id":"u1"}`
		req := httptest.NewRequest("POST", "/orders", bytes.NewReader([]byte(reqBody)))
		rr := httptest.NewRecorder()
		api.HandleCreateOrder(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}
	})
}

func TestHandleCreateOrderRejectsInvalidSeatCount(t *testing.T) {
	api := &API{Store: &FakeStore{
		CreateOrderFunc: func(ctx context.Context, req OrderRequest) (string, error) {
			t.Fatal("store should not be called")
			return "", nil
		},
	}}

	reqBody := `{"event_id":"e1","section_id":"s1","seat_ids":[],"idempotency_key":"ok-key","user_id":"u1"}`
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader([]byte(reqBody)))
	rr := httptest.NewRecorder()

	api.HandleCreateOrder(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rr, "invalid_seat_count")
}

func TestHandleCreateOrderRejectsUnknownFields(t *testing.T) {
	api := &API{Store: &FakeStore{
		CreateOrderFunc: func(ctx context.Context, req OrderRequest) (string, error) {
			t.Fatal("store should not be called")
			return "", nil
		},
	}}

	reqBody := `{"event_id":"e1","section_id":"s1","seat_ids":["seat1"],"idempotency_key":"ok-key","user_id":"u1","price":1}`
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader([]byte(reqBody)))
	rr := httptest.NewRecorder()

	api.HandleCreateOrder(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rr, "invalid_json")
}

func TestHandleConfirmOrder(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		api := &API{Store: &FakeStore{
			ConfirmOrderFunc: func(ctx context.Context, orderID string) (string, error) {
				return "CONFIRMED", nil
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/orders/order-123/confirm", nil)
		req.SetPathValue("id", "order-123")
		rr := httptest.NewRecorder()

		api.HandleConfirmOrder(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("not found", func(t *testing.T) {
		api := &API{Store: &FakeStore{
			ConfirmOrderFunc: func(ctx context.Context, orderID string) (string, error) {
				return "", ErrOrderNotFound
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/orders/missing/confirm", nil)
		req.SetPathValue("id", "missing")
		rr := httptest.NewRecorder()

		api.HandleConfirmOrder(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
		assertErrorCode(t, rr, "order_not_found")
	})

	t.Run("not confirmable", func(t *testing.T) {
		api := &API{Store: &FakeStore{
			ConfirmOrderFunc: func(ctx context.Context, orderID string) (string, error) {
				return "", ErrOrderNotConfirmable
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/orders/order-123/confirm", nil)
		req.SetPathValue("id", "order-123")
		rr := httptest.NewRecorder()

		api.HandleConfirmOrder(rr, req)

		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusConflict)
		}
		assertErrorCode(t, rr, "order_not_confirmable")
	})
}

func TestHandleCancelOrder(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		api := &API{Store: &FakeStore{
			CancelOrderFunc: func(ctx context.Context, orderID string) (string, error) {
				return "CANCELLED", nil
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/orders/order-123/cancel", nil)
		req.SetPathValue("id", "order-123")
		rr := httptest.NewRecorder()

		api.HandleCancelOrder(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("not cancellable", func(t *testing.T) {
		api := &API{Store: &FakeStore{
			CancelOrderFunc: func(ctx context.Context, orderID string) (string, error) {
				return "", ErrOrderNotCancellable
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/orders/order-123/cancel", nil)
		req.SetPathValue("id", "order-123")
		rr := httptest.NewRecorder()

		api.HandleCancelOrder(rr, req)

		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusConflict)
		}
		assertErrorCode(t, rr, "order_not_cancellable")
	})
}

func TestHandleCancelEvent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		api := &API{Store: &FakeStore{
			CancelOrdersForEventFunc: func(ctx context.Context, eventID string) (int, error) {
				if eventID != "evt-123" {
					t.Fatalf("eventID = %s, want evt-123", eventID)
				}
				return 3, nil
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/events/evt-123/cancel", nil)
		req.SetPathValue("id", "evt-123")
		rr := httptest.NewRecorder()

		api.HandleCancelEvent(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var out struct {
			EventID         string `json:"event_id"`
			CancelledOrders int    `json:"cancelled_orders"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
		}
		if out.EventID != "evt-123" || out.CancelledOrders != 3 {
			t.Fatalf("response = %+v, want event_id=evt-123 cancelled_orders=3", out)
		}
	})

	t.Run("no orders for event", func(t *testing.T) {
		api := &API{Store: &FakeStore{
			CancelOrdersForEventFunc: func(ctx context.Context, eventID string) (int, error) {
				return 0, nil
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/events/evt-empty/cancel", nil)
		req.SetPathValue("id", "evt-empty")
		rr := httptest.NewRecorder()

		api.HandleCancelEvent(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var out struct {
			CancelledOrders int `json:"cancelled_orders"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
		}
		if out.CancelledOrders != 0 {
			t.Fatalf("cancelled_orders = %d, want 0", out.CancelledOrders)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		api := &API{Store: &FakeStore{
			CancelOrdersForEventFunc: func(ctx context.Context, eventID string) (int, error) {
				return 0, errors.New("db unavailable")
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/events/evt-123/cancel", nil)
		req.SetPathValue("id", "evt-123")
		rr := httptest.NewRecorder()

		api.HandleCancelEvent(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
		assertErrorCode(t, rr, "internal_error")
	})
}

func assertErrorCode(t *testing.T, rr *httptest.ResponseRecorder, want string) {
	t.Helper()
	var out struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error response: %v body=%s", err, rr.Body.String())
	}
	if out.Error != want {
		t.Fatalf("error = %q, want %q", out.Error, want)
	}
}
