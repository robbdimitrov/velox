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
	CreateOrderFunc func(ctx context.Context, req OrderRequest) (string, error)
}

func (f *FakeStore) CreateOrder(ctx context.Context, req OrderRequest) (string, error) {
	return f.CreateOrderFunc(ctx, req)
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
