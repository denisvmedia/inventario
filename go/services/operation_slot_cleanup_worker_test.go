package services_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// fakeOperationSlotRegistry stubs registry.OperationSlotRegistry for the
// cleanup worker run-loop tests. Only CleanupExpiredSlots is exercised (it
// bumps an atomic counter); every other method is inherited from the
// embedded nil interface and panics if the worker touches it, surfacing
// accidental wide usage.
type fakeOperationSlotRegistry struct {
	registry.OperationSlotRegistry

	cleanupCalls atomic.Int32
}

func (f *fakeOperationSlotRegistry) CleanupExpiredSlots(_ context.Context) (int, error) {
	f.cleanupCalls.Add(1)
	return 0, nil
}

// TestOperationSlotCleanupWorker_CallsCleanupExpiredSlots drives the worker
// through at least one tick with a short interval and asserts
// CleanupExpiredSlots was invoked, then that Stop returns cleanly.
func TestOperationSlotCleanupWorker_CallsCleanupExpiredSlots(t *testing.T) {
	c := qt.New(t)

	reg := &fakeOperationSlotRegistry{}
	worker := services.NewOperationSlotCleanupWorker(reg,
		services.WithOperationSlotCleanupInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	worker.Start(ctx)

	c.Assert(eventually(c, 500*time.Millisecond, func() bool {
		return reg.cleanupCalls.Load() >= 1
	}), qt.IsTrue)

	// Stop returns cleanly (no hang, no panic).
	worker.Stop()
}

// TestOperationSlotCleanupWorker_NilRegistryStartNoOp ensures Start is a safe
// no-op when constructed with a nil registry — no goroutine, no panic.
func TestOperationSlotCleanupWorker_NilRegistryStartNoOp(t *testing.T) {
	worker := services.NewOperationSlotCleanupWorker(nil,
		services.WithOperationSlotCleanupInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Start must not panic; Stop must still return cleanly afterward.
	worker.Start(ctx)
	worker.Stop()
}

// TestOperationSlotCleanupWorker_SoftPauseSkipsCleanup verifies the #1308
// soft-pause contract: while the checker reports the worker type as paused,
// CleanupExpiredSlots is never called; flipping the checker to unpaused lets
// the next tick run the cleanup.
func TestOperationSlotCleanupWorker_SoftPauseSkipsCleanup(t *testing.T) {
	c := qt.New(t)

	reg := &fakeOperationSlotRegistry{}
	pause := &fakePauseChecker{}
	pause.paused.Store(true)

	worker := services.NewOperationSlotCleanupWorker(reg,
		services.WithOperationSlotCleanupInterval(10*time.Millisecond),
		services.WithOperationSlotCleanupPauseController(pause),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop()

	// Let several ticks elapse while paused — CleanupExpiredSlots must stay at 0.
	c.Assert(consistently(100*time.Millisecond, func() bool {
		return reg.cleanupCalls.Load() == 0
	}), qt.IsTrue)

	// Resume: the next tick must run the cleanup.
	pause.paused.Store(false)
	c.Assert(eventually(c, 500*time.Millisecond, func() bool {
		return reg.cleanupCalls.Load() >= 1
	}), qt.IsTrue)
}
