package services_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/services"
)

func TestInMemoryCSRFService_GenerateAndGetToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := services.NewInMemoryCSRFService()
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

	svc := services.NewInMemoryCSRFService()
	defer svc.Stop()

	// Try to get a token for a user that never logged in
	token, err := svc.GetToken(ctx, "nonexistent-user")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Equals, "")
}

func TestInMemoryCSRFService_DeleteToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := services.NewInMemoryCSRFService()
	defer svc.Stop()

	userID := "test-user-456"

	// Generate a token
	token, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	// Delete the token
	err = svc.DeleteToken(ctx, userID)
	c.Assert(err, qt.IsNil)

	// Verify it's gone
	retrieved, err := svc.GetToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved, qt.Equals, "")
}

func TestInMemoryCSRFService_TokenReplacement(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := services.NewInMemoryCSRFService()
	defer svc.Stop()

	userID := "test-user-789"

	// Generate first token
	token1, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)

	// Generate second token (should replace the first)
	token2, err := svc.GenerateToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(token2, qt.Not(qt.Equals), token1)

	// Verify only the second token is valid
	retrieved, err := svc.GetToken(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved, qt.Equals, token2)
}

func TestInMemoryCSRFService_MultipleUsers(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := services.NewInMemoryCSRFService()
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

func TestNoOpCSRFService(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := services.NoOpCSRFService{}

	// Generate always returns the same noop token
	token1, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token1, qt.Equals, "noop-csrf-token")

	token2, err := svc.GenerateToken(ctx, "user-2")
	c.Assert(err, qt.IsNil)
	c.Assert(token2, qt.Equals, "noop-csrf-token")

	// Get always returns the same noop token
	retrieved, err := svc.GetToken(ctx, "any-user")
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved, qt.Equals, "noop-csrf-token")

	// Delete is a no-op
	err = svc.DeleteToken(ctx, "any-user")
	c.Assert(err, qt.IsNil)
}

func TestInMemoryCSRFService_StopCleanup(t *testing.T) {
	c := qt.New(t)

	svc := services.NewInMemoryCSRFService()

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
	if memSvc, ok := svc.(*services.InMemoryCSRFService); ok {
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
	if memSvc, ok := svc.(*services.InMemoryCSRFService); ok {
		defer memSvc.Stop()
	}
}

func TestInMemoryCSRFService_ConcurrentAccess(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := services.NewInMemoryCSRFService()
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
			if err != nil {
				t.Errorf("Failed to generate token: %v", err)
			}
			retrieved, err := svc.GetToken(ctx, userID)
			if err != nil {
				t.Errorf("Failed to get token: %v", err)
			}
			if retrieved != token {
				t.Errorf("Token mismatch: expected %s, got %s", token, retrieved)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	c.Assert(true, qt.IsTrue) // If we got here, no race conditions occurred
}
