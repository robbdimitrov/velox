package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	outboxBackoffBase = 1 * time.Second
	outboxBackoffMax  = 60 * time.Second
)

func StartOutboxRelay(ctx context.Context, db *sql.DB, cl *kgo.Client, health *PipelineHealth) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processOutbox(ctx, db, cl, health)
		}
	}
}

// outboxBackoff computes a bounded exponential delay before an outbox row is
// retried again, based on how many publish attempts have already failed.
func outboxBackoff(attempts int) time.Duration {
	if attempts <= 0 {
		return 0
	}
	delay := outboxBackoffBase
	for i := 0; i < attempts && delay < outboxBackoffMax; i++ {
		delay *= 2
	}
	if delay > outboxBackoffMax {
		delay = outboxBackoffMax
	}
	return delay
}

func processOutbox(ctx context.Context, db *sql.DB, cl *kgo.Client, health *PipelineHealth) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		health.MarkError("outbox", err)
		slog.Error("outbox begin tx error", "error", err)
		return
	}
	defer tx.Rollback()

	// created_at ordering approximates commit order for the relay. The
	// primary key is a random (v4) UUID, so publishing "by primary key
	// order" as docs/architecture.md literally states would not preserve
	// event order; created_at is kept intentionally here.
	rows, err := tx.QueryContext(ctx, `
		SELECT id, event_type, payload, headers, publish_attempts, last_attempt_at
		FROM orders.outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at ASC
		LIMIT 100 FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		health.MarkError("outbox", err)
		slog.Error("outbox query error", "error", err)
		return
	}
	defer rows.Close()

	type record struct {
		id            string
		eType         string
		payload       []byte
		headers       []byte
		attempts      int
		lastAttemptAt sql.NullTime
	}
	var records []record
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.id, &r.eType, &r.payload, &r.headers, &r.attempts, &r.lastAttemptAt); err != nil {
			health.MarkError("outbox", err)
			slog.Error("outbox scan error", "error", err)
			return
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		health.MarkError("outbox", err)
		slog.Error("outbox rows error", "error", err)
		return
	}
	rows.Close()

	now := time.Now()
	hadError := false
	for _, r := range records {
		if r.attempts > 0 && r.lastAttemptAt.Valid {
			if now.Before(r.lastAttemptAt.Time.Add(outboxBackoff(r.attempts))) {
				continue
			}
		}

		var hdrs map[string]string
		if len(r.headers) > 0 {
			_ = json.Unmarshal(r.headers, &hdrs)
		}

		brokerHeaders := []kgo.RecordHeader{
			{Key: "event_type", Value: []byte(r.eType)},
		}
		for k, v := range hdrs {
			brokerHeaders = append(brokerHeaders, kgo.RecordHeader{Key: k, Value: []byte(v)})
		}

		rec := &kgo.Record{
			Topic:   "order.events.v1",
			Key:     []byte(r.id),
			Value:   r.payload,
			Headers: brokerHeaders,
		}

		produceCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		produceErr := cl.ProduceSync(produceCtx, rec).FirstErr()
		cancel()
		if produceErr == nil {
			if _, err := tx.ExecContext(ctx, `UPDATE orders.outbox_events SET published_at = now() WHERE id = $1`, r.id); err != nil {
				hadError = true
				health.MarkError("outbox", err)
				slog.Error("failed to mark outbox event published", "error", err)
			}
			continue
		}

		hadError = true
		health.MarkError("outbox", produceErr)
		slog.Error("broker publish error", "error", produceErr)
		if _, err := tx.ExecContext(ctx, `
			UPDATE orders.outbox_events
			SET publish_attempts = publish_attempts + 1, last_error = $2, last_attempt_at = now()
			WHERE id = $1
		`, r.id, produceErr.Error()); err != nil {
			health.MarkError("outbox", err)
			slog.Error("failed to record outbox publish failure", "error", err)
		}
	}
	if err := tx.Commit(); err != nil {
		health.MarkError("outbox", err)
		slog.Error("outbox tx commit error", "error", err)
		return
	}
	if hadError {
		return
	}
	health.MarkSuccess("outbox")
}
