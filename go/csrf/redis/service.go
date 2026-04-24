package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	redisv9 "github.com/redis/go-redis/v9"

	"github.com/denisvmedia/inventario/csrf"
)

// Service implements csrf.Service using Redis sorted sets.
//
// Each user's tokens are stored in a ZSET where the member is the token value
// and the score is the expiry time expressed as a unix microsecond timestamp.
// Microsecond precision is used (rather than unix seconds) so that tokens
// generated in rapid succession receive strictly monotonic scores; this makes
// rank-based LRU eviction deterministic instead of falling back on Redis's
// lexicographic tie-break between equal scores. Entries automatically expire
// and are pruned via ZREMRANGEBYSCORE and ZREMRANGEBYRANK in the same pipeline
// as GenerateToken, so no external cleanup goroutine is needed.
//
// Use this in production when running more than one server instance.
type Service struct {
	client *redisv9.Client
}

var _ csrf.Service = (*Service)(nil)

// New creates a new Redis-backed CSRF service from an existing client.
func New(client *redisv9.Client) *Service {
	return &Service{client: client}
}

// NewFromURL creates a Redis-backed CSRF service from a connection URL.
//
// A PING connectivity check is performed at construction time; failures are
// logged as warnings but the service is still returned consistent with the
// fail-open design where a Redis outage must not take the API offline.
func NewFromURL(redisURL string) (*Service, error) {
	opts, err := redisv9.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	client := redisv9.NewClient(opts)
	if pingErr := client.Ping(context.Background()).Err(); pingErr != nil {
		slog.Warn("Redis CSRF service unreachable at startup; CSRF protection will fail open until Redis becomes available",
			"error", pingErr)
	}
	return New(client), nil
}

// key returns the ZSET key for the given user's CSRF tokens.
func key(userID string) string { return fmt.Sprintf("csrf:%s", userID) }

// GenerateToken adds a new CSRF token to the user's ZSET (score = expiry unix
// microseconds), prunes expired entries and entries beyond the rolling window,
// then resets the key TTL.
func (s *Service) GenerateToken(ctx context.Context, userID string) (string, error) {
	token, err := csrf.GenerateToken()
	if err != nil {
		return "", err
	}
	k := key(userID)
	now := time.Now()
	expiry := now.Add(csrf.TokenTTL)

	pipe := s.client.Pipeline()
	// Add the new token with score = expiry unix microseconds.
	pipe.ZAdd(ctx, k, redisv9.Z{Score: float64(expiry.UnixMicro()), Member: token})
	// Remove already-expired entries (score < now).
	pipe.ZRemRangeByScore(ctx, k, "-inf", fmt.Sprintf("%d", now.UnixMicro()-1))
	// Refresh the key-level TTL so it outlives all stored tokens.
	pipe.Expire(ctx, k, csrf.TokenTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("failed to store CSRF token: %w", err)
	}

	// Prune to MaxTokensPerUser only when the window is actually full.
	// We check ZCARD first to avoid ZRemRangeByRank's out-of-range rank
	// behaviour which can silently drop entries when the set is small.
	count, err := s.client.ZCard(ctx, k).Result()
	if err == nil && count > int64(csrf.MaxTokensPerUser) {
		// Remove the oldest (count - MaxTokensPerUser) entries.
		_ = s.client.ZRemRangeByRank(ctx, k, 0, count-int64(csrf.MaxTokensPerUser)-1).Err()
	}

	return token, nil
}

// ValidateToken reports whether the given token is in the user's ZSET and has
// not yet expired (i.e. its score > now.UnixMicro()).
func (s *Service) ValidateToken(ctx context.Context, userID, token string) (bool, error) {
	k := key(userID)
	score, err := s.client.ZScore(ctx, k, token).Result()
	if err == redisv9.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to validate CSRF token: %w", err)
	}
	// Score is the expiry unix microseconds; accept only if still in the future.
	return time.Now().UnixMicro() < int64(score), nil
}

// GetToken returns the most recently generated valid token for the user (the
// entry with the highest score that has not yet expired), or "" when none exist.
func (s *Service) GetToken(ctx context.Context, userID string) (string, error) {
	k := key(userID)
	now := time.Now()
	// ZRangeArgs with Rev+ByScore returns members with highest score first; score >= now means not expired.
	results, err := s.client.ZRangeArgs(ctx, redisv9.ZRangeArgs{
		Key:     k,
		Start:   fmt.Sprintf("%d", now.UnixMicro()),
		Stop:    "+inf",
		ByScore: true,
		Rev:     true,
		Offset:  0,
		Count:   1,
	}).Result()
	if err != nil && err != redisv9.Nil {
		return "", fmt.Errorf("failed to get CSRF token: %w", err)
	}
	if len(results) == 0 {
		return "", nil
	}
	return results[0], nil
}

// RevokeToken removes a single specific CSRF token for the given user (e.g. on
// logout of the current session). Other concurrent sessions' tokens remain valid.
func (s *Service) RevokeToken(ctx context.Context, userID, token string) error {
	return s.client.ZRem(ctx, key(userID), token).Err()
}

// DeleteAllTokens removes every CSRF token for the given user, invalidating
// all concurrent sessions at once (e.g. on password change or forced global logout).
func (s *Service) DeleteAllTokens(ctx context.Context, userID string) error {
	return s.client.Del(ctx, key(userID)).Err()
}
