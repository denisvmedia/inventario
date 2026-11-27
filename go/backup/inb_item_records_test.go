//go:build !legacy_xml_backup

package backup_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
)

// recordsFixture is an inbFixture whose item has a loan, a service record,
// a maintenance schedule, two supply links and a reminder. The service
// record and the reminder are recorded by a second member of the group.
type recordsFixture struct {
	*inbFixture
	item       *models.Commodity
	member     *models.User
	membership *models.GroupMembership
}

func newRecordsFixture(c *qt.C) *recordsFixture {
	f := newInbFixture(c)
	member := must.Must(f.fs.UserRegistry.Create(f.ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Email:               "b@example.com",
		Name:                "User B",
		IsActive:            true,
	}))
	membership := must.Must(f.fs.GroupMembershipRegistry.Create(f.ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             f.group.ID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
		JoinedAt:            time.Now(),
	}))
	item := must.Must(must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx)).List(f.ctx))[0]
	memberCtx := appctx.WithUser(f.ctx, member)

	must.Must(must.Must(f.fs.CommodityLoanRegistryFactory.CreateUserRegistry(f.ctx)).Create(f.ctx, models.CommodityLoan{
		CommodityID:         item.ID,
		BorrowerName:        "Alice",
		BorrowerContact:     "alice@example.com",
		BorrowerNote:        "for the weekend",
		LentAt:              "2026-03-01",
		DueBackAt:           new(models.Date("2026-03-08")),
		ReturnedAt:          new(models.Date("2026-03-07")),
		ReminderSentDueSoon: true,
	}))
	must.Must(must.Must(f.fs.CommodityServiceRegistryFactory.CreateUserRegistry(memberCtx)).Create(memberCtx, models.CommodityService{
		CommodityID:      item.ID,
		ProviderName:     "Repair Shop",
		Reason:           "screen flickers",
		SentAt:           "2026-04-01",
		ExpectedReturnAt: new(models.Date("2026-04-15")),
		CostAmount:       decimal.RequireFromString("12.50"),
		CostCurrency:     "USD",
	}))
	must.Must(must.Must(f.fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry(f.ctx)).Create(f.ctx, models.MaintenanceSchedule{
		CommodityID:  item.ID,
		Title:        "Dust the vents",
		IntervalDays: 90,
		NextDueAt:    "2026-06-01",
		LastDoneAt:   new(models.Date("2026-03-03")),
		Notes:        "use the soft brush",
		Enabled:      true,
	}))
	links := must.Must(f.fs.SupplyLinkRegistryFactory.CreateUserRegistry(f.ctx))
	must.Must(links.Create(f.ctx, models.SupplyLink{CommodityID: item.ID, Label: "Remote", URL: "https://example.com/remote", SortOrder: 1}))
	must.Must(links.Create(f.ctx, models.SupplyLink{CommodityID: item.ID, Label: "Batteries", URL: "https://example.com/aaa", Notes: "AAA", SortOrder: 0}))
	must.Must(must.Must(f.fs.CommodityReminderRegistryFactory.CreateUserRegistry(memberCtx)).Create(memberCtx, models.CommodityReminder{
		CommodityID:      item.ID,
		Title:            "Renew the subscription",
		DueOn:            "2026-09-01",
		RemindDaysBefore: models.ValuerSlice[int]{7, 1},
		RepeatUnit:       models.ReminderRepeatYear,
		RepeatInterval:   1,
	}))
	return &recordsFixture{inbFixture: f, item: item, member: member, membership: membership}
}

// archivedItem returns the archived item with UUID itemUUID from the
// location documents of archive.
func archivedItem(c *qt.C, archive []byte, itemUUID string) types.INBCommodity {
	_, jsons := innerMembers(c, archive)
	for name, body := range jsons {
		if !strings.HasPrefix(name, "location-") {
			continue
		}
		var doc types.INBLocationDoc
		c.Assert(json.Unmarshal(body, &doc), qt.IsNil)
		for _, com := range doc.Commodities {
			if com.ID == itemUUID {
				return com
			}
		}
	}
	c.Fatalf("item %s is not in the archive", itemUUID)
	return types.INBCommodity{}
}

// restoreStep returns the restore log step called name, or nil.
func restoreStep(f *inbFixture, restoreID, name string) *models.RestoreStep {
	steps := must.Must(f.fs.RestoreStepRegistryFactory.CreateServiceRegistry().ListByRestoreOperation(f.ctx, restoreID))
	for _, s := range steps {
		if s.Name == name {
			return s
		}
	}
	return nil
}

const itemRecordsStep = "Loans, service records, maintenance and reminders"

func TestINBExport_ItemRecords(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newRecordsFixture(c)

	_, archive := f.runExport(c, signer)
	_, jsons := innerMembers(c, archive)
	var manifest types.INBManifest
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	c.Check(manifest.Version, qt.Equals, export.INBFormatVersion)

	com := archivedItem(c, archive, f.item.UUID)
	c.Assert(com.Loans, qt.HasLen, 1)
	c.Check(com.Loans[0].AuthorID, qt.Equals, f.user.UUID)
	c.Check(com.Loans[0].BorrowerName, qt.Equals, "Alice")
	c.Check(com.Loans[0].DueBackAt, qt.Equals, "2026-03-08")
	c.Check(com.Loans[0].ReturnedAt, qt.Equals, "2026-03-07")
	c.Check(com.Loans[0].ReminderSentDueSoon, qt.IsTrue)
	c.Assert(com.ServiceRecords, qt.HasLen, 1)
	c.Check(com.ServiceRecords[0].AuthorID, qt.Equals, f.member.UUID)
	c.Check(com.ServiceRecords[0].CostAmount, qt.Equals, "12.5")
	c.Check(com.ServiceRecords[0].CostCurrency, qt.Equals, "USD")
	c.Assert(com.MaintenanceSchedules, qt.HasLen, 1)
	c.Check(com.MaintenanceSchedules[0].IntervalDays, qt.Equals, 90)
	c.Check(com.MaintenanceSchedules[0].Enabled, qt.IsTrue)
	c.Assert(com.SupplyLinks, qt.HasLen, 2)
	c.Check(com.SupplyLinks[0].Label, qt.Equals, "Batteries")
	c.Check(com.SupplyLinks[1].Label, qt.Equals, "Remote")
	c.Assert(com.Reminders, qt.HasLen, 1)
	c.Check(com.Reminders[0].AuthorID, qt.Equals, f.member.UUID)
	c.Check(com.Reminders[0].RemindDaysBefore, qt.DeepEquals, []int{7, 1})
	c.Check(com.Reminders[0].RepeatUnit, qt.Equals, "year")
}

// TestINBExport_SelectedItemCarriesItsRecords: a selection carries the
// records of the items it names.
func TestINBExport_SelectedItemCarriesItsRecords(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newRecordsFixture(c)

	_, archive := f.runExportRow(c, signer, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Type:                     models.ExportTypeSelectedItems,
		Status:                   models.ExportStatusPending,
		Description:              "selected backup",
		SelectedItems: models.ValuerSlice[models.ExportSelectedItem]{
			{ID: f.item.ID, Type: models.ExportSelectedItemTypeCommodity, Name: f.item.Name},
		},
	})

	com := archivedItem(c, archive, f.item.UUID)
	c.Check(com.RecordCount(), qt.Equals, 6)
}

func TestINBRoundTrip_ItemRecordsFullReplace(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newRecordsFixture(c)
	blobKey, _ := f.runExport(c, signer)

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	restored := f.commodityByUUID(c, f.item.UUID)
	c.Assert(restored.ID, qt.Not(qt.Equals), f.item.ID)

	loans := must.Must(must.Must(f.fs.CommodityLoanRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, restored.ID))
	c.Assert(loans, qt.HasLen, 1)
	c.Check(loans[0].BorrowerName, qt.Equals, "Alice")
	c.Check(loans[0].BorrowerContact, qt.Equals, "alice@example.com")
	c.Check(loans[0].BorrowerNote, qt.Equals, "for the weekend")
	c.Check(loans[0].LentAt, qt.Equals, models.Date("2026-03-01"))
	c.Check(loans[0].DueBackAt, qt.DeepEquals, new(models.Date("2026-03-08")))
	c.Check(loans[0].ReturnedAt, qt.DeepEquals, new(models.Date("2026-03-07")))
	c.Check(loans[0].ReminderSentDueSoon, qt.IsTrue)
	c.Check(loans[0].ReminderSentOverdue, qt.IsFalse)
	c.Check(loans[0].CreatedByUserID, qt.Equals, f.user.ID)

	services := must.Must(must.Must(f.fs.CommodityServiceRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, restored.ID))
	c.Assert(services, qt.HasLen, 1)
	c.Check(services[0].ProviderName, qt.Equals, "Repair Shop")
	c.Check(services[0].CostAmount.Equal(decimal.RequireFromString("12.50")), qt.IsTrue)
	c.Check(services[0].CostCurrency, qt.Equals, "USD")
	c.Check(services[0].ExpectedReturnAt, qt.DeepEquals, new(models.Date("2026-04-15")))
	c.Check(services[0].ReturnedAt, qt.IsNil)
	c.Check(services[0].CreatedByUserID, qt.Equals, f.member.ID)

	schedules := must.Must(must.Must(f.fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, restored.ID))
	c.Assert(schedules, qt.HasLen, 1)
	c.Check(schedules[0].Title, qt.Equals, "Dust the vents")
	c.Check(schedules[0].NextDueAt, qt.Equals, models.Date("2026-06-01"))
	c.Check(schedules[0].LastDoneAt, qt.DeepEquals, new(models.Date("2026-03-03")))
	c.Check(schedules[0].Enabled, qt.IsTrue)

	links := must.Must(must.Must(f.fs.SupplyLinkRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, restored.ID))
	c.Assert(links, qt.HasLen, 2)
	c.Check(links[0].Label, qt.Equals, "Batteries")
	c.Check(links[0].Notes, qt.Equals, "AAA")
	c.Check(links[1].Label, qt.Equals, "Remote")
	c.Check(links[1].SortOrder, qt.Equals, 1)

	reminders := must.Must(must.Must(f.fs.CommodityReminderRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, restored.ID))
	c.Assert(reminders, qt.HasLen, 1)
	c.Check(reminders[0].Title, qt.Equals, "Renew the subscription")
	c.Check([]int(reminders[0].RemindDaysBefore), qt.DeepEquals, []int{7, 1})
	c.Check(reminders[0].RepeatUnit, qt.Equals, models.ReminderRepeatYear)
	c.Check(reminders[0].CreatedByUserID, qt.Equals, f.member.ID)

	step := restoreStep(f.inbFixture, final.ID, itemRecordsStep)
	c.Assert(step, qt.IsNotNil)
	c.Check(step.Result, qt.Equals, models.RestoreStepResultSuccess)
	c.Check(step.Reason, qt.Equals, "6 restored, 0 kept as they were, 0 failed")
}

// TestINBRestore_RecordsOfAFormerMemberGoToTheRestorer: a record whose
// author is not a member of the group is recorded as the member restoring,
// so a reminder never emails someone outside the group.
func TestINBRestore_RecordsOfAFormerMemberGoToTheRestorer(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newRecordsFixture(c)
	blobKey, _ := f.runExport(c, signer)
	c.Assert(f.fs.GroupMembershipRegistry.Delete(f.ctx, f.membership.ID), qt.IsNil)

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	restored := f.commodityByUUID(c, f.item.UUID)
	services := must.Must(must.Must(f.fs.CommodityServiceRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, restored.ID))
	c.Assert(services, qt.HasLen, 1)
	c.Check(services[0].CreatedByUserID, qt.Equals, f.user.ID)
	reminders := must.Must(must.Must(f.fs.CommodityReminderRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, restored.ID))
	c.Assert(reminders, qt.HasLen, 1)
	c.Check(reminders[0].CreatedByUserID, qt.Equals, f.user.ID)
}

// TestINBRestore_MergeKeepsTheRecordsOfAnItemTheGroupHas: the records of an
// item a merge keeps stay as the group has them, and the archived ones are
// not added.
func TestINBRestore_MergeKeepsTheRecordsOfAnItemTheGroupHas(t *testing.T) {
	for _, strategy := range []types.RestoreStrategy{types.RestoreStrategyMergeAdd, types.RestoreStrategyMergeUpdate} {
		t.Run(string(strategy), func(t *testing.T) {
			c := qt.New(t)
			signer := testSigner(c)
			f := newRecordsFixture(c)
			blobKey, _ := f.runExport(c, signer)

			linkReg := must.Must(f.fs.SupplyLinkRegistryFactory.CreateUserRegistry(f.ctx))
			links := must.Must(linkReg.ListByCommodity(f.ctx, f.item.ID))
			c.Assert(linkReg.Delete(f.ctx, links[0].ID), qt.IsNil)

			final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(strategy)})
			c.Assert(err, qt.IsNil)
			c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

			c.Check(must.Must(linkReg.ListByCommodity(f.ctx, f.item.ID)), qt.HasLen, 1)
			loans := must.Must(must.Must(f.fs.CommodityLoanRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, f.item.ID))
			c.Check(loans, qt.HasLen, 1)
			step := restoreStep(f.inbFixture, final.ID, itemRecordsStep)
			c.Assert(step, qt.IsNotNil)
			c.Check(step.Reason, qt.Equals, "0 restored, 6 kept as they were, 0 failed")
		})
	}
}

// TestINBRestore_DryRunCountsItemRecords: a dry run writes no record and
// reports what the restore would do. The item is deleted after the backup,
// so a merge would bring it back with its records.
func TestINBRestore_DryRunCountsItemRecords(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newRecordsFixture(c)
	blobKey, _ := f.runExport(c, signer)
	c.Assert(must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx)).Delete(f.ctx, f.item.ID), qt.IsNil)

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyMergeAdd),
		DryRun:   true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	c.Check(must.Must(must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx)).List(f.ctx)), qt.HasLen, 0)
	step := restoreStep(f.inbFixture, final.ID, itemRecordsStep)
	c.Assert(step, qt.IsNotNil)
	c.Check(step.Reason, qt.Equals, "6 restored, 0 kept as they were, 0 failed")
}

// TestINBRestore_InvalidItemRecordIsCounted: a record that fails validation
// is an error of the restore; its item and the other records come back.
func TestINBRestore_InvalidItemRecordIsCounted(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	doc := inbDocForFixture(c, f, func(com *types.INBCommodity) {
		com.Loans = []types.INBLoan{{ID: "loan-1", BorrowerName: "", LentAt: "2026-03-01"}}
		com.SupplyLinks = []types.INBSupplyLink{{ID: "link-1", Label: "Manual", URL: "https://example.com/manual"}}
	})
	writeArchive(c, f, "t/tenant-a/restores/bad-loan.inb", signedArchiveFromDoc(c, signer, doc))

	final, err := restoreInb(c, f, signer, "t/tenant-a/restores/bad-loan.inb")
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Check(final.ErrorCount, qt.Equals, 1)

	restored := f.commodityByUUID(c, doc.Commodities[0].ID)
	c.Check(must.Must(must.Must(f.fs.CommodityLoanRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, restored.ID)), qt.HasLen, 0)
	c.Check(must.Must(must.Must(f.fs.SupplyLinkRegistryFactory.CreateUserRegistry(f.ctx)).ListByCommodity(f.ctx, restored.ID)), qt.HasLen, 1)
	step := restoreStep(f, final.ID, itemRecordsStep)
	c.Assert(step, qt.IsNotNil)
	c.Check(step.Result, qt.Equals, models.RestoreStepResultError)
	c.Check(step.Reason, qt.Equals, "1 restored, 0 kept as they were, 1 failed")
}

// TestINBRestore_MalformedServiceCostFails: an unparseable cost is a corrupt
// archive, as an unparseable price of the item is.
func TestINBRestore_MalformedServiceCostFails(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	doc := inbDocForFixture(c, f, func(com *types.INBCommodity) {
		com.ServiceRecords = []types.INBServiceRecord{{ID: "svc-1", ProviderName: "Shop", SentAt: "2026-04-01", CostAmount: "twelve", CostCurrency: "USD"}}
	})
	writeArchive(c, f, "t/tenant-a/restores/bad-cost.inb", signedArchiveFromDoc(c, signer, doc))

	final, err := restoreInb(c, f, signer, "t/tenant-a/restores/bad-cost.inb")
	c.Assert(err, qt.IsNotNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)
	c.Check(final.ErrorMessage, qt.Contains, "service record")
}
