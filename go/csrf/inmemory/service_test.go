package inmemory_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/csrf"
	"github.com/denisvmedia/inventario/csrf/inmemory"
)

// Compile-time interface check.
var _ csrf.Service = (*inmemory.Service)(nil)

func TestService_GenerateToken(t *testing.T) {
	c := qt.New(t)
	svc := inmemory.New()
	defer svc.Stop()

	token, err := svc.GenerateToken(context.Background(), "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")
}

func TestService_GenerateToken_EachCallProducesUniqueValue(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	token1, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	token2, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token1, qt.Not(qt.Equals), token2)
}

func TestService_GetToken_ReturnsNewestToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	_, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	token2, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	got, err := svc.GetToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, token2)
}

func TestService_GetToken_UnknownUserReturnsEmpty(t *testing.T) {
	c := qt.New(t)
	svc := inmemory.New()
	defer svc.Stop()

	got, err := svc.GetToken(context.Background(), "ghost-user")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, "")
}

func TestService_ValidateToken_ValidTokenReturnsTrue(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	token, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	valid, err := svc.ValidateToken(ctx, "user-1", token)
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)
}

func TestService_ValidateToken_UnknownTokenReturnsFalse(t *testing.T) {
	c := qt.New(t)
	svc := inmemory.New()
	defer svc.Stop()

	valid, err := svc.ValidateToken(context.Background(), "user-1", "not-a-real-token")
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsFalse)
}

func TestService_ValidateToken_UnknownUserReturnsFalse(t *testing.T) {
	c := qt.New(t)
	svc := inmemory.New()
	defer svc.Stop()

	valid, err := svc.ValidateToken(context.Background(), "ghost-user", "any-token")
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsFalse)
}

// TestService_MultipleTokensCoexist verifies that generating a second token
// does not invalidate the first — all tokens in the rolling window are valid
// simultaneously, supporting multiple concurrent sessions (browser tabs, etc.).
func TestService_MultipleTokensCoexist(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	token1, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	token2, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	valid1, err := svc.ValidateToken(ctx, "user-1", token1)
	c.Assert(err, qt.IsNil)
	c.Assert(valid1, qt.IsTrue)

	valid2, err := svc.ValidateToken(ctx, "user-1", token2)
	c.Assert(err, qt.IsNil)
	c.Assert(valid2, qt.IsTrue)
}

// TestService_LRUEviction verifies that once the rolling window is full the
// oldest token is evicted and only that one becomes invalid.
func TestService_LRUEviction(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	tokens := make([]string, csrf.MaxTokensPerUser)
	for i := range csrf.MaxTokensPerUser {
		tok, err := svc.GenerateToken(ctx, "eviction-user")
		c.Assert(err, qt.IsNil)
		tokens[i] = tok
	}

	// All tokens in the full window must be valid.
	for _, tok := range tokens {
		valid, err := svc.ValidateToken(ctx, "eviction-user", tok)
		c.Assert(err, qt.IsNil)
		c.Assert(valid, qt.IsTrue)
	}

	// One more token pushes tokens[0] (the oldest) out.
	extra, err := svc.GenerateToken(ctx, "eviction-user")
	c.Assert(err, qt.IsNil)

	valid, err := svc.ValidateToken(ctx, "eviction-user", extra)
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)

	evicted, err := svc.ValidateToken(ctx, "eviction-user", tokens[0])
	c.Assert(err, qt.IsNil)
	c.Assert(evicted, qt.IsFalse)

	for _, tok := range tokens[1:] {
		valid, err := svc.ValidateToken(ctx, "eviction-user", tok)
		c.Assert(err, qt.IsNil)
		c.Assert(valid, qt.IsTrue)
	}
}

// TestService_UserIsolation verifies that different users have completely
// isolated token namespaces — tokens are never shared across users.
func TestService_UserIsolation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	tokenA, err := svc.GenerateToken(ctx, "user-A")
	c.Assert(err, qt.IsNil)

	tokenB, err := svc.GenerateToken(ctx, "user-B")
	c.Assert(err, qt.IsNil)

	validA, err := svc.ValidateToken(ctx, "user-A", tokenA)
	c.Assert(err, qt.IsNil)
	c.Assert(validA, qt.IsTrue)

	crossValid, err := svc.ValidateToken(ctx, "user-B", tokenA)
	c.Assert(err, qt.IsNil)
	c.Assert(crossValid, qt.IsFalse)

	validB, err := svc.ValidateToken(ctx, "user-B", tokenB)
	c.Assert(err, qt.IsNil)
	c.Assert(validB, qt.IsTrue)
}

// TestService_RevokeToken verifies that RevokeToken removes exactly the
// specified token while leaving all other tokens for the same user intact.
func TestService_RevokeToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	token1, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	token2, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	token3, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	err = svc.RevokeToken(ctx, "user-1", token2)
	c.Assert(err, qt.IsNil)

	valid2, err := svc.ValidateToken(ctx, "user-1", token2)
	c.Assert(err, qt.IsNil)
	c.Assert(valid2, qt.IsFalse)

	valid1, err := svc.ValidateToken(ctx, "user-1", token1)
	c.Assert(err, qt.IsNil)
	c.Assert(valid1, qt.IsTrue)

	valid3, err := svc.ValidateToken(ctx, "user-1", token3)
	c.Assert(err, qt.IsNil)
	c.Assert(valid3, qt.IsTrue)

	// GetToken must still return the newest remaining token.
	got, err := svc.GetToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, token3)
}

func TestService_RevokeToken_UnknownUserIsNoOp(t *testing.T) {
	c := qt.New(t)
	svc := inmemory.New()
	defer svc.Stop()

	err := svc.RevokeToken(context.Background(), "ghost-user", "any-token")
	c.Assert(err, qt.IsNil)
}

func TestService_DeleteAllTokens(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	_, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	_, err = svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	err = svc.DeleteAllTokens(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	got, err := svc.GetToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, "")
}

func TestService_DeleteAllTokens_UnknownUserIsNoOp(t *testing.T) {
	c := qt.New(t)
	svc := inmemory.New()
	defer svc.Stop()

	err := svc.DeleteAllTokens(context.Background(), "ghost-user")
	c.Assert(err, qt.IsNil)
}

// TestService_DeleteAllTokens_DoesNotAffectOtherUsers verifies scope is per-user.
func TestService_DeleteAllTokens_DoesNotAffectOtherUsers(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	tokenA, err := svc.GenerateToken(ctx, "user-A")
	c.Assert(err, qt.IsNil)
	_, err = svc.GenerateToken(ctx, "user-B")
	c.Assert(err, qt.IsNil)

	err = svc.DeleteAllTokens(ctx, "user-B")
	c.Assert(err, qt.IsNil)

	valid, err := svc.ValidateToken(ctx, "user-A", tokenA)
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)

	got, err := svc.GetToken(ctx, "user-B")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, "")
}

// TestService_Stop_IsIdempotent verifies that Stop can be called multiple times
// without panicking and that the service remains fully functional afterwards.
func TestService_Stop_IsIdempotent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := inmemory.New()

	svc.Stop()
	svc.Stop()
	svc.Stop()

	token, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	got, err := svc.GetToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, token)
}

// TestService_ConcurrentAccess exercises the service under concurrent load to
// detect data races when run with the -race flag.
func TestService_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	svc := inmemory.New()
	defer svc.Stop()

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := fmt.Sprintf("concurrent-user-%d", id)

			tok, err := svc.GenerateToken(ctx, userID)
			if err != nil {
				t.Errorf("GenerateToken(%s): %v", userID, err)
				return
			}
			if _, err := svc.ValidateToken(ctx, userID, tok); err != nil {
				t.Errorf("ValidateToken(%s): %v", userID, err)
			}
			if _, err := svc.GetToken(ctx, userID); err != nil {
				t.Errorf("GetToken(%s): %v", userID, err)
			}
			if err := svc.RevokeToken(ctx, userID, tok); err != nil {
				t.Errorf("RevokeToken(%s): %v", userID, err)
			}
		}(i)
	}
	wg.Wait()
}
