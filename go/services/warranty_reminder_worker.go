// WarrantyReminderWorker mirrors StorageQuotaReminderWorker /
// LoanReminderWorker / MaintenanceReminderWorker by design — same
// Start/Stop/run/tick lifecycle. Each worker still owns its own
// Prometheus counters + threshold label set + clock-injection knob,
// so a shared generic base would erase those per-worker specifics
// for negligible LOC savings.
//
//nolint:dupl // intentional symmetry with storage / loan / maintenance workers
package services

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/denisvmedia/inventario/models"
)

const defaultWarrantyReminderInterval = 1 * time.Hour

// Prometheus counters for the warranty reminder worker. Registered
// lazily at package init via promauto against the default registry.
//
// Labels:
//   - threshold: "60", "30", or "7" — matches WarrantyReminderThreshold.
//
// `…_failures_total` is unlabelled because every failure is logged with
// the threshold tag and it is not useful to slice the counter by
// reason — the operator goes to logs once they see the gauge tick.
var (
	warrantyRemindersSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_warranty_reminders_sent_total",
		Help: "Number of warranty reminder emails enqueued, partitioned by threshold (60/30/7 days).",
	}, []string{"threshold"})
	warrantyReminderFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_warranty_reminder_failures_total",
		Help: "Number of per-commodity warranty reminder failures (logged; will be retried next tick).",
	})
)

// WarrantyReminderWorker periodically runs WarrantyReminderService.
// Mirrors GroupPurgeWorker — single goroutine driven by a ticker, with
// a best-effort graceful stop.
type WarrantyReminderWorker struct {
	service  *WarrantyReminderService
	interval time.Duration
	clock    func() time.Time
	pause    PauseChecker
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// WarrantyReminderOption customizes a WarrantyReminderWorker.
type WarrantyReminderOption func(*warrantyReminderOptions)

type warrantyReminderOptions struct {
	interval time.Duration
	clock    func() time.Time
	pause    PauseChecker
}

// WithWarrantyReminderInterval overrides the default tick cadence.
// Non-positive values are ignored — the configured default is kept.
func WithWarrantyReminderInterval(d time.Duration) WarrantyReminderOption {
	return func(o *warrantyReminderOptions) {
		if d > 0 {
			o.interval = d
		}
	}
}

// WithWarrantyReminderClock overrides the now-source the worker hands
// to RemindOnce. Used by tests to pin the clock; production passes
// time.Now indirectly via the default.
func WithWarrantyReminderClock(now func() time.Time) WarrantyReminderOption {
	return func(o *warrantyReminderOptions) {
		if now != nil {
			o.clock = now
		}
	}
}

// WithWarrantyReminderPauseController wires the soft-pause controller so
// the worker skips its sweep while the warranty-reminder worker type is
// paused (#1308). A nil checker leaves the worker unpaused.
func WithWarrantyReminderPauseController(pc PauseChecker) WarrantyReminderOption {
	return func(o *warrantyReminderOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// NewWarrantyReminderWorker constructs the worker. The default cadence
// is one hour — the threshold windows are days, not minutes, so a
// longer cadence wastes ticks; a shorter one risks flapping at
// midnight UTC.
func NewWarrantyReminderWorker(service *WarrantyReminderService, opts ...WarrantyReminderOption) *WarrantyReminderWorker {
	options := warrantyReminderOptions{
		interval: defaultWarrantyReminderInterval,
		clock:    time.Now,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &WarrantyReminderWorker{
		service:  service,
		interval: options.interval,
		clock:    options.clock,
		pause:    options.pause,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the goroutine. No-op if no service is configured.
func (w *WarrantyReminderWorker) Start(ctx context.Context) {
	if w.service == nil {
		slog.Warn("WarrantyReminderWorker: no service configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.Info("Warranty reminder worker started", "interval", w.interval)
}

// Stop signals the worker and waits for the goroutine to exit.
func (w *WarrantyReminderWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Warranty reminder worker stopped")
}

func (w *WarrantyReminderWorker) run(ctx context.Context) {
	// Run once at startup so the first tick doesn't have to wait the
	// full interval after a deploy. Production cadence is hourly, so
	// the latency cost of waiting a full hour is real.
	w.tick(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick runs a single sweep. The service returns a
// WarrantyReminderStats with the per-threshold breakdown so the
// worker can emit one Prometheus series per threshold value
// (matching the documented label set: 60 / 30 / 7).
func (w *WarrantyReminderWorker) tick(ctx context.Context) {
	// Soft-pause (#1308): skip the sweep while paused. The ticker keeps
	// running so resuming takes effect on the next tick without a restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeWarrantyReminder) {
		return
	}

	stats, err := w.service.RemindOnce(ctx, w.clock())
	if err != nil {
		slog.Error("Warranty reminder sweep failed", "error", err)
		return
	}
	if stats.Failed > 0 {
		warrantyReminderFailuresTotal.Add(float64(stats.Failed))
	}
	for threshold, count := range stats.SentByThreshold {
		if count > 0 {
			warrantyRemindersSentTotal.
				WithLabelValues(strconv.Itoa(int(threshold))).
				Add(float64(count))
		}
	}
	if total := stats.Sent(); total > 0 {
		slog.Info("Warranty reminder sweep completed",
			"reminders_sent", total,
			"failed", stats.Failed,
		)
	} else {
		slog.Debug("Warranty reminder sweep completed",
			"reminders_sent", 0,
			"failed", stats.Failed,
		)
	}
}
