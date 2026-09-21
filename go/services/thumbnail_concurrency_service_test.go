package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// The per-user slot limit is what stops one account's upload burst from
// occupying every thumbnail worker. #2114 N7 flagged thumbnail generation;
// AcquireSlot, ReleaseSlot and GetUserSlots were the last of this service at
// 0%.
//
// These are thin wrappers, which is exactly why they are worth a test: a
// wrapper that forwards the wrong limit or the wrong duration leaves the
// guard in place and ineffective, and nothing downstream notices.

func TestThumbnailConcurrency_LimitIsPerUserAndEnforced(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := services.NewThumbnailConcurrencyService(memory.NewFactorySet(), 2, time.Minute)

	first, err := svc.AcquireSlot(ctx, "user-1", "job-1")
	c.Assert(err, qt.IsNil)
	c.Assert(first, qt.IsNotNil)

	second, err := svc.AcquireSlot(ctx, "user-1", "job-2")
	c.Assert(err, qt.IsNil)
	c.Assert(second, qt.IsNotNil)

	// The configured maximum is the maximum: a third is refused rather than
	// queued or silently granted.
	third, err := svc.AcquireSlot(ctx, "user-1", "job-3")
	c.Assert(err, qt.ErrorIs, registry.ErrResourceLimitExceeded)
	c.Check(third, qt.IsNil)

	// Another user is unaffected — the limit is per account, not global,
	// or one busy user would stall everyone.
	other, err := svc.AcquireSlot(ctx, "user-2", "job-4")
	c.Assert(err, qt.IsNil)
	c.Assert(other, qt.IsNotNil)
}

func TestThumbnailConcurrency_ReleasingFreesTheSlot(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := services.NewThumbnailConcurrencyService(memory.NewFactorySet(), 1, time.Minute)

	_, err := svc.AcquireSlot(ctx, "user-1", "job-1")
	c.Assert(err, qt.IsNil)

	_, err = svc.AcquireSlot(ctx, "user-1", "job-2")
	c.Assert(err, qt.ErrorIs, registry.ErrResourceLimitExceeded)

	c.Assert(svc.ReleaseSlot(ctx, "user-1", "job-1"), qt.IsNil)

	// A worker that finishes gives the slot back; without this the limit
	// becomes a lifetime quota rather than a concurrency cap.
	_, err = svc.AcquireSlot(ctx, "user-1", "job-2")
	c.Assert(err, qt.IsNil)
}

func TestThumbnailConcurrency_GetUserSlotsReportsWhatIsHeld(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	svc := services.NewThumbnailConcurrencyService(memory.NewFactorySet(), 3, time.Minute)

	empty, err := svc.GetUserSlots(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Check(empty, qt.HasLen, 0)

	_, err = svc.AcquireSlot(ctx, "user-1", "job-1")
	c.Assert(err, qt.IsNil)
	_, err = svc.AcquireSlot(ctx, "user-1", "job-2")
	c.Assert(err, qt.IsNil)
	_, err = svc.AcquireSlot(ctx, "user-2", "job-3")
	c.Assert(err, qt.IsNil)

	held, err := svc.GetUserSlots(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(held, qt.HasLen, 2)
	for _, slot := range held {
		c.Check(slot.UserID, qt.Equals, "user-1")
	}
}

// A worker that dies without releasing must not hold its slot forever, or
// the user's limit ratchets down to zero with no way back.
func TestThumbnailConcurrency_ExpiredSlotsAreReclaimed(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// A duration already in the past makes every slot expired on sight.
	svc := services.NewThumbnailConcurrencyService(memory.NewFactorySet(), 1, -time.Minute)

	_, err := svc.AcquireSlot(ctx, "user-1", "job-1")
	c.Assert(err, qt.IsNil)

	_, err = svc.AcquireSlot(ctx, "user-1", "job-2")
	c.Assert(err, qt.IsNil, qt.Commentf("an expired slot still counted against the limit"))

	c.Assert(svc.CleanupExpiredSlots(ctx), qt.IsNil)
}
