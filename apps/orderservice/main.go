package main

import (
	"context"
	"encoding/json"
	"log"
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
		dbURL = "postgres://postgres:postgres@localhost:5432/velox?sslmode=disable"
	}
	kafkaBrokers := os.Getenv("VELOX_KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}

	store, err := internal.NewStore(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer store.Close()

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBrokers),
		kgo.ConsumeTopics("inventory.events.v1"),
		kgo.ConsumerGroup("orderservice"),
	)
	if err != nil {
		log.Fatalf("failed to create kafka client: %v", err)
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

	log.Printf("orderservice listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
