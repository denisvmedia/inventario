package services_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/csrf"
	csrfinmemory "github.com/denisvmedia/inventario/csrf/inmemory"
	csrfnoop "github.com/denisvmedia/inventario/csrf/noop"
	"github.com/denisvmedia/inventario/services"
)

func TestInMemoryCSRFService_GenerateAndGetToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfinmemory.New()
	defer svc.Stop()

	userID := "test-user-123"

	// Generate a token
	token, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	// Retrieve the same token
	retrieved, err := svc.GetToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved, qt.Equals, token)
}

func TestInMemoryCSRFService_GetNonExistentToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfinmemory.New()
	defer svc.Stop()

	// Try to get a token for a user that never logged in
	token, err := svc.GetToken(ctx, "nonexistent-user")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Equals, "")
}

func TestInMemoryCSRFService_DeleteAllTokens(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfinmemory.New()
	defer svc.Stop()

	userID := "test-user-456"

	// Generate a token
	token, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	// Delete all tokens for the user.
	err = svc.DeleteAllTokens(ctx, userID)
	c.Assert(err, qt.IsNil)

	// Verify it's gone
	retrieved, err := svc.GetToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved, qt.Equals, "")
}

// TestInMemoryCSRFService_MultipleTokensCoexist verifies that generating a
// second token does not invalidate the first — both must remain valid so that
// parallel sessions (browser tabs, test workers) do not break each other.
func TestInMemoryCSRFService_MultipleTokensCoexist(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfinmemory.New()
	defer svc.Stop()

	userID := "test-user-789"

	token1, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)

	token2, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(token2, qt.Not(qt.Equals), token1)

	// GetToken must return the most recently generated token.
	retrieved, err := svc.GetToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved, qt.Equals, token2)

	// Both tokens must still be individually valid.
	valid1, err := svc.ValidateToken(ctx, userID, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(valid1, qt.IsTrue)

	valid2, err := svc.ValidateToken(ctx, userID, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(valid2, qt.IsTrue)
}

// TestInMemoryCSRFService_LRUEviction verifies that once the rolling window is
// full the oldest token is evicted (and only that one becomes invalid).
func TestInMemoryCSRFService_LRUEviction(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfinmemory.New()
	defer svc.Stop()

	userID := "eviction-user"
	maxTokens := csrf.MaxTokensPerUser

	// Fill the window completely.
	tokens := make([]string, maxTokens)
	for i := range maxTokens {
		tok, err := svc.GenerateToken(ctx, userID)
		c.Assert(err, qt.IsNil)
		tokens[i] = tok
	}

	// All tokens in the window must be valid.
	for _, tok := range tokens {
		valid, err := svc.ValidateToken(ctx, userID, tok)
		c.Assert(err, qt.IsNil)
		c.Assert(valid, qt.IsTrue)
	}

	// One more token pushes the oldest (tokens[0]) out.
	extra, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)

	// The new token must be valid.
	valid, err := svc.ValidateToken(ctx, userID, extra)
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)

	// The evicted token (oldest, tokens[0]) must no longer be valid.
	evicted, err := svc.ValidateToken(ctx, userID, tokens[0])
	c.Assert(err, qt.IsNil)
	c.Assert(evicted, qt.IsFalse)

	// All other tokens still in the window must still be valid.
	for _, tok := range tokens[1:] {
		valid, err := svc.ValidateToken(ctx, userID, tok)
		c.Assert(err, qt.IsNil)
		c.Assert(valid, qt.IsTrue)
	}
}

func TestInMemoryCSRFService_MultipleUsers(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfinmemory.New()
	defer svc.Stop()

	user1 := "user-1"
	user2 := "user-2"

	// Generate tokens for both users
	token1, err := svc.GenerateToken(ctx, user1)
	c.Assert(err, qt.IsNil)

	token2, err := svc.GenerateToken(ctx, user2)
	c.Assert(err, qt.IsNil)

	// Tokens should be different
	c.Assert(token1, qt.Not(qt.Equals), token2)

	// Each user should get their own token
	retrieved1, err := svc.GetToken(ctx, user1)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved1, qt.Equals, token1)

	retrieved2, err := svc.GetToken(ctx, user2)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved2, qt.Equals, token2)
}

// TestInMemoryCSRFService_RevokeToken verifies that RevokeToken removes exactly
// the specified token while leaving all other tokens for the same user intact.
func TestInMemoryCSRFService_RevokeToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfinmemory.New()
	defer svc.Stop()

	userID := "revoke-user"

	token1, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)

	token2, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)

	token3, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)

	// Revoke only the middle token.
	err = svc.RevokeToken(ctx, userID, token2)
	c.Assert(err, qt.IsNil)

	// token2 must no longer be valid.
	valid2, err := svc.ValidateToken(ctx, userID, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(valid2, qt.IsFalse)

	// token1 and token3 must still be valid.
	valid1, err := svc.ValidateToken(ctx, userID, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(valid1, qt.IsTrue)

	valid3, err := svc.ValidateToken(ctx, userID, token3)
	c.Assert(err, qt.IsNil)
	c.Assert(valid3, qt.IsTrue)

	// GetToken must still return the newest remaining token.
	retrieved, err := svc.GetToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved, qt.Equals, token3)
}

// TestInMemoryCSRFService_RevokeToken_NonExistentUser verifies that revoking a
// token for a user that has no tokens is a safe no-op.
func TestInMemoryCSRFService_RevokeToken_NonExistentUser(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfinmemory.New()
	defer svc.Stop()

	err := svc.RevokeToken(ctx, "ghost-user", "some-token")
	c.Assert(err, qt.IsNil)
}

func TestNoOpCSRFService(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfnoop.Service{}

	// Generate always returns the same noop token
	token1, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token1, qt.Equals, "noop-csrf-token")

	token2, err := svc.GenerateToken(ctx, "user-2")
	c.Assert(err, qt.IsNil)
	c.Assert(token2, qt.Equals, "noop-csrf-token")

	// ValidateToken always returns true.
	valid, err := svc.ValidateToken(ctx, "any-user", "any-token")
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)

	// Get always returns the same noop token
	retrieved, err := svc.GetToken(ctx, "any-user")
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved, qt.Equals, "noop-csrf-token")

	// RevokeToken is a no-op
	err = svc.RevokeToken(ctx, "any-user", "any-token")
	c.Assert(err, qt.IsNil)

	// DeleteAllTokens is a no-op
	err = svc.DeleteAllTokens(ctx, "any-user")
	c.Assert(err, qt.IsNil)
}

func TestInMemoryCSRFService_StopCleanup(t *testing.T) {
	c := qt.New(t)

	svc := csrfinmemory.New()

	// Stop should be safe to call multiple times
	svc.Stop()
	svc.Stop()
	svc.Stop()

	// Service should still work after Stop (just no cleanup)
	ctx := context.Background()
	token, err := svc.GenerateToken(ctx, "test-user")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	retrieved, err := svc.GetToken(ctx, "test-user")
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved, qt.Equals, token)
}

func TestNewCSRFService_FallbackToInMemory(t *testing.T) {
	c := qt.New(t)

	// Empty URL should create in-memory service
	svc := services.NewCSRFService("")
	c.Assert(svc, qt.IsNotNil)

	// Should work as an in-memory service
	ctx := context.Background()
	token, err := svc.GenerateToken(ctx, "test-user")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	// Clean up if it's an in-memory service
	if memSvc, ok := svc.(*csrfinmemory.Service); ok {
		defer memSvc.Stop()
	}
}

func TestNewCSRFService_InvalidRedisURL(t *testing.T) {
	c := qt.New(t)

	// Invalid Redis URL should fall back to in-memory with error logged
	svc := services.NewCSRFService("invalid://not-a-valid-url")
	c.Assert(svc, qt.IsNotNil)

	// Should still work (fallback to in-memory)
	ctx := context.Background()
	token, err := svc.GenerateToken(ctx, "test-user")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	// Clean up if it's an in-memory service
	if memSvc, ok := svc.(*csrfinmemory.Service); ok {
		defer memSvc.Stop()
	}
}

func TestInMemoryCSRFService_ConcurrentAccess(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := csrfinmemory.New()
	defer svc.Stop()

	// Test concurrent token generation and retrieval using WaitGroup
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Use id in userID to avoid collisions across goroutines
			userID := fmt.Sprintf("user-%d", id)
			token, err := svc.GenerateToken(ctx, userID)
			c.Check(err, qt.IsNil, qt.Commentf("Failed to generate token for %s", userID))
			retrieved, err := svc.GetToken(ctx, userID)
			c.Check(err, qt.IsNil, qt.Commentf("Failed to get token for %s", userID))
			if retrieved != token {
				t.Errorf("Token mismatch: expected %s, got %s", token, retrieved)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	c.Assert(true, qt.IsTrue) // If we got here, no race conditions occurred
}
