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

type mockOrderCreator struct {
	err     error
	orderID string
}

func (m *mockOrderCreator) CreateOrder(ctx context.Context, req OrderRequest) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.orderID, nil
}

func TestHandleCreateOrder(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        OrderRequest
		mockID         string
		mockErr        error
		expectedStatus int
	}{
		{
			name: "valid request",
			reqBody: OrderRequest{
				EventID:        "evt_1",
				SectionID:      "sec_1",
				SeatIDs:        []string{"seat_1"},
				IdempotencyKey: "idem_1",
				UserID:         "user_1",
			},
			mockID:         "order_123",
			expectedStatus: http.StatusOK,
		},
		{
			name: "missing required fields",
			reqBody: OrderRequest{
				EventID: "evt_1",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "store error",
			reqBody: OrderRequest{
				EventID:        "evt_1",
				SectionID:      "sec_1",
				SeatIDs:        []string{"seat_1"},
				IdempotencyKey: "idem_1",
				UserID:         "user_1",
			},
			mockErr:        errors.New("store error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &API{
				Store: &mockOrderCreator{
					err:     tt.mockErr,
					orderID: tt.mockID,
				},
			}

			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(body))
			rec := httptest.NewRecorder()

			api.HandleCreateOrder(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}
