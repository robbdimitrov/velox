package internal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const (
	maxOrderRequestBytes = 1 << 20
	maxSeatsPerOrder     = 8
)

type OrderCreator interface {
	CreateOrder(ctx context.Context, req OrderRequest) (string, error)
}

// OrderLifecycleStore is implemented by the order store to support the
// explicit user confirm/cancel reservation lifecycle.
type OrderLifecycleStore interface {
	OrderCreator
	ConfirmOrder(ctx context.Context, orderID string) (string, error)
	CancelOrder(ctx context.Context, orderID string) (string, error)
	CancelOrdersForEvent(ctx context.Context, eventID string) (int, error)
}

type API struct {
	Store OrderLifecycleStore
}

func (api *API) HandleCreateOrder(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxOrderRequestBytes)
	var req OrderRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	if req.EventID == "" || req.SectionID == "" || req.IdempotencyKey == "" || req.UserID == "" {
		writeError(w, http.StatusBadRequest, "missing_required_fields")
		return
	}
	if len(req.SeatIDs) == 0 || len(req.SeatIDs) > maxSeatsPerOrder {
		writeError(w, http.StatusBadRequest, "invalid_seat_count")
		return
	}

	orderID, err := api.Store.CreateOrder(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrIdempotencyConflict) || err.Error() == "conflict: request in progress or hash mismatch":
			writeError(w, http.StatusConflict, "idempotency_key_conflict")
		case errors.Is(err, ErrEventNotBookable):
			writeError(w, http.StatusConflict, "event_not_bookable")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(OrderResponse{
		OrderID: orderID,
		Status:  "PENDING",
	})
}

func (api *API) HandleConfirmOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	status, err := api.Store.ConfirmOrder(r.Context(), orderID)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			writeError(w, http.StatusNotFound, "order_not_found")
		case errors.Is(err, ErrOrderNotConfirmable):
			writeError(w, http.StatusConflict, "order_not_confirmable")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(OrderResponse{
		OrderID: orderID,
		Status:  status,
	})
}

func (api *API) HandleCancelOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	status, err := api.Store.CancelOrder(r.Context(), orderID)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			writeError(w, http.StatusNotFound, "order_not_found")
		case errors.Is(err, ErrOrderNotCancellable):
			writeError(w, http.StatusConflict, "order_not_cancellable")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(OrderResponse{
		OrderID: orderID,
		Status:  status,
	})
}

// HandleCancelEvent bulk-cancels outstanding orders for organizer event
// cancellation. Zero matching orders is valid and returns cancelled_orders: 0.
func (api *API) HandleCancelEvent(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("id")
	cancelled, err := api.Store.CancelOrdersForEvent(r.Context(), eventID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"event_id":         eventID,
		"cancelled_orders": cancelled,
	})
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
