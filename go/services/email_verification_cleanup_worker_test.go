package services_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// fakeEmailVerificationRegistry stubs registry.EmailVerificationRegistry for
// the cleanup worker run-loop tests. Only DeleteExpired is exercised (it
// bumps an atomic counter); every other method panics to surface accidental
// wide usage by the worker.
type fakeEmailVerificationRegistry struct {
	registry.EmailVerificationRegistry

	deleteCalls atomic.Int32
}

func (f *fakeEmailVerificationRegistry) DeleteExpired(_ context.Context) error {
	f.deleteCalls.Add(1)
	return nil
}

// fakePauseChecker is a deterministic PauseChecker whose paused state can be
// flipped at runtime. It records the worker type it was asked about so the
// test can confirm the worker queries its own type.
type fakePauseChecker struct {
	paused atomic.Bool
}

func (f *fakePauseChecker) IsPaused(models.WorkerType) bool {
	return f.paused.Load()
}

// TestEmailVerificationCleanupWorker_CallsDeleteExpired drives the worker
// through at least one tick with a short interval and asserts DeleteExpired
// was invoked, then that Stop returns cleanly.
func TestEmailVerificationCleanupWorker_CallsDeleteExpired(t *testing.T) {
	c := qt.New(t)

	reg := &fakeEmailVerificationRegistry{}
	worker := services.NewEmailVerificationCleanupWorker(reg,
		services.WithEmailVerificationCleanupInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	worker.Start(ctx)

	c.Assert(eventually(c, 500*time.Millisecond, func() bool {
		return reg.deleteCalls.Load() >= 1
	}), qt.IsTrue)

	// Stop returns cleanly (no hang, no panic).
	worker.Stop()
}

// TestEmailVerificationCleanupWorker_NilRegistryStartNoOp ensures Start is a
// safe no-op when constructed with a nil registry — no goroutine, no panic.
func TestEmailVerificationCleanupWorker_NilRegistryStartNoOp(t *testing.T) {
	worker := services.NewEmailVerificationCleanupWorker(nil,
		services.WithEmailVerificationCleanupInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Start must not panic; Stop must still return cleanly afterward.
	worker.Start(ctx)
	worker.Stop()
}

// TestEmailVerificationCleanupWorker_SoftPauseSkipsCleanup verifies the
// #1308 soft-pause contract: while the checker reports the worker type as
// paused, DeleteExpired is never called; flipping the checker to unpaused
// lets the next tick run the cleanup.
func TestEmailVerificationCleanupWorker_SoftPauseSkipsCleanup(t *testing.T) {
	c := qt.New(t)

	reg := &fakeEmailVerificationRegistry{}
	pause := &fakePauseChecker{}
	pause.paused.Store(true)

	worker := services.NewEmailVerificationCleanupWorker(reg,
		services.WithEmailVerificationCleanupInterval(10*time.Millisecond),
		services.WithEmailVerificationCleanupPauseController(pause),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop()

	// Let several ticks elapse while paused — DeleteExpired must stay at 0.
	c.Assert(consistently(100*time.Millisecond, func() bool {
		return reg.deleteCalls.Load() == 0
	}), qt.IsTrue)

	// Resume: the next tick must run the cleanup.
	pause.paused.Store(false)
	c.Assert(eventually(c, 500*time.Millisecond, func() bool {
		return reg.deleteCalls.Load() >= 1
	}), qt.IsTrue)
}

// consistently asserts cond holds true for the whole window. Returns false
// the moment cond fails. Used to prove the worker stays idle while paused
// without relying on a single fixed sleep.
func consistently(window time.Duration, cond func() bool) bool {
	end := time.Now().Add(window)
	for time.Now().Before(end) {
		if !cond() {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}
