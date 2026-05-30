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

const defaultStorageQuotaReminderInterval = 1 * time.Hour

// Prometheus counters for the storage quota reminder worker.
// Registered lazily at package init via promauto against the default
// registry.
//
// Labels:
//   - threshold: "90" — matches StorageQuotaThreshold percentage.
//
// Failures + resets stay unlabelled because every event is logged
// with the threshold tag and slicing by reason is rarely useful —
// the operator goes to logs once they see the counter tick.
var (
	storageQuotaRemindersSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_storage_quota_reminders_sent_total",
		Help: "Number of storage quota warning emails enqueued, partitioned by threshold percent.",
	}, []string{"threshold"})
	storageQuotaRemindersResetTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_storage_quota_reminders_reset_total",
		Help: "Number of storage quota reminder rows wiped because usage dropped back below the threshold.",
	}, []string{"threshold"})
	storageQuotaReminderFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_storage_quota_reminder_failures_total",
		Help: "Number of per-group storage quota reminder failures (logged; will be retried next tick).",
	})
)

// StorageQuotaReminderWorker periodically runs
// StorageQuotaReminderService. Mirrors WarrantyReminderWorker —
// single goroutine driven by a ticker, with a best-effort graceful
// stop.
type StorageQuotaReminderWorker struct {
	service  *StorageQuotaReminderService
	interval time.Duration
	clock    func() time.Time
	pause    PauseChecker
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// StorageQuotaReminderOption customizes a StorageQuotaReminderWorker.
type StorageQuotaReminderOption func(*storageQuotaReminderOptions)

type storageQuotaReminderOptions struct {
	interval time.Duration
	clock    func() time.Time
	pause    PauseChecker
}

// WithStorageQuotaReminderInterval overrides the default tick
// cadence. Non-positive values are ignored — the configured default
// is kept.
func WithStorageQuotaReminderInterval(d time.Duration) StorageQuotaReminderOption {
	return func(o *storageQuotaReminderOptions) {
		if d > 0 {
			o.interval = d
		}
	}
}

// WithStorageQuotaReminderClock overrides the now-source the worker
// hands to RemindOnce. Used by tests to pin the clock; production
// passes time.Now indirectly via the default.
func WithStorageQuotaReminderClock(now func() time.Time) StorageQuotaReminderOption {
	return func(o *storageQuotaReminderOptions) {
		if now != nil {
			o.clock = now
		}
	}
}

// WithStorageQuotaReminderPauseController wires the soft-pause controller
// so the worker skips its sweep while the storage-quota-reminder worker
// type is paused (#1308). A nil checker leaves the worker unpaused.
func WithStorageQuotaReminderPauseController(pc PauseChecker) StorageQuotaReminderOption {
	return func(o *storageQuotaReminderOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// NewStorageQuotaReminderWorker constructs the worker. The default
// cadence is one hour — quota usage rarely changes faster than that
// and a shorter cadence risks repeated SUM(size_bytes) queries
// across every tenant for no behavioural gain.
func NewStorageQuotaReminderWorker(service *StorageQuotaReminderService, opts ...StorageQuotaReminderOption) *StorageQuotaReminderWorker {
	options := storageQuotaReminderOptions{
		interval: defaultStorageQuotaReminderInterval,
		clock:    time.Now,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &StorageQuotaReminderWorker{
		service:  service,
		interval: options.interval,
		clock:    options.clock,
		pause:    options.pause,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the goroutine. No-op if no service is configured.
func (w *StorageQuotaReminderWorker) Start(ctx context.Context) {
	if w.service == nil {
		slog.Warn("StorageQuotaReminderWorker: no service configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.Info("Storage quota reminder worker started", "interval", w.interval)
}

// Stop signals the worker and waits for the goroutine to exit.
func (w *StorageQuotaReminderWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Storage quota reminder worker stopped")
}

func (w *StorageQuotaReminderWorker) run(ctx context.Context) {
	// Run once at startup so the first tick doesn't have to wait the
	// full interval after a deploy. Same rationale as the warranty
	// reminder worker.
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

// tick runs a single sweep. The service returns per-threshold stats
// so the worker can emit one Prometheus series per threshold value.
func (w *StorageQuotaReminderWorker) tick(ctx context.Context) {
	// Soft-pause (#1308): skip the sweep while paused. The ticker keeps
	// running so resuming takes effect on the next tick without a restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeStorageQuotaReminder) {
		return
	}

	stats, err := w.service.RemindOnce(ctx, w.clock())
	if err != nil {
		slog.Error("Storage quota reminder sweep failed", "error", err)
		return
	}
	if stats.Failed > 0 {
		storageQuotaReminderFailuresTotal.Add(float64(stats.Failed))
	}
	for threshold, count := range stats.SentByThreshold {
		if count > 0 {
			storageQuotaRemindersSentTotal.
				WithLabelValues(strconv.Itoa(int(threshold))).
				Add(float64(count))
		}
	}
	for threshold, count := range stats.ResetByThreshold {
		if count > 0 {
			storageQuotaRemindersResetTotal.
				WithLabelValues(strconv.Itoa(int(threshold))).
				Add(float64(count))
		}
	}
	if total := stats.Sent(); total > 0 || stats.Reset() > 0 {
		slog.Info("Storage quota reminder sweep completed",
			"reminders_sent", total,
			"reminders_reset", stats.Reset(),
			"failed", stats.Failed,
		)
	} else {
		slog.Debug("Storage quota reminder sweep completed",
			"reminders_sent", 0,
			"reminders_reset", 0,
			"failed", stats.Failed,
		)
	}
}
