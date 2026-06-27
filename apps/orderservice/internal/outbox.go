package internal

import (
	"context"
	"database/sql"
	"log"
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
		SELECT id, event_type, payload 
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
	}
	var records []record
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.id, &r.eType, &r.payload); err != nil {
			log.Printf("outbox scan error: %v", err)
			return
		}
		records = append(records, r)
	}
	rows.Close()

	for _, r := range records {
		rec := &kgo.Record{
			Topic: "order.events.v1",
			Key:   []byte(r.id),
			Value: r.payload,
			Headers: []kgo.RecordHeader{
				{Key: "event_type", Value: []byte(r.eType)},
			},
		}

		err := cl.ProduceSync(ctx, rec).FirstErr()
		if err == nil {
			_, _ = tx.ExecContext(ctx, `UPDATE orders.outbox_events SET published_at = now() WHERE id = $1`, r.id)
		} else {
			log.Printf("kafka publish error: %v", err)
		}
	}
	_ = tx.Commit()
}
