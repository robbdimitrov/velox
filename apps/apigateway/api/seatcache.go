package api

import (
	"context"
	"encoding/json"
	"time"
)

const (
	seatSnapshotCacheTTL = 500 * time.Millisecond
	hotloadLockTTL       = 1 * time.Second
)

type seatCacheEntry struct {
	Seats    []Seat    `json:"seats"`
	CachedAt time.Time `json:"cached_at"`
}

// getSeatsCached uses a short-TTL snapshot cache plus a Redis single-flight
// lock; without Redis it reads through to the store.
func (s *Server) getSeatsCached(ctx context.Context, eventID, sectionID string) ([]Seat, int64, error) {
	if s.cacheClient == nil {
		return s.store.ListSeats(ctx, eventID, sectionID)
	}

	cacheKey := "seatsnapshot:" + eventID + ":" + sectionID
	if cached, err := s.cacheClient.Get(ctx, cacheKey).Result(); err == nil {
		var entry seatCacheEntry
		if json.Unmarshal([]byte(cached), &entry) == nil {
			return entry.Seats, time.Since(entry.CachedAt).Milliseconds(), nil
		}
	}

	lockKey := "hotload:" + eventID + ":" + sectionID
	acquired, err := s.cacheClient.SetNX(ctx, lockKey, s.workerID, hotloadLockTTL).Result()
	if err != nil || !acquired {
		// Cold miss with no refresh lock; read through rather than serve nothing.
		return s.store.ListSeats(ctx, eventID, sectionID)
	}

	seats, snapshotAgeMS, err := s.store.ListSeats(ctx, eventID, sectionID)
	if err != nil {
		return nil, 0, err
	}
	entry := seatCacheEntry{Seats: seats, CachedAt: time.Now()}
	if buf, marshalErr := json.Marshal(entry); marshalErr == nil {
		s.cacheClient.Set(ctx, cacheKey, buf, seatSnapshotCacheTTL)
	}
	return seats, snapshotAgeMS, nil
}
