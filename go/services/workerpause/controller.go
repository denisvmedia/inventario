// Package workerpause hosts the runtime side of the background-worker
// soft-pause control plane (#1308). The data layer (models.WorkerControl
// + registry.WorkerControlRegistry) records which worker types an
// operator has paused; this package turns those rows into a lock-free,
// hot-path-cheap IsPaused check that every worker's claim phase consults
// each tick.
//
// The design splits the slow path (a DB List poll, once per
// refreshInterval) from the fast path (a single atomic.Bool read per
// worker tick) so a paused/running flip costs the workers nothing at
// steady state and resuming is near-instant without a process restart.
package workerpause

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/denisvmedia/inventario/models"
)

// defaultRefreshInterval is how often the controller re-polls the
// WorkerControlRegistry. 10s keeps a paused/resumed flip visible to the
// workers within a few seconds while adding negligible DB load (one
// indexed List per interval). Overridable via WithRefreshInterval.
const defaultRefreshInterval = 10 * time.Second

// workerPausedGauge exposes the live pause state per worker type on
// /metrics. Initialised to 0 for every canonical type at the first
// refresh so the series always exist for dashboards/alerts, then driven
// to 1/0 as operators pause/resume. Package-level + promauto so it
// registers against the default registry exactly like the per-worker
// metrics do.
var workerPausedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "inventario_worker_paused",
	Help: "1 when the named background worker type is soft-paused (#1308), else 0.",
}, []string{"type"})

// PausedGaugeFor returns the live inventario_worker_paused gauge series
// for worker type t. Exposed for tests (the external _test package can't
// reach the unexported workerPausedGauge directly) so they can assert the
// metric tracks the pause state; not part of the runtime API.
func PausedGaugeFor(t models.WorkerType) prometheus.Gauge {
	return workerPausedGauge.WithLabelValues(string(t))
}

// pauseRegistry is the minimal slice of registry.WorkerControlRegistry
// the controller needs. Declared locally (rather than importing the full
// registry interface) so the polling side depends only on List — the
// Pause/Resume writers live behind the CLI/admin surface, not here.
type pauseRegistry interface {
	List(ctx context.Context) ([]*models.WorkerControl, error)
}

// Controller polls the WorkerControlRegistry on an interval and exposes a
// lock-free IsPaused check the workers consult on their hot path. The
// pause state for each canonical worker type is held in an atomic.Bool so
// reads never contend with the refresh goroutine.
type Controller struct {
	reg             pauseRegistry
	refreshInterval time.Duration

	// states is pre-populated for every models.AllWorkerTypes() at
	// construction and never has keys added/removed afterwards, so it is
	// safe to read concurrently without a lock — only the atomic values
	// mutate. IsPaused for an unknown type returns false.
	states map[models.WorkerType]*atomic.Bool

	// lastLogged tracks the most recently logged paused state per type so
	// transitions are logged exactly once (not on every poll). Guarded by
	// mu because RefreshOnce may be called from both Start's synchronous
	// preflight and the ticker goroutine.
	mu         sync.Mutex
	lastLogged map[models.WorkerType]bool
	// listErrored records whether the previous poll failed, so the
	// fail-safe warning is logged once on the error transition rather than
	// on every failing poll.
	listErrored bool

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// Option customises a Controller.
type Option func(*Controller)

// WithRefreshInterval overrides the registry poll cadence. Non-positive
// values are ignored — the 10s default is kept.
func WithRefreshInterval(d time.Duration) Option {
	return func(c *Controller) {
		if d > 0 {
			c.refreshInterval = d
		}
	}
}

// NewController constructs a Controller over reg. Every canonical worker
// type is pre-registered as not-paused so IsPaused is a pure atomic read
// (no map mutation) on the hot path and unknown types simply return false.
func NewController(reg pauseRegistry, opts ...Option) *Controller {
	c := &Controller{
		reg:             reg,
		refreshInterval: defaultRefreshInterval,
		states:          make(map[models.WorkerType]*atomic.Bool),
		lastLogged:      make(map[models.WorkerType]bool),
		stopCh:          make(chan struct{}),
	}
	for _, t := range models.AllWorkerTypes() {
		c.states[t] = &atomic.Bool{}
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// IsPaused reports whether worker type t is currently soft-paused. It is
// a single lock-free atomic read and returns false for any type not in
// the canonical set (so a worker that passes an unknown type fails open
// and keeps running).
func (c *Controller) IsPaused(t models.WorkerType) bool {
	if c == nil {
		return false
	}
	state, ok := c.states[t]
	if !ok {
		return false
	}
	return state.Load()
}

// Start performs one synchronous RefreshOnce so the pause state is
// correct before any worker ticks, then launches a goroutine that
// re-polls every refreshInterval until Stop or ctx cancellation. The
// initial refresh error (if any) is logged but does not block startup —
// the workers fail open (run) until the first successful poll.
func (c *Controller) Start(ctx context.Context) {
	if c == nil {
		return
	}
	if err := c.RefreshOnce(ctx); err != nil {
		slog.Warn("Worker pause controller: initial refresh failed; workers run until first successful poll", "error", err)
	}
	c.wg.Go(func() {
		c.run(ctx)
	})
	slog.Info("Worker pause controller started", "refresh_interval", c.refreshInterval)
}

// Stop signals the poll goroutine and waits for it to exit. Idempotent.
func (c *Controller) Stop() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	c.wg.Wait()
	slog.Info("Worker pause controller stopped")
}

func (c *Controller) run(ctx context.Context) {
	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			// Errors are swallowed here (already logged inside
			// RefreshOnce) — a transient List failure must not kill the
			// poll loop, and the last-known state is retained meanwhile.
			_ = c.RefreshOnce(ctx)
		}
	}
}

// RefreshOnce reads the current control rows and reconciles the atomic
// pause flags + the Prometheus gauge.
//
// Fail-safe: on a List error the last-known state is RETAINED (atomics are
// not flipped to false) so a DB blip can't silently un-pause a worker an
// operator deliberately stopped. The error is logged once on the
// error transition and returned to the caller.
//
// On success the controller marks every canonical type 1/0 based on rows
// where paused==true AND the type IsValid (unknown/legacy rows are
// ignored), logs each paused/resumed transition exactly once, and
// initialises every canonical gauge series so /metrics always carries the
// full label set.
func (c *Controller) RefreshOnce(ctx context.Context) error {
	rows, err := c.reg.List(ctx)
	if err != nil {
		c.mu.Lock()
		if !c.listErrored {
			c.listErrored = true
			slog.Warn("Worker pause controller: failed to refresh pause state; retaining last-known state", "error", err)
		}
		c.mu.Unlock()
		return err
	}

	paused := make(map[models.WorkerType]bool, len(rows))
	for _, row := range rows {
		if row == nil || !row.Paused {
			continue
		}
		if !row.WorkerType.IsValid() {
			continue
		}
		paused[row.WorkerType] = true
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.listErrored = false

	for t, state := range c.states {
		isPaused := paused[t]
		state.Store(isPaused)
		gaugeValue := 0.0
		if isPaused {
			gaugeValue = 1
		}
		workerPausedGauge.WithLabelValues(string(t)).Set(gaugeValue)

		if isPaused {
			c.logPausedTransition(t, rowFor(rows, t))
		} else {
			c.logResumedTransition(t)
		}
	}

	return nil
}

// logPausedTransition logs "worker paused" exactly once when t flips from
// running (or unknown) to paused versus the last logged value. Called
// with c.mu held.
func (c *Controller) logPausedTransition(t models.WorkerType, row *models.WorkerControl) {
	if prev, seen := c.lastLogged[t]; seen && prev {
		return // already logged as paused
	}
	c.lastLogged[t] = true

	attrs := []any{"type", string(t)}
	if row != nil {
		if row.Reason != nil {
			attrs = append(attrs, "reason", *row.Reason)
		}
		if row.PausedBy != nil {
			attrs = append(attrs, "paused_by", *row.PausedBy)
		}
	}
	slog.Info("worker paused", attrs...)
}

// logResumedTransition logs "worker resumed" exactly once when t flips
// from paused back to running. The initial "everything is running"
// baseline is intentionally silent — only a paused→running flip is
// noteworthy. Called with c.mu held.
func (c *Controller) logResumedTransition(t models.WorkerType) {
	if prev, seen := c.lastLogged[t]; !seen || !prev {
		c.lastLogged[t] = false
		return // never logged as paused → nothing to announce
	}
	c.lastLogged[t] = false
	slog.Info("worker resumed", "type", string(t))
}

// rowFor returns the control row for type t, or nil when none is present.
func rowFor(rows []*models.WorkerControl, t models.WorkerType) *models.WorkerControl {
	for _, row := range rows {
		if row != nil && row.WorkerType == t {
			return row
		}
	}
	return nil
}
