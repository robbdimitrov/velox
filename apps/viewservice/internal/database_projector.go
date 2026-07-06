package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrStoreNotFound = errors.New("store not found")

type DatabaseStore struct {
	db *sql.DB
}

func OpenDatabaseStore(ctx context.Context, databaseURL string) (*DatabaseStore, error) {
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
	return &DatabaseStore{db: db}, nil
}

func (s *DatabaseStore) Close() error {
	return s.db.Close()
}

func (s *DatabaseStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *DatabaseStore) IsEventProcessed(ctx context.Context, eventID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM projection.processed_events WHERE event_id = $1", eventID).Scan(&count)
	return count > 0, err
}

func (s *DatabaseStore) GetAggregateVersion(ctx context.Context, aggregateID string) (int64, error) {
	var version int64
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(aggregate_version), 0) FROM projection.processed_events WHERE aggregate_id = $1", aggregateID).Scan(&version)
	return version, err
}

func (s *DatabaseStore) ApplyEvent(ctx context.Context, event Event, sourceTopic string, sourcePartition int32, sourceOffset int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Resolve IDs across seatservice's flat events and orderservice's envelope.
	eventID := event.ResolvedEventID()
	aggregateID := event.ResolvedAggregateID()

	// processed_events.event_id is uuid-typed; skip non-UUID events before any
	// writes so malformed informational records cannot block the consumer.
	if _, err := uuid.Parse(eventID); err != nil {
		slog.Warn("skipping event with non-UUID event_id, cannot dedupe via processed_events",
			"event_id", eventID, "event_type", event.Type, "source_topic", sourceTopic, "source_offset", sourceOffset)
		return nil
	}

	// Dedup by event_id across Kafka offsets before inserting processed_events.
	var processed bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM projection.processed_events WHERE event_id = $1)", eventID).Scan(&processed)
	if err != nil {
		return err
	}
	if processed {
		return nil
	}

	var currentVersion int64
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(aggregate_version), 0) FROM projection.processed_events WHERE aggregate_id = $1", aggregateID).Scan(&currentVersion)
	if err != nil {
		return err
	}
	if event.AggregateVersion > 0 && currentVersion >= event.AggregateVersion {
		return ErrStaleAggregateVersion
	}

	// Order events are unversioned, but processed_events requires > 0.
	// Use a sentinel that is ignored by the staleness check above.
	storedVersion := event.AggregateVersion
	if storedVersion <= 0 {
		storedVersion = 1
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO projection.processed_events (event_id, aggregate_id, aggregate_version, source_topic, source_partition, source_offset)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, eventID, aggregateID, storedVersion, sourceTopic, sourcePartition, sourceOffset)
	if err != nil {
		return err
	}

	switch event.Type {
	case "SeatReservationHeld", "SeatReservationExpired", "SeatReservationConfirmed", "SeatReservationCancelled", "SeatTicketIssued":
		var expiresAt *time.Time
		if event.Seat.ExpiresAtMS > 0 {
			t := time.UnixMilli(event.Seat.ExpiresAtMS)
			expiresAt = &t
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO projection.seat_snapshots (event_id, section_id, seat_id, status, aggregate_version, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (event_id, section_id, seat_id) DO UPDATE SET
				status = EXCLUDED.status,
				aggregate_version = EXCLUDED.aggregate_version,
				expires_at = EXCLUDED.expires_at,
				updated_at = now()
			WHERE projection.seat_snapshots.aggregate_version < EXCLUDED.aggregate_version
		`, event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID, event.Seat.Status, event.AggregateVersion, expiresAt)
		if err != nil {
			return err
		}

		if event.Type == "SeatReservationConfirmed" {
			if err := issueWalletTicket(ctx, tx, event); err != nil {
				return err
			}
		}
		if event.Type == "SeatReservationCancelled" {
			if err := cancelWalletTicket(ctx, tx, event); err != nil {
				return err
			}
		}

		// Notify apigateway with a bounded seat delta; pg_notify stays parameterized.
		notificationPayload, err := json.Marshal(map[string]any{
			"event_id":   event.Seat.EventID,
			"section_id": event.Seat.SectionID,
			"changes": []map[string]any{
				{
					"seat_id":              event.Seat.SeatID,
					"status":               event.Seat.Status,
					"version":              event.AggregateVersion,
					"expires_at_server_ms": event.Seat.ExpiresAtMS,
				},
			},
		})
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, "SELECT pg_notify('seat_updates', $1)", string(notificationPayload))
		if err != nil {
			return err
		}

		vendorNotificationPayload, err := json.Marshal(map[string]any{
			"event_id": event.Seat.EventID,
		})
		if err == nil {
			if _, err := tx.ExecContext(ctx, "SELECT pg_notify('vendor_updates', $1)", string(vendorNotificationPayload)); err != nil {
				return err
			}
		}

	case "OrderCreated", "OrderConfirmed", "OrderCancelled", "OrderExpired":
		// Project order state.
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
			vendorNotificationPayload, err := json.Marshal(map[string]any{
				"event_id": event.Order.EventID,
			})
			if err == nil {
				if _, err := tx.ExecContext(ctx, "SELECT pg_notify('vendor_updates', $1)", string(vendorNotificationPayload)); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

// issueWalletTicket resolves the owner from the order projection, then creates
// a wallet ticket. If the order projection lags, ticket issuance is skipped.
func issueWalletTicket(ctx context.Context, tx *sql.Tx, event Event) error {
	if event.CorrelationID == "" {
		return nil
	}
	var userID string
	err := tx.QueryRowContext(ctx, "SELECT user_id FROM projection.order_summaries WHERE order_id = $1", event.CorrelationID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO projection.wallet_tickets (ticket_id, user_id, order_id, event_id, section_id, seat_id, status, aggregate_version)
		VALUES ($1, $2, $3, $4, $5, $6, 'ISSUED', $7)
		ON CONFLICT (ticket_id) DO NOTHING
	`, event.EventID, userID, event.CorrelationID, event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID, event.AggregateVersion)
	return err
}

// cancelWalletTicket marks issued tickets CANCELLED for event cancellation.
// Seats that were never confirmed simply have no wallet row to update.
func cancelWalletTicket(ctx context.Context, tx *sql.Tx, event Event) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE projection.wallet_tickets
		SET status = 'CANCELLED', updated_at = now()
		WHERE event_id = $1 AND section_id = $2 AND seat_id = $3 AND status = 'ISSUED'
	`, event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID)
	return err
}
