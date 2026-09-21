package services_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// #2114 N7 flagged thumbnail generation; the worker half of it was untested.
// The lifecycle is where the sharp edges are, and a Stop that does not signal
// its loops does not fail — it hangs, taking the whole shutdown with it. Every
// Stop below is therefore bounded, so that failure arrives as a failure.

// stubPauseController reports a fixed pause state and records what was asked.
type stubPauseController struct {
	mu     sync.Mutex
	paused bool
	asked  []models.WorkerType
}

func (s *stubPauseController) IsPaused(wt models.WorkerType) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = append(s.asked, wt)
	return s.paused
}

func (s *stubPauseController) askedFor() []models.WorkerType {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]models.WorkerType(nil), s.asked...)
}

func newTestThumbnailWorker(opts ...services.ThumbnailWorkerOption) *services.ThumbnailGenerationWorker {
	config := services.ThumbnailGenerationConfig{
		MaxConcurrentPerUser: 2,
		RateLimitPerMinute:   50,
		SlotDuration:         time.Minute,
	}
	return services.NewThumbnailGenerationWorker(memory.NewFactorySet(), "memory://", config, opts...)
}

// stopWithin calls Stop and fails if it does not return in time. Stop drains
// the loops, so a Stop that never signals them blocks forever; unbounded, that
// is a test run that burns its whole budget instead of reporting anything.
func stopWithin(c *qt.C, w *services.ThumbnailGenerationWorker, budget time.Duration) {
	c.Helper()

	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(budget):
		c.Fatalf("Stop did not return within %s; the loops were never signalled", budget)
	}
}

func TestThumbnailWorker_StartAndStop(t *testing.T) {
	c := qt.New(t)

	w := newTestThumbnailWorker(services.WithThumbnailPollInterval(5 * time.Millisecond))
	c.Check(w.IsRunning(), qt.IsFalse)

	w.Start(context.Background())
	c.Check(w.IsRunning(), qt.IsTrue)

	stopWithin(c, w, 5*time.Second)
	c.Check(w.IsRunning(), qt.IsFalse)
}

// Start twice must not start a second pair of loops. It would, without the
// guard, and then Stop's drain would be waiting on goroutines that the stop
// channel already released — the worker would look stopped while a second
// job processor kept claiming work.
func TestThumbnailWorker_StartIsIdempotent(t *testing.T) {
	c := qt.New(t)

	w := newTestThumbnailWorker(services.WithThumbnailPollInterval(5 * time.Millisecond))
	w.Start(context.Background())
	w.Start(context.Background())
	c.Check(w.IsRunning(), qt.IsTrue)

	stopWithin(c, w, 5*time.Second)
	c.Check(w.IsRunning(), qt.IsFalse)
}

// Stop closes the stop channel, so a second Stop must not reach that close.
// Shutdown paths get called twice more often than anyone plans for, and a
// panic there takes down the process that was trying to exit cleanly.
func TestThumbnailWorker_StopIsIdempotent(t *testing.T) {
	c := qt.New(t)

	w := newTestThumbnailWorker(services.WithThumbnailPollInterval(5 * time.Millisecond))
	w.Start(context.Background())

	stopWithin(c, w, 5*time.Second)
	stopWithin(c, w, 5*time.Second)
	c.Check(w.IsRunning(), qt.IsFalse)
}

// Stopping something that was never started is a no-op, not a panic and not
// a wait on goroutines that do not exist.
func TestThumbnailWorker_StopWithoutStart(t *testing.T) {
	c := qt.New(t)

	w := newTestThumbnailWorker()

	stopWithin(c, w, 5*time.Second)
	c.Check(w.IsRunning(), qt.IsFalse)
}

// After Stop returns, the job processor is no longer ticking. This is the
// observable half of the drain: it catches a Stop that never signalled, and
// one that signalled but returned before the loops noticed.
func TestThumbnailWorker_StopSilencesTheJobProcessor(t *testing.T) {
	c := qt.New(t)

	pause := &stubPauseController{}
	w := newTestThumbnailWorker(
		services.WithThumbnailPollInterval(2*time.Millisecond),
		services.WithThumbnailPauseController(pause),
	)
	w.Start(context.Background())

	// Let the processor tick at least once so there is something to drain.
	c.Assert(eventually(c, 2*time.Second, func() bool { return len(pause.askedFor()) > 0 }), qt.IsTrue,
		qt.Commentf("the job processor never ticked"))

	stopWithin(c, w, 5*time.Second)

	// Nothing more is asked after Stop returns.
	settled := len(pause.askedFor())
	time.Sleep(30 * time.Millisecond)
	c.Check(pause.askedFor(), qt.HasLen, settled,
		qt.Commentf("the job processor kept running after Stop returned"))
}

// A paused worker keeps ticking but claims nothing, so resuming takes effect
// on the next tick rather than needing a restart.
func TestThumbnailWorker_PausedWorkerKeepsAskingAndClaimsNothing(t *testing.T) {
	c := qt.New(t)

	pause := &stubPauseController{paused: true}
	w := newTestThumbnailWorker(
		services.WithThumbnailPollInterval(2*time.Millisecond),
		services.WithThumbnailPauseController(pause),
	)
	w.Start(context.Background())
	defer stopWithin(c, w, 5*time.Second)

	c.Assert(eventually(c, 2*time.Second, func() bool { return len(pause.askedFor()) >= 2 }), qt.IsTrue,
		qt.Commentf("a paused worker stopped ticking; resuming would need a restart"))

	// It asks about its own worker type and no other.
	for _, wt := range pause.askedFor() {
		c.Check(wt, qt.Equals, models.WorkerTypeThumbnail)
	}
}

func TestThumbnailWorker_OptionsConfigureTheWorker(t *testing.T) {
	c := qt.New(t)

	w := newTestThumbnailWorker(
		services.WithThumbnailPollInterval(777*time.Millisecond),
		services.WithThumbnailBatchSize(13),
		services.WithThumbnailCleanupInterval(time.Hour),
		services.WithThumbnailJobRetentionPeriod(48*time.Hour),
		services.WithThumbnailJobBatchTimeout(9*time.Second),
		services.WithDetachedThumbnailJobTimeout(11*time.Second),
	)

	stats, err := w.GetStats(context.Background())
	c.Assert(err, qt.IsNil)
	c.Check(stats["poll_interval"], qt.Equals, (777 * time.Millisecond).String())
	c.Check(stats["batch_size"], qt.Equals, 13)
	c.Check(stats["worker_running"], qt.Equals, false)
}

func TestThumbnailWorker_GetStatsReportsJobCountsAndSlots(t *testing.T) {
	c := qt.New(t)

	w := newTestThumbnailWorker()

	stats, err := w.GetStats(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(stats["job_counts"], qt.IsNotNil)
	c.Check(stats["active_slots"], qt.Equals, 0)
}
