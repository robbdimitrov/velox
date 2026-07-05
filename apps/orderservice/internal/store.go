package internal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrIdempotencyConflict = errors.New("idempotency key conflict")

type Store struct {
	db *sql.DB
}

func NewStore(dbURL string) (*Store, error) {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) CreateOrder(ctx context.Context, req OrderRequest) (string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	sort.Strings(req.SeatIDs)
	reqBytes, _ := json.Marshal(req)
	hash := sha256.Sum256(reqBytes)

	var existingHash []byte
	var responseRef sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT request_hash, response_ref 
		FROM orders.idempotency_keys 
		WHERE service = 'orderservice' AND user_id = $1 AND idempotency_key = $2
	`, req.UserID, req.IdempotencyKey).Scan(&existingHash, &responseRef)

	if err == nil {
		if !bytes.Equal(existingHash, hash[:]) {
			return "", ErrIdempotencyConflict
		}
		if responseRef.Valid {
			return responseRef.String, nil
		}
		return "", ErrIdempotencyConflict
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.idempotency_keys (service, user_id, idempotency_key, request_hash, expires_at)
		VALUES ('orderservice', $1, $2, $3, $4)
	`, req.UserID, req.IdempotencyKey, hash[:], time.Now().Add(24*time.Hour))
	if err != nil {
		return "", err
	}

	orderID := uuid.New().String()

	var total int64 = 0

	type seatInfo struct {
		SeatID     string
		PriceMinor int64
	}
	var seats []seatInfo
	for _, seatID := range req.SeatIDs {
		var price int64
		err := tx.QueryRowContext(ctx, `SELECT price_amount_minor FROM projection.seat_snapshots WHERE event_id = $1 AND section_id = $2 AND seat_id = $3`, req.EventID, req.SectionID, seatID).Scan(&price)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		seats = append(seats, seatInfo{SeatID: seatID, PriceMinor: price})
		total += price
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.orders (id, user_id, status, idempotency_key, request_hash, total_amount_minor)
		VALUES ($1, $2, 'PENDING', $3, $4, $5)
	`, orderID, req.UserID, req.IdempotencyKey, hash[:], total)
	if err != nil {
		return "", err
	}

	for _, seat := range seats {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO orders.order_seats (order_id, event_id, section_id, seat_id, price_amount_minor)
			VALUES ($1, $2, $3, $4, $5)
		`, orderID, req.EventID, req.SectionID, seat.SeatID, seat.PriceMinor)
		if err != nil {
			return "", err
		}
	}

	eventID := uuid.New().String()
	envelope := map[string]any{
		"Type": "OrderCreated",
		"Order": map[string]any{
			"outbox_event_id":    eventID,
			"order_id":           orderID,
			"user_id":            req.UserID,
			"event_id":           req.EventID,
			"section_id":         req.SectionID,
			"seat_ids":           req.SeatIDs,
			"reservation_id":     orderID,
			"status":             "PENDING",
			"total_amount_minor": total,
			"created_at":         time.Now(),
		},
	}
	payloadBytes, _ := json.Marshal(envelope)

	headers := map[string]string{}
	// Since we are in orderservice package 'internal', we don't have access to 'main.RequestIDKey'
	// Wait, I will use a string key or define it in 'internal'
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		headers["X-Request-ID"] = reqID
	}
	headersBytes, _ := json.Marshal(headers)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders.outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers)
		VALUES ($1, 'order', $2, 'OrderCreated', $3, $4)
	`, eventID, orderID, payloadBytes, headersBytes)
	if err != nil {
		return "", err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE orders.idempotency_keys 
		SET response_ref = $1 
		WHERE service = 'orderservice' AND user_id = $2 AND idempotency_key = $3
	`, orderID, req.UserID, req.IdempotencyKey)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return orderID, nil
}
