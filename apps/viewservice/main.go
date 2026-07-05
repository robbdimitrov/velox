package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

const dlqTopic = "dlq.inventory.events.v1"
const consumerGroup = "viewservice-consumers"

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
			dbPass := os.Getenv("DATABASE_PASSWORD")
			if dbHost != "" && dbPass != "" {
				dbURL = "postgres://velox:" + dbPass + "@" + dbHost + ":5432/velox"
			} else {
				dbURL = "postgres://velox:velox@database.velox.svc.cluster.local:5432/velox"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := internal.OpenDatabaseStore(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokerAddrs),
		kgo.ConsumeTopics("inventory.events.v1", "order.events.v1"),
		kgo.ConsumerGroup(consumerGroup),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		slog.Error("failed to create broker client", "error", err)
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
			slog.Error("broker fetch errors", "errors", errs)
			continue
		}

		fetches.EachRecord(func(record *kgo.Record) {
			var reqID string
			for _, h := range record.Headers {
				if h.Key == "X-Request-ID" {
					reqID = string(h.Value)
				}
			}

			var event internal.Event
			if err := json.Unmarshal(record.Value, &event); err != nil {
				slog.Error("failed to unmarshal event", "error", err, "request_id", reqID)
				sendToDLQ(ctx, cl, record, reqID, "envelope_deserialize_error", err.Error())
				cl.CommitRecords(ctx, record)
				return
			}

			slog.Info("processing event", "topic", record.Topic, "type", event.Type, "request_id", reqID)
			err := store.ApplyEvent(ctx, event, record.Topic, record.Partition, record.Offset)
			switch {
			case err == nil:
				cl.CommitRecords(ctx, record)
			case errors.Is(err, internal.ErrStaleAggregateVersion):
				// Expected under duplicate/out-of-order delivery: drop and
				// commit rather than retrying a message that will never
				// succeed (docs/infrastructure.md: reject lower versions).
				slog.Info("dropping stale/duplicate event", "topic", record.Topic, "offset", record.Offset, "event_id", event.EventID, "request_id", reqID)
				cl.CommitRecords(ctx, record)
			default:
				// Transient/infra failure: leave the offset uncommitted so
				// this record is retried with backoff via redelivery.
				slog.Error("failed to apply event", "topic", record.Topic, "offset", record.Offset, "error", err, "request_id", reqID)
			}
		})
	}
}

// sendToDLQ publishes an unrecoverable record (one that can never succeed on
// retry, e.g. a schema/deserialize failure) to the DLQ topic per
// docs/infrastructure.md's poison-pill handling, so the caller can safely
// commit the source offset without silently dropping the failure.
func sendToDLQ(ctx context.Context, cl *kgo.Client, record *kgo.Record, requestID, errorClass, errorMessage string) {
	hash := sha256.Sum256(record.Value)
	now := time.Now().UTC()
	dlqRecord := map[string]any{
		"source_topic":     record.Topic,
		"source_partition": record.Partition,
		"source_offset":    record.Offset,
		"consumer_group":   consumerGroup,
		"error_class":      errorClass,
		"error_message":    errorMessage,
		"payload_hash":     hex.EncodeToString(hash[:]),
		"first_seen_at":    now,
		"last_seen_at":     now,
		"correlation_id":   requestID,
	}
	payload, err := json.Marshal(dlqRecord)
	if err != nil {
		slog.Error("failed to marshal DLQ record", "error", err)
		return
	}
	dlqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result := cl.ProduceSync(dlqCtx, &kgo.Record{Topic: dlqTopic, Key: []byte(errorClass), Value: payload})
	if err := result.FirstErr(); err != nil {
		slog.Error("failed to publish DLQ record", "error", err)
	}
}
