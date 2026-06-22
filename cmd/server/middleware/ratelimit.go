package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter enforces per-IP request limits using a Redis-backed fixed window.
type RateLimiter struct {
	client *redis.Client
	max    int
	window time.Duration
}

// NewRateLimiter creates a rate limiter backed by the given Redis client.
func NewRateLimiter(client *redis.Client, max int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client: client,
		max:    max,
		window: window,
	}
}

// Limit returns middleware that rejects requests when an IP exceeds the configured limit.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		allowed, remaining, retryAfter, err := rl.allow(r.Context(), ip)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.max))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			if retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			}
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(ctx context.Context, ip string) (allowed bool, remaining int, retryAfter int, err error) {
	if rl.client == nil {
		return false, 0, 0, fmt.Errorf("rate limiter: Redis client is nil")
	}

	windowStart := time.Now().Unix() / int64(rl.window.Seconds())
	key := fmt.Sprintf("ratelimit:%s:%d", ip, windowStart)

	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, 0, err
	}

	if count == 1 {
		if err := rl.client.Expire(ctx, key, rl.window).Err(); err != nil {
			return false, 0, 0, err
		}
	}

	remaining = rl.max - int(count)
	if remaining < 0 {
		remaining = 0
	}

	if count > int64(rl.max) {
		ttl, err := rl.client.TTL(ctx, key).Result()
		if err != nil {
			return false, 0, 0, err
		}
		if ttl > 0 {
			retryAfter = int(ttl.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
		}
		return false, 0, retryAfter, nil
	}

	return true, remaining, 0, nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
