package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// MarkClaimed is the single-use guard on passwordless sign-in: a magic link
// accepted twice is a login link that can be replayed. The guard is one
// conditional UPDATE — `claimed_at IS NULL AND expires_at > now` — so it is a
// property of the statement rather than of the Go around it, and it needs a
// real database to mean anything. #2114 N1 lists magic_link_tokens among the
// DSN-gated gaps.

func magicLinkRegistry(c *qt.C) registry.MagicLinkTokenRegistry {
	c.Helper()

	dsn := skipIfNoPostgreSQL(c.TB.(*testing.T))
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	return postgres.NewFactorySet(dbx).MagicLinkTokenRegistry
}

func seedMagicLink(c *qt.C, reg registry.MagicLinkTokenRegistry, userID, tenantID, token string, expiresAt time.Time) *models.MagicLinkToken {
	c.Helper()

	created, err := reg.Create(context.Background(), models.MagicLinkToken{
		UserID:    userID,
		TenantID:  tenantID,
		Email:     "magic@example.com",
		Token:     token,
		ExpiresAt: expiresAt,
	})
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { _ = reg.Delete(context.Background(), created.ID) })
	return created
}

// Exactly one of N requests carrying the same live token may claim it. Two
// winners is a sign-in link that works twice.
func TestMagicLinkTokenRegistryPostgres_MarkClaimed_Concurrent(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)
	reg := magicLinkRegistry(c)

	const token = "race-magic-token-0001"
	seedMagicLink(c, reg, user.ID, user.TenantID, token, time.Now().Add(15*time.Minute))

	const goroutines = 8
	var wg sync.WaitGroup
	results := make([]bool, goroutines)
	errs := make([]error, goroutines)
	start := make(chan struct{})

	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			<-start
			ok, err := reg.MarkClaimed(context.Background(), token)
			results[idx] = ok
			errs[idx] = err
		}(i)
	}
	close(start)
	wg.Wait()

	winners := 0
	for i, ok := range results {
		c.Check(errs[i], qt.IsNil, qt.Commentf("goroutine %d", i))
		if ok {
			winners++
		}
	}
	c.Assert(winners, qt.Equals, 1,
		qt.Commentf("%d of %d requests claimed the same magic link", winners, goroutines))

	// And the row records when it was claimed, which is what makes a later
	// attempt observable as a replay rather than as a fresh sign-in.
	claimed, err := reg.GetByToken(context.Background(), token)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed.ClaimedAt, qt.IsNotNil)
}

func TestMagicLinkTokenRegistryPostgres_MarkClaimed_SecondAttemptLoses(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)
	reg := magicLinkRegistry(c)

	const token = "single-use-magic-token-0002"
	seedMagicLink(c, reg, user.ID, user.TenantID, token, time.Now().Add(15*time.Minute))

	ctx := context.Background()
	first, err := reg.MarkClaimed(ctx, token)
	c.Assert(err, qt.IsNil)
	c.Check(first, qt.IsTrue)

	second, err := reg.MarkClaimed(ctx, token)
	c.Assert(err, qt.IsNil)
	c.Check(second, qt.IsFalse)
}

// The expiry lives inside the same statement, so a token that has run out is
// refused by the claim itself rather than by a separate check the caller has
// to remember.
func TestMagicLinkTokenRegistryPostgres_MarkClaimed_ExpiredIsRefused(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)
	reg := magicLinkRegistry(c)

	const token = "expired-magic-token-0003"
	seedMagicLink(c, reg, user.ID, user.TenantID, token, time.Now().Add(-time.Minute))

	ok, err := reg.MarkClaimed(context.Background(), token)
	c.Assert(err, qt.IsNil)
	c.Check(ok, qt.IsFalse)

	// Refused, and left alone: an expired link is not silently stamped.
	stored, err := reg.GetByToken(context.Background(), token)
	c.Assert(err, qt.IsNil)
	c.Check(stored.ClaimedAt, qt.IsNil)
}

// A token nobody issued is a false, not an error — the caller maps both to
// the same "this link is no longer valid" answer, and an error here would
// distinguish a guessed token from an expired one.
func TestMagicLinkTokenRegistryPostgres_MarkClaimed_UnknownTokenIsFalse(t *testing.T) {
	c := qt.New(t)

	reg := magicLinkRegistry(c)

	ok, err := reg.MarkClaimed(context.Background(), "no-such-magic-token-0004")
	c.Assert(err, qt.IsNil)
	c.Check(ok, qt.IsFalse)
}

func TestMagicLinkTokenRegistryPostgres_MarkClaimed_EmptyTokenIsRefused(t *testing.T) {
	c := qt.New(t)

	reg := magicLinkRegistry(c)

	_, err := reg.MarkClaimed(context.Background(), "")
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
}
