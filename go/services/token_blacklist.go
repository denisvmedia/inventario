package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBlacklister defines the interface for token blacklisting operations.
// It supports per-token revocation (using the JWT JTI claim) and per-user revocation
// (to force-logout all devices for a user simultaneously).
type TokenBlacklister interface {
	// BlacklistToken adds an access token (by its JTI) to the blacklist until the given expiry.
	BlacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error

	// IsBlacklisted reports whether the given token JTI is blacklisted.
	IsBlacklisted(ctx context.Context, tokenID string) (bool, error)

	// BlacklistUserTokens blacklists all tokens for a user for the given duration.
	// This can be used to force-logout across all devices when a password changes.
	BlacklistUserTokens(ctx context.Context, userID string, duration time.Duration) error

	// IsUserBlacklisted reports whether all tokens for the given user have been revoked.
	IsUserBlacklisted(ctx context.Context, userID string) (bool, error)

	// UserBlacklistedSince returns the time at which the user's tokens were revoked
	// and a boolean indicating whether a blacklist entry exists. If no entry exists
	// the returned time is the zero value and the boolean is false.
	// Use this instead of IsUserBlacklisted when you need to compare against a
	// token's issued-at (iat) timestamp to allow re-authentication after a password
	// change: reject only tokens whose iat is before the returned timestamp.
	UserBlacklistedSince(ctx context.Context, userID string) (time.Time, bool, error)

	// UnblacklistUser removes the user-level blacklist entry. This may be used by
	// administrators to explicitly unlock a user before the blacklist TTL expires.
	UnblacklistUser(ctx context.Context, userID string) error
}

// -----------------------------------------------------------------------
// Redis implementation (recommended for production / multi-instance deployments)
// -----------------------------------------------------------------------

// RedisTokenBlacklister implements TokenBlacklister using Redis.
// Entries automatically expire via Redis TTL, so no manual cleanup is needed.
// Use this in production when running more than one server instance.
type RedisTokenBlacklister struct {
	client *redis.Client
}

// NewRedisTokenBlacklister creates a new Redis-backed token blacklist.
func NewRedisTokenBlacklister(client *redis.Client) *RedisTokenBlacklister {
	return &RedisTokenBlacklister{client: client}
}

// NewRedisTokenBlacklisterFromURL creates a new Redis-backed token blacklist from a URL.
// A connectivity check (PING) is performed at construction time. If Redis is unreachable
// the error is logged as a warning but the blacklister is still returned — consistent with
// the fail-open design where a Redis outage must not take the API offline.
func NewRedisTokenBlacklisterFromURL(redisURL string) (*RedisTokenBlacklister, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	client := redis.NewClient(opts)
	if pingErr := client.Ping(context.Background()).Err(); pingErr != nil {
		slog.Warn("Redis token blacklist unreachable at startup; blacklisting will fail open until Redis becomes available", "error", pingErr)
	}
	return NewRedisTokenBlacklister(client), nil
}

func (s *RedisTokenBlacklister) BlacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil // already expired, nothing to do
	}
	key := fmt.Sprintf("blacklist:token:%s", tokenID)
	return s.client.Set(ctx, key, "1", ttl).Err()
}

func (s *RedisTokenBlacklister) IsBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	key := fmt.Sprintf("blacklist:token:%s", tokenID)
	n, err := s.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (s *RedisTokenBlacklister) BlacklistUserTokens(ctx context.Context, userID string, duration time.Duration) error {
	if duration <= 0 {
		// A zero or negative duration would create a Redis entry with no TTL,
		// permanently blacklisting the user. Treat this as a no-op instead.
		return nil
	}
	key := fmt.Sprintf("blacklist:user:%s", userID)
	// Store the Unix timestamp (seconds) so callers can compare against token iat claims.
	return s.client.Set(ctx, key, time.Now().Unix(), duration).Err()
}

func (s *RedisTokenBlacklister) IsUserBlacklisted(ctx context.Context, userID string) (bool, error) {
	_, blacklisted, err := s.UserBlacklistedSince(ctx, userID)
	return blacklisted, err
}

func (s *RedisTokenBlacklister) UserBlacklistedSince(ctx context.Context, userID string) (time.Time, bool, error) {
	key := fmt.Sprintf("blacklist:user:%s", userID)
	val, err := s.client.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	// Legacy entries written by older versions stored "1" as the blacklist marker.
	// A Unix timestamp of 1 (Jan 1970) is unreasonably small and indicates such a
	// legacy entry. Treat it conservatively as "revoke all tokens" by returning the
	// current time, so the iat-based check rejects all currently-valid tokens until
	// the entry expires or is removed.
	const minReasonableUnixTimestamp = int64(1_000_000_000) // ~Sep 2001
	if val < minReasonableUnixTimestamp {
		return time.Now(), true, nil
	}
	return time.Unix(val, 0), true, nil
}

func (s *RedisTokenBlacklister) UnblacklistUser(ctx context.Context, userID string) error {
	key := fmt.Sprintf("blacklist:user:%s", userID)
	return s.client.Del(ctx, key).Err()
}

// -----------------------------------------------------------------------
// In-memory implementation (for development / single-instance deployments)
// -----------------------------------------------------------------------

type blacklistEntry struct {
	expiresAt time.Time
	since     time.Time // when the user was blacklisted; zero for per-token entries
}

// InMemoryTokenBlacklister implements TokenBlacklister using an in-process map with TTL.
// It is thread-safe but does NOT persist across process restarts and does NOT share
// state between multiple server instances.
type InMemoryTokenBlacklister struct {
	mu       sync.RWMutex
	tokens   map[string]blacklistEntry // keyed by JTI
	users    map[string]blacklistEntry // keyed by userID
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewInMemoryTokenBlacklister creates a new in-memory token blacklist.
func NewInMemoryTokenBlacklister() *InMemoryTokenBlacklister {
	bl := &InMemoryTokenBlacklister{
		tokens: make(map[string]blacklistEntry),
		users:  make(map[string]blacklistEntry),
		stopCh: make(chan struct{}),
	}
	go bl.cleanupLoop()
	return bl
}

// Stop terminates the background cleanup goroutine. It is safe to call multiple times.
func (s *InMemoryTokenBlacklister) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

func (s *InMemoryTokenBlacklister) BlacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error {
	if time.Until(expiresAt) <= 0 {
		return nil
	}
	s.mu.Lock()
	s.tokens[tokenID] = blacklistEntry{expiresAt: expiresAt}
	s.mu.Unlock()
	return nil
}

func (s *InMemoryTokenBlacklister) IsBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	s.mu.RLock()
	entry, ok := s.tokens[tokenID]
	s.mu.RUnlock()
	if !ok {
		return false, nil
	}
	if time.Now().After(entry.expiresAt) {
		// Lazily delete expired entry
		s.mu.Lock()
		delete(s.tokens, tokenID)
		s.mu.Unlock()
		return false, nil
	}
	return true, nil
}

func (s *InMemoryTokenBlacklister) BlacklistUserTokens(ctx context.Context, userID string, duration time.Duration) error {
	now := time.Now()
	s.mu.Lock()
	s.users[userID] = blacklistEntry{expiresAt: now.Add(duration), since: time.Unix(now.Unix(), 0)}
	s.mu.Unlock()
	return nil
}

func (s *InMemoryTokenBlacklister) IsUserBlacklisted(ctx context.Context, userID string) (bool, error) {
	_, blacklisted, err := s.UserBlacklistedSince(ctx, userID)
	return blacklisted, err
}

func (s *InMemoryTokenBlacklister) UserBlacklistedSince(ctx context.Context, userID string) (time.Time, bool, error) {
	s.mu.RLock()
	entry, ok := s.users[userID]
	s.mu.RUnlock()
	if !ok {
		return time.Time{}, false, nil
	}
	if time.Now().After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.users, userID)
		s.mu.Unlock()
		return time.Time{}, false, nil
	}
	return entry.since, true, nil
}

func (s *InMemoryTokenBlacklister) UnblacklistUser(_ context.Context, userID string) error {
	s.mu.Lock()
	delete(s.users, userID)
	s.mu.Unlock()
	return nil
}

// cleanupLoop periodically removes expired entries to prevent unbounded memory growth.
// It exits when Stop() is called, preventing goroutine leaks in test environments.
func (s *InMemoryTokenBlacklister) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for k, v := range s.tokens {
				if now.After(v.expiresAt) {
					delete(s.tokens, k)
				}
			}
			for k, v := range s.users {
				if now.After(v.expiresAt) {
					delete(s.users, k)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

// -----------------------------------------------------------------------
// No-op implementation (blacklisting disabled)
// -----------------------------------------------------------------------

// NoOpTokenBlacklister implements TokenBlacklister as a no-op. Tokens are never
// considered blacklisted. Suitable only for development environments where immediate
// revocation is not required; short-lived access tokens (15 min) limit exposure.
type NoOpTokenBlacklister struct{}

func (NoOpTokenBlacklister) BlacklistToken(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (NoOpTokenBlacklister) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (NoOpTokenBlacklister) BlacklistUserTokens(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (NoOpTokenBlacklister) IsUserBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (NoOpTokenBlacklister) UserBlacklistedSince(_ context.Context, _ string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (NoOpTokenBlacklister) UnblacklistUser(_ context.Context, _ string) error {
	return nil
}

// -----------------------------------------------------------------------
// Constructor helper
// -----------------------------------------------------------------------

// NewTokenBlacklister creates the appropriate TokenBlacklister based on configuration.
// If redisURL is non-empty, a Redis-backed blacklist is used (recommended for production
// and multi-instance deployments). Otherwise falls back to InMemoryTokenBlacklister
// with a warning that it is not suitable for multi-instance use.
func NewTokenBlacklister(redisURL string) TokenBlacklister {
	if redisURL != "" {
		bl, err := NewRedisTokenBlacklisterFromURL(redisURL)
		if err != nil {
			slog.Error("Failed to create Redis token blacklister, falling back to in-memory", "error", err)
			return newInMemoryWithWarning()
		}
		slog.Info("Using Redis token blacklist")
		return bl
	}
	return newInMemoryWithWarning()
}

func newInMemoryWithWarning() *InMemoryTokenBlacklister {
	slog.Warn("Using in-memory token blacklist — not suitable for multi-instance deployments; set --token-blacklist-redis-url for production")
	return NewInMemoryTokenBlacklister()
}
