//go:build !legacy_xml_backup

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	exportpkg "go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/internal/backupsign"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// TestINBItemRecordsRoundTripPostgres is the postgres half of the item
// records round trip (#2884, format 3.9): a full replace deletes the item
// with its loans, service records, maintenance schedules, supply links and
// reminders, and brings them all back on the restored item, each recorded
// by the member who recorded it. Skips when POSTGRES_TEST_DSN is unset.
func TestINBItemRecordsRoundTripPostgres(t *testing.T) {
	c := qt.New(t)
	dsn := testDSN(t)
	c.Assert(setupFreshDatabase(dsn), qt.IsNil)

	registrySetFunc, cleanup := postgres.NewPostgresRegistrySet()
	defer cleanup()
	fs, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil)

	ctx := context.Background()
	uniq := fmt.Sprintf("%d", time.Now().UnixNano())
	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	tenant, user, group := seedFidelityTenant(c, fs, ctx, uniq)
	uctx := appctx.WithGroup(appctx.WithUser(ctx, user), group)
	member := must.Must(fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "records-member-" + uniq + "@test.com",
		Name:                "Records Member",
		IsActive:            true,
	}))
	for _, id := range []string{user.ID, member.ID} {
		must.Must(fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
			GroupID:             group.ID,
			MemberUserID:        id,
			Role:                models.GroupRoleAdmin,
			JoinedAt:            time.Now(),
		}))
	}
	memberCtx := appctx.WithUser(uctx, member)

	src := seedFullCommodity(c, fs, uctx, tenant.ID, group.ID, user.ID)
	must.Must(must.Must(fs.CommodityLoanRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.CommodityLoan{
		CommodityID:         src.ID,
		BorrowerName:        "Alice",
		LentAt:              "2026-03-01",
		DueBackAt:           new(models.Date("2026-03-08")),
		ReminderSentDueSoon: true,
	}))
	must.Must(must.Must(fs.CommodityServiceRegistryFactory.CreateUserRegistry(memberCtx)).Create(memberCtx, models.CommodityService{
		CommodityID:  src.ID,
		ProviderName: "Repair Shop",
		SentAt:       "2026-04-01",
		ReturnedAt:   new(models.Date("2026-04-10")),
		CostAmount:   decimal.RequireFromString("12.50"),
		CostCurrency: "USD",
	}))
	must.Must(must.Must(fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.MaintenanceSchedule{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", tenant.ID, group.ID, user.ID),
		CommodityID:              src.ID,
		Title:                    "Clean the lens",
		IntervalDays:             30,
		NextDueAt:                "2026-06-01",
		Enabled:                  true,
	}))
	must.Must(must.Must(fs.SupplyLinkRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.SupplyLink{
		CommodityID: src.ID,
		Label:       "Film",
		URL:         "https://example.com/film",
	}))
	must.Must(must.Must(fs.CommodityReminderRegistryFactory.CreateUserRegistry(memberCtx)).Create(memberCtx, models.CommodityReminder{
		CommodityID:      src.ID,
		Title:            "Service the shutter",
		DueOn:            "2026-09-01",
		RemindDaysBefore: models.ValuerSlice[int]{7},
		RepeatUnit:       models.ReminderRepeatYear,
		RepeatInterval:   1,
	}))

	signer := must.Must(backupsign.NewSigner(make([]byte, backupsign.SeedSize)))
	exportSvc := exportpkg.NewExportService(fs, uploadLocation, signer)
	srv := fs.CreateServiceRegistrySet()
	exportRec := must.Must(srv.ExportRegistry.Create(ctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", tenant.ID, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusPending,
		Description:              "item records export",
	}))
	c.Assert(exportSvc.ProcessExport(ctx, exportRec.ID), qt.IsNil)
	exportRec = must.Must(srv.ExportRegistry.Get(ctx, exportRec.ID))
	c.Assert(exportRec.Status, qt.Equals, models.ExportStatusCompleted)

	final := runFidelityRestore(c, fs, signer, uploadLocation, uctx, tenant, group, user, exportRec.FilePath)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("restore error: %s", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	comReg := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(uctx))
	restored := commodityByName(c, comReg, uctx, src.Name)
	c.Assert(restored.ID, qt.Not(qt.Equals), src.ID)

	loans := must.Must(must.Must(fs.CommodityLoanRegistryFactory.CreateUserRegistry(uctx)).List(uctx))
	c.Assert(loans, qt.HasLen, 1)
	c.Check(loans[0].CommodityID, qt.Equals, restored.ID)
	c.Check(loans[0].BorrowerName, qt.Equals, "Alice")
	c.Check(loans[0].ReminderSentDueSoon, qt.IsTrue)
	c.Check(loans[0].CreatedByUserID, qt.Equals, user.ID)

	services := must.Must(must.Must(fs.CommodityServiceRegistryFactory.CreateUserRegistry(uctx)).List(uctx))
	c.Assert(services, qt.HasLen, 1)
	c.Check(services[0].CommodityID, qt.Equals, restored.ID)
	c.Check(services[0].CostAmount.Equal(decimal.RequireFromString("12.50")), qt.IsTrue)
	c.Check(services[0].CreatedByUserID, qt.Equals, member.ID)

	schedules := must.Must(must.Must(fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry(uctx)).List(uctx))
	c.Assert(schedules, qt.HasLen, 1)
	c.Check(schedules[0].CommodityID, qt.Equals, restored.ID)
	c.Check(schedules[0].IntervalDays, qt.Equals, 30)

	links := must.Must(must.Must(fs.SupplyLinkRegistryFactory.CreateUserRegistry(uctx)).List(uctx))
	c.Assert(links, qt.HasLen, 1)
	c.Check(links[0].CommodityID, qt.Equals, restored.ID)

	reminders := must.Must(must.Must(fs.CommodityReminderRegistryFactory.CreateUserRegistry(uctx)).List(uctx))
	c.Assert(reminders, qt.HasLen, 1)
	c.Check(reminders[0].CommodityID, qt.Equals, restored.ID)
	c.Check([]int(reminders[0].RemindDaysBefore), qt.DeepEquals, []int{7})
	c.Check(reminders[0].CreatedByUserID, qt.Equals, member.ID)
}
