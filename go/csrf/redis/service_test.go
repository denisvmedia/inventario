package redis_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	qt "github.com/frankban/quicktest"
	redisv9 "github.com/redis/go-redis/v9"

	"github.com/denisvmedia/inventario/csrf"
	csrfredis "github.com/denisvmedia/inventario/csrf/redis"
)

// Compile-time interface check.
var _ csrf.Service = (*csrfredis.Service)(nil)

// newTestService starts a miniredis instance and returns a CSRF Service backed
// by it. The caller must call mr.Close() when done (typically via defer).
func newTestService(t *testing.T) (*csrfredis.Service, *miniredis.Miniredis) {
	t.Helper()
	c := qt.New(t)

	mr, err := miniredis.Run()
	c.Assert(err, qt.IsNil)

	svc, err := csrfredis.NewFromURL(fmt.Sprintf("redis://%s/0", mr.Addr()))
	c.Assert(err, qt.IsNil)

	return svc, mr
}

func TestNewFromURL_InvalidURL(t *testing.T) {
	c := qt.New(t)
	_, err := csrfredis.NewFromURL("://bad-url")
	c.Assert(err, qt.IsNotNil)
}

func TestService_GenerateToken(t *testing.T) {
	c := qt.New(t)
	svc, mr := newTestService(t)
	defer mr.Close()

	token, err := svc.GenerateToken(context.Background(), "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")
}

func TestService_GenerateToken_EachCallProducesUniqueValue(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

	token1, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	token2, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token1, qt.Not(qt.Equals), token2)
}

// TestService_GetToken_ReturnsAValidToken verifies that GetToken returns a
// non-empty token that passes ValidateToken. The Redis implementation orders by
// unix-second score; tokens generated within the same second have equal scores
// and are ordered lexicographically, so the test does not assume a specific one.
func TestService_GetToken_ReturnsAValidToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

	_, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	_, err = svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	got, err := svc.GetToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Not(qt.Equals), "")

	valid, err := svc.ValidateToken(ctx, "user-1", got)
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)
}

func TestService_GetToken_UnknownUserReturnsEmpty(t *testing.T) {
	c := qt.New(t)
	svc, mr := newTestService(t)
	defer mr.Close()

	got, err := svc.GetToken(context.Background(), "ghost-user")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, "")
}

func TestService_ValidateToken_ValidTokenReturnsTrue(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

	token, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	valid, err := svc.ValidateToken(ctx, "user-1", token)
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)
}

func TestService_ValidateToken_UnknownTokenReturnsFalse(t *testing.T) {
	c := qt.New(t)
	svc, mr := newTestService(t)
	defer mr.Close()

	valid, err := svc.ValidateToken(context.Background(), "user-1", "not-a-real-token")
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsFalse)
}

func TestService_ValidateToken_UnknownUserReturnsFalse(t *testing.T) {
	c := qt.New(t)
	svc, mr := newTestService(t)
	defer mr.Close()

	valid, err := svc.ValidateToken(context.Background(), "ghost-user", "any-token")
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsFalse)
}

// TestService_ValidateToken_ExpiredScoreReturnsFalse inserts a token with a
// past expiry unix timestamp directly into the ZSET and verifies it is rejected.
func TestService_ValidateToken_ExpiredScoreReturnsFalse(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

	// Bypass GenerateToken and write an already-expired entry directly.
	client := redisv9.NewClient(&redisv9.Options{Addr: mr.Addr()})
	pastScore := float64(time.Now().Add(-time.Minute).Unix())
	err := client.ZAdd(ctx, "csrf:user-1", redisv9.Z{Score: pastScore, Member: "expired-token"}).Err()
	c.Assert(err, qt.IsNil)

	valid, err := svc.ValidateToken(ctx, "user-1", "expired-token")
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsFalse)
}



// TestService_GetToken_SkipsExpiredEntries inserts both a fresh and an expired
// token and verifies that GetToken returns only the non-expired one.
func TestService_GetToken_SkipsExpiredEntries(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

	// Insert an expired token with a past unix timestamp score.
	client := redisv9.NewClient(&redisv9.Options{Addr: mr.Addr()})
	pastScore := float64(time.Now().Add(-time.Minute).Unix())
	err := client.ZAdd(ctx, "csrf:user-1", redisv9.Z{Score: pastScore, Member: "old-token"}).Err()
	c.Assert(err, qt.IsNil)

	fresh, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	got, err := svc.GetToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, fresh)
}

// TestService_MultipleTokensCoexist verifies the rolling window — all tokens
// generated within the window are simultaneously valid.
func TestService_MultipleTokensCoexist(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

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

// TestService_LRUEviction verifies that adding MaxTokensPerUser+1 tokens evicts
// exactly one entry. Because tokens generated within the same second share the
// same unix-timestamp score, the evicted entry is determined by Redis's
// lexicographic tie-breaking, so the test counts surviving tokens instead of
// naming the evicted one.
func TestService_LRUEviction(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

	tokens := make([]string, csrf.MaxTokensPerUser)
	for i := range csrf.MaxTokensPerUser {
		tok, err := svc.GenerateToken(ctx, "eviction-user")
		c.Assert(err, qt.IsNil)
		tokens[i] = tok
	}

	for _, tok := range tokens {
		valid, err := svc.ValidateToken(ctx, "eviction-user", tok)
		c.Assert(err, qt.IsNil)
		c.Assert(valid, qt.IsTrue)
	}

	extra, err := svc.GenerateToken(ctx, "eviction-user")
	c.Assert(err, qt.IsNil)

	valid, err := svc.ValidateToken(ctx, "eviction-user", extra)
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)

	// Exactly MaxTokensPerUser-1 original tokens survive.
	validCount := 0
	for _, tok := range tokens {
		ok, err := svc.ValidateToken(ctx, "eviction-user", tok)
		c.Assert(err, qt.IsNil)
		if ok {
			validCount++
		}
	}
	c.Assert(validCount, qt.Equals, csrf.MaxTokensPerUser-1)
}

// TestService_UserIsolation verifies that different users have completely
// isolated ZSET keys.
func TestService_UserIsolation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

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

func TestService_RevokeToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

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
}

func TestService_RevokeToken_UnknownUserIsNoOp(t *testing.T) {
	c := qt.New(t)
	svc, mr := newTestService(t)
	defer mr.Close()

	err := svc.RevokeToken(context.Background(), "ghost-user", "any-token")
	c.Assert(err, qt.IsNil)
}

func TestService_DeleteAllTokens(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

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
	svc, mr := newTestService(t)
	defer mr.Close()

	err := svc.DeleteAllTokens(context.Background(), "ghost-user")
	c.Assert(err, qt.IsNil)
}

// TestService_DeleteAllTokens_DoesNotAffectOtherUsers verifies scope is per-user.
func TestService_DeleteAllTokens_DoesNotAffectOtherUsers(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc, mr := newTestService(t)
	defer mr.Close()

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
