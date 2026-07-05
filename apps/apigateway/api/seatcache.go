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

// getSeatsCached fronts ListSeats with a short-TTL cache plus a Redis
// single-flight lock, per docs/infrastructure.md's cache-stampede control:
// only the lock holder refreshes the snapshot from Postgres; other
// concurrent requests for the same hot section fall back to the existing
// cached snapshot (marked with its real age) instead of all hammering the
// database at once. A nil cache client (soft dependency degraded/unavailable)
// falls back to reading straight through to the store.
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
		// Another worker already owns the refresh, or Redis errored on the
		// lock attempt; there's no cached snapshot to fall back to yet (a
		// cold miss), so read through directly rather than serving nothing.
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
