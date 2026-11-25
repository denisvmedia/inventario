package services_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/notifications"
)

// flakyUserRegistry fails Get for one user a set number of times with an
// error other than not-found, the way a dropped database connection does.
type flakyUserRegistry struct {
	registry.UserRegistry
	failID   string
	failures int
}

func (r *flakyUserRegistry) Get(ctx context.Context, id string) (*models.User, error) {
	if id == r.failID && r.failures > 0 {
		r.failures--
		return nil, errors.New("connection reset")
	}
	return r.UserRegistry.Get(ctx, id)
}

// flakyWarrantyEmail records warranty emails, or fails them while failing
// is set.
type flakyWarrantyEmail struct {
	*recordingEmailService
	failing bool
}

func (f *flakyWarrantyEmail) SendWarrantyReminderEmail(ctx context.Context, to, name, commodityName, expiryDate, commodityURL string, thresholdDays int) error {
	if f.failing {
		return errors.New("queue down")
	}
	return f.recordingEmailService.SendWarrantyReminderEmail(ctx, to, name, commodityName, expiryDate, commodityURL, thresholdDays)
}

// flakyMaintenanceEmail records maintenance emails, or fails them while
// failing is set.
type flakyMaintenanceEmail struct {
	*recordingMaintenanceEmailService
	failing bool
}

func (f *flakyMaintenanceEmail) SendMaintenanceReminderEmail(ctx context.Context, to, name, commodityName, title, dueDate, commodityURL string, thresholdDays int) error {
	if f.failing {
		return errors.New("queue down")
	}
	return f.recordingMaintenanceEmailService.SendMaintenanceReminderEmail(ctx, to, name, commodityName, title, dueDate, commodityURL, thresholdDays)
}

func recipientsByItem(pairs [][2]string) map[string][]string {
	out := map[string][]string{}
	for _, p := range pairs {
		out[p[0]] = append(out[p[0]], p[1])
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// A failed lookup of one admin must not leave that admin out of the
// recipient list the sweep reuses for the rest of the group: the rows
// written from it would keep the admin from ever getting the reminders.
func TestWarrantyReminderService_UserLookupFailureRetriesTheItem(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	partner := env.addAdmin(c, "partner@example.com")
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	for _, name := range []string{"first", "second"} {
		expires := dateOffset(now, 30)
		_, err := env.regSet.CommodityRegistry.Create(env.ctx, models.Commodity{
			Name:              name,
			ShortName:         name,
			Type:              models.CommodityTypeWhiteGoods,
			Status:            models.CommodityStatusInUse,
			Count:             1,
			WarrantyExpiresAt: &expires,
		})
		c.Assert(err, qt.IsNil)
	}
	env.fs.UserRegistry = &flakyUserRegistry{UserRegistry: env.fs.UserRegistry, failID: partner.ID, failures: 1}

	email := &recordingEmailService{}
	svc := services.NewWarrantyReminderService(env.fs, email, urlBuilder).
		WithPreferences(notifications.NewService(env.fs.SettingsRegistryFactory))
	sent := func() map[string][]string {
		var pairs [][2]string
		for _, call := range email.snapshot() {
			pairs = append(pairs, [2]string{call.commodityName, call.to})
		}
		return recipientsByItem(pairs)
	}

	// The item whose lookup failed is retried; the other one reaches both
	// admins.
	stats, err := svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 1)
	c.Assert(email.snapshot(), qt.HasLen, 2)
	for _, to := range sent() {
		c.Assert(to, qt.DeepEquals, []string{"owner@example.com", partner.Email})
	}

	stats, err = svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(sent(), qt.DeepEquals, map[string][]string{
		"first":  {"owner@example.com", partner.Email},
		"second": {"owner@example.com", partner.Email},
	})
}

func TestMaintenanceReminderService_UserLookupFailureRetriesTheSchedule(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	partner := env.addAdmin(c, "partner@example.com")
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	for _, title := range []string{"first", "second"} {
		_, err := env.regSet.MaintenanceScheduleRegistry.Create(env.ctx, models.MaintenanceSchedule{
			CommodityID:  env.commodity.ID,
			Title:        title,
			IntervalDays: 90,
			NextDueAt:    dateOffset(now, 7),
			Enabled:      true,
		})
		c.Assert(err, qt.IsNil)
	}
	env.fs.UserRegistry = &flakyUserRegistry{UserRegistry: env.fs.UserRegistry, failID: partner.ID, failures: 1}

	email := &recordingMaintenanceEmailService{}
	svc := services.NewMaintenanceReminderService(env.fs, email, urlBuilder).
		WithPreferences(notifications.NewService(env.fs.SettingsRegistryFactory))
	sent := func() map[string][]string {
		var pairs [][2]string
		for _, call := range email.snapshot() {
			pairs = append(pairs, [2]string{call.title, call.to})
		}
		return recipientsByItem(pairs)
	}

	stats, err := svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 1)
	c.Assert(email.snapshot(), qt.HasLen, 2)
	for _, to := range sent() {
		c.Assert(to, qt.DeepEquals, []string{"owner@example.com", partner.Email})
	}

	stats, err = svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(sent(), qt.DeepEquals, map[string][]string{
		"first":  {"owner@example.com", partner.Email},
		"second": {"owner@example.com", partner.Email},
	})
}

// A failed lender lookup is not "no recipient": the flag stays false and
// the next sweep sends the reminder.
func TestLoanReminderService_LenderLookupFailureLeavesFlagFalse(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, commodityID, factorySet := newLoanReminderServiceFixture(c)
	lender := appctx.UserFromContext(ctx)
	c.Assert(lender, qt.IsNotNil)
	dueDate := models.Date("2026-05-01")
	loan, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Lera",
		LentAt:       "2026-04-15",
		DueBackAt:    models.PDate(&dueDate),
	})
	c.Assert(err, qt.IsNil)
	factorySet.UserRegistry = &flakyUserRegistry{UserRegistry: factorySet.UserRegistry, failID: lender.ID, failures: 1}

	email := &recordingLoanEmailService{}
	svc := services.NewLoanReminderService(factorySet, email, nil)
	tick := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

	stats, err := svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 1)
	c.Assert(email.snapshot(), qt.HasLen, 0)
	reloaded, err := regSet.CommodityLoanRegistry.Get(ctx, loan.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.ReminderSentOverdue, qt.IsFalse)

	stats, err = svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent[registry.LoanReminderKindOverdue], qt.Equals, 1)
	reloaded, err = regSet.CommodityLoanRegistry.Get(ctx, loan.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.ReminderSentOverdue, qt.IsTrue)
}

// A row written without an email (no recipient was due that lead time)
// must not hold back a farther lead time chosen later. Here the 60-day
// row is silent and the 30-day email failed, so no reminder has gone out
// when the owner switches to a 90-day lead time.
func TestWarrantyReminderService_SilentRowDoesNotHoldBackAFartherLeadTime(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	expires := dateOffset(now, 30)
	_, err := env.regSet.CommodityRegistry.Create(env.ctx, models.Commodity{
		Name:              "Fridge",
		ShortName:         "fridge",
		Type:              models.CommodityTypeWhiteGoods,
		Status:            models.CommodityStatusInUse,
		Count:             1,
		WarrantyExpiresAt: &expires,
	})
	c.Assert(err, qt.IsNil)

	email := &flakyWarrantyEmail{recordingEmailService: &recordingEmailService{}, failing: true}
	svc := services.NewWarrantyReminderService(env.fs, email, urlBuilder).
		WithPreferences(notifications.NewService(env.fs.SettingsRegistryFactory))

	stats, err := svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 1)

	setUserSetting(c, env.fs, env.user, models.SettingNameNotificationsWarrantyLeadTimes, []int{90})
	email.failing = false
	stats, err = svc.RemindOnce(env.ctx, now.AddDate(0, 0, 1))
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(warrantySends(email.recordingEmailService), qt.DeepEquals, []sentTo{{To: "owner@example.com", Days: 90}})
}

func TestMaintenanceReminderService_SilentRowDoesNotHoldBackAFartherLeadTime(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	_, err := env.regSet.MaintenanceScheduleRegistry.Create(env.ctx, models.MaintenanceSchedule{
		CommodityID:  env.commodity.ID,
		Title:        "Clean the filter",
		IntervalDays: 90,
		NextDueAt:    dateOffset(now, 7),
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)

	email := &flakyMaintenanceEmail{recordingMaintenanceEmailService: &recordingMaintenanceEmailService{}, failing: true}
	svc := services.NewMaintenanceReminderService(env.fs, email, urlBuilder).
		WithPreferences(notifications.NewService(env.fs.SettingsRegistryFactory))

	// 7 days out with the defaults [14, 7, 1]: the 14-day row is silent and
	// the 7-day email fails.
	stats, err := svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 1)

	setUserSetting(c, env.fs, env.user, models.SettingNameNotificationsMaintenanceLeadTimes, []int{30})
	email.failing = false
	stats, err = svc.RemindOnce(env.ctx, now.AddDate(0, 0, 1))
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(maintenanceSends(email.recordingMaintenanceEmailService), qt.DeepEquals, []sentTo{{To: "owner@example.com", Days: 30}})
}
