package stall_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/backup/stall"
)

func TestBeat_RenewsUntilStopped(t *testing.T) {
	c := qt.New(t)
	var beats atomic.Int32
	stop := stall.Beat(context.Background(), 5*time.Millisecond, func(context.Context, time.Time) error {
		beats.Add(1)
		return nil
	})
	deadline := time.Now().Add(5 * time.Second)
	for beats.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	c.Assert(beats.Load() >= 3, qt.IsTrue, qt.Commentf("beats: %d", beats.Load()))

	stop()
	after := beats.Load()
	time.Sleep(30 * time.Millisecond)
	c.Assert(beats.Load(), qt.Equals, after, qt.Commentf("no beat after stop returns"))
}

// A worker whose context ends stops beating, so the sweep sees its runs go
// quiet.
func TestBeat_StopsWithTheContext(t *testing.T) {
	c := qt.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	var beats atomic.Int32
	stop := stall.Beat(ctx, 5*time.Millisecond, func(context.Context, time.Time) error {
		beats.Add(1)
		return nil
	})
	defer stop()
	cancel()
	time.Sleep(10 * time.Millisecond)
	after := beats.Load()
	time.Sleep(30 * time.Millisecond)
	c.Assert(beats.Load(), qt.Equals, after)
}

func TestNewSweep(t *testing.T) {
	c := qt.New(t)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	sweep := stall.NewSweep(now, "stopped")
	c.Assert(sweep.HeartbeatBefore, qt.Equals, now.Add(-stall.Threshold))
	c.Assert(sweep.CreatedBefore, qt.Equals, now.Add(-stall.UnbeatenThreshold))
	c.Assert(sweep.Now, qt.Equals, now)
	c.Assert(sweep.MaxRuns, qt.Equals, stall.MaxRuns)
	c.Assert(sweep.ErrorMessage, qt.Equals, "stopped")
	c.Assert(stall.Threshold > stall.HeartbeatInterval, qt.IsTrue, qt.Commentf("a live run must beat several times within the threshold"))
}
