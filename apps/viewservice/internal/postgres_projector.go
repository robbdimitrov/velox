package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrStoreNotFound = errors.New("store not found")

type PostgresStore struct {
	db *sql.DB
}

func OpenPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) IsEventProcessed(ctx context.Context, eventID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM projection.processed_events WHERE event_id = $1", eventID).Scan(&count)
	return count > 0, err
}

func (s *PostgresStore) GetAggregateVersion(ctx context.Context, aggregateID string) (int64, error) {
	var version int64
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(aggregate_version), 0) FROM projection.processed_events WHERE aggregate_id = $1", aggregateID).Scan(&version)
	return version, err
}

func (s *PostgresStore) ApplyEvent(ctx context.Context, event Event, sourceTopic string, sourcePartition int32, sourceOffset int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if processed
	var processed bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM projection.processed_events WHERE source_topic = $1 AND source_partition = $2 AND source_offset = $3)", sourceTopic, sourcePartition, sourceOffset).Scan(&processed)
	if err != nil {
		return err
	}
	if processed {
		return nil
	}

	var currentVersion int64
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(aggregate_version), 0) FROM projection.processed_events WHERE aggregate_id = $1", event.AggregateID).Scan(&currentVersion)
	if err != nil {
		return err
	}
	if event.AggregateVersion > 0 && currentVersion >= event.AggregateVersion {
		return ErrStaleAggregateVersion
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO projection.processed_events (event_id, aggregate_id, aggregate_version, source_topic, source_partition, source_offset)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, event.EventID, event.AggregateID, event.AggregateVersion, sourceTopic, sourcePartition, sourceOffset)
	if err != nil {
		return err
	}

	switch event.Type {
	case "SeatReservationHeld", "SeatReservationExpired", "SeatTicketIssued":
		_, err = tx.ExecContext(ctx, `
			INSERT INTO projection.seat_snapshots (event_id, section_id, seat_id, status, aggregate_version)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (event_id, section_id, seat_id) DO UPDATE SET
				status = EXCLUDED.status,
				aggregate_version = EXCLUDED.aggregate_version,
				updated_at = now()
			WHERE projection.seat_snapshots.aggregate_version < EXCLUDED.aggregate_version
		`, event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID, event.Seat.Status, event.AggregateVersion)
		if err != nil {
			return err
		}

		// Notify apigateway about projection update for SSE
		notificationPayload, err := json.Marshal(map[string]interface{}{
			"event_id": event.Seat.EventID,
		})
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, fmt.Sprintf("NOTIFY seat_updates, '%s'", string(notificationPayload)))
		if err != nil {
			return err
		}

		vendorNotificationPayload, err := json.Marshal(map[string]interface{}{
			"event_id": event.Seat.EventID,
		})
		if err == nil {
			_, _ = tx.ExecContext(ctx, fmt.Sprintf("NOTIFY vendor_updates, '%s'", string(vendorNotificationPayload)))
		}

	case "OrderCreated", "OrderConfirmed", "OrderExpired":
		// Handle Order projection
		_, err = tx.ExecContext(ctx, `
			INSERT INTO projection.order_summaries (order_id, user_id, status, total_amount_minor, currency, event_id)
			VALUES ($1, $2, $3, $4, 'USD', $5)
			ON CONFLICT (order_id) DO UPDATE SET
				status = EXCLUDED.status,
				total_amount_minor = EXCLUDED.total_amount_minor,
				event_id = EXCLUDED.event_id,
				updated_at = now()
		`, event.Order.OrderID, event.Order.UserID, event.Order.Status, event.Order.TotalAmountMinor, event.Order.EventID)
		if err != nil {
			return err
		}
		
		if event.Order.EventID != "" {
			vendorNotificationPayload, err := json.Marshal(map[string]interface{}{
				"event_id": event.Order.EventID,
			})
			if err == nil {
				_, _ = tx.ExecContext(ctx, fmt.Sprintf("NOTIFY vendor_updates, '%s'", string(vendorNotificationPayload)))
			}
		}
	}

	return tx.Commit()
}
