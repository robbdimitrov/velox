package internal

import (
	"context"
	"encoding/json"
	"net/http"
)

type OrderCreator interface {
	CreateOrder(ctx context.Context, req OrderRequest) (string, error)
}

type API struct {
	Store OrderCreator
}

func (api *API) HandleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.EventID == "" || req.SectionID == "" || len(req.SeatIDs) == 0 || req.IdempotencyKey == "" || req.UserID == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	orderID, err := api.Store.CreateOrder(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(OrderResponse{
		OrderID: orderID,
		Status:  "PENDING",
	})
}
