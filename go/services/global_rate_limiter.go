package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// GlobalRateLimiter enforces API-wide per-IP request limits.
type GlobalRateLimiter interface {
	Check(ctx context.Context, ip string) (RateLimitResult, error)
	RateLimitHits() uint64
}

// NewGlobalRateLimiter creates an API-wide per-IP limiter.
// If redisURL is provided, Redis is used (recommended for production).
// If redisURL is empty, an in-memory fallback is used.
// If redisURL is set but Redis is temporarily unavailable at startup, a Redis
// limiter is still returned and requests fail open until Redis is reachable.
func NewGlobalRateLimiter(redisURL string, limit int, window time.Duration) GlobalRateLimiter {
	if limit <= 0 || window <= 0 {
		slog.Warn("Global rate limiter disabled due to non-positive limit/window")
		return NewNoOpGlobalRateLimiter()
	}

	if redisURL != "" {
		limiter, err := NewRedisGlobalRateLimiterFromURL(redisURL, limit, window)
		if err != nil {
			slog.Error("Failed to create Redis global rate limiter, falling back to in-memory", "error", err)
			return newInMemoryGlobalRateLimiterWithWarning(limit, window)
		}
		slog.Info("Using Redis global rate limiter", "limit", limit, "window", window.String())
		return limiter
	}

	return newInMemoryGlobalRateLimiterWithWarning(limit, window)
}

func newInMemoryGlobalRateLimiterWithWarning(limit int, window time.Duration) *InMemoryGlobalRateLimiter {
	slog.Warn("Using in-memory global rate limiter — not suitable for multi-instance deployments")
	return NewInMemoryGlobalRateLimiter(limit, window)
}

// NewNoOpGlobalRateLimiter returns a limiter that never blocks requests.
func NewNoOpGlobalRateLimiter() GlobalRateLimiter {
	return NoOpGlobalRateLimiter{}
}

// -----------------------------------------------------------------------
// Redis implementation
// -----------------------------------------------------------------------

type RedisGlobalRateLimiter struct {
	client *redis.Client
	now    func() time.Time

	limit  int
	window time.Duration

	rateScript *redis.Script
	hits       atomic.Uint64
}

func NewRedisGlobalRateLimiter(client *redis.Client, limit int, window time.Duration) *RedisGlobalRateLimiter {
	return &RedisGlobalRateLimiter{
		client:     client,
		now:        time.Now,
		limit:      limit,
		window:     window,
		rateScript: redis.NewScript(redisSlidingWindowScript),
	}
}

func NewRedisGlobalRateLimiterFromURL(redisURL string, limit int, window time.Duration) (*RedisGlobalRateLimiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opts)
	if pingErr := client.Ping(context.Background()).Err(); pingErr != nil {
		slog.Warn("Redis global rate limiter unreachable at startup; limiter will fail open until Redis becomes available", "error", pingErr)
	}

	return NewRedisGlobalRateLimiter(client, limit, window), nil
}

func (l *RedisGlobalRateLimiter) Check(ctx context.Context, ip string) (RateLimitResult, error) {
	now := l.now()
	key := fmt.Sprintf("rate:global:ip:%s", hashKeyPart(ip))

	result, err := l.rateScript.Run(ctx, l.client, []string{key}, now.UnixNano(), l.window.Nanoseconds(), l.limit).Result()
	if err != nil {
		return RateLimitResult{}, err
	}

	slice, ok := result.([]any)
	if !ok || len(slice) < 3 {
		return RateLimitResult{}, fmt.Errorf("unexpected redis rate limiter result: %T", result)
	}

	allowed := asInt64(slice[0]) == 1
	count := int(asInt64(slice[1]))
	resetAtNs := asInt64(slice[2])

	remaining := max(l.limit-count, 0)
	resetAt := now.Add(l.window)
	if resetAtNs > 0 {
		resetAt = time.Unix(0, resetAtNs)
	}

	if !allowed {
		l.hits.Add(1)
	}

	return RateLimitResult{
		Allowed:   allowed,
		Limit:     l.limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}

func (l *RedisGlobalRateLimiter) RateLimitHits() uint64 {
	return l.hits.Load()
}

// -----------------------------------------------------------------------
// In-memory implementation
// -----------------------------------------------------------------------

type InMemoryGlobalRateLimiter struct {
	now func() time.Time

	limit   int
	window  time.Duration
	mu      sync.Mutex
	windows map[string][]time.Time
	hits    atomic.Uint64
	stopCh  chan struct{}
}

func NewInMemoryGlobalRateLimiter(limit int, window time.Duration) *InMemoryGlobalRateLimiter {
	lim := &InMemoryGlobalRateLimiter{
		now:     time.Now,
		limit:   limit,
		window:  window,
		windows: make(map[string][]time.Time),
		stopCh:  make(chan struct{}),
	}
	lim.startCleanup()
	return lim
}

func (l *InMemoryGlobalRateLimiter) Check(_ context.Context, ip string) (RateLimitResult, error) {
	now := l.now()
	cutoff := now.Add(-l.window)
	key := fmt.Sprintf("rate:global:ip:%s", hashKeyPart(ip))

	l.mu.Lock()
	defer l.mu.Unlock()

	ts := l.windows[key]
	keep := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	ts = keep

	if len(ts) == 0 {
		delete(l.windows, key)
	}

	if len(ts) >= l.limit {
		l.windows[key] = ts
		l.hits.Add(1)
		return RateLimitResult{
			Allowed:   false,
			Limit:     l.limit,
			Remaining: 0,
			ResetAt:   ts[0].Add(l.window),
		}, nil
	}

	ts = append(ts, now)
	l.windows[key] = ts

	return RateLimitResult{
		Allowed:   true,
		Limit:     l.limit,
		Remaining: max(l.limit-len(ts), 0),
		ResetAt:   ts[0].Add(l.window),
	}, nil
}

func (l *InMemoryGlobalRateLimiter) RateLimitHits() uint64 {
	return l.hits.Load()
}

// startCleanup launches a background goroutine that periodically removes
// expired per-IP windows. The goroutine exits when Stop is called.
func (l *InMemoryGlobalRateLimiter) startCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.cleanup()
			case <-l.stopCh:
				return
			}
		}
	}()
}

// Stop stops the background cleanup goroutine. It must be called exactly once
// when the limiter is no longer needed to avoid goroutine leaks.
func (l *InMemoryGlobalRateLimiter) Stop() {
	close(l.stopCh)
}

func (l *InMemoryGlobalRateLimiter) cleanup() {
	cutoff := l.now().Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()

	for key, ts := range l.windows {
		if len(ts) == 0 || !ts[len(ts)-1].After(cutoff) {
			delete(l.windows, key)
		}
	}
}

// -----------------------------------------------------------------------
// No-op implementation
// -----------------------------------------------------------------------

type NoOpGlobalRateLimiter struct{}

func (NoOpGlobalRateLimiter) Check(_ context.Context, _ string) (RateLimitResult, error) {
	return RateLimitResult{
		Allowed:   true,
		Limit:     0,
		Remaining: 0,
		ResetAt:   time.Now(),
	}, nil
}

func (NoOpGlobalRateLimiter) RateLimitHits() uint64 {
	return 0
}
