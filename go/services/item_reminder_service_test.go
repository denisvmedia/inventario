package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/notifications"
)

// recordingItemReminderEmail records SendItemReminderEmail calls; every
// other EmailService method comes from the stub.
type recordingItemReminderEmail struct {
	*services.StubEmailService
	mu    sync.Mutex
	calls []recordedItemReminder
	fail  error
}

type recordedItemReminder struct {
	to, commodityName, title, dueDate, url string
	daysLeft                               int
}

func newRecordingItemReminderEmail() *recordingItemReminderEmail {
	return &recordingItemReminderEmail{StubEmailService: services.NewStubEmailService()}
}

func (r *recordingItemReminderEmail) SendItemReminderEmail(_ context.Context, to, _, commodityName, title, dueDate, commodityURL string, daysLeft int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail != nil {
		return r.fail
	}
	r.calls = append(r.calls, recordedItemReminder{to: to, commodityName: commodityName, title: title, dueDate: dueDate, url: commodityURL, daysLeft: daysLeft})
	return nil
}

func (r *recordingItemReminderEmail) snapshot() []recordedItemReminder {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedItemReminder(nil), r.calls...)
}

type itemReminderEnv struct {
	fs        *registry.FactorySet
	ctx       context.Context
	regSet    *registry.Set
	user      *models.User
	group     *models.LocationGroup
	commodity *models.Commodity
}

func newItemReminderEnv(c *qt.C) *itemReminderEnv {
	c.Helper()
	bg := context.Background()
	fs := memory.NewFactorySet()
	tenant, err := fs.TenantRegistry.Create(bg, models.Tenant{Name: "T", Slug: "t", Status: models.TenantStatusActive})
	c.Assert(err, qt.IsNil)
	user, err := fs.UserRegistry.Create(bg, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "owner@example.com",
		Name:                "Owner",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	group, err := fs.LocationGroupRegistry.Create(bg, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Name:                "Home",
		Slug:                "home-slug",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupMembershipRegistry.Create(bg, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		GroupID:             group.ID,
		MemberUserID:        user.ID,
		Role:                models.GroupRoleAdmin,
	})
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithGroup(appctx.WithUser(bg, user), group)
	regSet := must.Must(fs.CreateUserRegistrySet(ctx))
	commodity, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:      "Dishwasher",
		ShortName: "dw",
		Type:      models.CommodityTypeWhiteGoods,
		Status:    models.CommodityStatusInUse,
		Count:     1,
	})
	c.Assert(err, qt.IsNil)
	return &itemReminderEnv{fs: fs, ctx: ctx, regSet: regSet, user: user, group: group, commodity: commodity}
}

func (e *itemReminderEnv) addReminder(c *qt.C, r models.CommodityReminder) *models.CommodityReminder {
	c.Helper()
	r.CommodityID = e.commodity.ID
	if r.Title == "" {
		r.Title = "Return window ends"
	}
	created, err := services.NewCommodityReminderService(e.fs).Create(e.ctx, r)
	c.Assert(err, qt.IsNil)
	return created
}

func urlBuilder(slug, id string) string {
	return "https://inv.example/g/" + slug + "/commodities/" + id
}

func TestDueLeadTime(t *testing.T) {
	tests := []struct {
		name     string
		leads    []int
		daysLeft int
		want     int
		wantOK   bool
	}{
		{name: "not reached yet", leads: []int{30, 7}, daysLeft: 31, wantOK: false},
		{name: "exactly at a lead time", leads: []int{30, 7}, daysLeft: 30, want: 30, wantOK: true},
		{name: "between lead times", leads: []int{30, 7}, daysLeft: 12, want: 30, wantOK: true},
		{name: "late start picks the tightest reached lead", leads: []int{30, 7, 1}, daysLeft: 3, want: 7, wantOK: true},
		{name: "on the day", leads: []int{7, 0}, daysLeft: 0, want: 0, wantOK: true},
		{name: "on the day without a zero lead still uses the smallest", leads: []int{7, 1}, daysLeft: 0, want: 1, wantOK: true},
		{name: "past", leads: []int{7}, daysLeft: -1, wantOK: false},
		{name: "unordered input", leads: []int{1, 30, 7}, daysLeft: 5, want: 7, wantOK: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got, ok := services.DueLeadTime(tc.leads, tc.daysLeft)
			c.Assert(ok, qt.Equals, tc.wantOK)
			if tc.wantOK {
				c.Assert(got, qt.Equals, tc.want)
			}
		})
	}
}

func TestItemReminderService_SendsOncePerLeadTime(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	env.addReminder(c, models.CommodityReminder{
		DueOn:            dateOffset(now, 10),
		RemindDaysBefore: models.ValuerSlice[int]{30, 7, 1},
	})

	email := newRecordingItemReminderEmail()
	svc := services.NewItemReminderService(env.fs, email, urlBuilder)

	stats, err := svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats, qt.Equals, services.ItemReminderStats{Sent: 1})
	calls := email.snapshot()
	c.Assert(calls, qt.HasLen, 1)
	c.Assert(calls[0], qt.Equals, recordedItemReminder{
		to:            "owner@example.com",
		commodityName: "Dishwasher",
		title:         "Return window ends",
		dueDate:       string(dateOffset(now, 10)),
		url:           urlBuilder("home-slug", env.commodity.ID),
		daysLeft:      10,
	})

	// The same day and the next one stay quiet: the 30-day email went out.
	for _, at := range []time.Time{now.Add(time.Hour), now.AddDate(0, 0, 1)} {
		stats, err = svc.RemindOnce(context.Background(), at)
		c.Assert(err, qt.IsNil)
		c.Assert(stats.Sent, qt.Equals, 0)
	}

	// Three days later the 7-day lead is reached, then the 1-day lead.
	stats, err = svc.RemindOnce(context.Background(), now.AddDate(0, 0, 3))
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 1)
	stats, err = svc.RemindOnce(context.Background(), now.AddDate(0, 0, 9))
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 1)
	calls = email.snapshot()
	c.Assert(calls, qt.HasLen, 3)
	c.Assert(calls[1].daysLeft, qt.Equals, 7)
	c.Assert(calls[2].daysLeft, qt.Equals, 1)

	// The occurrence day itself has no lead time left to send.
	stats, err = svc.RemindOnce(context.Background(), now.AddDate(0, 0, 10))
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 0)
}

func TestItemReminderService_RepeatingReminderSendsForEachOccurrence(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	env.addReminder(c, models.CommodityReminder{
		Title:            "Insurance renewal",
		DueOn:            "2026-01-31",
		RemindDaysBefore: models.ValuerSlice[int]{2},
		RepeatUnit:       models.ReminderRepeatMonth,
		RepeatInterval:   1,
	})
	email := newRecordingItemReminderEmail()
	svc := services.NewItemReminderService(env.fs, email, nil)

	for _, today := range []string{"2026-02-26", "2026-02-27", "2026-03-29", "2026-03-30"} {
		_, err := svc.RemindOnce(context.Background(), day(today))
		c.Assert(err, qt.IsNil)
	}
	calls := email.snapshot()
	c.Assert(calls, qt.HasLen, 2)
	c.Assert(calls[0].dueDate, qt.Equals, "2026-02-28")
	c.Assert(calls[0].daysLeft, qt.Equals, 2)
	c.Assert(calls[1].dueDate, qt.Equals, "2026-03-31")
	c.Assert(calls[1].daysLeft, qt.Equals, 2)
	c.Assert(calls[0].url, qt.Equals, "")
}

func TestItemReminderService_TwoWorkersSendOnce(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	env.addReminder(c, models.CommodityReminder{
		DueOn:            dateOffset(now, 5),
		RemindDaysBefore: models.ValuerSlice[int]{7},
	})

	email := newRecordingItemReminderEmail()
	// Two services over the same store stand in for two replicas.
	workers := []*services.ItemReminderService{
		services.NewItemReminderService(env.fs, email, nil),
		services.NewItemReminderService(env.fs, email, nil),
	}
	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Go(func() {
			_, err := w.RemindOnce(context.Background(), now)
			c.Check(err, qt.IsNil)
		})
	}
	wg.Wait()
	c.Assert(email.snapshot(), qt.HasLen, 1)
}

func TestItemReminderService_FailedEnqueueIsRetried(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	env.addReminder(c, models.CommodityReminder{
		DueOn:            dateOffset(now, 1),
		RemindDaysBefore: models.ValuerSlice[int]{1},
	})
	email := newRecordingItemReminderEmail()
	email.fail = errors.New("queue down")
	svc := services.NewItemReminderService(env.fs, email, nil)

	stats, err := svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats, qt.Equals, services.ItemReminderStats{Failed: 1})

	email.fail = nil
	stats, err = svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats, qt.Equals, services.ItemReminderStats{Sent: 1})
}

func TestItemReminderService_SkipsWhenRecipientCannotReceive(t *testing.T) {
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		setup func(c *qt.C, env *itemReminderEnv) *notifications.Service
	}{
		{
			name: "opted out of item reminders",
			setup: func(c *qt.C, env *itemReminderEnv) *notifications.Service {
				reg := must.Must(env.fs.SettingsRegistryFactory.CreateUserRegistry(env.ctx))
				c.Assert(reg.Patch(env.ctx, string(models.SettingNameNotificationsItemReminder), false), qt.IsNil)
				return notifications.NewService(env.fs.SettingsRegistryFactory)
			},
		},
		{
			name: "email channel off",
			setup: func(c *qt.C, env *itemReminderEnv) *notifications.Service {
				reg := must.Must(env.fs.SettingsRegistryFactory.CreateUserRegistry(env.ctx))
				c.Assert(reg.Patch(env.ctx, string(models.SettingNameNotificationsChannelEmail), false), qt.IsNil)
				return notifications.NewService(env.fs.SettingsRegistryFactory)
			},
		},
		{
			name: "left the group",
			setup: func(c *qt.C, env *itemReminderEnv) *notifications.Service {
				m, err := env.fs.GroupMembershipRegistry.GetByGroupAndUser(context.Background(), env.group.ID, env.user.ID)
				c.Assert(err, qt.IsNil)
				c.Assert(env.fs.GroupMembershipRegistry.Delete(context.Background(), m.ID), qt.IsNil)
				return nil
			},
		},
		{
			name: "deactivated account",
			setup: func(c *qt.C, env *itemReminderEnv) *notifications.Service {
				u := *env.user
				u.IsActive = false
				_, err := env.fs.UserRegistry.Update(context.Background(), u)
				c.Assert(err, qt.IsNil)
				return nil
			},
		},
		{
			name: "item deleted",
			setup: func(c *qt.C, env *itemReminderEnv) *notifications.Service {
				c.Assert(env.regSet.CommodityRegistry.Delete(env.ctx, env.commodity.ID), qt.IsNil)
				return nil
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			env := newItemReminderEnv(c)
			env.addReminder(c, models.CommodityReminder{
				DueOn:            dateOffset(now, 3),
				RemindDaysBefore: models.ValuerSlice[int]{7},
			})
			prefs := tc.setup(c, env)
			email := newRecordingItemReminderEmail()
			svc := services.NewItemReminderService(env.fs, email, nil)
			if prefs != nil {
				svc.WithPreferences(prefs)
			}
			stats, err := svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats, qt.Equals, services.ItemReminderStats{})
			c.Assert(email.snapshot(), qt.HasLen, 0)
		})
	}
}

func TestItemReminderService_OptInAgainSendsCurrentLead(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	env.addReminder(c, models.CommodityReminder{
		DueOn:            dateOffset(now, 3),
		RemindDaysBefore: models.ValuerSlice[int]{7},
	})
	settings := must.Must(env.fs.SettingsRegistryFactory.CreateUserRegistry(env.ctx))
	c.Assert(settings.Patch(env.ctx, string(models.SettingNameNotificationsItemReminder), false), qt.IsNil)

	email := newRecordingItemReminderEmail()
	svc := services.NewItemReminderService(env.fs, email, nil).WithPreferences(notifications.NewService(env.fs.SettingsRegistryFactory))
	_, err := svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(email.snapshot(), qt.HasLen, 0)

	c.Assert(settings.Patch(env.ctx, string(models.SettingNameNotificationsItemReminder), true), qt.IsNil)
	_, err = svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(email.snapshot(), qt.HasLen, 1)
}

func TestCommodityReminderService_CreateNormalizesAndValidates(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	svc := services.NewCommodityReminderService(env.fs)

	created, err := svc.Create(env.ctx, models.CommodityReminder{
		CommodityID:      env.commodity.ID,
		Title:            "Inspection",
		DueOn:            "2027-03-01",
		RemindDaysBefore: models.ValuerSlice[int]{1, 14, 14},
	})
	c.Assert(err, qt.IsNil)
	c.Assert([]int(created.RemindDaysBefore), qt.DeepEquals, []int{14, 1})
	c.Assert(created.RepeatUnit, qt.Equals, models.ReminderRepeatNone)
	c.Assert(created.RepeatInterval, qt.Equals, 1)
	c.Assert(created.GroupID, qt.Equals, env.group.ID)

	_, err = svc.Create(env.ctx, models.CommodityReminder{
		CommodityID:      env.commodity.ID,
		Title:            "Inspection",
		DueOn:            "2027-03-01",
		RemindDaysBefore: models.ValuerSlice[int]{400},
	})
	c.Assert(err, qt.ErrorMatches, ".*remind_days_before.*")

	updated, err := svc.Update(env.ctx, created.ID, services.CommodityReminderUpdate{
		RepeatUnit:     new(models.ReminderRepeatYear),
		RepeatInterval: new(2),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(updated.RepeatUnit, qt.Equals, models.ReminderRepeatYear)
	c.Assert(updated.RepeatInterval, qt.Equals, 2)
	c.Assert(updated.Title, qt.Equals, "Inspection")

	_, err = svc.Update(env.ctx, created.ID, services.CommodityReminderUpdate{RepeatUnit: new(models.ReminderRepeatUnit("hourly"))})
	c.Assert(err, qt.ErrorMatches, ".*repeat_unit.*")
}

// day parses YYYY-MM-DD as a mid-morning UTC clock.
func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t.Add(9 * time.Hour)
}

// The worker sweeps once on start, so Start followed by Stop runs exactly one
// tick; the soft pause has to stop that tick before anything is sent.
func TestItemReminderWorker_SweepsOnStartAndHonorsThePause(t *testing.T) {
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		paused   models.WorkerType
		wantSent int
	}{
		{name: "running", paused: "", wantSent: 1},
		{name: "paused", paused: models.WorkerTypeItemReminder, wantSent: 0},
		{name: "another worker paused", paused: models.WorkerTypeWeeklyDigest, wantSent: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			env := newItemReminderEnv(c)
			env.addReminder(c, models.CommodityReminder{
				DueOn:            dateOffset(now, 2),
				RemindDaysBefore: models.ValuerSlice[int]{7},
			})
			email := newRecordingItemReminderEmail()
			pause := &pausedChecker{paused: tc.paused}
			worker := services.NewItemReminderWorker(
				services.NewItemReminderService(env.fs, email, nil),
				services.WithItemReminderClock(func() time.Time { return now }),
				services.WithItemReminderInterval(time.Hour),
				services.WithItemReminderPauseController(pause),
			)
			worker.Start(context.Background())
			worker.Stop()
			c.Assert(email.snapshot(), qt.HasLen, tc.wantSent)
			c.Assert(pause.asked.Load() > 0, qt.IsTrue)
		})
	}
}

func TestItemReminderService_NoRemindersWhileTrashed(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	env.addReminder(c, models.CommodityReminder{
		DueOn:            dateOffset(now, 10),
		RemindDaysBefore: models.ValuerSlice[int]{30},
	})
	trash := services.NewTrashService(env.fs, "")
	_, err := trash.TrashCommodity(env.ctx, env.commodity.ID)
	c.Assert(err, qt.IsNil)

	email := newRecordingItemReminderEmail()
	svc := services.NewItemReminderService(env.fs, email, nil)
	stats, err := svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 0)

	upcoming, err := services.NewUpcomingService().List(env.ctx, env.regSet, now, 30)
	c.Assert(err, qt.IsNil)
	c.Assert(upcoming, qt.HasLen, 0)

	// Restored, the item gets the reminder it missed.
	_, err = trash.RestoreCommodity(env.ctx, env.commodity.ID)
	c.Assert(err, qt.IsNil)
	stats, err = svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 1)
	upcoming, err = services.NewUpcomingService().List(env.ctx, env.regSet, now, 30)
	c.Assert(err, qt.IsNil)
	c.Assert(upcoming, qt.HasLen, 1)
}

func TestReminderTargetActive(t *testing.T) {
	c := qt.New(t)
	c.Assert(services.ReminderTargetActive(nil), qt.IsFalse)
	c.Assert(services.ReminderTargetActive(&models.Commodity{}), qt.IsTrue)
	c.Assert(services.ReminderTargetActive(&models.Commodity{DeletedAt: new(time.Now())}), qt.IsFalse)
}
