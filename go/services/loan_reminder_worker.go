package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const defaultLoanReminderInterval = 1 * time.Hour

// Prometheus counters for the loan reminder worker (#1509). Labels:
//   - kind: "overdue" or "due_soon" — matches LoanReminderKind.
//
// `_failures_total` is unlabelled because every failure is logged with
// the kind tag and slicing the counter by reason adds no value (the
// operator drops into logs once they see the gauge tick).
var (
	loanRemindersSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_loan_reminders_sent_total",
		Help: "Number of loan reminder emails enqueued, partitioned by kind (overdue|due_soon).",
	}, []string{"kind"})
	loanReminderFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_loan_reminder_failures_total",
		Help: "Number of per-loan reminder failures (logged; will be retried next tick).",
	})
)

// LoanReminderWorker periodically runs LoanReminderService.RemindOnce.
// Same shape as WarrantyReminderWorker: single goroutine driven by a
// ticker, best-effort graceful stop on Stop().
type LoanReminderWorker struct {
	service  *LoanReminderService
	interval time.Duration
	clock    func() time.Time
	pause    PauseChecker
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// LoanReminderOption customises a LoanReminderWorker.
type LoanReminderOption func(*loanReminderOptions)

type loanReminderOptions struct {
	interval time.Duration
	clock    func() time.Time
	pause    PauseChecker
}

// WithLoanReminderInterval overrides the default tick cadence. Non-
// positive values are ignored.
func WithLoanReminderInterval(d time.Duration) LoanReminderOption {
	return func(o *loanReminderOptions) {
		if d > 0 {
			o.interval = d
		}
	}
}

// WithLoanReminderClock overrides the now-source the worker hands to
// RemindOnce. Tests pin time.Time{} via a closure; production passes
// time.Now indirectly via the default.
func WithLoanReminderClock(now func() time.Time) LoanReminderOption {
	return func(o *loanReminderOptions) {
		if now != nil {
			o.clock = now
		}
	}
}

// WithLoanReminderPauseController wires the soft-pause controller so the
// worker skips its sweep while the loan-reminder worker type is paused
// (#1308). A nil checker leaves the worker unpaused.
func WithLoanReminderPauseController(pc PauseChecker) LoanReminderOption {
	return func(o *loanReminderOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// NewLoanReminderWorker constructs the worker. Default cadence: one
// hour — same reasoning as warranty / storage-quota workers: cadence
// is hours-scale, not minutes, so a longer interval wastes ticks and a
// shorter one risks flapping at midnight UTC.
func NewLoanReminderWorker(service *LoanReminderService, opts ...LoanReminderOption) *LoanReminderWorker {
	options := loanReminderOptions{
		interval: defaultLoanReminderInterval,
		clock:    time.Now,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &LoanReminderWorker{
		service:  service,
		interval: options.interval,
		clock:    options.clock,
		pause:    options.pause,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the goroutine. No-op when no service is configured.
func (w *LoanReminderWorker) Start(ctx context.Context) {
	if w.service == nil {
		slog.Warn("LoanReminderWorker: no service configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.Info("Loan reminder worker started", "interval", w.interval)
}

// Stop signals the worker and waits for the goroutine to exit.
func (w *LoanReminderWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Loan reminder worker stopped")
}

func (w *LoanReminderWorker) run(ctx context.Context) {
	// Run once at startup so the first tick doesn't have to wait the
	// full interval after a deploy. Matches the warranty worker.
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

// tick runs a single sweep. The service returns LoanReminderStats with
// a per-kind breakdown so the worker emits one Prometheus series per
// kind label.
func (w *LoanReminderWorker) tick(ctx context.Context) {
	// Soft-pause (#1308): skip the sweep while paused. The ticker keeps
	// running so resuming takes effect on the next tick without a restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeLoanReminder) {
		return
	}

	stats, err := w.service.RemindOnce(ctx, w.clock())
	if err != nil {
		slog.Error("Loan reminder sweep failed", "error", err)
		return
	}
	if stats.Failed > 0 {
		loanReminderFailuresTotal.Add(float64(stats.Failed))
	}
	for kind, count := range stats.Sent {
		if count > 0 {
			loanRemindersSentTotal.
				WithLabelValues(string(kind)).
				Add(float64(count))
		}
	}
	if total := stats.Total(); total > 0 {
		slog.Info("Loan reminder sweep completed",
			"reminders_sent", total,
			"failed", stats.Failed,
		)
	} else {
		slog.Debug("Loan reminder sweep completed",
			"reminders_sent", 0,
			"failed", stats.Failed,
		)
	}
}

// Ensure compile-time that LoanReminderKind matches the worker's label
// expectations. If a new kind is added without updating the worker's
// Prometheus emit loop, this assertion stays trivially true (the worker
// emits for whichever keys are in the map) — kept for documentation.
var _ = registry.LoanReminderKindOverdue
