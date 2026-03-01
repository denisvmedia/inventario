package inmemory

import (
	"context"
	"sync"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/denisvmedia/inventario/csrf"
)

// Service implements csrf.Service using per-user expirable LRU caches.
//
// Each user gets an LRU cache of up to csrf.MaxTokensPerUser tokens; entries
// expire automatically after csrf.TokenTTL. The rolling window means the oldest
// token is evicted when the window is full, so multiple parallel sessions
// (browser tabs, e2e test workers) do not invalidate each other's tokens.
//
// The service is thread-safe but does NOT persist across restarts and does NOT
// share state between multiple server instances.
type Service struct {
	mu    sync.RWMutex
	users map[string]*expirable.LRU[string, struct{}]
}

var _ csrf.Service = (*Service)(nil)

// New creates a new in-memory CSRF service.
func New() *Service {
	return &Service{
		users: make(map[string]*expirable.LRU[string, struct{}]),
	}
}

// Stop is a no-op; expirable.LRU manages its own internal goroutine lifecycle.
// It is safe to call multiple times and exists for API compatibility.
func (s *Service) Stop() {}

// getUserCache returns the per-user LRU cache, creating it on first access.
// Must be called with s.mu held for writing.
func (s *Service) getUserCache(userID string) *expirable.LRU[string, struct{}] {
	if c, ok := s.users[userID]; ok {
		return c
	}
	c := expirable.NewLRU[string, struct{}](csrf.MaxTokensPerUser, nil, csrf.TokenTTL)
	s.users[userID] = c
	return c
}

// GenerateToken creates a new CSRF token and adds it to the user's rolling window.
// When the window is full, the oldest token is evicted automatically.
func (s *Service) GenerateToken(_ context.Context, userID string) (string, error) {
	token, err := csrf.GenerateToken()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	cache := s.getUserCache(userID)
	cache.Add(token, struct{}{})
	s.mu.Unlock()
	return token, nil
}

// ValidateToken reports whether token is in the user's rolling window and not expired.
// It does not update the LRU recency of the token. Empty per-user caches are pruned
// to prevent unbounded memory growth for one-time users.
func (s *Service) ValidateToken(_ context.Context, userID, token string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cache, ok := s.users[userID]
	if !ok {
		return false, nil
	}
	// Contains does not update LRU order; expired entries are treated as absent.
	valid := cache.Contains(token)
	// Prune the per-user entry when all tokens have expired to avoid accumulating
	// empty caches for one-time users indefinitely.
	// We skip the Keys() call (which allocates) on the common/hot path: if the
	// token is valid, at least one live token exists so pruning is unnecessary.
	if !valid && len(cache.Keys()) == 0 {
		delete(s.users, userID)
	}
	return valid, nil
}

// GetToken returns the most recently generated valid token for userID.
// Returns "" when no valid tokens exist (all expired or none generated).
// Empty per-user caches are pruned to prevent unbounded memory growth.
func (s *Service) GetToken(_ context.Context, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cache, ok := s.users[userID]
	if !ok {
		return "", nil
	}
	// Keys() returns from oldest to newest with expired entries filtered out.
	keys := cache.Keys()
	if len(keys) == 0 {
		// All tokens expired: prune the empty cache entry.
		delete(s.users, userID)
		return "", nil
	}
	return keys[len(keys)-1], nil
}

// RevokeToken removes a single specific CSRF token for the given user (e.g. on
// logout of the current session). Other concurrent sessions' tokens remain valid.
func (s *Service) RevokeToken(_ context.Context, userID, token string) error {
	s.mu.Lock()
	if cache, ok := s.users[userID]; ok {
		cache.Remove(token)
	}
	s.mu.Unlock()
	return nil
}

// DeleteAllTokens removes every CSRF token for the given user, invalidating
// all concurrent sessions at once (e.g. on password change or forced global logout).
func (s *Service) DeleteAllTokens(_ context.Context, userID string) error {
	s.mu.Lock()
	if cache, ok := s.users[userID]; ok {
		cache.Purge()
		delete(s.users, userID)
	}
	s.mu.Unlock()
	return nil
}
