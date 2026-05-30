package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

func TestInMemoryImpersonationStore_PutGetDelete(t *testing.T) {
	c := qt.New(t)
	store := services.NewInMemoryImpersonationStore()
	ctx := context.Background()

	slot := services.ImpersonationSlot{
		JTI:            "jti-1",
		OperatorKind:   services.ImpersonationOperatorBackoffice,
		OperatorUserID: "admin-1",
		TargetUserID:   "target-1",
		TargetTenantID: "tenant-1",
		StartedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(30 * time.Minute),
	}

	c.Assert(store.Put(ctx, slot), qt.IsNil)

	got, err := store.Get(ctx, "jti-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got.OperatorUserID, qt.Equals, "admin-1")
	c.Assert(got.OperatorKind, qt.Equals, services.ImpersonationOperatorBackoffice)
	c.Assert(got.TargetUserID, qt.Equals, "target-1")

	c.Assert(store.Delete(ctx, "jti-1"), qt.IsNil)

	_, err = store.Get(ctx, "jti-1")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestInMemoryImpersonationStore_GetMissingReturnsNotFound(t *testing.T) {
	c := qt.New(t)
	store := services.NewInMemoryImpersonationStore()

	_, err := store.Get(context.Background(), "never-recorded")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestInMemoryImpersonationStore_DeleteIsIdempotent(t *testing.T) {
	c := qt.New(t)
	store := services.NewInMemoryImpersonationStore()

	// Deleting a slot that was never recorded must not error — a double
	// `end` call relies on this.
	c.Assert(store.Delete(context.Background(), "missing"), qt.IsNil)
}

func TestInMemoryImpersonationStore_ExpiredSlotIsPruned(t *testing.T) {
	c := qt.New(t)
	store := services.NewInMemoryImpersonationStore()
	ctx := context.Background()

	expired := services.ImpersonationSlot{
		JTI:       "expired-jti",
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	c.Assert(store.Put(ctx, expired), qt.IsNil)

	// An expired slot is pruned lazily on the next access — Get reports
	// it as not found rather than returning a stale session.
	_, err := store.Get(ctx, "expired-jti")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestInMemoryAuthRateLimiter_CheckImpersonationAttempt(t *testing.T) {
	c := qt.New(t)
	lim := services.NewInMemoryAuthRateLimiter()
	ctx := context.Background()
	adminID := "admin-1"

	// The configured impersonation limit is 10/hour.
	for range 10 {
		res, err := lim.CheckImpersonationAttempt(ctx, adminID)
		c.Assert(err, qt.IsNil)
		c.Assert(res.Allowed, qt.IsTrue)
		c.Assert(res.Limit, qt.Equals, 10)
	}

	res, err := lim.CheckImpersonationAttempt(ctx, adminID)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsFalse)
	c.Assert(res.Remaining, qt.Equals, 0)
}
