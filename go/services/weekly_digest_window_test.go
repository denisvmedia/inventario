package services_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services"
)

// The week boundary is the digest's idempotency key, so every instant inside a
// week has to produce the same answer from any timezone. A drifting boundary
// would let one user get two digests for what they think is one week.
func TestWeekStartUTC(t *testing.T) {
	c := qt.New(t)

	// 2026-09-21 is a Monday.
	monday := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name string
		in   time.Time
	}{
		{"the Monday itself", monday},
		{"Monday just before midnight", time.Date(2026, 9, 21, 23, 59, 59, 0, time.UTC)},
		{"midweek", time.Date(2026, 9, 24, 13, 5, 0, 0, time.UTC)},
		{"the Sunday that ends the week", time.Date(2026, 9, 27, 23, 59, 59, 0, time.UTC)},
		// A Sunday evening in Auckland is already Monday in UTC, so the naive
		// "take the local weekday" version puts this in the wrong week.
		{"Sunday evening in a far-east zone", time.Date(2026, 9, 27, 20, 0, 0, 0, time.FixedZone("NZST", 12*3600))},
		{"Monday morning in a far-west zone", time.Date(2026, 9, 21, 20, 0, 0, 0, time.FixedZone("HST", -10*3600))},
	} {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(services.WeekStartUTC(tc.in), qt.Equals, monday)
		})
	}

	// The Sunday before must land on the previous Monday, not on this one:
	// Go's Weekday() counts from Sunday, so an unadjusted offset is off by one
	// exactly here.
	c.Run("the Sunday before belongs to the previous week", func(c *qt.C) {
		c.Assert(services.WeekStartUTC(time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)),
			qt.Equals, monday.AddDate(0, 0, -7))
	})
}

func TestWeeklyDigestWindowFor(t *testing.T) {
	c := qt.New(t)

	// Sent on Monday 2026-09-28, the digest covers 2026-09-21 to 2026-09-27.
	sentAt := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	window := services.WeeklyDigestWindowFor(sentAt)

	c.Assert(window.Start, qt.Equals, time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC))
	c.Assert(window.End, qt.Equals, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC))

	c.Run("the start is inside and the end is not", func(c *qt.C) {
		c.Assert(window.Contains(window.Start), qt.IsTrue)
		c.Assert(window.Contains(window.End), qt.IsFalse,
			qt.Commentf("an inclusive end would let two consecutive weeks claim the same instant"))
		c.Assert(window.Contains(window.End.Add(-time.Nanosecond)), qt.IsTrue)
		c.Assert(window.Contains(window.Start.Add(-time.Nanosecond)), qt.IsFalse)
	})
}
