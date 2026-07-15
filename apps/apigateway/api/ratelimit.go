package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client   *redis.Client
	rate     float64
	capacity int
}

func NewRateLimiter(client *redis.Client, rate float64, capacity int) *RateLimiter {
	return &RateLimiter{client: client, rate: rate, capacity: capacity}
}

var tbScript = redis.NewScript(`
local tokens_key = KEYS[1]
local timestamp_key = KEYS[2]
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = 1

local ttl = math.floor((capacity/rate)*2)
if ttl < 1 then ttl = 1 end

local last_tokens = tonumber(redis.call("get", tokens_key))
if last_tokens == nil then
  last_tokens = capacity
end

local last_refreshed = tonumber(redis.call("get", timestamp_key))
if last_refreshed == nil then
  last_refreshed = 0
end

local delta = math.max(0, now-last_refreshed)
local filled_tokens = math.min(capacity, last_tokens+(delta*rate))
local allowed = filled_tokens >= requested
local new_tokens = filled_tokens
if allowed then
  new_tokens = filled_tokens - requested
end

redis.call("setex", tokens_key, ttl, new_tokens)
redis.call("setex", timestamp_key, ttl, now)

if allowed then
  return 1
end
return 0
`)

// allow checks a single token bucket identified by key. A nil client (no
// Redis configured) always allows, so rate limiting degrades open rather than
// blocking traffic when the soft dependency is unavailable.
func (rl *RateLimiter) allow(ctx context.Context, key string) (bool, error) {
	if rl.client == nil {
		return true, nil
	}
	keys := []string{"tb:tokens:" + key, "tb:ts:" + key}
	now := time.Now().UnixNano() / 1e9 // seconds

	res, err := tbScript.Run(ctx, rl.client, keys, rl.rate, rl.capacity, now).Result()
	if err != nil {
		return false, err
	}
	return res.(int64) != 0, nil
}

// Middleware enforces a single IP-keyed bucket ahead of authentication. Used
// for endpoints that don't require an authenticated user.
func (rl *RateLimiter) Middleware(endpoint string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowed, err := rl.allow(r.Context(), endpoint+":ip:"+clientIP(r))
		if err != nil {
			// Mutation rate limiting fails closed when Redis is degraded; keep
			// raw Redis details in server logs only.
			slog.Error("rate limiter unavailable", "error", err)
			writeError(w, http.StatusServiceUnavailable, "rate_limiter_unavailable")
			return
		}
		if !allowed {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"message": "Too Many Requests", "code": "rate_limited"})
			return
		}
		next.ServeHTTP(w, r)
	}
}

// AuthedMiddleware enforces endpoint-scoped IP and account buckets. It must
// run after authentication because account buckets need the user ID.
func (rl *RateLimiter) AuthedMiddleware(endpoint string, next func(http.ResponseWriter, *http.Request, User)) func(http.ResponseWriter, *http.Request, User) {
	return func(w http.ResponseWriter, r *http.Request, user User) {
		ipAllowed, err := rl.allow(r.Context(), endpoint+":ip:"+clientIP(r))
		if err != nil {
			// Mutation rate limiting fails closed when Redis is degraded; keep
			// raw Redis details in server logs only.
			slog.Error("rate limiter unavailable", "error", err)
			writeError(w, http.StatusServiceUnavailable, "rate_limiter_unavailable")
			return
		}
		if !ipAllowed {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"message": "Too Many Requests", "code": "rate_limited"})
			return
		}

		accountAllowed, err := rl.allow(r.Context(), endpoint+":account:"+user.ID)
		if err != nil {
			// Mutation rate limiting fails closed when Redis is degraded; keep
			// raw Redis details in server logs only.
			slog.Error("rate limiter unavailable", "error", err)
			writeError(w, http.StatusServiceUnavailable, "rate_limiter_unavailable")
			return
		}
		if !accountAllowed {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"message": "Too Many Requests", "code": "rate_limited"})
			return
		}

		next(w, r, user)
	}
}
