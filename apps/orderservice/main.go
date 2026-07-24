package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"velox/apps/orderservice/internal"
)

func main() {
	addr := os.Getenv("VELOX_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dbURL := os.Getenv("VELOX_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
		if dbURL == "" {
			dbHost := os.Getenv("DATABASE_HOST")
			dbPass := os.Getenv("DATABASE_PASSWORD")
			if dbHost != "" && dbPass != "" {
				dbURL = "postgres://velox:" + dbPass + "@" + dbHost + ":5432/velox?sslmode=disable"
			} else {
				dbURL = "postgres://velox:velox@localhost:5432/velox?sslmode=disable"
			}
		}
	}
	brokerAddrs := os.Getenv("VELOX_KAFKA_BROKERS")
	if brokerAddrs == "" {
		brokerAddrs = os.Getenv("KAFKA_BROKERS")
		if brokerAddrs == "" {
			brokerAddrs = "localhost:9092"
		}
	}

	store, err := internal.NewStore(dbURL)
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokerAddrs),
		kgo.ConsumeTopics("inventory.events.v1"),
		kgo.ConsumerGroup("orderservice"),
	)
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}
	defer cl.Close()

	ctx := context.Background()
	pipelineHealth := internal.NewPipelineHealth("outbox", "consumer")

	// Start background processes
	go internal.StartOutboxRelay(ctx, store.DB(), cl, pipelineHealth)
	go internal.StartConsumer(ctx, store.DB(), cl, pipelineHealth)

	api := &internal.API{Store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "orderservice"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(pingCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "degraded", "database": "unavailable"})
			return
		}
		if err := cl.Ping(pingCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "degraded", "broker": "unavailable"})
			return
		}
		pipelineHealth.MarkRecovered("outbox", "consumer")
		if err := pipelineHealth.Err(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "degraded", "pipelines": pipelineHealth.Snapshot()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "orderservice", "pipelines": pipelineHealth.Snapshot()})
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		var b strings.Builder
		b.WriteString(pipelineHealth.Metrics("orderservice"))
		countCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if count, err := store.CountUnpublishedOutboxEvents(countCtx); err == nil {
			fmt.Fprintf(&b, "velox_outbox_unpublished_events{service=%q} %d\n", "orderservice", count)
		}
		_, _ = w.Write([]byte(b.String()))
	})
	mux.HandleFunc("POST /orders", api.HandleCreateOrder)
	mux.HandleFunc("POST /orders/{id}/confirm", api.HandleConfirmOrder)
	mux.HandleFunc("POST /orders/{id}/cancel", api.HandleCancelOrder)
	mux.HandleFunc("POST /events/{id}/cancel", api.HandleCancelEvent)

	slog.Info("orderservice listening", "addr", addr)
	if err := http.ListenAndServe(addr, tracingMiddleware(mux)); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

const RequestIDKey string = "request_id"

func tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID != "" {
			ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
			r = r.WithContext(ctx)
		}

		slog.Info("incoming request",
			"method", r.Method,
			"path", r.URL.Path,
			"request_id", reqID,
		)

		next.ServeHTTP(w, r)
	})
}
