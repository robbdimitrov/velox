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
			if err := issueOrBufferWalletTicket(ctx, tx, event); err != nil {
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

		organizerNotificationPayload, err := json.Marshal(map[string]any{
			"event_id": event.Seat.EventID,
		})
		if err == nil {
			if _, err := tx.ExecContext(ctx, "SELECT pg_notify('vendor_updates', $1)", string(organizerNotificationPayload)); err != nil {
				return err
			}
		}

	case "OrderCreated", "OrderConfirmed", "OrderCancelled", "OrderExpired":
		// Project order state.
		_, err = tx.ExecContext(ctx, `
			INSERT INTO projection.order_summaries (order_id, user_id, status, event_id)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (order_id) DO UPDATE SET
				status = EXCLUDED.status,
				event_id = EXCLUDED.event_id,
				updated_at = now()
		`, event.Order.OrderID, event.Order.UserID, event.Order.Status, event.Order.EventID)
		if err != nil {
			return err
		}
		if err := issuePendingWalletTicketsForOrder(ctx, tx, event.Order.OrderID); err != nil {
			return err
		}

		if event.Order.EventID != "" {
			organizerNotificationPayload, err := json.Marshal(map[string]any{
				"event_id": event.Order.EventID,
			})
			if err == nil {
				if _, err := tx.ExecContext(ctx, "SELECT pg_notify('vendor_updates', $1)", string(organizerNotificationPayload)); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

// issueOrBufferWalletTicket resolves the owner from the order projection, then
// creates a wallet ticket. If the order projection lags, the confirmation is
// buffered in the same transaction and drained when the order arrives.
func issueOrBufferWalletTicket(ctx context.Context, tx *sql.Tx, event Event) error {
	if event.CorrelationID == "" {
		return nil
	}
	var userID string
	err := tx.QueryRowContext(ctx, "SELECT user_id FROM projection.order_summaries WHERE order_id = $1", event.CorrelationID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO projection.pending_wallet_ticket_events (
				ticket_id, order_id, event_id, section_id, seat_id, aggregate_version
			)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (ticket_id) DO UPDATE SET
				order_id = EXCLUDED.order_id,
				event_id = EXCLUDED.event_id,
				section_id = EXCLUDED.section_id,
				seat_id = EXCLUDED.seat_id,
				aggregate_version = EXCLUDED.aggregate_version
		`, event.EventID, event.CorrelationID, event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID, event.AggregateVersion)
		return err
	}
	if err != nil {
		return err
	}

	return insertWalletTicket(ctx, tx, event.EventID, userID, event.CorrelationID, event.Seat.EventID, event.Seat.SectionID, event.Seat.SeatID, "ISSUED", event.AggregateVersion)
}

func issuePendingWalletTicketsForOrder(ctx context.Context, tx *sql.Tx, orderID string) error {
	if orderID == "" {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT p.ticket_id, p.event_id, p.section_id, p.seat_id, p.aggregate_version,
			os.user_id, COALESCE(ss.status, 'RESERVED')
		FROM projection.pending_wallet_ticket_events p
		JOIN projection.order_summaries os ON os.order_id = p.order_id
		LEFT JOIN projection.seat_snapshots ss
			ON ss.event_id = p.event_id AND ss.section_id = p.section_id AND ss.seat_id = p.seat_id
		WHERE p.order_id = $1
		ORDER BY p.created_at, p.ticket_id
	`, orderID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pendingTicket struct {
		ticketID         string
		eventID          string
		sectionID        string
		seatID           string
		userID           string
		seatStatus       string
		aggregateVersion int64
	}
	var pending []pendingTicket
	for rows.Next() {
		var ticket pendingTicket
		if err := rows.Scan(&ticket.ticketID, &ticket.eventID, &ticket.sectionID, &ticket.seatID, &ticket.aggregateVersion, &ticket.userID, &ticket.seatStatus); err != nil {
			return err
		}
		pending = append(pending, ticket)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, ticket := range pending {
		status := "ISSUED"
		if ticket.seatStatus == "CANCELLED" {
			status = "CANCELLED"
		}
		if err := insertWalletTicket(ctx, tx, ticket.ticketID, ticket.userID, orderID, ticket.eventID, ticket.sectionID, ticket.seatID, status, ticket.aggregateVersion); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM projection.pending_wallet_ticket_events WHERE ticket_id = $1", ticket.ticketID); err != nil {
			return err
		}
	}
	return nil
}

func insertWalletTicket(ctx context.Context, tx *sql.Tx, ticketID, userID, orderID, eventID, sectionID, seatID, status string, aggregateVersion int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO projection.wallet_tickets (ticket_id, user_id, order_id, event_id, section_id, seat_id, status, aggregate_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (ticket_id) DO UPDATE SET
			status = EXCLUDED.status,
			updated_at = now()
	`, ticketID, userID, orderID, eventID, sectionID, seatID, status, aggregateVersion)
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
