package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

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

	// Start background processes
	go internal.StartOutboxRelay(ctx, store.DB(), cl)
	go internal.StartConsumer(ctx, store.DB(), cl)

	api := &internal.API{Store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "orderservice"})
	})
	mux.HandleFunc("POST /orders", api.HandleCreateOrder)

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
