package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if s.store != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := s.store.Ping(ctx); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded"})
				return
			}
		}
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
	mux.HandleFunc("GET /events/{eventId}/announcements", s.handleEventAnnouncements)

	rl := NewRateLimiter(s.cacheClient, 10.0, 100) // 10 TPS, 100 max burst
	mux.HandleFunc("POST /reservations", s.requireAuth(rl.AuthedMiddleware("reservations_create", s.handleCreateReservation)))
	mux.HandleFunc("POST /reservations/{reservationId}/confirm", s.requireAuth(rl.AuthedMiddleware("reservations_confirm", s.handleConfirmReservation)))
	mux.HandleFunc("POST /reservations/{reservationId}/cancel", s.requireAuth(rl.AuthedMiddleware("reservations_cancel", s.handleCancelReservation)))
	mux.HandleFunc("GET /orders", s.requireAuth(s.handleOrders))
	mux.HandleFunc("GET /orders/{orderId}", s.requireAuth(s.handleOrder))
	mux.HandleFunc("GET /wallet/tickets", s.requireAuth(s.handleWalletTickets))
	mux.HandleFunc("GET /organizer/events", s.requireRole(RoleOrganizer, s.handleOrganizerEvents))
	mux.HandleFunc("GET /organizer/events/{eventId}/orders", s.requireRole(RoleOrganizer, s.handleOrganizerOrders))
	mux.HandleFunc("GET /organizer/events/{eventId}/inventory", s.requireRole(RoleOrganizer, s.handleOrganizerInventory))
	mux.HandleFunc("GET /organizer/metrics/stream", s.requireRole(RoleOrganizer, s.handleOrganizerMetricsStream))
	mux.HandleFunc("GET /api/organizer/venues", s.requireRole(RoleOrganizer, s.handleListVenues))
	mux.HandleFunc("POST /api/organizer/venues", s.requireRole(RoleOrganizer, s.handleCreateVenue))
	mux.HandleFunc("GET /api/organizer/venues/{id}/staff", s.requireRole(RoleOrganizer, s.handleListVenueStaff))
	mux.HandleFunc("POST /api/organizer/events", s.requireRole(RoleOrganizer, s.handleCreateEvent))
	mux.HandleFunc("POST /organizer/events/{eventId}/announcements", s.requireRole(RoleOrganizer, s.handleCreateAnnouncement))
	mux.HandleFunc("POST /organizer/events/{eventId}/cancel", s.requireRole(RoleOrganizer, s.handleCancelEvent))
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
