package services_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// notActiveTenantStatuses are the tenant statuses whose users cannot sign in.
var notActiveTenantStatuses = []models.TenantStatus{
	models.TenantStatusSuspended,
	models.TenantStatusInactive,
}

func setTenantStatus(c *qt.C, fs *registry.FactorySet, tenantID string, status models.TenantStatus) {
	c.Helper()
	tenant, err := fs.TenantRegistry.Get(context.Background(), tenantID)
	c.Assert(err, qt.IsNil)
	tenant.Status = status
	_, err = fs.TenantRegistry.Update(context.Background(), *tenant)
	c.Assert(err, qt.IsNil)
}

func TestWarrantyReminderService_SkipsTenantThatIsNotActive(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	expires := models.Date(now.AddDate(0, 0, 30).Format(time.DateOnly))
	for _, status := range notActiveTenantStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			env := newSectionReminderEnv(c, &expires)
			setTenantStatus(c, env.fs, env.group.TenantID, status)
			email := &recordingEmailService{}
			svc := services.NewWarrantyReminderService(env.fs, email, nil)

			stats, err := svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(email.snapshot(), qt.HasLen, 0)
			sent, err := env.fs.WarrantyReminderRegistry.HasSent(context.Background(), env.commodity.ID, 30)
			c.Assert(err, qt.IsNil)
			c.Assert(sent, qt.IsFalse)

			// Active again, the tenant's users get the reminder that is due.
			setTenantStatus(c, env.fs, env.group.TenantID, models.TenantStatusActive)
			stats, err = svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.SentByThreshold[models.WarrantyReminder30Days], qt.Equals, 1)
			c.Assert(email.snapshot(), qt.HasLen, 1)
		})
	}
}

func TestMaintenanceReminderService_SkipsTenantThatIsNotActive(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	for _, status := range notActiveTenantStatuses {
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
			setTenantStatus(c, env.fs, env.group.TenantID, status)
			email := &recordingMaintenanceEmailService{}
			svc := services.NewMaintenanceReminderService(env.fs, email, nil)

			stats, err := svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(email.snapshot(), qt.HasLen, 0)
			sent, err := env.fs.MaintenanceReminderRegistry.HasSent(context.Background(), schedule.ID, int(models.MaintenanceReminder7Days))
			c.Assert(err, qt.IsNil)
			c.Assert(sent, qt.IsFalse)

			setTenantStatus(c, env.fs, env.group.TenantID, models.TenantStatusActive)
			stats, err = svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.SentByThreshold[models.MaintenanceReminder7Days], qt.Equals, 1)
			c.Assert(email.snapshot(), qt.HasLen, 1)
		})
	}
}

func TestLoanReminderService_SkipsTenantThatIsNotActive(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	dueDate := models.Date("2026-05-17")
	for _, status := range notActiveTenantStatuses {
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
			setTenantStatus(c, env.fs, env.group.TenantID, status)
			email := &recordingLoanEmailService{}
			svc := services.NewLoanReminderService(env.fs, email, nil)

			stats, err := svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(email.snapshot(), qt.HasLen, 0)
			stored, err := env.regSet.CommodityLoanRegistry.Get(env.ctx, loan.ID)
			c.Assert(err, qt.IsNil)
			c.Assert(stored.ReminderSentDueSoon, qt.IsFalse)

			setTenantStatus(c, env.fs, env.group.TenantID, models.TenantStatusActive)
			stats, err = svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Sent[registry.LoanReminderKindDueSoon], qt.Equals, 1)
			c.Assert(email.snapshot(), qt.HasLen, 1)
		})
	}
}

func TestItemReminderService_SkipsTenantThatIsNotActive(t *testing.T) {
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	for _, status := range notActiveTenantStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			env := newItemReminderEnv(c)
			env.addReminder(c, models.CommodityReminder{
				DueOn:            dateOffset(now, 10),
				RemindDaysBefore: models.ValuerSlice[int]{30},
			})
			setTenantStatus(c, env.fs, env.group.TenantID, status)
			email := newRecordingItemReminderEmail()
			svc := services.NewItemReminderService(env.fs, email, nil)

			stats, err := svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(stats.Sent, qt.Equals, 0)
			c.Assert(email.snapshot(), qt.HasLen, 0)

			setTenantStatus(c, env.fs, env.group.TenantID, models.TenantStatusActive)
			stats, err = svc.RemindOnce(context.Background(), now)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Sent, qt.Equals, 1)
			c.Assert(email.snapshot(), qt.HasLen, 1)
		})
	}
}

func TestStorageQuotaReminderService_SkipsTenantThatIsNotActive(t *testing.T) {
	for _, status := range notActiveTenantStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			f := newStorageQuotaFixture(c)
			f.addFileBytes(c, 95)
			setTenantStatus(c, f.factorySet, f.tenantID, status)
			email := &recordingStorageQuotaEmailService{}
			svc := f.newService(email)

			stats, err := svc.RemindOnce(context.Background(), time.Now().UTC())
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(email.snapshot(), qt.HasLen, 0)
			has, err := f.factorySet.StorageQuotaReminderRegistry.HasSent(context.Background(), f.groupID, int(models.StorageQuota90Percent))
			c.Assert(err, qt.IsNil)
			c.Assert(has, qt.IsFalse)

			setTenantStatus(c, f.factorySet, f.tenantID, models.TenantStatusActive)
			stats, err = svc.RemindOnce(context.Background(), time.Now().UTC())
			c.Assert(err, qt.IsNil)
			c.Assert(stats.SentByThreshold[models.StorageQuota90Percent], qt.Equals, 1)
			c.Assert(email.snapshot(), qt.HasLen, 1)
		})
	}
}

// A suspended tenant's week is not claimed, so once the tenant is active
// again the digest goes out.
func TestWeeklyDigestService_SkipsTenantThatIsNotActive(t *testing.T) {
	for _, status := range notActiveTenantStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			f := newDigestFixture(c)
			household := f.addGroup(c, "Household")
			drill := f.addCommodity(c, household, "Drill", nil)
			f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))
			setTenantStatus(c, f.factorySet, f.tenantID, status)
			email := &recordingDigestEmailService{}
			svc := f.newService(email)

			stats, err := svc.SendOnce(context.Background(), f.sentAt)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Failed, qt.Equals, 0)
			c.Assert(stats.Sent, qt.Equals, 0)
			c.Assert(email.recorded(), qt.HasLen, 0)

			setTenantStatus(c, f.factorySet, f.tenantID, models.TenantStatusActive)
			stats, err = svc.SendOnce(context.Background(), f.sentAt)
			c.Assert(err, qt.IsNil)
			c.Assert(stats.Sent, qt.Equals, 1)
			c.Assert(email.recorded(), qt.HasLen, 1)
		})
	}
}

// recordingExportReadyEmail records the export-ready emails.
type recordingExportReadyEmail struct {
	services.StubEmailService
	mu sync.Mutex
	to []string
}

func (r *recordingExportReadyEmail) SendPersonalDataExportReadyEmail(_ context.Context, to, _, _ string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.to = append(r.to, to)
	return nil
}

func (r *recordingExportReadyEmail) recipients() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.to...)
}

func TestPersonalDataExportNotifier_SkipsTenantThatIsNotActive(t *testing.T) {
	for _, status := range notActiveTenantStatuses {
		t.Run(string(status), func(t *testing.T) {
			c := qt.New(t)
			env := newItemReminderEnv(c)
			setTenantStatus(c, env.fs, env.user.TenantID, status)
			email := &recordingExportReadyEmail{}
			notifier := services.NewPersonalDataExportNotifier(email, env.fs.SettingsRegistryFactory, env.fs.TenantRegistry, "https://inv.example")
			export := &models.PersonalDataExport{ExpiresAt: time.Now().Add(time.Hour)}

			c.Assert(notifier.PersonalDataExportReady(context.Background(), env.user, export), qt.IsNil)
			c.Assert(email.recipients(), qt.HasLen, 0)

			setTenantStatus(c, env.fs, env.user.TenantID, models.TenantStatusActive)
			c.Assert(notifier.PersonalDataExportReady(context.Background(), env.user, export), qt.IsNil)
			c.Assert(email.recipients(), qt.DeepEquals, []string{env.user.Email})
		})
	}
}

// tenantUnreadable fails every tenant lookup.
type tenantUnreadable struct {
	registry.TenantRegistry
}

func (tenantUnreadable) Get(context.Context, string) (*models.Tenant, error) {
	return nil, errRegistryDown
}

// The export-ready email is sent once and nothing retries it, so a tenant
// lookup that fails sends it rather than dropping it.
func TestPersonalDataExportNotifier_SendsWhenTheTenantCannotBeRead(t *testing.T) {
	c := qt.New(t)
	env := newItemReminderEnv(c)
	email := &recordingExportReadyEmail{}
	notifier := services.NewPersonalDataExportNotifier(email, env.fs.SettingsRegistryFactory,
		tenantUnreadable{TenantRegistry: env.fs.TenantRegistry}, "https://inv.example")
	export := &models.PersonalDataExport{ExpiresAt: time.Now().Add(time.Hour)}

	c.Assert(notifier.PersonalDataExportReady(context.Background(), env.user, export), qt.IsNil)
	c.Assert(email.recipients(), qt.DeepEquals, []string{env.user.Email})
}
