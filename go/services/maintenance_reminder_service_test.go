package services_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// recordingMaintenanceEmailService is a minimal services.EmailService
// that captures SendMaintenanceReminderEmail invocations so tests can
// assert the (schedule, threshold, recipient) tuples. Every other
// method is a no-op — the maintenance worker only ever calls
// SendMaintenanceReminderEmail.
type recordingMaintenanceEmailService struct {
	mu    sync.Mutex
	calls []recordedMaintenanceEmail
}

type recordedMaintenanceEmail struct {
	to            string
	name          string
	commodityName string
	title         string
	dueDate       string
	commodityURL  string
	thresholdDays int
}

func (r *recordingMaintenanceEmailService) SendVerificationEmail(_ context.Context, _, _, _ string) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendPasswordResetEmail(_ context.Context, _, _, _ string) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendMagicLinkEmail(_ context.Context, _, _, _ string) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendPasswordChangedEmail(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendWelcomeEmail(_ context.Context, _, _ string) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendWarrantyReminderEmail(_ context.Context, _, _, _, _, _ string, _ int) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendGroupInviteEmail(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendStorageQuotaWarningEmail(_ context.Context, _, _, _ string, _, _ int, _, _ string, _ []string, _, _ string) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendLoanReminderEmail(_ context.Context, _, _, _, _, _, _, _, _ string, _ int) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendFeedbackEmail(_ context.Context, _, _, _, _, _, _, _ string, _ []string) error {
	return nil
}
func (r *recordingMaintenanceEmailService) SendMaintenanceReminderEmail(_ context.Context, to, name, commodityName, title, dueDate, commodityURL string, thresholdDays int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedMaintenanceEmail{
		to:            to,
		name:          name,
		commodityName: commodityName,
		title:         title,
		dueDate:       dueDate,
		commodityURL:  commodityURL,
		thresholdDays: thresholdDays,
	})
	return nil
}

func (r *recordingMaintenanceEmailService) snapshot() []recordedMaintenanceEmail {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedMaintenanceEmail, len(r.calls))
	copy(out, r.calls)
	return out
}

// dateOffset is a tiny helper that returns YYYY-MM-DD relative to a
// reference clock — keeps test setup readable.
func dateOffset(now time.Time, days int) models.Date {
	return models.Date(now.AddDate(0, 0, days).Format("2006-01-02"))
}

func newMaintenanceScheduleFixture(c *qt.C) (context.Context, *registry.Set, string, *registry.FactorySet) {
	c.Helper()
	ctx, regSet, areaID, factorySet := newWarrantyServiceFixture(c)
	return ctx, regSet, areaID, factorySet
}

// TestMaintenanceReminderService_RemindOnce_TickClock pins the
// acceptance criterion from issue #1368: a schedule whose next_due_at
// is 7 days out fires the 14-day AND 7-day reminders on the first
// tick, and a second tick is a no-op (idempotency rows guard against
// duplicates).
func TestMaintenanceReminderService_RemindOnce_TickClock(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newMaintenanceScheduleFixture(c)

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	commodity, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID:    new(areaID),
		Name:      "espresso machine",
		ShortName: "espresso",
		Type:      models.CommodityTypeWhiteGoods,
		Status:    models.CommodityStatusInUse,
		Count:     1,
	})
	c.Assert(err, qt.IsNil)

	dueIn7 := dateOffset(now, 7)
	_, err = regSet.MaintenanceScheduleRegistry.Create(ctx, models.MaintenanceSchedule{
		CommodityID:  commodity.ID,
		Title:        "Descale espresso machine",
		IntervalDays: 90,
		NextDueAt:    dueIn7,
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)

	email := &recordingMaintenanceEmailService{}
	svc := services.NewMaintenanceReminderService(factorySet, email, nil)

	stats, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	// 14-day + 7-day thresholds both match a 7-days-out schedule.
	c.Assert(stats.Sent(), qt.Equals, 2)
	c.Assert(stats.SentByThreshold[models.MaintenanceReminder14Days], qt.Equals, 1)
	c.Assert(stats.SentByThreshold[models.MaintenanceReminder7Days], qt.Equals, 1)
	c.Assert(stats.SentByThreshold[models.MaintenanceReminder1Day], qt.Equals, 0)
	c.Assert(stats.Failed, qt.Equals, 0)

	// Recipient resolution falls back to the commodity's creator when
	// no admin memberships exist (single-user install).
	c.Assert(email.snapshot(), qt.HasLen, 2)

	// Second tick at the same clock — every reminder row already
	// exists, so nothing new is sent. The Failed counter must stay
	// at zero.
	stats2, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats2.Sent(), qt.Equals, 0)
	c.Assert(stats2.Failed, qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 2)
}

// TestMaintenanceReminderService_OverdueOnce verifies that a schedule
// past its due date emits the overdue-threshold reminder exactly once
// — the 14/7/1 thresholds are no longer applicable once the row is
// past due.
func TestMaintenanceReminderService_OverdueOnce(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newMaintenanceScheduleFixture(c)

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	commodity, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID: new(areaID), Name: "kettle", ShortName: "kettle",
		Type: models.CommodityTypeWhiteGoods, Status: models.CommodityStatusInUse, Count: 1,
	})
	c.Assert(err, qt.IsNil)

	// next_due_at = 3 days ago → overdue.
	_, err = regSet.MaintenanceScheduleRegistry.Create(ctx, models.MaintenanceSchedule{
		CommodityID:  commodity.ID,
		Title:        "Descale kettle",
		IntervalDays: 60,
		NextDueAt:    dateOffset(now, -3),
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)

	email := &recordingMaintenanceEmailService{}
	svc := services.NewMaintenanceReminderService(factorySet, email, nil)

	stats, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.SentByThreshold[models.MaintenanceReminderOverdue], qt.Equals, 1)
	c.Assert(stats.Sent(), qt.Equals, 1)
	c.Assert(email.snapshot(), qt.HasLen, 1)
	c.Assert(email.snapshot()[0].thresholdDays, qt.Equals, 0)

	// Second sweep — already sent, no duplicate.
	stats2, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats2.Sent(), qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 1)
}

// TestMaintenanceReminderService_DisabledSchedule confirms disabled
// schedules are ignored by the worker even if next_due_at is in the
// reminder window.
func TestMaintenanceReminderService_DisabledSchedule(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newMaintenanceScheduleFixture(c)

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	commodity, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID: new(areaID), Name: "boiler", ShortName: "boiler",
		Type: models.CommodityTypeWhiteGoods, Status: models.CommodityStatusInUse, Count: 1,
	})
	c.Assert(err, qt.IsNil)
	_, err = regSet.MaintenanceScheduleRegistry.Create(ctx, models.MaintenanceSchedule{
		CommodityID:  commodity.ID,
		Title:        "Annual service",
		IntervalDays: 365,
		NextDueAt:    dateOffset(now, 1),
		Enabled:      false,
	})
	c.Assert(err, qt.IsNil)

	email := &recordingMaintenanceEmailService{}
	svc := services.NewMaintenanceReminderService(factorySet, email, nil)
	stats, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 0)
}

// TestMaintenanceScheduleService_MarkDoneAdvances pins the acceptance
// criterion from issue #1368: a "every 90 days" schedule marked done
// advances next_due_at by exactly 90 days.
func TestMaintenanceScheduleService_MarkDoneAdvances(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newMaintenanceScheduleFixture(c)

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	commodity, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID: new(areaID), Name: "fridge", ShortName: "fridge",
		Type: models.CommodityTypeWhiteGoods, Status: models.CommodityStatusInUse, Count: 1,
	})
	c.Assert(err, qt.IsNil)
	created, err := regSet.MaintenanceScheduleRegistry.Create(ctx, models.MaintenanceSchedule{
		CommodityID:  commodity.ID,
		Title:        "Replace water filter",
		IntervalDays: 90,
		NextDueAt:    dateOffset(now, 30),
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)

	svc := services.NewMaintenanceScheduleService(factorySet)
	updated, err := svc.MarkDone(ctx, created.ID, nil, now)
	c.Assert(err, qt.IsNil)
	expected := dateOffset(now, 90)
	c.Assert(string(updated.NextDueAt), qt.Equals, string(expected))
	c.Assert(updated.LastDoneAt, qt.IsNotNil)
	c.Assert(string(*updated.LastDoneAt), qt.Equals, now.UTC().Format("2006-01-02"))
}

// TestMaintenanceScheduleService_MarkDoneClearsReminders confirms the
// idempotency rows are cleared on MarkDone so the next cycle starts
// fresh.
func TestMaintenanceScheduleService_MarkDoneClearsReminders(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newMaintenanceScheduleFixture(c)

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	commodity, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID: new(areaID), Name: "espresso", ShortName: "espresso",
		Type: models.CommodityTypeWhiteGoods, Status: models.CommodityStatusInUse, Count: 1,
	})
	c.Assert(err, qt.IsNil)
	created, err := regSet.MaintenanceScheduleRegistry.Create(ctx, models.MaintenanceSchedule{
		CommodityID:  commodity.ID,
		Title:        "Descale machine",
		IntervalDays: 60,
		NextDueAt:    dateOffset(now, 3),
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)

	// First sweep — 14/7/1 thresholds all match a 3-days-out schedule.
	email := &recordingMaintenanceEmailService{}
	reminder := services.NewMaintenanceReminderService(factorySet, email, nil)
	_, err = reminder.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)

	// Mark done — clears reminders, advances next_due_at.
	scheduleSvc := services.NewMaintenanceScheduleService(factorySet)
	_, err = scheduleSvc.MarkDone(ctx, created.ID, nil, now)
	c.Assert(err, qt.IsNil)

	// Second sweep — the new next_due_at is 60d out so no thresholds
	// match. Nothing fires.
	stats, err := reminder.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 0)
}

// TestMaintenanceScheduleService_CreateDefaultsNextDueAt confirms that
// an empty NextDueAt is defaulted to today + interval_days at create
// time.
func TestMaintenanceScheduleService_CreateDefaultsNextDueAt(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newMaintenanceScheduleFixture(c)

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	commodity, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID: new(areaID), Name: "vacuum", ShortName: "vacuum",
		Type: models.CommodityTypeWhiteGoods, Status: models.CommodityStatusInUse, Count: 1,
	})
	c.Assert(err, qt.IsNil)

	svc := services.NewMaintenanceScheduleService(factorySet)
	created, err := svc.Create(ctx, models.MaintenanceSchedule{
		CommodityID:  commodity.ID,
		Title:        "Empty bag",
		IntervalDays: 30,
		Enabled:      true,
	}, now)
	c.Assert(err, qt.IsNil)
	expected := dateOffset(now, 30)
	c.Assert(string(created.NextDueAt), qt.Equals, string(expected))
}
