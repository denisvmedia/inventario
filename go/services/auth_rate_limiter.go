package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	loginAttemptsLimit  = 5
	loginAttemptsWindow = 15 * time.Minute

	registrationAttemptsLimit  = 3
	registrationAttemptsWindow = time.Hour

	passwordResetAttemptsLimit  = 3
	passwordResetAttemptsWindow = time.Hour

	failedLoginLockoutThreshold = 5
	failedLoginWindow           = 15 * time.Minute
	accountLockoutDuration      = 15 * time.Minute
)

// RateLimitResult describes the decision of a rate limiting check.
// ResetAt is the time when a new request is expected to be allowed again.
type RateLimitResult struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// AuthRateLimiter provides rate limiting and lockout protection for auth flows.
//
// Design goals:
//   - Sliding-window rate limiting (not token bucket) to match Issue #836.
//   - Per-email account lockout to mitigate distributed brute force.
//   - Fail-open behavior on backend errors (Redis outage must not take API down).
type AuthRateLimiter interface {
	CheckLoginAttempt(ctx context.Context, ip string) (RateLimitResult, error)
	CheckRegistrationAttempt(ctx context.Context, ip string) (RateLimitResult, error)
	CheckPasswordResetAttempt(ctx context.Context, email string) (RateLimitResult, error)

	IsAccountLocked(ctx context.Context, email string) (locked bool, resetAt time.Time, err error)
	RecordFailedLogin(ctx context.Context, email string) (locked bool, resetAt time.Time, err error)
	ClearFailedLogins(ctx context.Context, email string) error
}

// NewAuthRateLimiter creates an AuthRateLimiter.
// If redisURL is provided, a Redis-backed limiter is used (recommended for production).
// Otherwise, an in-memory limiter is used (suitable only for single-instance deployments).
func NewAuthRateLimiter(redisURL string) AuthRateLimiter {
	if redisURL != "" {
		lim, err := NewRedisAuthRateLimiterFromURL(redisURL)
		if err != nil {
			slog.Error("Failed to create Redis auth rate limiter, falling back to in-memory", "error", err)
			return newInMemoryAuthRateLimiterWithWarning()
		}
		slog.Info("Using Redis auth rate limiter")
		return lim
	}
	return newInMemoryAuthRateLimiterWithWarning()
}

func newInMemoryAuthRateLimiterWithWarning() *InMemoryAuthRateLimiter {
	slog.Warn("Using in-memory auth rate limiter — not suitable for multi-instance deployments; set --auth-rate-limit-redis-url for production")
	return NewInMemoryAuthRateLimiter()
}

// NewNoOpAuthRateLimiter returns a limiter that always allows requests and never locks accounts.
func NewNoOpAuthRateLimiter() AuthRateLimiter {
	return NoOpAuthRateLimiter{}
}

func hashKeyPart(s string) string {
	n := strings.TrimSpace(strings.ToLower(s))
	h := sha256.Sum256([]byte(n))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// -----------------------------------------------------------------------
// Redis implementation
// -----------------------------------------------------------------------

type RedisAuthRateLimiter struct {
	client *redis.Client
	now    func() time.Time

	rateScript        *redis.Script
	failedLoginScript *redis.Script
}

func NewRedisAuthRateLimiter(client *redis.Client) *RedisAuthRateLimiter {
	return &RedisAuthRateLimiter{
		client:            client,
		now:               time.Now,
		rateScript:        redis.NewScript(redisSlidingWindowScript),
		failedLoginScript: redis.NewScript(redisFailedLoginScript),
	}
}

// NewRedisAuthRateLimiterFromURL creates a Redis-backed limiter from a URL.
// Startup connectivity is checked with PING; failures are logged as warnings,
// but the limiter is still returned to keep startup consistent with fail-open design.
func NewRedisAuthRateLimiterFromURL(redisURL string) (*RedisAuthRateLimiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	client := redis.NewClient(opts)
	if pingErr := client.Ping(context.Background()).Err(); pingErr != nil {
		slog.Warn("Redis auth rate limiter unreachable at startup; rate limiting will fail open until Redis becomes available", "error", pingErr)
	}
	return NewRedisAuthRateLimiter(client), nil
}

func (l *RedisAuthRateLimiter) CheckLoginAttempt(ctx context.Context, ip string) (RateLimitResult, error) {
	key := fmt.Sprintf("rate:auth:login:ip:%s", hashKeyPart(ip))
	return l.checkRate(ctx, key, loginAttemptsLimit, loginAttemptsWindow)
}

func (l *RedisAuthRateLimiter) CheckRegistrationAttempt(ctx context.Context, ip string) (RateLimitResult, error) {
	key := fmt.Sprintf("rate:auth:register:ip:%s", hashKeyPart(ip))
	return l.checkRate(ctx, key, registrationAttemptsLimit, registrationAttemptsWindow)
}

func (l *RedisAuthRateLimiter) CheckPasswordResetAttempt(ctx context.Context, email string) (RateLimitResult, error) {
	key := fmt.Sprintf("rate:auth:reset:email:%s", hashKeyPart(email))
	return l.checkRate(ctx, key, passwordResetAttemptsLimit, passwordResetAttemptsWindow)
}

func (l *RedisAuthRateLimiter) checkRate(ctx context.Context, key string, limit int, window time.Duration) (RateLimitResult, error) {
	now := l.now()
	result, err := l.rateScript.Run(ctx, l.client, []string{key}, now.UnixNano(), window.Nanoseconds(), limit).Result()
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

	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	resetAt := now.Add(window)
	if resetAtNs > 0 {
		resetAt = time.Unix(0, resetAtNs)
	}

	return RateLimitResult{
		Allowed:   allowed,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}

func (l *RedisAuthRateLimiter) IsAccountLocked(ctx context.Context, email string) (bool, time.Time, error) {
	key := fmt.Sprintf("auth:lockout:%s", hashKeyPart(email))
	ttl, err := l.client.PTTL(ctx, key).Result()
	if err != nil {
		return false, time.Time{}, err
	}
	if ttl <= 0 {
		return false, time.Time{}, nil
	}
	return true, l.now().Add(ttl), nil
}

func (l *RedisAuthRateLimiter) RecordFailedLogin(ctx context.Context, email string) (bool, time.Time, error) {
	failedKey := fmt.Sprintf("auth:failed-login:%s", hashKeyPart(email))
	lockKey := fmt.Sprintf("auth:lockout:%s", hashKeyPart(email))

	result, err := l.failedLoginScript.Run(
		ctx,
		l.client,
		[]string{failedKey, lockKey},
		int64(failedLoginWindow/time.Millisecond),
		int64(accountLockoutDuration/time.Millisecond),
		failedLoginLockoutThreshold,
	).Result()
	if err != nil {
		return false, time.Time{}, err
	}

	slice, ok := result.([]any)
	if !ok || len(slice) < 2 {
		return false, time.Time{}, fmt.Errorf("unexpected redis failed-login result: %T", result)
	}

	locked := asInt64(slice[0]) == 1
	ttlMs := asInt64(slice[1])
	if ttlMs <= 0 {
		// Best-effort fallback.
		return locked, l.now().Add(accountLockoutDuration), nil
	}
	return locked, l.now().Add(time.Duration(ttlMs) * time.Millisecond), nil
}

func (l *RedisAuthRateLimiter) ClearFailedLogins(ctx context.Context, email string) error {
	failedKey := fmt.Sprintf("auth:failed-login:%s", hashKeyPart(email))
	lockKey := fmt.Sprintf("auth:lockout:%s", hashKeyPart(email))
	return l.client.Del(ctx, failedKey, lockKey).Err()
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}

const redisSlidingWindowScript = `
-- Sliding-window rate limiter.
--
-- KEYS[1] = zset key
-- ARGV[1] = now (ns)
-- ARGV[2] = window (ns)
-- ARGV[3] = limit (int)
--
-- Returns: {allowed(0/1), count_after, reset_at_ns}

local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

local window_start = now - window

redis.call('ZREMRANGEBYSCORE', key, 0, window_start)
local count = redis.call('ZCARD', key)

local oldest = nil
if count > 0 then
  local oldest_with_score = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  oldest = tonumber(oldest_with_score[2])
end

if count >= limit then
  local reset_at = (oldest or now) + window
  return {0, count, reset_at}
end

local seq_key = key .. ':seq'
local seq = redis.call('INCR', seq_key)
local member = tostring(now) .. ':' .. tostring(seq)

redis.call('ZADD', key, now, member)
-- We only need to retain data for up to the window.
local ttl_ms = math.ceil(window / 1000000)
redis.call('PEXPIRE', key, ttl_ms)
redis.call('PEXPIRE', seq_key, ttl_ms)

count = count + 1
if oldest == nil then
  oldest = now
end

local reset_at = oldest + window
return {1, count, reset_at}
`

const redisFailedLoginScript = `
-- Failed login tracking + account lockout.
--
-- KEYS[1] = failed counter key
-- KEYS[2] = lockout key
-- ARGV[1] = failed window (ms)
-- ARGV[2] = lockout duration (ms)
-- ARGV[3] = threshold (int)
--
-- Returns: {locked(0/1), ttl_ms}

local failed_key = KEYS[1]
local lock_key = KEYS[2]
local failed_window_ms = tonumber(ARGV[1])
local lockout_ms = tonumber(ARGV[2])
local threshold = tonumber(ARGV[3])

if redis.call('EXISTS', lock_key) == 1 then
  local ttl = redis.call('PTTL', lock_key)
  return {1, ttl}
end

local val = redis.call('INCR', failed_key)
if val == 1 then
  redis.call('PEXPIRE', failed_key, failed_window_ms)
end

if val >= threshold then
  redis.call('SET', lock_key, '1', 'PX', lockout_ms)
  redis.call('DEL', failed_key)
  return {1, lockout_ms}
end

local ttl = redis.call('PTTL', failed_key)
return {0, ttl}
`

// -----------------------------------------------------------------------
// In-memory implementation
// -----------------------------------------------------------------------

type InMemoryAuthRateLimiter struct {
	now func() time.Time

	mu      sync.Mutex
	windows map[string][]time.Time
	failed  map[string]*inMemoryFailedLogin
}

type inMemoryFailedLogin struct {
	count       int
	expiresAt   time.Time
	lockedUntil time.Time
}

func NewInMemoryAuthRateLimiter() *InMemoryAuthRateLimiter {
	return &InMemoryAuthRateLimiter{
		now:     time.Now,
		windows: make(map[string][]time.Time),
		failed:  make(map[string]*inMemoryFailedLogin),
	}
}

func (l *InMemoryAuthRateLimiter) CheckLoginAttempt(_ context.Context, ip string) (RateLimitResult, error) {
	key := fmt.Sprintf("rate:auth:login:ip:%s", hashKeyPart(ip))
	return l.checkRate(key, loginAttemptsLimit, loginAttemptsWindow), nil
}

func (l *InMemoryAuthRateLimiter) CheckRegistrationAttempt(_ context.Context, ip string) (RateLimitResult, error) {
	key := fmt.Sprintf("rate:auth:register:ip:%s", hashKeyPart(ip))
	return l.checkRate(key, registrationAttemptsLimit, registrationAttemptsWindow), nil
}

func (l *InMemoryAuthRateLimiter) CheckPasswordResetAttempt(_ context.Context, email string) (RateLimitResult, error) {
	key := fmt.Sprintf("rate:auth:reset:email:%s", hashKeyPart(email))
	return l.checkRate(key, passwordResetAttemptsLimit, passwordResetAttemptsWindow), nil
}

func (l *InMemoryAuthRateLimiter) checkRate(key string, limit int, window time.Duration) RateLimitResult {
	now := l.now()
	cutoff := now.Add(-window)

	l.mu.Lock()
	defer l.mu.Unlock()

	ts := l.windows[key]
	// prune
	keep := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	ts = keep

	// Evict the key when all timestamps have fallen outside the window to prevent
	// unbounded map growth under many distinct client IPs.
	if len(ts) == 0 {
		delete(l.windows, key)
	}

	if len(ts) >= limit {
		oldest := ts[0]
		l.windows[key] = ts
		return RateLimitResult{
			Allowed:   false,
			Limit:     limit,
			Remaining: 0,
			ResetAt:   oldest.Add(window),
		}
	}

	ts = append(ts, now)
	l.windows[key] = ts

	remaining := limit - len(ts)
	if remaining < 0 {
		remaining = 0
	}

	return RateLimitResult{
		Allowed:   true,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   ts[0].Add(window),
	}
}

func (l *InMemoryAuthRateLimiter) IsAccountLocked(_ context.Context, email string) (bool, time.Time, error) {
	key := fmt.Sprintf("auth:lockout:%s", hashKeyPart(email))

	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.failed[key]
	if entry == nil {
		return false, time.Time{}, nil
	}

	// Active lockout.
	if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
		return true, entry.lockedUntil, nil
	}

	// Lockout expired: evict to prevent unbounded growth.
	if !entry.lockedUntil.IsZero() {
		delete(l.failed, key)
		return false, time.Time{}, nil
	}

	// No lockout recorded. If the failure window has also expired, evict.
	if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
		delete(l.failed, key)
	}

	return false, time.Time{}, nil
}

func (l *InMemoryAuthRateLimiter) RecordFailedLogin(_ context.Context, email string) (bool, time.Time, error) {
	key := fmt.Sprintf("auth:lockout:%s", hashKeyPart(email))

	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.failed[key]
	if entry == nil {
		entry = &inMemoryFailedLogin{}
		l.failed[key] = entry
	}

	// If lockout is active, keep it.
	if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
		return true, entry.lockedUntil, nil
	}

	// Reset window if expired.
	if entry.expiresAt.IsZero() || now.After(entry.expiresAt) {
		entry.count = 0
		entry.expiresAt = now.Add(failedLoginWindow)
		entry.lockedUntil = time.Time{}
	}

	entry.count++
	if entry.count >= failedLoginLockoutThreshold {
		entry.lockedUntil = now.Add(accountLockoutDuration)
		return true, entry.lockedUntil, nil
	}

	return false, entry.expiresAt, nil
}

func (l *InMemoryAuthRateLimiter) ClearFailedLogins(_ context.Context, email string) error {
	key := fmt.Sprintf("auth:lockout:%s", hashKeyPart(email))
	l.mu.Lock()
	delete(l.failed, key)
	l.mu.Unlock()
	return nil
}

// -----------------------------------------------------------------------
// No-op implementation
// -----------------------------------------------------------------------

type NoOpAuthRateLimiter struct{}

func (NoOpAuthRateLimiter) CheckLoginAttempt(_ context.Context, _ string) (RateLimitResult, error) {
	return RateLimitResult{Allowed: true, Limit: loginAttemptsLimit, Remaining: loginAttemptsLimit, ResetAt: time.Now().Add(loginAttemptsWindow)}, nil
}

func (NoOpAuthRateLimiter) CheckRegistrationAttempt(_ context.Context, _ string) (RateLimitResult, error) {
	return RateLimitResult{Allowed: true, Limit: registrationAttemptsLimit, Remaining: registrationAttemptsLimit, ResetAt: time.Now().Add(registrationAttemptsWindow)}, nil
}

func (NoOpAuthRateLimiter) CheckPasswordResetAttempt(_ context.Context, _ string) (RateLimitResult, error) {
	return RateLimitResult{Allowed: true, Limit: passwordResetAttemptsLimit, Remaining: passwordResetAttemptsLimit, ResetAt: time.Now().Add(passwordResetAttemptsWindow)}, nil
}

func (NoOpAuthRateLimiter) IsAccountLocked(_ context.Context, _ string) (bool, time.Time, error) {
	return false, time.Time{}, nil
}

func (NoOpAuthRateLimiter) RecordFailedLogin(_ context.Context, _ string) (bool, time.Time, error) {
	return false, time.Time{}, nil
}

func (NoOpAuthRateLimiter) ClearFailedLogins(_ context.Context, _ string) error {
	return nil
}
