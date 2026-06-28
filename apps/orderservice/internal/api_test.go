package internal

import (
	"bytes"
	"context"
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
