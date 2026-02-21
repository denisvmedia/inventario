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

	for i := 0; i < 5; i++ {
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

	for i := 0; i < 4; i++ {
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
