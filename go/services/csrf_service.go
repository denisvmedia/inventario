package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// csrfTokenTTL is the lifetime of a CSRF token.
	// It is refreshed on every successful login and token refresh.
	csrfTokenTTL = time.Hour
)

// CSRFService manages per-user CSRF tokens.
//
// Each user has at most one active CSRF token at a time, which is replaced
// whenever a new token is generated (e.g. on login or token refresh).
// Tokens must be included in the X-CSRF-Token header of all state-changing
// HTTP requests (POST/PUT/PATCH/DELETE).
type CSRFService interface {
	// GenerateToken creates a new CSRF token for the given user, replacing any
	// existing token. Returns the new token value.
	GenerateToken(ctx context.Context, userID string) (string, error)

	// GetToken returns the current CSRF token for the given user.
	// Returns ("", nil) when no token is found (e.g. user has never logged in
	// on this backend instance, or the token has expired).
	GetToken(ctx context.Context, userID string) (string, error)

	// DeleteToken removes the CSRF token for the given user (e.g. on logout).
	DeleteToken(ctx context.Context, userID string) error
}

// generateCSRFToken produces a cryptographically secure, URL-safe CSRF token.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// -----------------------------------------------------------------------
// Redis implementation (recommended for production / multi-instance)
// -----------------------------------------------------------------------

// RedisCSRFService implements CSRFService using Redis.
// Entries automatically expire via Redis TTL so no manual cleanup is needed.
// Use this in production when running more than one server instance.
type RedisCSRFService struct {
	client *redis.Client
}

// NewRedisCSRFService creates a new Redis-backed CSRF service.
func NewRedisCSRFService(client *redis.Client) *RedisCSRFService {
	return &RedisCSRFService{client: client}
}

// NewRedisCSRFServiceFromURL creates a Redis-backed CSRF service from a URL.
// A PING connectivity check is performed at construction time; failures are
// logged as warnings but the service is still returned consistent with the
// fail-open design where a Redis outage must not take the API offline.
func NewRedisCSRFServiceFromURL(redisURL string) (*RedisCSRFService, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	client := redis.NewClient(opts)
	if pingErr := client.Ping(context.Background()).Err(); pingErr != nil {
		slog.Warn("Redis CSRF service unreachable at startup; CSRF protection will fail open until Redis becomes available", "error", pingErr)
	}
	return NewRedisCSRFService(client), nil
}

func (s *RedisCSRFService) GenerateToken(ctx context.Context, userID string) (string, error) {
	token, err := generateCSRFToken()
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("csrf:%s", userID)
	if err := s.client.Set(ctx, key, token, csrfTokenTTL).Err(); err != nil {
		return "", fmt.Errorf("failed to store CSRF token: %w", err)
	}
	return token, nil
}

func (s *RedisCSRFService) GetToken(ctx context.Context, userID string) (string, error) {
	key := fmt.Sprintf("csrf:%s", userID)
	token, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get CSRF token: %w", err)
	}
	return token, nil
}

func (s *RedisCSRFService) DeleteToken(ctx context.Context, userID string) error {
	key := fmt.Sprintf("csrf:%s", userID)
	return s.client.Del(ctx, key).Err()
}

// -----------------------------------------------------------------------
// In-memory implementation (for development / single-instance)
// -----------------------------------------------------------------------

type csrfEntry struct {
	token     string
	expiresAt time.Time
}

// InMemoryCSRFService implements CSRFService using an in-process map with TTL.
// It is thread-safe but does NOT persist across process restarts and does NOT
// share state between multiple server instances.
type InMemoryCSRFService struct {
	mu     sync.Mutex
	tokens map[string]csrfEntry
	stopCh chan struct{}
	once   sync.Once
}

// NewInMemoryCSRFService creates a new in-memory CSRF service and starts the
// background cleanup goroutine.
func NewInMemoryCSRFService() *InMemoryCSRFService {
	s := &InMemoryCSRFService{
		tokens: make(map[string]csrfEntry),
		stopCh: make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

// Stop terminates the background cleanup goroutine. Safe to call multiple times.
func (s *InMemoryCSRFService) Stop() {
	s.once.Do(func() { close(s.stopCh) })
}

func (s *InMemoryCSRFService) GenerateToken(_ context.Context, userID string) (string, error) {
	token, err := generateCSRFToken()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.tokens[userID] = csrfEntry{token: token, expiresAt: time.Now().Add(csrfTokenTTL)}
	s.mu.Unlock()
	return token, nil
}

func (s *InMemoryCSRFService) GetToken(_ context.Context, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tokens[userID]
	if !ok {
		return "", nil
	}
	if time.Now().After(entry.expiresAt) {
		delete(s.tokens, userID)
		return "", nil
	}
	return entry.token, nil
}

func (s *InMemoryCSRFService) DeleteToken(_ context.Context, userID string) error {
	s.mu.Lock()
	delete(s.tokens, userID)
	s.mu.Unlock()
	return nil
}

func (s *InMemoryCSRFService) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
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
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

// -----------------------------------------------------------------------
// No-op implementation (CSRF protection disabled)
// -----------------------------------------------------------------------

// NoOpCSRFService is a CSRF service that never rejects requests.
// Use only in test environments where CSRF protection is deliberately disabled.
type NoOpCSRFService struct{}

func (NoOpCSRFService) GenerateToken(_ context.Context, _ string) (string, error) {
	return "noop-csrf-token", nil
}

func (NoOpCSRFService) GetToken(_ context.Context, _ string) (string, error) {
	return "noop-csrf-token", nil
}

func (NoOpCSRFService) DeleteToken(_ context.Context, _ string) error {
	return nil
}

// -----------------------------------------------------------------------
// Constructor helper
// -----------------------------------------------------------------------

// NewCSRFService creates the appropriate CSRFService based on configuration.
// If redisURL is non-empty, a Redis-backed service is used (recommended for
// production and multi-instance deployments). Otherwise falls back to
// InMemoryCSRFService with a warning that it is not suitable for multi-instance use.
func NewCSRFService(redisURL string) CSRFService {
	if redisURL != "" {
		svc, err := NewRedisCSRFServiceFromURL(redisURL)
		if err != nil {
			slog.Error("Failed to create Redis CSRF service, falling back to in-memory", "error", err)
			return newInMemoryCSRFServiceWithWarning()
		}
		slog.Info("Using Redis CSRF service")
		return svc
	}
	return newInMemoryCSRFServiceWithWarning()
}

func newInMemoryCSRFServiceWithWarning() *InMemoryCSRFService {
	slog.Warn("Using in-memory CSRF service — not suitable for multi-instance deployments; set --csrf-redis-url for production")
	return NewInMemoryCSRFService()
}
