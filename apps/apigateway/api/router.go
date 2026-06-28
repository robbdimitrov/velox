package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /auth/register", s.handleRegister)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("GET /auth/me", s.handleMe)
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
	mux.HandleFunc("GET /vendor/metrics/stream", s.requireRole(RoleVendor, s.handleVendorMetricsStream))
	mux.HandleFunc("GET /api/vendor/venues", s.requireRole(RoleVendor, s.handleListVenues))
	mux.HandleFunc("GET /api/vendor/venues/{id}/staff", s.requireRole(RoleVendor, s.handleListVenueStaff))
	mux.HandleFunc("POST /api/vendor/events", s.requireRole(RoleVendor, s.handleCreateEvent))
	handler := limitBody(mux, 1<<20)
	return tracingMiddleware(handler)
}

type contextKey string

const RequestIDKey contextKey = "request_id"

func tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", reqID)

		slog.Info("incoming request",
			"method", r.Method,
			"path", r.URL.Path,
			"request_id", reqID,
			"remote_addr", r.RemoteAddr,
		)

		next.ServeHTTP(w, r)
	})
}
