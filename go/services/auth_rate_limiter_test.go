package services

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestInMemoryAuthRateLimiter_SlidingWindow(t *testing.T) {
	c := qt.New(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start

	lim := NewInMemoryAuthRateLimiter()
	lim.now = func() time.Time { return now }

	ctx := context.Background()
	ip := "1.2.3.4"

	for i := range 5 {
		res, err := lim.CheckLoginAttempt(ctx, ip)
		c.Assert(err, qt.IsNil)
		c.Assert(res.Allowed, qt.IsTrue)
		c.Assert(res.Limit, qt.Equals, 5)
		c.Assert(res.Remaining, qt.Equals, 4-i)
		now = now.Add(10 * time.Second)
	}

	res, err := lim.CheckLoginAttempt(ctx, ip)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsFalse)
	c.Assert(res.Remaining, qt.Equals, 0)
	c.Assert(res.ResetAt, qt.Equals, start.Add(15*time.Minute))

	// After the window passes, requests should be allowed again.
	now = start.Add(16 * time.Minute)
	res, err = lim.CheckLoginAttempt(ctx, ip)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsTrue)
	c.Assert(res.Remaining, qt.Equals, 4)
}

func TestInMemoryAuthRateLimiter_AccountLockout(t *testing.T) {
	c := qt.New(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start

	lim := NewInMemoryAuthRateLimiter()
	lim.now = func() time.Time { return now }

	ctx := context.Background()
	email := "test@example.com"

	for range 4 {
		locked, _, err := lim.RecordFailedLogin(ctx, email)
		c.Assert(err, qt.IsNil)
		c.Assert(locked, qt.IsFalse)
		now = now.Add(time.Minute)
	}

	locked, resetAt, err := lim.RecordFailedLogin(ctx, email)
	c.Assert(err, qt.IsNil)
	c.Assert(locked, qt.IsTrue)
	c.Assert(resetAt, qt.Equals, now.Add(15*time.Minute))

	isLocked, _, err := lim.IsAccountLocked(ctx, email)
	c.Assert(err, qt.IsNil)
	c.Assert(isLocked, qt.IsTrue)

	// After lockout passes, it should unlock.
	now = now.Add(16 * time.Minute)
	isLocked, _, err = lim.IsAccountLocked(ctx, email)
	c.Assert(err, qt.IsNil)
	c.Assert(isLocked, qt.IsFalse)

	c.Assert(lim.ClearFailedLogins(ctx, email), qt.IsNil)
}

// TestInMemoryAuthRateLimiter_CheckRefreshAttempt_PerIP pins the #967 H1
// dedicated refresh budget: per-IP, generous (refreshAttemptsLimit), and
// independent of the login budget. The (refreshAttemptsLimit+1)-th call from
// one IP is denied while a different IP keeps its own bucket.
func TestInMemoryAuthRateLimiter_CheckRefreshAttempt_PerIP(t *testing.T) {
	c := qt.New(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start

	lim := NewInMemoryAuthRateLimiter()
	lim.now = func() time.Time { return now }

	ctx := context.Background()
	ip := "7.7.7.7"

	for i := range refreshAttemptsLimit {
		res, err := lim.CheckRefreshAttempt(ctx, ip)
		c.Assert(err, qt.IsNil)
		c.Assert(res.Allowed, qt.IsTrue)
		c.Assert(res.Limit, qt.Equals, refreshAttemptsLimit)
		c.Assert(res.Remaining, qt.Equals, refreshAttemptsLimit-1-i)
		now = now.Add(time.Second)
	}

	res, err := lim.CheckRefreshAttempt(ctx, ip)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsFalse)
	c.Assert(res.Remaining, qt.Equals, 0)

	// A different IP keeps its own bucket.
	res, err = lim.CheckRefreshAttempt(ctx, "6.6.6.6")
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsTrue)
}

func TestInMemoryAuthRateLimiter_CheckPublicScanAttempt_PerIP(t *testing.T) {
	c := qt.New(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start

	lim := NewInMemoryAuthRateLimiter()
	lim.now = func() time.Time { return now }

	ctx := context.Background()
	ip := "9.9.9.9"

	for i := range publicScanAttemptsLimit {
		res, err := lim.CheckPublicScanAttempt(ctx, ip)
		c.Assert(err, qt.IsNil)
		c.Assert(res.Allowed, qt.IsTrue)
		c.Assert(res.Limit, qt.Equals, publicScanAttemptsLimit)
		c.Assert(res.Remaining, qt.Equals, publicScanAttemptsLimit-1-i)
		now = now.Add(time.Second)
	}

	res, err := lim.CheckPublicScanAttempt(ctx, ip)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsFalse)
	c.Assert(res.Remaining, qt.Equals, 0)

	// A different IP keeps its own bucket.
	res, err = lim.CheckPublicScanAttempt(ctx, "8.8.8.8")
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsTrue)
}

func TestInMemoryAuthRateLimiter_CheckPublicScanGlobalCap_SharedCounter(t *testing.T) {
	c := qt.New(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start

	lim := NewInMemoryAuthRateLimiter()
	lim.now = func() time.Time { return now }

	ctx := context.Background()

	// The global cap ignores the caller entirely — it is one shared
	// counter — so exhausting it then asserting the next call is denied
	// proves the constant-key behavior without keying on any input.
	for range publicScanGlobalCapLimit {
		res, err := lim.CheckPublicScanGlobalCap(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(res.Allowed, qt.IsTrue)
		c.Assert(res.Limit, qt.Equals, publicScanGlobalCapLimit)
		now = now.Add(time.Second)
	}

	res, err := lim.CheckPublicScanGlobalCap(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Allowed, qt.IsFalse)
	c.Assert(res.Remaining, qt.Equals, 0)
}

func TestNoOpAuthRateLimiter_PublicScan_AlwaysAllows(t *testing.T) {
	c := qt.New(t)

	lim := NewNoOpAuthRateLimiter()
	ctx := context.Background()

	ipRes, err := lim.CheckPublicScanAttempt(ctx, "1.1.1.1")
	c.Assert(err, qt.IsNil)
	c.Assert(ipRes.Allowed, qt.IsTrue)

	globalRes, err := lim.CheckPublicScanGlobalCap(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(globalRes.Allowed, qt.IsTrue)
}
