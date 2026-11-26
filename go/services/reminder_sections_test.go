package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// sectionReminderEnv is one group with an admin who receives the reminders
// and one item in it, for the tests that hide a section of the group.
type sectionReminderEnv struct {
	fs        *registry.FactorySet
	ctx       context.Context
	regSet    *registry.Set
	groups    *services.GroupService
	group     *models.LocationGroup
	commodity *models.Commodity
}

func newSectionReminderEnv(c *qt.C, warranty *models.Date) *sectionReminderEnv {
	c.Helper()
	bg := context.Background()
	fs := memory.NewFactorySet()
	tenant, err := fs.TenantRegistry.Create(bg, models.Tenant{Name: "T", Slug: "t", Status: models.TenantStatusActive})
	c.Assert(err, qt.IsNil)
	user, err := fs.UserRegistry.Create(bg, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "admin@example.com",
		Name:                "Admin",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	group, err := fs.LocationGroupRegistry.Create(bg, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Name:                "Home",
		Slug:                must.Must(models.GenerateGroupSlug()),
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
		Name:              "Dishwasher",
		ShortName:         "dw",
		Type:              models.CommodityTypeWhiteGoods,
		Status:            models.CommodityStatusInUse,
		Count:             1,
		WarrantyExpiresAt: warranty,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(commodity.GroupID, qt.Equals, group.ID)

	return &sectionReminderEnv{
		fs:        fs,
		ctx:       ctx,
		regSet:    regSet,
		groups:    services.NewGroupService(fs.LocationGroupRegistry, fs.GroupMembershipRegistry, fs.GroupInviteRegistry),
		group:     group,
		commodity: commodity,
	}
}

func (e *sectionReminderEnv) hide(c *qt.C, sections ...models.GroupSection) {
	c.Helper()
	_, err := e.groups.SetHiddenSections(context.Background(), e.group.ID, sections...)
	c.Assert(err, qt.IsNil)
}

func TestWarrantyReminderService_SkipsGroupThatHidesWarranties(t *testing.T) {
	c := qt.New(t)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	expires := models.Date(now.AddDate(0, 0, 30).Format(time.DateOnly))
	env := newSectionReminderEnv(c, &expires)
	email := &recordingEmailService{}
	svc := services.NewWarrantyReminderService(env.fs, email, nil)

	env.hide(c, models.GroupSectionWarranties)
	stats, err := svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 0)
	// No idempotency row either, so showing the section resumes the reminder.
	sent, err := env.fs.WarrantyReminderRegistry.HasSent(context.Background(), env.commodity.ID, 30)
	c.Assert(err, qt.IsNil)
	c.Assert(sent, qt.IsFalse)

	// Hiding other sections does not stop warranty reminders.
	env.hide(c, models.GroupSectionLoans, models.GroupSectionMaintenance, models.GroupSectionUpcoming)
	stats, err = svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.SentByThreshold[models.WarrantyReminder30Days], qt.Equals, 1)
	calls := email.snapshot()
	c.Assert(calls, qt.HasLen, 1)
	c.Assert(calls[0].to, qt.Equals, "admin@example.com")
}

func TestMaintenanceReminderService_SkipsGroupThatHidesMaintenance(t *testing.T) {
	c := qt.New(t)
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	env := newSectionReminderEnv(c, nil)
	schedule, err := env.regSet.MaintenanceScheduleRegistry.Create(env.ctx, models.MaintenanceSchedule{
		CommodityID:  env.commodity.ID,
		Title:        "Clean the filter",
		IntervalDays: 90,
		NextDueAt:    dateOffset(now, 7),
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(schedule.GroupID, qt.Equals, env.group.ID)
	email := &recordingMaintenanceEmailService{}
	svc := services.NewMaintenanceReminderService(env.fs, email, nil)

	env.hide(c, models.GroupSectionMaintenance)
	stats, err := svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 0)
	sent, err := env.fs.MaintenanceReminderRegistry.HasSent(context.Background(), schedule.ID, int(models.MaintenanceReminder7Days))
	c.Assert(err, qt.IsNil)
	c.Assert(sent, qt.IsFalse)

	env.hide(c, models.GroupSectionWarranties)
	stats, err = svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.SentByThreshold[models.MaintenanceReminder7Days], qt.Equals, 1)
	c.Assert(email.snapshot(), qt.HasLen, 1)
}

func TestLoanReminderService_SkipsGroupThatHidesLoans(t *testing.T) {
	c := qt.New(t)
	env := newSectionReminderEnv(c, nil)
	dueDate := models.Date("2026-05-17")
	loan, err := env.regSet.CommodityLoanRegistry.Create(env.ctx, models.CommodityLoan{
		CommodityID:  env.commodity.ID,
		BorrowerName: "Pavel",
		LentAt:       "2026-05-10",
		DueBackAt:    models.PDate(&dueDate),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(loan.GroupID, qt.Equals, env.group.ID)
	email := &recordingLoanEmailService{}
	svc := services.NewLoanReminderService(env.fs, email, nil)
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

	env.hide(c, models.GroupSectionLoans)
	stats, err := svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Total(), qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 0)
	// The sent flag stays unset while the section is hidden.
	stored, err := env.regSet.CommodityLoanRegistry.Get(env.ctx, loan.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.ReminderSentDueSoon, qt.IsFalse)

	env.hide(c)
	stats, err = svc.RemindOnce(context.Background(), now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent[registry.LoanReminderKindDueSoon], qt.Equals, 1)
	c.Assert(email.snapshot(), qt.HasLen, 1)
}

func TestWeeklyDigestService_LeavesOutHiddenSections(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)
	inWindow := f.window.Start.Add(time.Hour)
	soon := models.Date(f.sentAt.AddDate(0, 0, 10).Format(time.DateOnly))

	household := f.addGroup(c, "Household")
	stamps := f.addGroup(c, "Stamps")
	stamps.HiddenSections = models.GroupSections{models.GroupSectionWarranties}
	_, err := f.factorySet.LocationGroupRegistry.Update(context.Background(), *stamps)
	c.Assert(err, qt.IsNil)

	f.addCommodity(c, household, "Fridge", &soon)
	f.addCommodity(c, stamps, "Penny Black", &soon)
	for _, group := range []*models.LocationGroup{household, stamps} {
		item := f.addCommodity(c, group, "Album", nil)
		f.addEvent(c, group, item.ID, models.CommodityEventKindCreated, inWindow)
	}

	email := &recordingDigestEmailService{}
	_, err = f.newService(email).SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)

	sends := email.recorded()
	c.Assert(sends, qt.HasLen, 1)
	// The group that hides warranties still reports its activity, without
	// the warranty line.
	c.Assert(sends[0].digest.Groups, qt.HasLen, 2)
	c.Assert(sends[0].digest.Upcoming, qt.HasLen, 1)
	c.Assert(sends[0].digest.Upcoming[0].Name, qt.Equals, "Fridge")
	c.Assert(sends[0].digest.Upcoming[0].GroupName, qt.Equals, "Household")
}
