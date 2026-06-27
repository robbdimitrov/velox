package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"velox/apps/viewservice/internal"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	addr := os.Getenv("VELOX_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dbURL := os.Getenv("VELOX_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://velox:velox@localhost:5432/velox"
	}
	kafkaBrokers := os.Getenv("VELOX_KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := internal.OpenPostgresStore(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer store.Close()

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBrokers),
		kgo.ConsumeTopics("inventory.events.v1", "order.events.v1"),
		kgo.ConsumerGroup("viewservice-consumers"),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		log.Fatalf("failed to create kafka client: %v", err)
	}
	defer cl.Close()

	go consumeEvents(ctx, cl, store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "viewservice"})
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("viewservice listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")
	
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Printf("http server shutdown error: %v", err)
	}
}

func consumeEvents(ctx context.Context, cl *kgo.Client, store *internal.PostgresStore) {
	for {
		fetches := cl.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			log.Printf("kafka fetch errors: %v", errs)
			continue
		}

		fetches.EachRecord(func(record *kgo.Record) {
			var event internal.Event
			if err := json.Unmarshal(record.Value, &event); err != nil {
				log.Printf("failed to unmarshal event: %v", err)
				return
			}
			
			err := store.ApplyEvent(ctx, event, record.Topic, record.Partition, record.Offset)
			if err != nil {
				log.Printf("failed to apply event (topic=%s, offset=%d): %v", record.Topic, record.Offset, err)
			} else {
				// Manually commit offsets after successful processing
				cl.CommitRecords(ctx, record)
			}
		})
	}
}
