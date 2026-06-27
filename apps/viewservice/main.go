package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"velox/apps/viewservice/internal"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	setupLogger()

	addr := os.Getenv("VELOX_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dbURL := os.Getenv("VELOX_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
		if dbURL == "" {
			dbHost := os.Getenv("DATABASE_HOST")
			dbPass := os.Getenv("POSTGRES_PASSWORD")
			if dbHost != "" && dbPass != "" {
				dbURL = "postgres://velox:" + dbPass + "@" + dbHost + ":5432/velox"
			} else {
				dbURL = "postgres://velox:velox@postgres.velox.svc.cluster.local:5432/velox"
			}
		}
	}
	kafkaBrokers := os.Getenv("VELOX_KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := internal.OpenPostgresStore(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBrokers),
		kgo.ConsumeTopics("inventory.events.v1", "order.events.v1"),
		kgo.ConsumerGroup("viewservice-consumers"),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		slog.Error("failed to create kafka client", "error", err)
		os.Exit(1)
	}
	defer cl.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go consumeEvents(ctx, cl, store, &wg)

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
		slog.Info("viewservice listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	slog.Info("shutting down...")

	cancel() // Signal consumer to stop
	
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}

	wg.Wait()
}

func setupLogger() {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

type EventStore interface {
	ApplyEvent(ctx context.Context, event internal.Event, sourceTopic string, sourcePartition int32, sourceOffset int64) error
}

func consumeEvents(ctx context.Context, cl *kgo.Client, store EventStore, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		fetches := cl.PollFetches(ctx)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			slog.Error("kafka fetch errors", "errors", errs)
			continue
		}

		fetches.EachRecord(func(record *kgo.Record) {
			var event internal.Event
			if err := json.Unmarshal(record.Value, &event); err != nil {
				slog.Error("failed to unmarshal event", "error", err)
				return
			}
			
			err := store.ApplyEvent(ctx, event, record.Topic, record.Partition, record.Offset)
			if err != nil {
				slog.Error("failed to apply event", "topic", record.Topic, "offset", record.Offset, "error", err)
			} else {
				// Manually commit offsets after successful processing
				cl.CommitRecords(ctx, record)
			}
		})
	}
}
