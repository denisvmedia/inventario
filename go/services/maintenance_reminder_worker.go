// MaintenanceReminderWorker mirrors WarrantyReminderWorker /
// LoanReminderWorker / StorageQuotaReminderWorker by design — same
// Start/Stop/run/tick lifecycle. Each worker still owns its own
// Prometheus counters + threshold label set + clock-injection knob,
// so a shared generic base would erase those per-worker specifics
// for negligible LOC savings.
//
//nolint:dupl // intentional symmetry with warranty / loan / storage workers
package services

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const defaultMaintenanceReminderInterval = 1 * time.Hour

// Prometheus counters for the maintenance reminder worker (#1368).
// Labels:
//   - threshold: "14", "7", "1", "0" — matches
//     MaintenanceReminderThreshold's int value. "0" is the overdue
//     sentinel.
var (
	maintenanceRemindersSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_maintenance_reminders_sent_total",
		Help: "Number of maintenance reminder emails enqueued, partitioned by threshold (14/7/1 days remaining, or 0 for overdue).",
	}, []string{"threshold"})
	maintenanceReminderFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_maintenance_reminder_failures_total",
		Help: "Number of per-schedule maintenance reminder failures (logged; will be retried next tick).",
	})
)

// MaintenanceReminderWorker periodically runs MaintenanceReminderService.
// Mirrors WarrantyReminderWorker.
type MaintenanceReminderWorker struct {
	service  *MaintenanceReminderService
	interval time.Duration
	clock    func() time.Time
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// MaintenanceReminderOption customizes a MaintenanceReminderWorker.
type MaintenanceReminderOption func(*maintenanceReminderOptions)

type maintenanceReminderOptions struct {
	interval time.Duration
	clock    func() time.Time
}

// WithMaintenanceReminderInterval overrides the default tick cadence.
func WithMaintenanceReminderInterval(d time.Duration) MaintenanceReminderOption {
	return func(o *maintenanceReminderOptions) {
		if d > 0 {
			o.interval = d
		}
	}
}

// WithMaintenanceReminderClock overrides the now-source the worker
// hands to RemindOnce.
func WithMaintenanceReminderClock(now func() time.Time) MaintenanceReminderOption {
	return func(o *maintenanceReminderOptions) {
		if now != nil {
			o.clock = now
		}
	}
}

func NewMaintenanceReminderWorker(service *MaintenanceReminderService, opts ...MaintenanceReminderOption) *MaintenanceReminderWorker {
	options := maintenanceReminderOptions{
		interval: defaultMaintenanceReminderInterval,
		clock:    time.Now,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &MaintenanceReminderWorker{
		service:  service,
		interval: options.interval,
		clock:    options.clock,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the goroutine. No-op if no service is configured.
func (w *MaintenanceReminderWorker) Start(ctx context.Context) {
	if w.service == nil {
		slog.Warn("MaintenanceReminderWorker: no service configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.Info("Maintenance reminder worker started", "interval", w.interval)
}

// Stop signals the worker and waits for the goroutine to exit.
func (w *MaintenanceReminderWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Maintenance reminder worker stopped")
}

func (w *MaintenanceReminderWorker) run(ctx context.Context) {
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

func (w *MaintenanceReminderWorker) tick(ctx context.Context) {
	stats, err := w.service.RemindOnce(ctx, w.clock())
	if err != nil {
		slog.Error("Maintenance reminder sweep failed", "error", err)
		return
	}
	if stats.Failed > 0 {
		maintenanceReminderFailuresTotal.Add(float64(stats.Failed))
	}
	for threshold, count := range stats.SentByThreshold {
		if count > 0 {
			maintenanceRemindersSentTotal.
				WithLabelValues(strconv.Itoa(int(threshold))).
				Add(float64(count))
		}
	}
	if total := stats.Sent(); total > 0 {
		slog.Info("Maintenance reminder sweep completed",
			"reminders_sent", total,
			"failed", stats.Failed,
		)
	} else {
		slog.Debug("Maintenance reminder sweep completed",
			"reminders_sent", 0,
			"failed", stats.Failed,
		)
	}
}
