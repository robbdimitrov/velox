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
	
	rl := NewRateLimiter(s.rdb, 10.0, 100) // 10 TPS, 100 max burst
	mux.HandleFunc("POST /reservations", rl.Middleware(s.requireAuth(s.handleCreateReservation)))
	mux.HandleFunc("POST /reservations/{reservationId}/confirm", s.requireAuth(s.handleConfirmReservation))
	mux.HandleFunc("GET /orders", s.requireAuth(s.handleOrders))
	mux.HandleFunc("GET /orders/{orderId}", s.requireAuth(s.handleOrder))
	mux.HandleFunc("GET /organizer/events", s.requireAuth(s.handleOrganizerEvents))
	mux.HandleFunc("GET /organizer/events/{eventId}/orders", s.requireAuth(s.handleOrganizerOrders))
	mux.HandleFunc("GET /organizer/events/{eventId}/inventory", s.requireAuth(s.handleOrganizerInventory))
	mux.HandleFunc("GET /organizer/metrics/stream", s.requireAuth(s.handleOrganizerMetricsStream))
	mux.HandleFunc("GET /api/organizer/venues", s.requireAuth(s.handleListVenues))
	mux.HandleFunc("GET /api/organizer/venues/{id}/staff", s.requireAuth(s.handleListVenueStaff))
	mux.HandleFunc("POST /api/organizer/events", s.requireAuth(s.handleCreateEvent))
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
