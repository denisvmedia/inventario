package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const defaultGroupPurgeInterval = 5 * time.Minute

// Prometheus counters for the group purge worker. Registered lazily at
// package init via promauto against the default registry. Scraped through
// /metrics alongside the rest of the app's instrumentation.
var (
	groupsPurgedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_groups_purged_total",
		Help: "Number of location_groups successfully hard-deleted by the purge worker.",
	})
	groupsPurgeFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_groups_purge_failures_total",
		Help: "Number of location_groups whose per-group purge raised an error (will be retried next tick).",
	})
	groupInvitesExpiredPurgedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_group_invites_expired_purged_total",
		Help: "Number of unused, expired group invites removed by the housekeeping sweep.",
	})
)

// GroupPurgeWorker periodically runs the GroupPurgeService sweep: it purges
// any LocationGroups in pending_deletion state and deletes expired unused
// group invites. It mirrors the RefreshTokenCleanupWorker pattern — a single
// goroutine driven by a ticker, with a best-effort graceful stop.
type GroupPurgeWorker struct {
	service       *GroupPurgeService
	purgeInterval time.Duration
	stopCh        chan struct{}
	stopOnce      sync.Once
	wg            sync.WaitGroup
}

// GroupPurgeOption customizes a GroupPurgeWorker created by
// NewGroupPurgeWorker.
type GroupPurgeOption func(*groupPurgeOptions)

type groupPurgeOptions struct {
	purgeInterval time.Duration
}

// WithGroupPurgeInterval overrides the default purge interval. Non-positive
// values are ignored.
func WithGroupPurgeInterval(d time.Duration) GroupPurgeOption {
	return func(o *groupPurgeOptions) {
		if d > 0 {
			o.purgeInterval = d
		}
	}
}

// NewGroupPurgeWorker creates a worker with the default five-minute interval,
// overridable via GroupPurgeOption values (e.g. WithGroupPurgeInterval).
func NewGroupPurgeWorker(service *GroupPurgeService, opts ...GroupPurgeOption) *GroupPurgeWorker {
	options := groupPurgeOptions{
		purgeInterval: defaultGroupPurgeInterval,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &GroupPurgeWorker{
		service:       service,
		purgeInterval: options.purgeInterval,
		stopCh:        make(chan struct{}),
	}
}

// Start launches the background purge goroutine. It is a no-op if the
// service is nil.
func (w *GroupPurgeWorker) Start(ctx context.Context) {
	if w.service == nil {
		slog.Warn("GroupPurgeWorker: no service configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.Info("Group purge worker started", "interval", w.purgeInterval)
}

// Stop signals the worker to stop and waits for it to finish.
func (w *GroupPurgeWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Group purge worker stopped")
}

func (w *GroupPurgeWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.purgeInterval)
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

// tick runs a single purge+expire sweep. Errors are logged but do not stop
// the worker — a transient failure is expected to be retried on the next
// tick.
func (w *GroupPurgeWorker) tick(ctx context.Context) {
	if purged, failed, err := w.service.PurgeOnce(ctx); err != nil {
		slog.Error("Group purge sweep failed", "error", err)
	} else {
		if purged > 0 {
			groupsPurgedTotal.Add(float64(purged))
			slog.Info("Group purge sweep completed", "groups_purged", purged, "groups_failed", failed)
		} else {
			slog.Debug("Group purge sweep completed", "groups_purged", 0, "groups_failed", failed)
		}
		if failed > 0 {
			groupsPurgeFailuresTotal.Add(float64(failed))
		}
	}

	if expired, err := w.service.CleanExpiredInvites(ctx); err != nil {
		slog.Error("Expired invite cleanup failed", "error", err)
	} else if expired > 0 {
		groupInvitesExpiredPurgedTotal.Add(float64(expired))
		slog.Info("Expired invites cleaned up", "count", expired)
	} else {
		slog.Debug("Expired invites cleaned up", "count", 0)
	}
}
