package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go.5x5.cz/inventario/models"
)

const defaultItemReminderInterval = 1 * time.Hour

var (
	itemRemindersSentTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_item_reminders_sent_total",
		Help: "Number of item reminder emails enqueued.",
	})
	itemReminderFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_item_reminder_failures_total",
		Help: "Number of per-reminder item reminder failures (logged; retried next tick).",
	})
)

// ItemReminderWorker periodically runs ItemReminderService. It follows the
// other reminder workers (same Start/Stop/run/tick lifecycle) and has its own
// counters, clock and pause switch, so an operator can pause or watch item
// reminders on their own.
type ItemReminderWorker struct {
	service  *ItemReminderService
	interval time.Duration
	clock    func() time.Time
	pause    PauseChecker
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// ItemReminderOption customizes an ItemReminderWorker.
type ItemReminderOption func(*itemReminderOptions)

type itemReminderOptions struct {
	interval time.Duration
	clock    func() time.Time
	pause    PauseChecker
}

// WithItemReminderInterval overrides the default tick cadence.
func WithItemReminderInterval(d time.Duration) ItemReminderOption {
	return func(o *itemReminderOptions) {
		if d > 0 {
			o.interval = d
		}
	}
}

// WithItemReminderClock overrides the now-source handed to RemindOnce.
func WithItemReminderClock(now func() time.Time) ItemReminderOption {
	return func(o *itemReminderOptions) {
		if now != nil {
			o.clock = now
		}
	}
}

// WithItemReminderPauseController wires the soft-pause controller so the
// worker skips its sweep while the item-reminder worker type is paused. A
// nil checker leaves the worker unpaused.
func WithItemReminderPauseController(pc PauseChecker) ItemReminderOption {
	return func(o *itemReminderOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

func NewItemReminderWorker(service *ItemReminderService, opts ...ItemReminderOption) *ItemReminderWorker {
	options := itemReminderOptions{
		interval: defaultItemReminderInterval,
		clock:    time.Now,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &ItemReminderWorker{
		service:  service,
		interval: options.interval,
		clock:    options.clock,
		pause:    options.pause,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the goroutine. No-op if no service is configured.
func (w *ItemReminderWorker) Start(ctx context.Context) {
	if w.service == nil {
		slog.Warn("ItemReminderWorker: no service configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.Info("Item reminder worker started", "interval", w.interval)
}

// Stop signals the worker and waits for the goroutine to exit.
func (w *ItemReminderWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Item reminder worker stopped")
}

func (w *ItemReminderWorker) run(ctx context.Context) {
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

func (w *ItemReminderWorker) tick(ctx context.Context) {
	// Soft-pause: skip the sweep while paused. The ticker keeps running so
	// resuming takes effect on the next tick without a restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeItemReminder) {
		return
	}

	stats, err := w.service.RemindOnce(ctx, w.clock())
	if err != nil {
		slog.Error("Item reminder sweep failed", "error", err)
		return
	}
	if stats.Failed > 0 {
		itemReminderFailuresTotal.Add(float64(stats.Failed))
	}
	if stats.Sent > 0 {
		itemRemindersSentTotal.Add(float64(stats.Sent))
		slog.Info("Item reminder sweep completed", "reminders_sent", stats.Sent, "failed", stats.Failed)
		return
	}
	slog.Debug("Item reminder sweep completed", "reminders_sent", 0, "failed", stats.Failed)
}
