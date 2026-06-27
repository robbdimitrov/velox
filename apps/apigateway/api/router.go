package api

import (
	"net/http"
)

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /sessions", s.handleCreateSession)
	mux.HandleFunc("DELETE /sessions", s.handleDeleteSession)
	mux.HandleFunc("GET /events", s.handleEvents)
	mux.HandleFunc("GET /events/{eventId}", s.handleEvent)
	mux.HandleFunc("GET /events/{eventId}/sections/{sectionId}/seats", s.handleSeats)
	mux.HandleFunc("GET /events/{eventId}/stream", s.handleSeatStream)
	mux.HandleFunc("POST /reservations", s.requireRole(RoleReserver, s.handleCreateReservation))
	mux.HandleFunc("POST /reservations/{reservationId}/confirm", s.requireRole(RoleReserver, s.handleConfirmReservation))
	mux.HandleFunc("GET /orders", s.requireRole(RoleReserver, s.handleOrders))
	mux.HandleFunc("GET /orders/{orderId}", s.requireRole(RoleReserver, s.handleOrder))
	mux.HandleFunc("GET /vendor/events", s.requireRole(RoleVendor, s.handleVendorEvents))
	mux.HandleFunc("GET /vendor/events/{eventId}/orders", s.requireRole(RoleVendor, s.handleVendorOrders))
	mux.HandleFunc("GET /vendor/events/{eventId}/inventory", s.requireRole(RoleVendor, s.handleVendorInventory))
	return limitBody(mux, 1<<20)
}
