package memory_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// newTestMagicLinkToken builds a valid, live MagicLinkToken with the given
// token value. The memory backend has no FK constraints, so the tenant/user
// IDs are arbitrary — but realistic values keep the tests readable.
func newTestMagicLinkToken(token string) models.MagicLinkToken {
	return models.MagicLinkToken{
		UserID:    "user-1",
		TenantID:  "tenant-1",
		Email:     "user@example.com",
		Token:     token,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
}

func TestMagicLinkTokenRegistry_Create_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	created, err := r.Create(ctx, newTestMagicLinkToken("token-happy"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.Token, qt.Equals, "token-happy")
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)
}

func TestMagicLinkTokenRegistry_Create_MissingFields(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		mut  func(*models.MagicLinkToken)
	}{
		{"user_id empty", func(mlt *models.MagicLinkToken) { mlt.UserID = "" }},
		{"tenant_id empty", func(mlt *models.MagicLinkToken) { mlt.TenantID = "" }},
		{"token empty", func(mlt *models.MagicLinkToken) { mlt.Token = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			r := memory.NewMagicLinkTokenRegistry()
			mlt := newTestMagicLinkToken("token-missing")
			tc.mut(&mlt)
			_, err := r.Create(ctx, mlt)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
		})
	}
}

func TestMagicLinkTokenRegistry_GetByToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	created, err := r.Create(ctx, newTestMagicLinkToken("token-by-token"))
	c.Assert(err, qt.IsNil)

	fetched, err := r.GetByToken(ctx, "token-by-token")
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)

	_, err = r.GetByToken(ctx, "missing-token")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestMagicLinkTokenRegistry_DeleteByUserID(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	target := newTestMagicLinkToken("token-target-1")
	target.UserID = "user-target"
	_, err := r.Create(ctx, target)
	c.Assert(err, qt.IsNil)

	target2 := newTestMagicLinkToken("token-target-2")
	target2.UserID = "user-target"
	_, err = r.Create(ctx, target2)
	c.Assert(err, qt.IsNil)

	other := newTestMagicLinkToken("token-other")
	other.UserID = "user-other"
	_, err = r.Create(ctx, other)
	c.Assert(err, qt.IsNil)

	err = r.DeleteByUserID(ctx, "user-target")
	c.Assert(err, qt.IsNil)

	// Both of the target user's rows are gone; the other user's row survives.
	all, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)
	c.Assert(all[0].UserID, qt.Equals, "user-other")
}

// TestMagicLinkTokenRegistry_DeleteExpired pins that DeleteExpired removes only
// records whose ExpiresAt is in the past and keeps future-dated ones.
func TestMagicLinkTokenRegistry_DeleteExpired(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	expired := newTestMagicLinkToken("token-expired")
	expired.ExpiresAt = time.Now().Add(-1 * time.Hour)
	_, err := r.Create(ctx, expired)
	c.Assert(err, qt.IsNil)

	future := newTestMagicLinkToken("token-future")
	future.ExpiresAt = time.Now().Add(1 * time.Hour)
	futureCreated, err := r.Create(ctx, future)
	c.Assert(err, qt.IsNil)

	err = r.DeleteExpired(ctx)
	c.Assert(err, qt.IsNil)

	all, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)
	c.Assert(all[0].ID, qt.Equals, futureCreated.ID)
}

func TestMagicLinkTokenRegistry_MarkClaimed_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	created, err := r.Create(ctx, newTestMagicLinkToken("token-mark-happy"))
	c.Assert(err, qt.IsNil)
	c.Assert(created.ClaimedAt, qt.IsNil)

	claimed, err := r.MarkClaimed(ctx, "token-mark-happy")
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue, qt.Commentf("first claim of a live token must win"))

	reloaded, err := r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.IsClaimed(), qt.IsTrue)
}

// TestMagicLinkTokenRegistry_MarkClaimed_Replay pins the single-use contract: a
// second claim of the same token returns (false, nil) so the caller knows the
// link was already burned.
func TestMagicLinkTokenRegistry_MarkClaimed_Replay(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	_, err := r.Create(ctx, newTestMagicLinkToken("token-mark-twice"))
	c.Assert(err, qt.IsNil)

	first, err := r.MarkClaimed(ctx, "token-mark-twice")
	c.Assert(err, qt.IsNil)
	c.Assert(first, qt.IsTrue)

	second, err := r.MarkClaimed(ctx, "token-mark-twice")
	c.Assert(err, qt.IsNil)
	c.Assert(second, qt.IsFalse, qt.Commentf("re-claiming an already-claimed token must not win"))
}

// TestMagicLinkTokenRegistry_MarkClaimed_Expired pins that an expired token can
// never be burned: the claim returns (false, nil) and the row stays unclaimed.
func TestMagicLinkTokenRegistry_MarkClaimed_Expired(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	expired := newTestMagicLinkToken("token-mark-expired")
	expired.ExpiresAt = time.Now().Add(-1 * time.Minute)
	created, err := r.Create(ctx, expired)
	c.Assert(err, qt.IsNil)

	claimed, err := r.MarkClaimed(ctx, "token-mark-expired")
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsFalse)

	reloaded, err := r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.IsClaimed(), qt.IsFalse, qt.Commentf("an expired token must remain unclaimed"))
}

func TestMagicLinkTokenRegistry_MarkClaimed_TokenNotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	claimed, err := r.MarkClaimed(ctx, "no-such-token")
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsFalse)
}

func TestMagicLinkTokenRegistry_MarkClaimed_EmptyToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	claimed, err := r.MarkClaimed(ctx, "")
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	c.Assert(claimed, qt.IsFalse)
}

// TestMagicLinkTokenRegistry_MarkClaimed_ConcurrentExactlyOnce is the core
// single-use regression: many goroutines racing to claim the same live token
// must produce exactly one winner, so the one-time sign-in side effects run
// once.
func TestMagicLinkTokenRegistry_MarkClaimed_ConcurrentExactlyOnce(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewMagicLinkTokenRegistry()

	_, err := r.Create(ctx, newTestMagicLinkToken("token-race"))
	c.Assert(err, qt.IsNil)

	const goroutines = 64
	var winners atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			<-start // line everyone up so the calls actually contend
			claimed, markErr := r.MarkClaimed(ctx, "token-race")
			c.Check(markErr, qt.IsNil)
			if claimed {
				winners.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	c.Assert(int(winners.Load()), qt.Equals, 1, qt.Commentf("exactly one concurrent claim may win"))
}
