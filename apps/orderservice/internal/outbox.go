package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func StartOutboxRelay(ctx context.Context, db *sql.DB, cl *kgo.Client) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processOutbox(ctx, db, cl)
		}
	}
}

func processOutbox(ctx context.Context, db *sql.DB, cl *kgo.Client) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, event_type, payload, headers
		FROM orders.outbox_events 
		WHERE published_at IS NULL 
		ORDER BY created_at ASC 
		LIMIT 100 FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	type record struct {
		id      string
		eType   string
		payload []byte
		headers []byte
	}
	var records []record
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.id, &r.eType, &r.payload, &r.headers); err != nil {
			slog.Error("outbox scan error", "error", err)
			return
		}
		records = append(records, r)
	}
	rows.Close()

	for _, r := range records {
		var hdrs map[string]string
		if len(r.headers) > 0 {
			_ = json.Unmarshal(r.headers, &hdrs)
		}

		kafkaHeaders := []kgo.RecordHeader{
			{Key: "event_type", Value: []byte(r.eType)},
		}
		for k, v := range hdrs {
			kafkaHeaders = append(kafkaHeaders, kgo.RecordHeader{Key: k, Value: []byte(v)})
		}

		rec := &kgo.Record{
			Topic:   "order.events.v1",
			Key:     []byte(r.id),
			Value:   r.payload,
			Headers: kafkaHeaders,
		}

		err := cl.ProduceSync(ctx, rec).FirstErr()
		if err == nil {
			_, _ = tx.ExecContext(ctx, `UPDATE orders.outbox_events SET published_at = now() WHERE id = $1`, r.id)
		} else {
			slog.Error("kafka publish error", "error", err)
		}
	}
	_ = tx.Commit()
}
