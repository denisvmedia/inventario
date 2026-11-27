package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// endStatuses are the statuses of an item the group no longer has.
var endStatuses = []models.CommodityStatus{
	models.CommodityStatusSold,
	models.CommodityStatusLost,
	models.CommodityStatusDisposed,
	models.CommodityStatusWrittenOff,
	models.CommodityStatusDonated,
	models.CommodityStatusStolen,
}

func (e *sectionReminderEnv) setStatus(c *qt.C, status models.CommodityStatus) {
	c.Helper()
	stored, err := e.regSet.CommodityRegistry.Get(e.ctx, e.commodity.ID)
	c.Assert(err, qt.IsNil)
	stored.Status = status
	_, err = e.regSet.CommodityRegistry.Update(e.ctx, *stored)
	c.Assert(err, qt.IsNil)
}

func TestReminderTargetActive_OnlyItemsInUse(t *testing.T) {
	c := qt.New(t)
	c.Assert(services.ReminderTargetActive(nil), qt.IsFalse)
	c.Assert(services.ReminderTargetActive(&models.Commodity{Status: models.CommodityStatusInUse}), qt.IsTrue)
	c.Assert(services.ReminderTargetActive(&models.Commodity{
		Status:    models.CommodityStatusInUse,
		DeletedAt: new(time.Now()),
	}), qt.IsFalse)
	for _, status := range endStatuses {
		c.Assert(services.ReminderTargetActive(&models.Commodity{Status: status}), qt.IsFalse, qt.Commentf("status %s", status))
	}
}

func TestWarrantyReminderService_OnlyItemsInUse(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	expires := models.Date(now.AddDate(0, 0, 30).Format(time.DateOnly))
	for _, status := range endStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			env := newSectionReminderEnv(c, &expires)
			env.setStatus(c, status)
			email := &recordingEmailService{}
			svc := services.NewWarrantyReminderService(env.fs, email, nil)

			stats, err := svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(stats.Sent(), qt.Equals, 0)
			c.Assert(email.snapshot(), qt.HasLen, 0)
			sent, err := env.fs.WarrantyReminderRegistry.HasSent(context.Background(), env.commodity.ID, 30)
			c.Assert(err, qt.IsNil)
			c.Assert(sent, qt.IsFalse)

			// Back in use, the item gets the reminder that is due.
			env.setStatus(c, models.CommodityStatusInUse)
			stats, err = svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.SentByThreshold[models.WarrantyReminder30Days], qt.Equals, 1)
			c.Assert(email.snapshot(), qt.HasLen, 1)
		})
	}
}

func TestMaintenanceReminderService_OnlyItemsInUse(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	for _, status := range endStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			env := newSectionReminderEnv(c, nil)
			schedule, err := env.regSet.MaintenanceScheduleRegistry.Create(env.ctx, models.MaintenanceSchedule{
				CommodityID:  env.commodity.ID,
				Title:        "Clean the filter",
				IntervalDays: 90,
				NextDueAt:    dateOffset(now, 7),
				Enabled:      true,
			})
			c.Assert(err, qt.IsNil)
			env.setStatus(c, status)
			email := &recordingMaintenanceEmailService{}
			svc := services.NewMaintenanceReminderService(env.fs, email, nil)

			stats, err := svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(stats.Sent(), qt.Equals, 0)
			c.Assert(email.snapshot(), qt.HasLen, 0)
			sent, err := env.fs.MaintenanceReminderRegistry.HasSent(context.Background(), schedule.ID, int(models.MaintenanceReminder7Days))
			c.Assert(err, qt.IsNil)
			c.Assert(sent, qt.IsFalse)

			env.setStatus(c, models.CommodityStatusInUse)
			stats, err = svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.SentByThreshold[models.MaintenanceReminder7Days], qt.Equals, 1)
			c.Assert(email.snapshot(), qt.HasLen, 1)
		})
	}
}

func TestMaintenanceReminderService_OverdueOnlyItemsInUse(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	c := qt.New(t)
	env := newSectionReminderEnv(c, nil)
	schedule, err := env.regSet.MaintenanceScheduleRegistry.Create(env.ctx, models.MaintenanceSchedule{
		CommodityID:  env.commodity.ID,
		Title:        "Service the bike",
		IntervalDays: 90,
		NextDueAt:    dateOffset(now, -3),
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)
	env.setStatus(c, models.CommodityStatusStolen)
	email := &recordingMaintenanceEmailService{}
	svc := services.NewMaintenanceReminderService(env.fs, email, nil)

	stats, err := svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 0)
	sent, err := env.fs.MaintenanceReminderRegistry.HasSent(context.Background(), schedule.ID, int(models.MaintenanceReminderOverdue))
	c.Assert(err, qt.IsNil)
	c.Assert(sent, qt.IsFalse)
}

func TestLoanReminderService_OnlyItemsInUse(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	dueDate := models.Date("2026-05-17")
	for _, status := range endStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			env := newSectionReminderEnv(c, nil)
			loan, err := env.regSet.CommodityLoanRegistry.Create(env.ctx, models.CommodityLoan{
				CommodityID:  env.commodity.ID,
				BorrowerName: "Pavel",
				LentAt:       "2026-05-10",
				DueBackAt:    models.PDate(&dueDate),
			})
			c.Assert(err, qt.IsNil)
			env.setStatus(c, status)
			email := &recordingLoanEmailService{}
			svc := services.NewLoanReminderService(env.fs, email, nil)

			stats, err := svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(stats.Total(), qt.Equals, 0)
			c.Assert(email.snapshot(), qt.HasLen, 0)
			stored, err := env.regSet.CommodityLoanRegistry.Get(env.ctx, loan.ID)
			c.Assert(err, qt.IsNil)
			c.Assert(stored.ReminderSentDueSoon, qt.IsFalse)

			env.setStatus(c, models.CommodityStatusInUse)
			stats, err = svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Sent[registry.LoanReminderKindDueSoon], qt.Equals, 1)
			calls := email.snapshot()
			c.Assert(calls, qt.HasLen, 1)
			c.Assert(calls[0].commodityName, qt.Equals, "Dishwasher")
		})
	}
}

func TestItemReminderService_OnlyItemsInUse(t *testing.T) {
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	for _, status := range endStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			env := newSectionReminderEnv(c, nil)
			_, err := services.NewCommodityReminderService(env.fs).Create(env.ctx, models.CommodityReminder{
				CommodityID:      env.commodity.ID,
				Title:            "Return window ends",
				DueOn:            dateOffset(now, 10),
				RemindDaysBefore: models.ValuerSlice[int]{30},
			})
			c.Assert(err, qt.IsNil)
			env.setStatus(c, status)
			email := newRecordingItemReminderEmail()
			svc := services.NewItemReminderService(env.fs, email, nil)

			stats, err := svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(stats.Sent, qt.Equals, 0)
			c.Assert(email.snapshot(), qt.HasLen, 0)

			env.setStatus(c, models.CommodityStatusInUse)
			stats, err = svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Sent, qt.Equals, 1)
			c.Assert(email.snapshot(), qt.HasLen, 1)
		})
	}
}

// The Upcoming page lists what the reminders would send: an item the group
// no longer has drops out of every source, and comes back with its status.
func TestUpcomingService_OnlyItemsInUse(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	expires := models.Date(now.AddDate(0, 0, 20).Format(time.DateOnly))
	dueBack := dateOffset(now, 5)
	for _, status := range endStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			env := newSectionReminderEnv(c, &expires)
			_, err := services.NewCommodityReminderService(env.fs).Create(env.ctx, models.CommodityReminder{
				CommodityID:      env.commodity.ID,
				Title:            "Return window ends",
				DueOn:            dateOffset(now, 10),
				RemindDaysBefore: models.ValuerSlice[int]{30},
			})
			c.Assert(err, qt.IsNil)
			_, err = env.regSet.CommodityLoanRegistry.Create(env.ctx, models.CommodityLoan{
				CommodityID:  env.commodity.ID,
				BorrowerName: "Pavel",
				LentAt:       models.Date(now.AddDate(0, 0, -2).Format(time.DateOnly)),
				DueBackAt:    models.PDate(&dueBack),
			})
			c.Assert(err, qt.IsNil)
			_, err = env.regSet.MaintenanceScheduleRegistry.Create(env.ctx, models.MaintenanceSchedule{
				CommodityID:  env.commodity.ID,
				Title:        "Clean the filter",
				IntervalDays: 90,
				NextDueAt:    dateOffset(now, 7),
				Enabled:      true,
			})
			c.Assert(err, qt.IsNil)
			env.setStatus(c, status)

			entries, err := services.NewUpcomingService().List(env.ctx, env.regSet, now, 30)
			c.Assert(err, qt.IsNil)
			c.Assert(entries, qt.HasLen, 0)

			env.setStatus(c, models.CommodityStatusInUse)
			entries, err = services.NewUpcomingService().List(env.ctx, env.regSet, now, 30)
			c.Assert(err, qt.IsNil)
			c.Assert(entries, qt.HasLen, 4)
		})
	}
}

// The "coming up" list of the weekly digest leaves out the warranty and the
// maintenance of an item the group no longer has, and of an item in Trash.
func TestWeeklyDigestService_UpcomingOnlyItemsInUse(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)
	inWindow := f.window.Start.Add(time.Hour)
	soon := models.Date(f.sentAt.AddDate(0, 0, 10).Format(time.DateOnly))
	household := f.addGroup(c, "Household")

	fridge := f.addCommodity(c, household, "Fridge", &soon)
	f.addEvent(c, household, fridge.ID, models.CommodityEventKindCreated, inWindow)
	sold := f.addCommodity(c, household, "Old fridge", &soon)
	sold.Status = models.CommodityStatusSold
	_, err := f.regSet.CommodityRegistry.Update(f.userCtx, *sold)
	c.Assert(err, qt.IsNil)
	bike := f.addCommodity(c, household, "Bike", nil)
	bike.Status = models.CommodityStatusStolen
	_, err = f.regSet.CommodityRegistry.Update(f.userCtx, *bike)
	c.Assert(err, qt.IsNil)
	_, err = f.regSet.MaintenanceScheduleRegistry.Create(f.userCtx, models.MaintenanceSchedule{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.tenantID,
			GroupID:         household.ID,
			CreatedByUserID: f.userID,
		},
		CommodityID:  bike.ID,
		Title:        "Service",
		IntervalDays: 90,
		NextDueAt:    models.Date(f.sentAt.AddDate(0, 0, 5).Format(time.DateOnly)),
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)

	email := &recordingDigestEmailService{}
	_, err = f.newService(email).SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)

	sends := email.recorded()
	c.Assert(sends, qt.HasLen, 1)
	c.Assert(sends[0].digest.Upcoming, qt.HasLen, 1)
	c.Assert(sends[0].digest.Upcoming[0].Name, qt.Equals, "Fridge")
}
