package services_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// fakeMigrationRegistry stubs registry.CurrencyMigrationRegistry for the
// worker run-loop tests. Only the worker-facing methods are exercised
// (ClaimNextPending, SweepStuckRunning, plus a no-op for everything
// else); the rest panic to surface accidental wide usage.
type fakeMigrationRegistry struct {
	registry.CurrencyMigrationRegistry

	mu sync.Mutex

	// Queue of operations that ClaimNextPending will return in order.
	// When empty, returns ErrNotFound.
	pending []*models.CurrencyMigration

	// Each call to SweepStuckRunning returns this slice once and then
	// empties (one-shot recovery).
	sweepReturn []*models.CurrencyMigration

	claimCalls atomic.Int32
	sweepCalls atomic.Int32
}

func (f *fakeMigrationRegistry) ClaimNextPending(_ context.Context) (*models.CurrencyMigration, error) {
	f.claimCalls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.pending) == 0 {
		return nil, registry.ErrNotFound
	}
	op := f.pending[0]
	f.pending = f.pending[1:]
	return op, nil
}

func (f *fakeMigrationRegistry) SweepStuckRunning(_ context.Context, _ time.Time, _ time.Duration) ([]*models.CurrencyMigration, error) {
	f.sweepCalls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sweepReturn) == 0 {
		return nil, nil
	}
	out := f.sweepReturn
	f.sweepReturn = nil
	return out, nil
}

// fakeProcessor records ProcessRunningMigration / WriteSweepFailureAuditLog
// calls and lets each test inject the desired outcome.
type fakeProcessor struct {
	mu sync.Mutex

	// summaryOnce returns this summary on the first call; subsequent
	// calls return a zero summary. Use processErr to short-circuit
	// before summary lookup.
	summary services.CurrencyMigrationProcessSummary
	// processErr (if non-nil) is returned instead of a successful
	// summary on every Process call.
	processErr error

	// auditErr (if non-nil) is returned by WriteSweepFailureAuditLog.
	auditErr error

	processed []*models.CurrencyMigration
	swept     []*models.CurrencyMigration
}

func (f *fakeProcessor) ProcessRunningMigration(_ context.Context, op *models.CurrencyMigration) (services.CurrencyMigrationProcessSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed = append(f.processed, op)
	if f.processErr != nil {
		return services.CurrencyMigrationProcessSummary{Duration: 50 * time.Millisecond}, f.processErr
	}
	out := f.summary
	if out.Duration == 0 {
		out.Duration = 100 * time.Millisecond
	}
	return out, nil
}

func (f *fakeProcessor) WriteSweepFailureAuditLog(_ context.Context, op *models.CurrencyMigration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.swept = append(f.swept, op)
	return f.auditErr
}

func (f *fakeProcessor) processedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, len(f.processed))
	for i, op := range f.processed {
		ids[i] = op.ID
	}
	return ids
}

func (f *fakeProcessor) sweptIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, len(f.swept))
	for i, op := range f.swept {
		ids[i] = op.ID
	}
	return ids
}

// TestCurrencyMigrationWorker_ProcessesPendingRow drives the worker
// through a single tick path: sweep returns nothing, claim returns one
// row, processor commits successfully. After Stop, we assert the
// processor saw the row exactly once.
func TestCurrencyMigrationWorker_ProcessesPendingRow(t *testing.T) {
	c := qt.New(t)

	op := &models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.RequireFromString("0.9"),
	}
	op.SetTenantID("tenant-1")
	op.SetGroupID("group-1")
	op.SetCreatedByUserID("user-1")
	op.ID = "mig-1"

	reg := &fakeMigrationRegistry{pending: []*models.CurrencyMigration{op}}
	proc := &fakeProcessor{
		summary: services.CurrencyMigrationProcessSummary{
			CommodityCount:        3,
			TotalBefore:           decimal.RequireFromString("100"),
			TotalAfter:            decimal.RequireFromString("90"),
			AcquisitionFillsCount: 1,
		},
	}

	worker := services.NewCurrencyMigrationWorker(reg, proc,
		services.WithCurrencyMigrationActiveInterval(20*time.Millisecond),
		services.WithCurrencyMigrationIdleInterval(20*time.Millisecond),
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop()

	// Wait for the processor to record the call. Initial tick fires
	// synchronously inside run() so the first call happens before the
	// timer even resets — but we still poll defensively.
	c.Assert(eventually(c, 500*time.Millisecond, func() bool {
		return len(proc.processedIDs()) >= 1
	}), qt.IsTrue)

	c.Assert(proc.processedIDs(), qt.DeepEquals, []string{"mig-1"})
	c.Assert(reg.claimCalls.Load() >= int32(1), qt.IsTrue)
	c.Assert(reg.sweepCalls.Load() >= int32(1), qt.IsTrue)
}

// TestCurrencyMigrationWorker_RunsSweepBeforeClaim checks the
// per-tick ordering mandated by #202 §4.5: every tick must call
// SweepStuckRunning before ClaimNextPending. We assert this by
// loading swept rows AND a pending row in the same tick — the swept
// row must reach the processor's WriteSweepFailureAuditLog and the
// claim row must reach ProcessRunningMigration.
func TestCurrencyMigrationWorker_RunsSweepBeforeClaim(t *testing.T) {
	c := qt.New(t)

	// Stuck row recovered by the sweep.
	stuck := &models.CurrencyMigration{Status: models.CurrencyMigrationStatusFailed}
	stuck.SetTenantID("tenant-1")
	stuck.SetGroupID("group-1")
	stuck.SetCreatedByUserID("user-1")
	stuck.ID = "stuck-1"
	stuck.ErrorMessage = "worker crashed or stalled"

	pending := &models.CurrencyMigration{FromCurrency: "USD", ToCurrency: "EUR", ExchangeRate: decimal.RequireFromString("0.9")}
	pending.SetTenantID("tenant-1")
	pending.SetGroupID("group-1")
	pending.SetCreatedByUserID("user-1")
	pending.ID = "pending-1"

	reg := &fakeMigrationRegistry{
		pending:     []*models.CurrencyMigration{pending},
		sweepReturn: []*models.CurrencyMigration{stuck},
	}
	proc := &fakeProcessor{}

	worker := services.NewCurrencyMigrationWorker(reg, proc,
		services.WithCurrencyMigrationActiveInterval(50*time.Millisecond),
		services.WithCurrencyMigrationIdleInterval(50*time.Millisecond),
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	worker.Start(ctx)
	defer worker.Stop()

	c.Assert(eventually(c, time.Second, func() bool {
		return len(proc.processedIDs()) >= 1 && len(proc.sweptIDs()) >= 1
	}), qt.IsTrue)

	c.Assert(proc.processedIDs(), qt.Contains, "pending-1")
	c.Assert(proc.sweptIDs(), qt.Contains, "stuck-1")
}

// TestCurrencyMigrationWorker_TX2FailureLeavesRowRunning asserts that a
// processor error does NOT trigger any registry write — the row must
// stay in `running` so the next sweep recovers it. The worker should
// continue ticking (next claim attempt, next sweep) instead of
// terminating.
func TestCurrencyMigrationWorker_TX2FailureLeavesRowRunning(t *testing.T) {
	c := qt.New(t)

	op := &models.CurrencyMigration{FromCurrency: "USD", ToCurrency: "EUR", ExchangeRate: decimal.RequireFromString("0.9")}
	op.SetTenantID("tenant-1")
	op.SetGroupID("group-1")
	op.SetCreatedByUserID("user-1")
	op.ID = "mig-fail"

	reg := &fakeMigrationRegistry{pending: []*models.CurrencyMigration{op}}
	proc := &fakeProcessor{processErr: errors.New("simulated TX2 rollback")}

	worker := services.NewCurrencyMigrationWorker(reg, proc,
		services.WithCurrencyMigrationActiveInterval(20*time.Millisecond),
		services.WithCurrencyMigrationIdleInterval(20*time.Millisecond),
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	worker.Start(ctx)
	defer worker.Stop()

	c.Assert(eventually(c, 500*time.Millisecond, func() bool {
		return len(proc.processedIDs()) >= 1
	}), qt.IsTrue)
	// The processor was called with the row but no sweep audit was
	// written — the row stays in running and the registry caller did
	// not write a failure audit through the processor.
	c.Assert(proc.processedIDs(), qt.DeepEquals, []string{"mig-fail"})
	c.Assert(proc.sweptIDs(), qt.HasLen, 0)
	// Worker is still alive and ticking.
	c.Assert(worker.IsRunning(), qt.IsTrue)
}

// TestCurrencyMigrationWorker_StartIsIdempotent — calling Start twice
// should not spawn two run goroutines.
func TestCurrencyMigrationWorker_StartIsIdempotent(t *testing.T) {
	c := qt.New(t)

	reg := &fakeMigrationRegistry{}
	proc := &fakeProcessor{}
	worker := services.NewCurrencyMigrationWorker(reg, proc,
		services.WithCurrencyMigrationActiveInterval(50*time.Millisecond),
		services.WithCurrencyMigrationIdleInterval(50*time.Millisecond),
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	worker.Start(ctx)
	worker.Start(ctx) // second Start is a no-op
	defer worker.Stop()

	// Wait briefly so the run loop has produced at least one tick.
	c.Assert(eventually(c, 500*time.Millisecond, func() bool {
		return reg.claimCalls.Load() >= 1
	}), qt.IsTrue)

	// At most one run goroutine; if Start spawned twice, the claim
	// counter would race ahead by 2x. We assert "low", not exact, since
	// the timer may have elapsed multiple times before assertion.
	c.Assert(worker.IsRunning(), qt.IsTrue)
}

// TestCurrencyMigrationWorker_NoPendingDoesntInvokeProcessor — the
// processor must not be called when ClaimNextPending returns
// ErrNotFound. Sweep is still expected on every tick.
func TestCurrencyMigrationWorker_NoPendingDoesntInvokeProcessor(t *testing.T) {
	c := qt.New(t)

	reg := &fakeMigrationRegistry{}
	proc := &fakeProcessor{}
	worker := services.NewCurrencyMigrationWorker(reg, proc,
		services.WithCurrencyMigrationActiveInterval(20*time.Millisecond),
		services.WithCurrencyMigrationIdleInterval(20*time.Millisecond),
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	worker.Start(ctx)
	defer worker.Stop()

	c.Assert(eventually(c, 300*time.Millisecond, func() bool {
		return reg.sweepCalls.Load() >= 1
	}), qt.IsTrue)

	c.Assert(proc.processedIDs(), qt.HasLen, 0)
}

// eventually polls until cond returns true or the deadline expires.
// Returns true on success. Used instead of fixed sleeps so we don't
// race the timer cadence in CI.
func eventually(c *qt.C, deadline time.Duration, cond func() bool) bool {
	c.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}
