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

// The digest is weekly, but the worker ticks hourly.
//
// A weekly ticker would tie sending to the moment the process started: restart
// on a Thursday and every digest goes out on Thursdays. Ticking hourly and
// letting the send hour decide keeps the schedule a property of the calendar
// instead of the deployment, and the idempotency claim is what stops the extra
// ticks from sending anything.
const (
	defaultWeeklyDigestInterval = 1 * time.Hour
	// defaultWeeklyDigestSendHour is the UTC hour the digest goes out on a
	// Monday. Per-user timezones are a follow-up (#1391); until then a single
	// hour is the honest choice, and 09:00 UTC lands in the working day across
	// Europe, which is where the alpha cohort is.
	defaultWeeklyDigestSendHour = 9
	// weeklyDigestClaimRetention is how long a claim row is kept. It exists to
	// stop a second send inside one week, so a few weeks of history is all that
	// is useful; the sweep drops the rest.
	weeklyDigestClaimRetention = 8 * 7 * 24 * time.Hour
)

var (
	weeklyDigestsSentTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_weekly_digests_sent_total",
		Help: "Number of weekly digest emails enqueued.",
	})
	weeklyDigestFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_weekly_digest_failures_total",
		Help: "Number of per-user weekly digest failures (logged; retried on the next tick).",
	})
)

// WeeklyDigestWorker runs WeeklyDigestService on Mondays.
type WeeklyDigestWorker struct {
	service  *WeeklyDigestService
	interval time.Duration
	sendHour int
	clock    func() time.Time
	pause    PauseChecker
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// WeeklyDigestOption customizes a WeeklyDigestWorker.
type WeeklyDigestOption func(*weeklyDigestOptions)

type weeklyDigestOptions struct {
	interval time.Duration
	sendHour int
	clock    func() time.Time
	pause    PauseChecker
}

// WithWeeklyDigestInterval overrides the tick cadence. Non-positive values are
// ignored.
func WithWeeklyDigestInterval(d time.Duration) WeeklyDigestOption {
	return func(o *weeklyDigestOptions) {
		if d > 0 {
			o.interval = d
		}
	}
}

// WithWeeklyDigestSendHour overrides the UTC hour on Monday at which a tick is
// allowed to send. Values outside 0–23 are ignored.
func WithWeeklyDigestSendHour(hour int) WeeklyDigestOption {
	return func(o *weeklyDigestOptions) {
		if hour >= 0 && hour <= 23 {
			o.sendHour = hour
		}
	}
}

// WithWeeklyDigestClock overrides the now-source, so a test can place the clock
// on a Monday morning without waiting for one.
func WithWeeklyDigestClock(now func() time.Time) WeeklyDigestOption {
	return func(o *weeklyDigestOptions) {
		if now != nil {
			o.clock = now
		}
	}
}

// WithWeeklyDigestPauseController wires the soft-pause controller so the worker
// skips its sweep while the weekly-digest worker type is paused (#1308).
func WithWeeklyDigestPauseController(pc PauseChecker) WeeklyDigestOption {
	return func(o *weeklyDigestOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// NewWeeklyDigestWorker constructs the worker.
func NewWeeklyDigestWorker(service *WeeklyDigestService, opts ...WeeklyDigestOption) *WeeklyDigestWorker {
	options := weeklyDigestOptions{
		interval: defaultWeeklyDigestInterval,
		sendHour: defaultWeeklyDigestSendHour,
		clock:    time.Now,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &WeeklyDigestWorker{
		service:  service,
		interval: options.interval,
		sendHour: options.sendHour,
		clock:    options.clock,
		pause:    options.pause,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the goroutine. No-op if no service is configured.
func (w *WeeklyDigestWorker) Start(ctx context.Context) {
	if w.service == nil {
		slog.Warn("WeeklyDigestWorker: no service configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.Info("Weekly digest worker started", "interval", w.interval, "send_hour_utc", w.sendHour)
}

// Stop signals the worker and waits for the goroutine to exit.
func (w *WeeklyDigestWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Weekly digest worker stopped")
}

func (w *WeeklyDigestWorker) run(ctx context.Context) {
	// Unlike the reminder workers this does NOT sweep at startup: a deploy on a
	// Monday morning would otherwise send the week's digests at whatever minute
	// the rollout happened, and a rollback-and-redeploy would do it again. The
	// claim row makes the second send harmless, but the first would still be
	// off-schedule.
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

// tick sends the week's digests when the clock is inside the send window, and
// does nothing otherwise.
func (w *WeeklyDigestWorker) tick(ctx context.Context) {
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeWeeklyDigest) {
		return
	}

	now := w.clock().UTC()
	// From Monday at the send hour to the end of the week, not just during that
	// one hour. A process that is down, paused or mid-deploy at 09:00 would
	// otherwise skip the week entirely; with the whole week open it catches up
	// on its next tick, and the per-week claim is what keeps that from being a
	// second email.
	if now.Before(WeekStartUTC(now).Add(time.Duration(w.sendHour) * time.Hour)) {
		return
	}

	stats, err := w.service.SendOnce(ctx, now)
	if err != nil {
		slog.Error("Weekly digest sweep failed", "error", err)
		weeklyDigestFailuresTotal.Inc()
		return
	}
	if stats.Sent > 0 {
		weeklyDigestsSentTotal.Add(float64(stats.Sent))
	}
	if stats.Failed > 0 {
		weeklyDigestFailuresTotal.Add(float64(stats.Failed))
	}
	slog.Info("Weekly digest sweep completed",
		"sent", stats.Sent,
		"nothing_to_say", stats.Nothing,
		"already_sent", stats.AlreadySent,
		"failed", stats.Failed,
	)

	w.sweepOldClaims(ctx, now)
}

// sweepOldClaims drops claim rows old enough to be useless, so the table stays
// a working set rather than a log.
func (w *WeeklyDigestWorker) sweepOldClaims(ctx context.Context, now time.Time) {
	if w.service == nil || w.service.factorySet == nil || w.service.factorySet.WeeklyDigestSendRegistry == nil {
		return
	}
	cutoff := WeekStartUTC(now.Add(-weeklyDigestClaimRetention))
	deleted, err := w.service.factorySet.WeeklyDigestSendRegistry.DeleteSentBefore(ctx, cutoff)
	if err != nil {
		slog.Error("Weekly digest: failed to sweep old claims", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("Weekly digest: swept old claims", "deleted", deleted, "before", cutoff.Format(time.DateOnly))
	}
}
