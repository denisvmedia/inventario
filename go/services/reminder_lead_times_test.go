package services_test

import (
	"context"
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

// Per-user lead times for the built-in reminders (#2805).

func setUserSetting(c *qt.C, fs *registry.FactorySet, user *models.User, name models.SettingName, value any) {
	c.Helper()
	ctx := appctx.WithUser(context.Background(), user)
	reg, err := fs.SettingsRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(reg.Patch(ctx, string(name), value), qt.IsNil)
}

// addAdmin adds a second admin to the env's group.
func (e *itemReminderEnv) addAdmin(c *qt.C, email string) *models.User {
	c.Helper()
	bg := context.Background()
	user, err := e.fs.UserRegistry.Create(bg, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: e.user.TenantID},
		Email:               email,
		Name:                "Partner",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	_, err = e.fs.GroupMembershipRegistry.Create(bg, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: e.user.TenantID},
		GroupID:             e.group.ID,
		MemberUserID:        user.ID,
		Role:                models.GroupRoleAdmin,
	})
	c.Assert(err, qt.IsNil)
	return user
}

type sentTo struct {
	To   string
	Days int
}

func sortedSends(in []sentTo) []sentTo {
	sort.Slice(in, func(i, j int) bool {
		if in[i].To != in[j].To {
			return in[i].To < in[j].To
		}
		return in[i].Days > in[j].Days
	})
	return in
}

func warrantySends(r *recordingEmailService) []sentTo {
	var out []sentTo
	for _, call := range r.snapshot() {
		out = append(out, sentTo{To: call.to, Days: call.thresholdDays})
	}
	return sortedSends(out)
}

func maintenanceSends(r *recordingMaintenanceEmailService) []sentTo {
	var out []sentTo
	for _, call := range r.snapshot() {
		out = append(out, sentTo{To: call.to, Days: call.thresholdDays})
	}
	return sortedSends(out)
}

func TestWarrantyReminderService_PerUserLeadTimes(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	partner := env.addAdmin(c, "partner@example.com")
	setUserSetting(c, env.fs, env.user, models.SettingNameNotificationsWarrantyLeadTimes, []any{float64(90), float64(10)})

	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	expires := dateOffset(now, 50)
	_, err := env.regSet.CommodityRegistry.Create(env.ctx, models.Commodity{
		Name:              "Fridge",
		ShortName:         "fridge",
		Type:              models.CommodityTypeWhiteGoods,
		Status:            models.CommodityStatusInUse,
		Count:             1,
		WarrantyExpiresAt: &expires,
	})
	c.Assert(err, qt.IsNil)

	email := &recordingEmailService{}
	svc := services.NewWarrantyReminderService(env.fs, email, urlBuilder).
		WithPreferences(notifications.NewService(env.fs.SettingsRegistryFactory))

	// 50 days out: the owner's 90 and the partner's default 60 are due.
	stats, err := svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(warrantySends(email), qt.DeepEquals, []sentTo{
		{To: "owner@example.com", Days: 90},
		{To: partner.Email, Days: 60},
	})

	// Same day again: nothing new.
	_, err = svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(email.snapshot(), qt.HasLen, 2)

	// 30 days out: only the partner chose 30.
	_, err = svc.RemindOnce(env.ctx, now.AddDate(0, 0, 20))
	c.Assert(err, qt.IsNil)
	// 10 days out: only the owner chose 10.
	_, err = svc.RemindOnce(env.ctx, now.AddDate(0, 0, 40))
	c.Assert(err, qt.IsNil)
	c.Assert(warrantySends(email), qt.DeepEquals, []sentTo{
		{To: "owner@example.com", Days: 90},
		{To: "owner@example.com", Days: 10},
		{To: partner.Email, Days: 60},
		{To: partner.Email, Days: 30},
	})
}

func TestMaintenanceReminderService_PerUserLeadTimes(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	partner := env.addAdmin(c, "partner@example.com")
	setUserSetting(c, env.fs, env.user, models.SettingNameNotificationsMaintenanceLeadTimes, []int{21})

	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	_, err := env.regSet.MaintenanceScheduleRegistry.Create(env.ctx, models.MaintenanceSchedule{
		CommodityID:  env.commodity.ID,
		Title:        "Clean the filter",
		IntervalDays: 90,
		NextDueAt:    dateOffset(now, 20),
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)

	email := &recordingMaintenanceEmailService{}
	svc := services.NewMaintenanceReminderService(env.fs, email, urlBuilder).
		WithPreferences(notifications.NewService(env.fs.SettingsRegistryFactory))

	// 20 days out: the owner's 21 is due; the partner's default 14 is not.
	_, err = svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(maintenanceSends(email), qt.DeepEquals, []sentTo{{To: "owner@example.com", Days: 21}})

	// 7 days out: the sweeps missed the partner's 14-day mark, so the
	// partner gets only the closest reached lead time, 7. The owner gets
	// nothing more.
	_, err = svc.RemindOnce(env.ctx, now.AddDate(0, 0, 13))
	c.Assert(err, qt.IsNil)
	// Overdue: everyone gets the overdue reminder, whatever their lead times.
	_, err = svc.RemindOnce(env.ctx, now.AddDate(0, 0, 21))
	c.Assert(err, qt.IsNil)
	c.Assert(maintenanceSends(email), qt.DeepEquals, []sentTo{
		{To: "owner@example.com", Days: 21},
		{To: "owner@example.com", Days: 0},
		{To: partner.Email, Days: 7},
		{To: partner.Email, Days: 0},
	})
}

// Raising a lead time must not send a burst of reminders for items that
// are already closer than it, or that already had a closer reminder.
func TestWarrantyReminderService_RaisingALeadTimeSendsNoBurst(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	addItem := func(name string, days int) {
		expires := dateOffset(now, days)
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
	addItem("tomorrow", 1)
	addItem("in-45", 45)
	addItem("in-75", 75)
	addItem("in-200", 200)

	email := &recordingEmailService{}
	svc := services.NewWarrantyReminderService(env.fs, email, urlBuilder).
		WithPreferences(notifications.NewService(env.fs.SettingsRegistryFactory))
	sends := func() map[string][]int {
		out := map[string][]int{}
		for _, call := range email.snapshot() {
			out[call.commodityName] = append(out[call.commodityName], call.thresholdDays)
		}
		return out
	}

	// Default lead times 60, 30, 7: one email per item inside 60 days, at
	// the closest lead time reached.
	_, err := svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(sends(), qt.DeepEquals, map[string][]int{"tomorrow": {7}, "in-45": {60}})

	// The owner raises the longest lead time to 365. Only the item that
	// had no reminder yet gets one; "tomorrow" is past every new lead
	// time except the 7 already sent, and "in-45" already had its 60.
	setUserSetting(c, env.fs, env.user, models.SettingNameNotificationsWarrantyLeadTimes, []int{365, 30, 7})
	_, err = svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(sends(), qt.DeepEquals, map[string][]int{
		"tomorrow": {7},
		"in-45":    {60},
		"in-75":    {365},
		"in-200":   {365},
	})

	// 15 days later "in-45" reaches 30 and gets it; nothing else is due.
	_, err = svc.RemindOnce(env.ctx, now.AddDate(0, 0, 15))
	c.Assert(err, qt.IsNil)
	c.Assert(sends(), qt.DeepEquals, map[string][]int{
		"tomorrow": {7},
		"in-45":    {60, 30},
		"in-75":    {365},
		"in-200":   {365},
	})
}

func TestMaintenanceReminderService_RaisingALeadTimeSendsNoBurst(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	addSchedule := func(title string, days int) {
		_, err := env.regSet.MaintenanceScheduleRegistry.Create(env.ctx, models.MaintenanceSchedule{
			CommodityID:  env.commodity.ID,
			Title:        title,
			IntervalDays: 365,
			NextDueAt:    dateOffset(now, days),
			Enabled:      true,
		})
		c.Assert(err, qt.IsNil)
	}
	addSchedule("tomorrow", 1)
	addSchedule("in-10", 10)
	addSchedule("in-40", 40)

	email := &recordingMaintenanceEmailService{}
	svc := services.NewMaintenanceReminderService(env.fs, email, urlBuilder).
		WithPreferences(notifications.NewService(env.fs.SettingsRegistryFactory))
	sends := func() map[string][]int {
		out := map[string][]int{}
		for _, call := range email.snapshot() {
			out[call.title] = append(out[call.title], call.thresholdDays)
		}
		return out
	}

	// Default lead times 14, 7, 1.
	_, err := svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(sends(), qt.DeepEquals, map[string][]int{"tomorrow": {1}, "in-10": {14}})

	setUserSetting(c, env.fs, env.user, models.SettingNameNotificationsMaintenanceLeadTimes, []int{60, 7, 1})
	_, err = svc.RemindOnce(env.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(sends(), qt.DeepEquals, map[string][]int{"tomorrow": {1}, "in-10": {14}, "in-40": {60}})
}

func TestLoanReminderService_LenderLeadTime(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, commodityID, factorySet := newLoanReminderServiceFixture(c)
	lender := appctx.UserFromContext(ctx)
	c.Assert(lender, qt.IsNotNil)

	tick := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	newLoan := func(borrower string, dueInDays int) *models.CommodityLoan {
		due := dateOffset(tick, dueInDays)
		loan, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
			CommodityID:  commodityID,
			BorrowerName: borrower,
			LentAt:       "2026-05-01",
			DueBackAt:    models.PDate(&due),
		})
		c.Assert(err, qt.IsNil)
		return loan
	}
	in10 := newLoan("Anya", 10)
	in20 := newLoan("Boris", 20)

	email := &recordingLoanEmailService{}
	svc := services.NewLoanReminderService(factorySet, email, nil).
		WithPreferences(notifications.NewService(factorySet.SettingsRegistryFactory))

	// The server default (7 days) covers neither loan.
	stats, err := svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent[registry.LoanReminderKindDueSoon], qt.Equals, 0)

	// With a 14-day lead time the loan due in 10 days is reminded; the one
	// due in 20 days stays a candidate.
	setUserSetting(c, factorySet, lender, models.SettingNameNotificationsLoanLeadTime, float64(14))
	stats, err = svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent[registry.LoanReminderKindDueSoon], qt.Equals, 1)
	reloaded, err := regSet.CommodityLoanRegistry.Get(ctx, in10.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.ReminderSentDueSoon, qt.IsTrue)
	reloaded, err = regSet.CommodityLoanRegistry.Get(ctx, in20.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.ReminderSentDueSoon, qt.IsFalse)

	// A shorter lead time than the server default holds a loan back.
	setUserSetting(c, factorySet, lender, models.SettingNameNotificationsLoanLeadTime, 3)
	in5 := newLoan("Carla", 5)
	stats, err = svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent[registry.LoanReminderKindDueSoon], qt.Equals, 0)
	reloaded, err = regSet.CommodityLoanRegistry.Get(ctx, in5.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.ReminderSentDueSoon, qt.IsFalse)
	c.Assert(email.snapshot(), qt.HasLen, 1)
}

func TestPreferencesCache_LeadTimes(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	prefs := notifications.NewService(env.fs.SettingsRegistryFactory)

	// Unset: the defaults.
	cache := prefs.NewCache()
	c.Assert(cache.WarrantyLeadTimes(env.ctx, env.user), qt.DeepEquals, []int{60, 30, 7})
	c.Assert(cache.MaintenanceLeadTimes(env.ctx, env.user), qt.DeepEquals, []int{14, 7, 1})
	_, ok := cache.LoanLeadTime(env.ctx, env.user)
	c.Assert(ok, qt.IsFalse)

	// Stored values come back sorted, without duplicates or values out
	// of range.
	setUserSetting(c, env.fs, env.user, models.SettingNameNotificationsWarrantyLeadTimes, []int{7, 400, 90, 7, 0})
	setUserSetting(c, env.fs, env.user, models.SettingNameNotificationsMaintenanceLeadTimes, []int{0})
	setUserSetting(c, env.fs, env.user, models.SettingNameNotificationsLoanLeadTime, 10)
	cache = prefs.NewCache()
	c.Assert(cache.WarrantyLeadTimes(env.ctx, env.user), qt.DeepEquals, []int{90, 7})
	c.Assert(cache.MaintenanceLeadTimes(env.ctx, env.user), qt.DeepEquals, []int{14, 7, 1})
	days, ok := cache.LoanLeadTime(env.ctx, env.user)
	c.Assert(ok, qt.IsTrue)
	c.Assert(days, qt.Equals, 10)

	// A nil cache (no preferences service) answers with the defaults.
	var none *notifications.Cache
	c.Assert(none.WarrantyLeadTimes(env.ctx, env.user), qt.DeepEquals, []int{60, 30, 7})
	_, ok = none.LoanLeadTime(env.ctx, env.user)
	c.Assert(ok, qt.IsFalse)
}
