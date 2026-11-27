// Package stall keeps the runs of exports, imports and restores visible to
// the sweeps that recover them when the process running them dies.
//
// A worker claims a job, records a heartbeat on the job's row at the claim,
// and renews it every HeartbeatInterval while the run lasts. A run whose
// heartbeat is older than Threshold belongs to a worker that stopped: the
// process crashed, was killed, or shut down in the middle of the run. The
// sweep of each worker type then queues the job again or fails it, so the
// job leaves in_progress either way and a failure shows among the failed
// jobs. The run time of a job does not matter, only whether its worker is
// still alive.
package stall

import (
	"context"
	"log/slog"
	"time"

	"go.5x5.cz/inventario/registry"
)

const (
	// HeartbeatInterval is how often a running job renews its heartbeat.
	HeartbeatInterval = time.Minute
	// Threshold is how old the heartbeat of a run may get before a sweep
	// takes the run for dead. It is ten missed heartbeats.
	Threshold = 10 * time.Minute
	// UnbeatenThreshold applies to a run that has no heartbeat at all,
	// because a version that records none claimed it: such a run is taken
	// for dead once its job is this old. Upgrading from such a version stops
	// its pods first, so no worker is left on the run; the job's creation is
	// the only time the run carries, and a day is a margin wider than a run.
	UnbeatenThreshold = 24 * time.Hour
	// SweepInterval is how often a worker sweeps for dead runs.
	SweepInterval = time.Minute
	// MaxRuns is how many runs an export or an import gets. A dead run of
	// a job that has used them fails the job instead of queuing it again,
	// so a job that kills its worker every time does not run forever.
	MaxRuns = 3
)

// NewSweep returns the sweep that, at now, takes the runs this package
// describes for dead and records errorMessage on the jobs it fails.
func NewSweep(now time.Time, errorMessage string) registry.StalledJobSweep {
	return registry.StalledJobSweep{
		HeartbeatBefore: now.Add(-Threshold),
		CreatedBefore:   now.Add(-UnbeatenThreshold),
		Now:             now,
		MaxRuns:         MaxRuns,
		ErrorMessage:    errorMessage,
	}
}

// Beat calls beat every interval, which is HeartbeatInterval outside tests,
// until the returned stop is called or ctx ends, so a worker that stops also
// stops its heartbeats. A failed beat is logged; the next one tries again.
func Beat(ctx context.Context, interval time.Duration, beat func(ctx context.Context, now time.Time) error) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if err := beat(ctx, now); err != nil && ctx.Err() == nil {
					slog.WarnContext(ctx, "Failed to record a job heartbeat", "error", err)
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
