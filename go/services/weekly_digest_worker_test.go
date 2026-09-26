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
func TestWeeklyDigestWorker_SendsOnlyOnMondayAtTheSendHour(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))

	email := &recordingDigestEmailService{}

	// A tick at each of these instants; only the Monday 09:00 one may send.
	for _, tc := range []struct {
		name     string
		at       time.Time
		wantSend bool
	}{
		{"Sunday 09:00", time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC), false},
		{"Monday 08:00", time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC), false},
		{"Monday 09:00", time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC), true},
		{"Monday 10:00", time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC), false},
		{"Tuesday 09:00", time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC), false},
	} {
		c.Run(tc.name, func(c *qt.C) {
			before := len(email.recorded())

			at := tc.at
			worker := services.NewWeeklyDigestWorker(
				f.newService(email),
				services.WithWeeklyDigestClock(func() time.Time { return at }),
				services.WithWeeklyDigestSendHour(9),
			)
			services.RunWeeklyDigestTickForTest(worker, context.Background())

			sent := len(email.recorded()) - before
			if tc.wantSend {
				c.Assert(sent, qt.Equals, 1)
			} else {
				c.Assert(sent, qt.Equals, 0)
			}
		})
	}
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
