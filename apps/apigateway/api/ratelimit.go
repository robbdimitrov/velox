package api

import (
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

func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if rl.client == nil {
			next.ServeHTTP(w, r)
			return
		}

		ip := r.RemoteAddr
		if ip == "" {
			ip = "unknown"
		}

		keys := []string{"tb:tokens:" + ip, "tb:ts:" + ip}
		now := time.Now().UnixNano() / 1e9 // seconds

		res, err := tbScript.Run(r.Context(), rl.client, keys, rl.rate, rl.capacity, now).Result()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		if res.(int64) == 0 {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"message": "Too Many Requests", "code": "rate_limited"})
			return
		}

		next.ServeHTTP(w, r)
	}
}
