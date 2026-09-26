package services_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// The worker ticks hourly and sends weekly, so the tick has to be the thing
// that decides. A weekly ticker would tie the send to the moment the process
// started: deploy on a Thursday and every digest goes out on Thursdays.
//
// The window opens at Monday's send hour and stays open until the week ends. A
// process that is down, paused or mid-deploy at 09:00 would otherwise skip the
// week; the per-week claim is what stops the wider window from sending twice.
func TestWeeklyDigestWorker_SendWindowOpensOnMondayAndStaysOpen(t *testing.T) {
	c := qt.New(t)

	for _, tc := range []struct {
		name     string
		at       time.Time
		wantSend bool
	}{
		// The only closed part of the week is Monday before the send hour. A
		// Sunday belongs to the week that began on the previous Monday, so its
		// window has been open for six days.
		{"Monday one second early", time.Date(2026, 9, 28, 8, 59, 59, 0, time.UTC), false},
		{"Monday at midnight, hours early", time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), false},
		{"Monday exactly at the send hour", time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC), true},
		{"Monday later that day", time.Date(2026, 9, 28, 23, 0, 0, 0, time.UTC), true},
		{"Wednesday, catching up after an outage", time.Date(2026, 9, 30, 4, 0, 0, 0, time.UTC), true},
		{"the Sunday that ends the week", time.Date(2026, 10, 4, 23, 59, 0, 0, time.UTC), true},
		{"a Sunday, still inside its own week's window", time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC), true},
	} {
		c.Run(tc.name, func(c *qt.C) {
			// A fixture per case: the claim row is shared state, so one case
			// sending would make the next report "already sent" and the table
			// would stop testing the gate.
			f := newDigestFixture(c)
			household := f.addGroup(c, "Household")
			drill := f.addCommodity(c, household, "Drill", nil)
			f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated,
				services.WeeklyDigestWindowFor(tc.at).Start.Add(time.Hour))

			email := &recordingDigestEmailService{}
			at := tc.at
			worker := services.NewWeeklyDigestWorker(
				f.newService(email),
				services.WithWeeklyDigestClock(func() time.Time { return at }),
				services.WithWeeklyDigestSendHour(9),
			)
			services.RunWeeklyDigestTickForTest(worker, context.Background())

			if tc.wantSend {
				c.Assert(email.recorded(), qt.HasLen, 1)
			} else {
				c.Assert(email.recorded(), qt.HasLen, 0)
			}
		})
	}
}

// Repeated ticks inside the open window send once, which is what makes the wide
// window safe.
func TestWeeklyDigestWorker_TicksThroughTheWindowSendOnce(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))

	email := &recordingDigestEmailService{}
	at := f.sentAt
	worker := services.NewWeeklyDigestWorker(
		f.newService(email),
		services.WithWeeklyDigestClock(func() time.Time { return at }),
		services.WithWeeklyDigestSendHour(9),
	)

	// Every hour from Monday 09:00 to Monday 17:00.
	for range 9 {
		services.RunWeeklyDigestTickForTest(worker, context.Background())
		at = at.Add(time.Hour)
	}
	c.Assert(email.recorded(), qt.HasLen, 1)
}

// Midnight is a legitimate send hour, and it is the one an int config field
// cannot express by accident: with 0 as the "unset" marker it would be silently
// replaced by the default.
func TestWeeklyDigestWorker_MidnightIsAValidSendHour(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))

	email := &recordingDigestEmailService{}
	// Monday 00:30, which is inside the window only when the send hour is 0.
	at := time.Date(2026, 9, 28, 0, 30, 0, 0, time.UTC)
	worker := services.NewWeeklyDigestWorker(
		f.newService(email),
		services.WithWeeklyDigestClock(func() time.Time { return at }),
		services.WithWeeklyDigestSendHour(0),
	)
	services.RunWeeklyDigestTickForTest(worker, context.Background())
	c.Assert(email.recorded(), qt.HasLen, 1)
}

// pausedChecker reports the named worker type as paused and records that it was
// asked, so a test can tell "paused" from "never consulted".
type pausedChecker struct {
	paused models.WorkerType
	asked  atomic.Int64
}

func (p *pausedChecker) IsPaused(t models.WorkerType) bool {
	p.asked.Add(1)
	return t == p.paused
}

// The soft-pause controller is the operator's stop button, so it has to be
// consulted before anything is sent.
func TestWeeklyDigestWorker_HonorsTheSoftPause(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))

	email := &recordingDigestEmailService{}
	pause := &pausedChecker{paused: models.WorkerTypeWeeklyDigest}
	worker := services.NewWeeklyDigestWorker(
		f.newService(email),
		services.WithWeeklyDigestClock(func() time.Time { return f.sentAt }),
		services.WithWeeklyDigestSendHour(9),
		services.WithWeeklyDigestPauseController(pause),
	)

	services.RunWeeklyDigestTickForTest(worker, context.Background())
	c.Assert(email.recorded(), qt.HasLen, 0)
	c.Assert(pause.asked.Load() > 0, qt.IsTrue, qt.Commentf("the pause controller was never consulted"))
}

// The digest is a pausable worker type, so it has to be in the canonical set —
// workerpause fails open for an unknown type, which would make it unpausable.
func TestWeeklyDigestWorkerType_IsPausable(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.WorkerTypeWeeklyDigest.IsValid(), qt.IsTrue)
	parsed, ok := models.ParseWorkerType("weekly-digest")
	c.Assert(ok, qt.IsTrue)
	c.Assert(parsed, qt.Equals, models.WorkerTypeWeeklyDigest)
	c.Assert(models.AllWorkerTypes(), qt.Contains, models.WorkerTypeWeeklyDigest)
}
