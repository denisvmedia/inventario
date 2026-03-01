// Package csrf defines the provider-agnostic CSRF token management contract
// and shared helpers used by all backend implementations.
//
// Architecture:
//   - The Service interface defines per-user token lifecycle operations.
//   - Implementations live in the sub-packages csrf/inmemory, csrf/redis, and csrf/noop.
//   - Higher-level wiring (factory selection) is handled by the services layer.
//
// Each user's tokens are stored in a rolling window of up to MaxTokensPerUser
// entries so that multiple concurrent sessions (browser tabs, parallel test
// workers, mobile + desktop) do not invalidate each other.
package csrf

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

const (
	// TokenTTL is the lifetime of a single CSRF token.
	TokenTTL = time.Hour

	// MaxTokensPerUser is the maximum number of simultaneously valid CSRF tokens
	// kept per user. A rolling window of this size allows multiple concurrent
	// sessions (e.g. parallel browser tabs or integration-test workers) without
	// invalidating each other's tokens. Oldest tokens are evicted once the
	// window is full.
	//
	// 100 is chosen to accommodate end-to-end test suites that run many tests
	// in parallel using the same user account (e.g. 5 Playwright workers × up
	// to 20 re-logins per worker over the full suite duration). In production
	// the typical number of concurrent sessions per user is far smaller (a few
	// browser tabs / mobile apps), so the overhead is negligible.
	MaxTokensPerUser = 100
)

// Service manages per-user CSRF tokens.
//
// A rolling window of up to MaxTokensPerUser tokens is kept per user so that
// multiple concurrent sessions do not invalidate each other. Each token is
// valid for TokenTTL. Tokens must be sent in the X-CSRF-Token header of all
// state-changing HTTP requests (POST/PUT/PATCH/DELETE).
type Service interface {
	// GenerateToken creates a new CSRF token for the given user and adds it to
	// the rolling window. When the window is full the oldest token is evicted.
	// Returns the new token value.
	GenerateToken(ctx context.Context, userID string) (string, error)

	// ValidateToken reports whether token is currently valid for userID (i.e.
	// it was recently generated and has not yet expired).
	ValidateToken(ctx context.Context, userID, token string) (bool, error)

	// GetToken returns the most recently generated, still-valid token for
	// userID, or "" when no valid token exists. Used by the X-CSRF-Token
	// response header on GET /auth/me so the frontend can recover after a page
	// reload.
	GetToken(ctx context.Context, userID string) (string, error)

	// RevokeToken removes a single specific CSRF token for the given user (e.g.
	// on logout of the current session). Other concurrent sessions' tokens remain
	// valid. Use DeleteAllTokens to invalidate all sessions at once.
	RevokeToken(ctx context.Context, userID, token string) error

	// DeleteAllTokens removes every CSRF token for the given user, invalidating
	// all concurrent sessions at once (e.g. on password change or forced
	// global logout).
	DeleteAllTokens(ctx context.Context, userID string) error
}

// GenerateToken produces a cryptographically secure, URL-safe random token
// suitable for use as a CSRF token. It is exported so that implementation
// sub-packages can share the same generation logic without duplication.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
