package services

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestInMemoryGlobalRateLimiter_CheckAndHitMetrics(t *testing.T) {
	c := qt.New(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start

	lim := NewInMemoryGlobalRateLimiter(2, time.Hour)
	t.Cleanup(lim.Stop)
	lim.now = func() time.Time { return now }

	ctx := context.Background()
	ip := "1.2.3.4"

	res, err := lim.Check(ctx, ip)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsTrue)
	c.Assert(res.Limit, qt.Equals, 2)
	c.Assert(res.Remaining, qt.Equals, 1)
	c.Assert(res.ResetAt, qt.Equals, start.Add(time.Hour))

	now = now.Add(10 * time.Minute)
	res, err = lim.Check(ctx, ip)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsTrue)
	c.Assert(res.Remaining, qt.Equals, 0)

	now = now.Add(10 * time.Minute)
	res, err = lim.Check(ctx, ip)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsFalse)
	c.Assert(res.Remaining, qt.Equals, 0)
	c.Assert(res.ResetAt, qt.Equals, start.Add(time.Hour))
	c.Assert(lim.RateLimitHits(), qt.Equals, uint64(1))

	// After the window passes, requests should be allowed again.
	now = start.Add(71 * time.Minute)
	res, err = lim.Check(ctx, ip)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsTrue)
	c.Assert(res.Remaining, qt.Equals, 1)
}

func TestNewGlobalRateLimiter_DisablesWhenLimitOrWindowInvalid(t *testing.T) {
	c := qt.New(t)

	lim := NewGlobalRateLimiter("", 0, time.Hour)
	res, err := lim.Check(context.Background(), "127.0.0.1")
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsTrue)
	c.Assert(lim.RateLimitHits(), qt.Equals, uint64(0))
}

func TestInMemoryGlobalRateLimiter_StopIsIdempotent(t *testing.T) {
	lim := NewInMemoryGlobalRateLimiter(1, time.Minute)
	t.Cleanup(lim.Stop)

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			lim.Stop()
		}()
	}

	wg.Wait()
	lim.Stop()
}
