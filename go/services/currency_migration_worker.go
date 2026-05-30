// Package services hosts cross-registry workflow + worker code that the
// /api/v1 handlers call into. This file owns the currency-migration
// background worker introduced in PR 3 of issue #202 (#1552). It is the
// "tick + claim + delegate" coordinator: the heavy SQL — TX2, advisory
// lock, conversion, audit writes — lives in
// registry/postgres.CurrencyMigrationProcessor, behind the small
// CurrencyMigrationProcessor interface this file declares.
package services

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// Default tick cadences (#1552 spec):
//
//   - active: 5s when there is pending work — keeps queue latency low
//     for an admin who just kicked off a migration.
//   - idle:   1m when the last claim returned ErrNotFound — most groups
//     never migrate, so a flat 5s tick wastes connection-pool slots.
//
// Both default values can be overridden via WithCurrencyMigrationActiveInterval
// / WithCurrencyMigrationIdleInterval; the bootstrap layer also exposes
// --currency-migration-interval to keep the operator-facing knob
// single-valued (active interval; idle is derived).
const (
	defaultCurrencyMigrationActiveInterval = 5 * time.Second
	defaultCurrencyMigrationIdleInterval   = 1 * time.Minute
)

// CurrencyMigrationProcessSummary is the per-run aggregate the
// processor returns to the worker after TX2 commits successfully.
// Mirrors postgres.CurrencyMigrationProcessSummary in shape, defined
// here too so the services package doesn't import the postgres package
// directly (which would create a cycle through the registry interfaces).
type CurrencyMigrationProcessSummary struct {
	CommodityCount        int
	TotalBefore           decimal.Decimal
	TotalAfter            decimal.Decimal
	AcquisitionFillsCount int
	Duration              time.Duration
}

// CurrencyMigrationProcessor is the small surface the worker depends on
// for TX2. Implemented by postgres.CurrencyMigrationProcessor; tests can
// substitute a fake so the worker run-loop is exercised without standing
// up postgres.
type CurrencyMigrationProcessor interface {
	// ProcessRunningMigration completes TX2 for an already-claimed
	// running migration. On error the row stays in `running` so the
	// recovery sweep recovers it; on success the row commits as
	// `completed` and the group lock + currency flip happen atomically.
	ProcessRunningMigration(ctx context.Context, op *models.CurrencyMigration) (CurrencyMigrationProcessSummary, error)

	// WriteSweepFailureAuditLog inserts one audit_logs.currency_migration.fail
	// row per swept migration. Best-effort — failures are logged but do
	// not propagate (we'd otherwise risk re-flapping the row through
	// the sweep on every tick).
	WriteSweepFailureAuditLog(ctx context.Context, op *models.CurrencyMigration) error
}

// Prometheus metrics (#1552 spec). Registered lazily at package init via
// promauto against the default registry — same pattern as the warranty
// reminder worker, so the /metrics scrape sees a single coherent
// namespace.
//
// Only currencyMigrationTotal is labelled (status: "completed" | "failed");
// the duration histogram is intentionally TX2-only and unlabelled — recovery
// timings for stuck rows go on a separate stall histogram so the TX2
// latency distribution is not skewed by the 10m+ stall window.
var (
	currencyMigrationTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_currency_migration_total",
		Help: "Number of currency migrations transitioned to a terminal status, partitioned by status (completed|failed).",
	}, []string{"status"})

	currencyMigrationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "inventario_currency_migration_duration_seconds",
		Help: "Wall-clock duration of a single TX2 (conversion + commit) for a successfully completed currency migration. " +
			"Rolled-back attempts and recovery-sweep stalls are NOT observed here — see inventario_currency_migration_stall_seconds.",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 12), // 50ms .. ~204s
	})

	currencyMigrationStallSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "inventario_currency_migration_stall_seconds",
		Help: "Time a currency migration spent in 'running' before the recovery sweep flipped it to 'failed' (started_at → completed_at). " +
			"Distinct from inventario_currency_migration_duration_seconds, which measures the TX2 commit path only.",
		Buckets: []float64{60, 5 * 60, 10 * 60, 30 * 60, 60 * 60, 6 * 60 * 60, 24 * 60 * 60},
	})

	currencyMigrationCommodityCount = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "inventario_currency_migration_commodity_count",
		Help:    "Number of commodities mutated by a single currency migration TX2.",
		Buckets: []float64{1, 10, 50, 100, 500, 1000, 5000, 10000},
	})

	currencyMigrationAcquisitionFillsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_currency_migration_acquisition_fills_total",
		Help: "Number of commodities whose acquisition_price/acquisition_currency were filled (write-once) by a currency migration.",
	})

	currencyMigrationDailyCapRejectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_currency_migration_daily_cap_rejections_total",
		Help: "Number of /currency-migrations start requests rejected because the per-group daily cap (2/day) was reached.",
	})

	currencyMigrationRecoverySweepsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_currency_migration_recovery_sweeps_total",
		Help: "Number of currency migration rows transitioned from running → failed by the periodic recovery sweep.",
	})
)

// CurrencyMigrationDailyCapRejected lets the apiserver bump the
// per-group daily-cap rejection counter without re-importing the
// promauto-registered metric across packages. The /metrics output
// stays a single counter.
func CurrencyMigrationDailyCapRejected() {
	currencyMigrationDailyCapRejectionsTotal.Inc()
}

// CurrencyMigrationWorkerOption customises the worker. Same shape as
// the other go/services workers (functional options on a private
// options struct).
type CurrencyMigrationWorkerOption func(*currencyMigrationWorkerOptions)

type currencyMigrationWorkerOptions struct {
	activeInterval time.Duration
	idleInterval   time.Duration
	clock          func() time.Time
	pause          PauseChecker
}

// WithCurrencyMigrationActiveInterval overrides the cadence used after
// the worker successfully processed (or attempted) a migration. Lower
// values reduce queue latency at the cost of connection-pool churn on
// idle deployments. Non-positive is ignored.
func WithCurrencyMigrationActiveInterval(d time.Duration) CurrencyMigrationWorkerOption {
	return func(o *currencyMigrationWorkerOptions) {
		if d > 0 {
			o.activeInterval = d
		}
	}
}

// WithCurrencyMigrationIdleInterval overrides the cadence used after a
// claim attempt that found no pending work. Defaults to 1m so a quiet
// deployment doesn't burn one tx + advisory-lock slot per 5 seconds.
// Non-positive is ignored.
func WithCurrencyMigrationIdleInterval(d time.Duration) CurrencyMigrationWorkerOption {
	return func(o *currencyMigrationWorkerOptions) {
		if d > 0 {
			o.idleInterval = d
		}
	}
}

// WithCurrencyMigrationClock overrides the clock the worker hands to
// SweepStuckRunning. Tests pin the clock to advance past the stuck
// threshold without sleeping; production passes time.Now indirectly via
// the default.
func WithCurrencyMigrationClock(now func() time.Time) CurrencyMigrationWorkerOption {
	return func(o *currencyMigrationWorkerOptions) {
		if now != nil {
			o.clock = now
		}
	}
}

// WithCurrencyMigrationPauseController wires the soft-pause controller so
// the worker skips claiming new migrations while the currency-migration
// worker type is paused (#1308). The recovery sweep still runs while
// paused — only the new claim is gated. A nil checker leaves the worker
// unpaused.
func WithCurrencyMigrationPauseController(pc PauseChecker) CurrencyMigrationWorkerOption {
	return func(o *currencyMigrationWorkerOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// CurrencyMigrationWorker periodically sweeps stuck running rows,
// claims one pending migration per tick, and hands the row to the
// CurrencyMigrationProcessor for TX2.
//
// The two-tx lifecycle (#202 §4.5):
//
//   - Recovery sweep: every tick BEFORE the claim attempt, plus once at
//     startup. Flips any `running` row whose started_at is older than
//     CurrencyMigrationStuckThreshold (10m, hard-coded in the postgres
//     registry) to `failed`, clearing the group lock signal in the same
//     statement.
//
//   - Claim TX1: registry.ClaimNextPending atomically picks one
//     pending row via SELECT FOR UPDATE SKIP LOCKED + UPDATE → running.
//     After commit, FE polls observe the row as running.
//
//   - Work TX2: the processor takes the inventario_background_worker
//     role, drops a per-group advisory lock, runs the conversion +
//     audit + event emission, flips group_currency, marks the row
//     `completed`, and writes the audit_logs row — atomically.
//
// Metrics, log lines, and stop-channel discipline match the
// warranty-reminder / refresh-token-cleanup workers.
type CurrencyMigrationWorker struct {
	registry  registry.CurrencyMigrationRegistry
	processor CurrencyMigrationProcessor

	activeInterval time.Duration
	idleInterval   time.Duration
	clock          func() time.Time
	pause          PauseChecker

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	mu      sync.Mutex
	running bool
}

// NewCurrencyMigrationWorker constructs the worker. registry is the
// service-mode CurrencyMigrationRegistry (background-worker RLS
// bypass); processor is the TX2 owner. Both are required.
func NewCurrencyMigrationWorker(reg registry.CurrencyMigrationRegistry, processor CurrencyMigrationProcessor, opts ...CurrencyMigrationWorkerOption) *CurrencyMigrationWorker {
	options := currencyMigrationWorkerOptions{
		activeInterval: defaultCurrencyMigrationActiveInterval,
		idleInterval:   defaultCurrencyMigrationIdleInterval,
		clock:          time.Now,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &CurrencyMigrationWorker{
		registry:       reg,
		processor:      processor,
		activeInterval: options.activeInterval,
		idleInterval:   options.idleInterval,
		clock:          options.clock,
		pause:          options.pause,
		stopCh:         make(chan struct{}),
	}
}

// Start launches the worker goroutine. No-op if registry/processor are
// nil (mirrors the warranty reminder worker's "no service configured"
// behaviour) or if Start has already been called.
func (w *CurrencyMigrationWorker) Start(ctx context.Context) {
	if w == nil {
		return
	}
	if w.registry == nil || w.processor == nil {
		slog.Warn("CurrencyMigrationWorker: missing registry or processor, skipping startup")
		return
	}
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	w.wg.Go(func() {
		defer func() {
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
		}()
		w.run(ctx)
	})
	slog.Info("Currency migration worker started",
		"active_interval", w.activeInterval,
		"idle_interval", w.idleInterval,
	)
}

// Stop signals the worker and waits for the loop goroutine to exit.
// Idempotent — stopping twice is a no-op.
func (w *CurrencyMigrationWorker) Stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Currency migration worker stopped")
}

// run is the main loop. Each iteration:
//
//  1. SweepStuckRunning(now-threshold) — flips long-stuck running rows
//     to failed (recovery from a crashed worker). The threshold itself
//     is hard-coded to 10 minutes inside the registry per #202 §4.5.
//
//  2. ClaimNextPending — picks one pending row, flips it to running.
//     Returns ErrNotFound when the queue is empty.
//
//  3. processor.ProcessRunningMigration — runs TX2. On error, the row
//     stays in `running` and the next sweep recovers it; on success,
//     the row commits as `completed` and the group's currency flips.
//
// The next-tick interval is `activeInterval` if a row was claimed
// this iteration (regardless of TX2 outcome — work is in flight so
// we want to drain quickly) and `idleInterval` otherwise.
func (w *CurrencyMigrationWorker) run(ctx context.Context) {
	// Run once at startup so the recovery sweep clears any dangling
	// running row left by a previous process before we even claim,
	// and so the queue is drained eagerly — without the eager tick
	// the worker would idle for `idleInterval` after process boot
	// even when there is pending work.
	startupFound := w.tick(ctx)
	first := w.idleInterval
	if startupFound {
		first = w.activeInterval
	}
	timer := time.NewTimer(first)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-timer.C:
			workWasFound := w.tick(ctx)
			next := w.idleInterval
			if workWasFound {
				next = w.activeInterval
			}
			timer.Reset(next)
		}
	}
}

// tick runs the per-iteration sweep + claim + process pipeline. Returns
// true iff the claim picked up a row (regardless of TX2 outcome) — the
// run loop uses this signal to switch to active cadence.
func (w *CurrencyMigrationWorker) tick(ctx context.Context) bool {
	w.runSweep(ctx)

	// Soft-pause (#1308): the recovery sweep above MUST still run while
	// paused (it recovers crashed-worker stuck rows), but claiming a new
	// migration is gated. Returning false keeps the worker on its idle
	// cadence so a paused worker doesn't spin at the active interval.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeCurrencyMigration) {
		return false
	}

	op, err := w.registry.ClaimNextPending(ctx)
	switch {
	case errors.Is(err, registry.ErrNotFound):
		return false
	case err != nil:
		slog.ErrorContext(ctx, "Currency migration claim failed", "error", err)
		return false
	}

	w.runWork(ctx, op)
	return true
}

// runSweep delegates to the registry's SweepStuckRunning, increments
// metrics for each transitioned row, and writes a per-row failure
// audit_logs entry through the processor. Failures inside the audit
// write are logged and swallowed — re-running the sweep would re-emit
// the audit row anyway, but the migration row is already in `failed`
// state so we can't re-flap.
func (w *CurrencyMigrationWorker) runSweep(ctx context.Context) {
	now := w.clock()
	swept, err := w.registry.SweepStuckRunning(ctx, now, currencyMigrationStuckThreshold)
	if err != nil {
		slog.ErrorContext(ctx, "Currency migration recovery sweep failed", "error", err)
		return
	}
	for _, op := range swept {
		if op == nil {
			continue
		}
		currencyMigrationTotal.WithLabelValues("failed").Inc()
		currencyMigrationRecoverySweepsTotal.Inc()
		// Observe the stall window — the time the row spent in `running`
		// before the sweep flipped it. This is deliberately a separate
		// histogram from currencyMigrationDuration (TX2 commit timings)
		// so a 10m+ stall does not skew the TX2 latency distribution.
		if op.StartedAt != nil && !op.StartedAt.IsZero() && op.CompletedAt != nil && !op.CompletedAt.IsZero() {
			currencyMigrationStallSeconds.Observe(op.CompletedAt.Sub(*op.StartedAt).Seconds())
		}

		if err := w.processor.WriteSweepFailureAuditLog(ctx, op); err != nil {
			slog.WarnContext(ctx, "Currency migration: failed to write recovery audit log",
				"migration_id", op.ID, "group_id", op.GroupID, "error", err,
			)
		}
		slog.WarnContext(ctx, "Currency migration recovered from stall",
			"migration_id", op.ID,
			"group_id", op.GroupID,
			"from", string(op.FromCurrency),
			"to", string(op.ToCurrency),
			"started_at", op.StartedAt,
		)
	}
}

// runWork hands the claimed row to the processor and emits the success
// metrics on commit. On TX2 failure we record the failure log + bump
// the duration histogram with the partial walk, but we do NOT increment
// the `failed` total counter — the row is still in `running`, not
// terminal. The recovery sweep flips it (and bumps the counter) once
// the threshold expires.
func (w *CurrencyMigrationWorker) runWork(ctx context.Context, op *models.CurrencyMigration) {
	slog.InfoContext(ctx, "Processing currency migration",
		"migration_id", op.ID,
		"group_id", op.GroupID,
		"from", string(op.FromCurrency),
		"to", string(op.ToCurrency),
	)

	summary, err := w.processor.ProcessRunningMigration(ctx, op)
	if err != nil {
		// TX2 failed and rolled back. Row is still `running`; the next
		// sweep (or this one — sweep runs BEFORE claim, so on the next
		// tick) will mark it `failed` once the stuck threshold passes.
		// Don't touch the row from here — racing with the sweep would
		// risk emitting two terminal-state events for one migration.
		// Don't observe the partial walk on currencyMigrationDuration
		// either — that histogram is documented as "successful TX2
		// commit only", and counting rolled-back attempts would skew
		// the latency distribution operators read for SLO purposes.
		slog.ErrorContext(ctx, "Currency migration TX2 failed",
			"migration_id", op.ID,
			"group_id", op.GroupID,
			"error", err,
		)
		return
	}

	currencyMigrationTotal.WithLabelValues("completed").Inc()
	currencyMigrationDuration.Observe(summary.Duration.Seconds())
	currencyMigrationCommodityCount.Observe(float64(summary.CommodityCount))
	if summary.AcquisitionFillsCount > 0 {
		currencyMigrationAcquisitionFillsTotal.Add(float64(summary.AcquisitionFillsCount))
	}

	slog.InfoContext(ctx, "Currency migration completed",
		"migration_id", op.ID,
		"group_id", op.GroupID,
		"commodity_count", summary.CommodityCount,
		"total_before", summary.TotalBefore.String(),
		"total_after", summary.TotalAfter.String(),
		"acquisition_fills", summary.AcquisitionFillsCount,
		"duration", summary.Duration.String(),
	)
}

// IsRunning reports whether Start has been called and the loop
// goroutine is alive. Used by tests that need a barrier between
// Start and the first tick.
func (w *CurrencyMigrationWorker) IsRunning() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// currencyMigrationStuckThreshold mirrors postgres.CurrencyMigrationStuckThreshold
// so this package doesn't import postgres directly. The duplication is
// load-bearing per #202 §4.5: the constant is "code, not config" — both
// places reference the same 10-minute window. If you change one, change
// the other in the same PR.
const currencyMigrationStuckThreshold = 10 * time.Minute
