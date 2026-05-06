package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// WarrantyReminderOption customizes a WarrantyReminderWorker.
type WarrantyReminderOption func(*warrantyReminderOptions)

type warrantyReminderOptions struct {
	interval time.Duration
	clock    func() time.Time
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

// tick runs a single sweep. The result of RemindOnce is broken down
// per-threshold for the counter — the service already returned a
// summed (sent, failed) pair, but we don't count per-threshold
// emissions inside the service since that would couple it to the
// metrics package. Re-deriving here keeps the service unit-testable
// without prometheus collectors.
func (w *WarrantyReminderWorker) tick(ctx context.Context) {
	sent, failed, err := w.service.RemindOnce(ctx, w.clock())
	if err != nil {
		slog.Error("Warranty reminder sweep failed", "error", err)
		return
	}
	if failed > 0 {
		warrantyReminderFailuresTotal.Add(float64(failed))
	}
	// Counter is incremented as `sent` — without per-threshold
	// breakdown the worker keeps a single label value "any" so the
	// counter exists and dashboards aren't surprised by a missing
	// series. Per-threshold accounting can be added when we expose the
	// service's threshold list externally.
	if sent > 0 {
		warrantyRemindersSentTotal.WithLabelValues("any").Add(float64(sent))
		slog.Info("Warranty reminder sweep completed",
			"reminders_sent", sent,
			"failed", failed,
		)
	} else {
		slog.Debug("Warranty reminder sweep completed",
			"reminders_sent", 0,
			"failed", failed,
		)
	}
}
