package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// defaultLoginEventRetention defines how long login_events rows stay in
// the database before the retention worker purges them. 90 days matches
// the default in issue #1379 — enough to surface "did someone try last
// month?" while keeping the table bounded for long-running deployments.
const defaultLoginEventRetention = 90 * 24 * time.Hour

// defaultLoginEventSweepInterval defines how often the retention worker
// looks for expired rows. Once a day is enough for an append-only table
// with a small daily volume; long sweep intervals also keep the burst
// load on the DB to a minimum.
const defaultLoginEventSweepInterval = 24 * time.Hour

// LoginEventRetentionWorker periodically deletes login_events rows
// older than its retention window. The login_events table is append-only
// and grows linearly with login activity; without this worker the table
// would creep towards unbounded growth on a multi-year deployment.
type LoginEventRetentionWorker struct {
	registry      registry.LoginEventRegistry
	retention     time.Duration
	sweepInterval time.Duration
	pause         PauseChecker
	stopCh        chan struct{}
	stopOnce      sync.Once
	wg            sync.WaitGroup
	clock         func() time.Time // override for tests; nil = time.Now
}

// LoginEventRetentionOption customizes a LoginEventRetentionWorker.
type LoginEventRetentionOption func(*loginEventRetentionOptions)

type loginEventRetentionOptions struct {
	retention     time.Duration
	sweepInterval time.Duration
	clock         func() time.Time
	pause         PauseChecker
}

// WithLoginEventRetention overrides the retention window. Non-positive
// values are ignored — the default 90d window is the only safe lower
// bound for a forensic audit trail.
func WithLoginEventRetention(d time.Duration) LoginEventRetentionOption {
	return func(o *loginEventRetentionOptions) {
		if d > 0 {
			o.retention = d
		}
	}
}

// WithLoginEventSweepInterval overrides the sweep cadence. Non-positive
// values are ignored.
func WithLoginEventSweepInterval(d time.Duration) LoginEventRetentionOption {
	return func(o *loginEventRetentionOptions) {
		if d > 0 {
			o.sweepInterval = d
		}
	}
}

// WithLoginEventRetentionClock overrides the wall clock used by the
// worker. Test-only — production callers leave it nil so the worker
// uses time.Now.
func WithLoginEventRetentionClock(fn func() time.Time) LoginEventRetentionOption {
	return func(o *loginEventRetentionOptions) {
		if fn != nil {
			o.clock = fn
		}
	}
}

// WithLoginEventRetentionPauseController wires the soft-pause controller
// so the worker skips its sweep while the login-event-retention worker
// type is paused (#1308). A nil checker leaves the worker unpaused.
func WithLoginEventRetentionPauseController(pc PauseChecker) LoginEventRetentionOption {
	return func(o *loginEventRetentionOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// NewLoginEventRetentionWorker constructs the worker. A nil registry
// produces an instance whose Start is a no-op; callers can wire the
// worker unconditionally and let the registry presence decide whether
// it actually runs (same shape as RefreshTokenCleanupWorker).
func NewLoginEventRetentionWorker(r registry.LoginEventRegistry, opts ...LoginEventRetentionOption) *LoginEventRetentionWorker {
	options := loginEventRetentionOptions{
		retention:     defaultLoginEventRetention,
		sweepInterval: defaultLoginEventSweepInterval,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &LoginEventRetentionWorker{
		registry:      r,
		retention:     options.retention,
		sweepInterval: options.sweepInterval,
		pause:         options.pause,
		stopCh:        make(chan struct{}),
		clock:         options.clock,
	}
}

// Start launches the background goroutine. It is a no-op when the
// registry is nil.
func (w *LoginEventRetentionWorker) Start(ctx context.Context) {
	if w.registry == nil {
		slog.Warn("LoginEventRetentionWorker: no registry configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.Info("Login event retention worker started", "retention", w.retention, "interval", w.sweepInterval)
}

// Stop signals the worker to stop and waits for it to finish.
func (w *LoginEventRetentionWorker) Stop() {
	w.stopOnce.Do(func() { close(w.stopCh) })
	w.wg.Wait()
	slog.Info("Login event retention worker stopped")
}

func (w *LoginEventRetentionWorker) run(ctx context.Context) {
	// Run once immediately so a fresh deployment doesn't wait a full
	// sweepInterval before its first purge.
	w.sweep(ctx)

	ticker := time.NewTicker(w.sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}

func (w *LoginEventRetentionWorker) sweep(ctx context.Context) {
	// Soft-pause (#1308): skip the sweep while paused. The ticker keeps
	// running so resuming takes effect on the next tick without a restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeLoginEventRetention) {
		return
	}

	now := time.Now
	if w.clock != nil {
		now = w.clock
	}
	cutoff := now().Add(-w.retention)
	deleted, err := w.registry.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		slog.Error("Failed to delete old login events", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("Pruned login_events", "deleted", deleted, "cutoff", cutoff)
	}
}
