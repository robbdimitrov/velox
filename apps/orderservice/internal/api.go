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

type API struct {
	Store OrderCreator
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
		if errors.Is(err, ErrIdempotencyConflict) || err.Error() == "conflict: request in progress or hash mismatch" {
			writeError(w, http.StatusConflict, "idempotency_key_conflict")
		} else {
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

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
