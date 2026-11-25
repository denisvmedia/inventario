package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

type trashFixture struct {
	ctx            context.Context
	factorySet     *registry.FactorySet
	regs           *registry.Set
	uploadLocation string
	trash          *services.TrashService
	entities       *services.EntityService
	bucket         *blob.Bucket
}

func newTrashFixture(c *qt.C) *trashFixture {
	c.Helper()
	uploadLocation := uploadLocationForTempDir(c.TempDir())
	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	bucket := must.Must(blob.OpenBucket(ctx, uploadLocation))
	c.Cleanup(func() { _ = bucket.Close() })
	return &trashFixture{
		ctx:            ctx,
		factorySet:     factorySet,
		regs:           must.Must(factorySet.CreateUserRegistrySet(ctx)),
		uploadLocation: uploadLocation,
		trash:          services.NewTrashService(factorySet, uploadLocation),
		entities:       services.NewEntityService(factorySet, uploadLocation),
		bucket:         bucket,
	}
}

func (f *trashFixture) area(c *qt.C) *models.Area {
	c.Helper()
	location := must.Must(f.regs.LocationRegistry.Create(f.ctx, models.Location{Name: "Home"}))
	return must.Must(f.regs.AreaRegistry.Create(f.ctx, models.Area{Name: "Garage", LocationID: location.ID}))
}

func (f *trashFixture) item(c *qt.C, name string, areaID *string) *models.Commodity {
	c.Helper()
	return must.Must(f.regs.CommodityRegistry.Create(f.ctx, models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeEquipment,
		Status:    models.CommodityStatusInUse,
		Count:     1,
		AreaID:    areaID,
	}))
}

// file creates a file row with a blob, linked to the given items.
func (f *trashFixture) file(c *qt.C, key string, itemIDs ...string) *models.FileEntity {
	c.Helper()
	c.Assert(f.bucket.WriteAll(f.ctx, key, []byte("pdf bytes"), nil), qt.IsNil)
	links := make([]models.FileLink, 0, len(itemIDs))
	for _, id := range itemIDs {
		links = append(links, models.FileLink{EntityType: models.FileLinkCommodity, EntityID: id, Meta: models.FileLinkMetaDocuments})
	}
	return must.Must(f.regs.FileRegistry.Create(f.ctx, models.FileEntity{
		Title:    key,
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Links:    links,
		File: &models.File{
			Path:         key,
			OriginalPath: key,
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))
}

func (f *trashFixture) blobExists(c *qt.C, key string) bool {
	c.Helper()
	return must.Must(f.bucket.Exists(f.ctx, key))
}

func (f *trashFixture) eventKinds(c *qt.C, commodityID string) []models.CommodityEventKind {
	c.Helper()
	events, _ := must.Must2(f.regs.CommodityEventRegistry.ListByCommodity(f.ctx, commodityID, 0, 100, registry.CommodityEventListOptions{}))
	kinds := make([]models.CommodityEventKind, 0, len(events))
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	return kinds
}

func TestTrashService_TrashAndRestoreCommodity(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	area := f.area(c)
	drill := f.item(c, "Drill", new(area.ID))
	saw := f.item(c, "Saw", new(area.ID))
	manual := f.file(c, "drill-manual.pdf", drill.ID)
	shared := f.file(c, "shared-receipt.pdf", drill.ID, saw.ID)

	trashed, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(trashed.IsTrashed(), qt.IsTrue)

	// The item and the file only it used are out of every ordinary read.
	_, err = f.regs.CommodityRegistry.Get(f.ctx, drill.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	items := must.Must(f.regs.CommodityRegistry.List(f.ctx))
	c.Assert(items, qt.HasLen, 1)
	c.Assert(items[0].ID, qt.Equals, saw.ID)
	_, err = f.regs.FileRegistry.Get(f.ctx, manual.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The file the saw still uses stays live.
	_ = must.Must(f.regs.FileRegistry.Get(f.ctx, shared.ID))

	// The item is in the Trash list with the file that went with it.
	listed, total, err := f.trash.ListTrashedCommodities(f.ctx, 0, 10, services.DefaultTrashRetention)
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(listed[0].Commodity.ID, qt.Equals, drill.ID)
	c.Assert(listed[0].Files.Count, qt.Equals, 1)
	c.Assert(listed[0].DeletedByName, qt.Equals, "Test User")
	c.Assert(listed[0].ExpiresAt, qt.Equals, trashed.DeletedAt.Add(services.DefaultTrashRetention).UTC())

	// A file that went with an item is not listed on its own.
	_, fileTotal, err := f.trash.ListTrashedFiles(f.ctx, 0, 10, services.DefaultTrashRetention)
	c.Assert(err, qt.IsNil)
	c.Assert(fileTotal, qt.Equals, 0)

	result, err := f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result.AreaRemoved, qt.IsFalse)
	c.Assert(result.FilesRestored, qt.Equals, 1)
	c.Assert(result.Commodity.AreaID, qt.DeepEquals, new(area.ID))

	restored := must.Must(f.regs.CommodityRegistry.Get(f.ctx, drill.ID))
	c.Assert(restored.IsTrashed(), qt.IsFalse)
	got := must.Must(f.regs.FileRegistry.Get(f.ctx, manual.ID))
	c.Assert(got.Links, qt.HasLen, 1)
	c.Assert(got.Links[0].EntityID, qt.Equals, drill.ID)
	c.Assert(f.blobExists(c, "drill-manual.pdf"), qt.IsTrue)

	// The area counts the item again, so the area cannot be deleted while
	// it is there.
	ids := must.Must(f.regs.AreaRegistry.GetCommodities(f.ctx, area.ID))
	c.Assert(ids, qt.Contains, drill.ID)
}

func TestTrashService_RestoreKeepsHistory(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	services.NewCommodityEventService(f.factorySet).EmitCreated(f.ctx, drill)

	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	_, err = f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	kinds := f.eventKinds(c, drill.ID)
	c.Assert(kinds, qt.Contains, models.CommodityEventKindCreated)
	c.Assert(kinds, qt.Contains, models.CommodityEventKindDeleted)
	c.Assert(kinds, qt.Contains, models.CommodityEventKindRestored)
}

func TestTrashService_RestoreAfterAreaDeleted(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	area := f.area(c)
	drill := f.item(c, "Drill", new(area.ID))

	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	// The trashed item does not hold the area: the area is empty now.
	c.Assert(f.entities.DeleteArea(f.ctx, area.ID), qt.IsNil)

	result, err := f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result.AreaRemoved, qt.IsTrue)
	c.Assert(result.Commodity.AreaID, qt.IsNil)

	restored := must.Must(f.regs.CommodityRegistry.Get(f.ctx, drill.ID))
	c.Assert(restored.AreaID, qt.IsNil)

	events, _ := must.Must2(f.regs.CommodityEventRegistry.ListByCommodity(f.ctx, drill.ID, 0, 10,
		registry.CommodityEventListOptions{Kinds: []models.CommodityEventKind{models.CommodityEventKindRestored}}))
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].After["area_removed"], qt.IsTrue)
}

func TestTrashService_DeleteAreaRecursiveTrashesItems(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	area := f.area(c)
	drill := f.item(c, "Drill", new(area.ID))
	manual := f.file(c, "drill-manual.pdf", drill.ID)

	c.Assert(f.entities.DeleteAreaRecursive(f.ctx, area.ID), qt.IsNil)

	_, err := f.regs.AreaRegistry.Get(f.ctx, area.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_ = must.Must(f.regs.CommodityRegistry.GetTrashed(f.ctx, drill.ID))
	_ = must.Must(f.regs.FileRegistry.GetTrashed(f.ctx, manual.ID))
	c.Assert(f.blobExists(c, "drill-manual.pdf"), qt.IsTrue)

	result, err := f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result.AreaRemoved, qt.IsTrue)
	c.Assert(result.FilesRestored, qt.Equals, 1)
}

func TestTrashService_FileDeletedEarlierStaysInTrash(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	manual := f.file(c, "drill-manual.pdf", drill.ID)
	photo := f.file(c, "drill-photo.pdf", drill.ID)

	// The manual is deleted on its own first, then the whole item.
	c.Assert(f.trash.TrashFile(f.ctx, manual.ID), qt.IsNil)
	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	result, err := f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result.FilesRestored, qt.Equals, 1)

	_ = must.Must(f.regs.FileRegistry.Get(f.ctx, photo.ID))
	_, err = f.regs.FileRegistry.Get(f.ctx, manual.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	files, total, err := f.trash.ListTrashedFiles(f.ctx, 0, 10, services.DefaultTrashRetention)
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(files[0].File.ID, qt.Equals, manual.ID)

	// Restoring the file on its own puts it back on the item.
	c.Assert(f.trash.RestoreFile(f.ctx, manual.ID), qt.IsNil)
	got := must.Must(f.regs.FileRegistry.Get(f.ctx, manual.ID))
	c.Assert(got.Links[0].EntityID, qt.Equals, drill.ID)
}

func TestTrashService_FileTrashedWithItemCannotBeRestoredAlone(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	manual := f.file(c, "drill-manual.pdf", drill.ID)
	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	c.Assert(f.trash.RestoreFile(f.ctx, manual.ID), qt.ErrorIs, services.ErrTrashedWithItem)
	c.Assert(f.trash.PurgeFile(f.ctx, manual.ID), qt.ErrorIs, services.ErrTrashedWithItem)
}

func TestTrashService_PurgeKeepsSharedDocument(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	saw := f.item(c, "Saw", nil)
	receipt := f.file(c, "receipt.pdf", drill.ID, saw.ID)
	manual := f.file(c, "drill-manual.pdf", drill.ID)

	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(f.trash.PurgeCommodity(f.ctx, drill.ID), qt.IsNil)

	_, err = f.regs.CommodityRegistry.GetTrashed(f.ctx, drill.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The receipt the saw uses survives, without its link to the purged
	// item.
	got := must.Must(f.regs.FileRegistry.Get(f.ctx, receipt.ID))
	c.Assert(got.Links, qt.HasLen, 1)
	c.Assert(got.Links[0].EntityID, qt.Equals, saw.ID)
	c.Assert(f.blobExists(c, "receipt.pdf"), qt.IsTrue)

	// The manual only the drill used is gone, bytes included.
	_, err = f.regs.FileRegistry.GetTrashed(f.ctx, manual.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(f.blobExists(c, "drill-manual.pdf"), qt.IsFalse)
}

func TestTrashService_SharedDocumentFollowsTheLastItem(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	saw := f.item(c, "Saw", nil)
	receipt := f.file(c, "receipt.pdf", drill.ID, saw.ID)

	// Both items go to Trash; the receipt goes with the second one.
	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	_ = must.Must(f.regs.FileRegistry.Get(f.ctx, receipt.ID))
	_, err = f.trash.TrashCommodity(f.ctx, saw.ID)
	c.Assert(err, qt.IsNil)
	got := must.Must(f.regs.FileRegistry.GetTrashed(f.ctx, receipt.ID))
	c.Assert(*got.DeletedWithCommodityID, qt.Equals, saw.ID)

	// Purging the saw hands the receipt to the drill, still in Trash.
	c.Assert(f.trash.PurgeCommodity(f.ctx, saw.ID), qt.IsNil)
	got = must.Must(f.regs.FileRegistry.GetTrashed(f.ctx, receipt.ID))
	c.Assert(*got.DeletedWithCommodityID, qt.Equals, drill.ID)
	c.Assert(f.blobExists(c, "receipt.pdf"), qt.IsTrue)

	// Restoring the drill brings the receipt back with it.
	result, err := f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result.FilesRestored, qt.Equals, 1)
	got = must.Must(f.regs.FileRegistry.Get(f.ctx, receipt.ID))
	c.Assert(got.Links, qt.HasLen, 1)
	c.Assert(got.Links[0].EntityID, qt.Equals, drill.ID)
}

func TestTrashService_DetachFromLiveItemFollowsTrashedHolder(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	saw := f.item(c, "Saw", nil)
	receipt := f.file(c, "receipt.pdf", drill.ID, saw.ID)

	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	// The saw is deleted for good through the restore path: the receipt is
	// left linked only to the drill in Trash, so it goes to Trash with it.
	c.Assert(f.entities.DeleteCommodityRecursive(f.ctx, saw.ID), qt.IsNil)

	got := must.Must(f.regs.FileRegistry.GetTrashed(f.ctx, receipt.ID))
	c.Assert(*got.DeletedWithCommodityID, qt.Equals, drill.ID)

	_, err = f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	_ = must.Must(f.regs.FileRegistry.Get(f.ctx, receipt.ID))
}

func TestTrashService_PurgeKeepsBlobSharedByAnotherRow(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	manual := f.file(c, "drill-manual.pdf", drill.ID)
	// A second row pointing at the same bytes, such as a copy made by a
	// merge restore.
	copyRow := must.Must(f.regs.FileRegistry.Create(f.ctx, models.FileEntity{
		Title:    "copy",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		File: &models.File{
			Path:         "copy",
			OriginalPath: "drill-manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(f.trash.PurgeCommodity(f.ctx, drill.ID), qt.IsNil)

	_, err = f.regs.FileRegistry.GetTrashed(f.ctx, manual.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_ = must.Must(f.regs.FileRegistry.Get(f.ctx, copyRow.ID))
	c.Assert(f.blobExists(c, "drill-manual.pdf"), qt.IsTrue)
}

func TestTrashService_EmptyTrash(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	f.file(c, "drill-manual.pdf", drill.ID)
	loose := f.file(c, "loose.pdf")
	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(f.trash.TrashFile(f.ctx, loose.ID), qt.IsNil)

	items, files, err := f.trash.EmptyTrash(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(items, qt.Equals, 1)
	c.Assert(files, qt.Equals, 1)

	_, total, err := f.trash.ListTrashedCommodities(f.ctx, 0, 10, services.DefaultTrashRetention)
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 0)
	_, total, err = f.trash.ListTrashedFiles(f.ctx, 0, 10, services.DefaultTrashRetention)
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 0)
	c.Assert(f.blobExists(c, "drill-manual.pdf"), qt.IsFalse)
	c.Assert(f.blobExists(c, "loose.pdf"), qt.IsFalse)
}

func TestTrashService_PurgeExpired(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	old := services.NewTrashService(f.factorySet, f.uploadLocation).WithClock(func() time.Time { return now.Add(-40 * 24 * time.Hour) })
	recent := services.NewTrashService(f.factorySet, f.uploadLocation).WithClock(func() time.Time { return now.Add(-time.Hour) })

	oldItem := f.item(c, "Old", nil)
	f.file(c, "old-manual.pdf", oldItem.ID)
	newItem := f.item(c, "New", nil)
	oldFile := f.file(c, "old-loose.pdf")

	_, err := old.TrashCommodity(f.ctx, oldItem.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(old.TrashFile(f.ctx, oldFile.ID), qt.IsNil)
	_, err = recent.TrashCommodity(f.ctx, newItem.ID)
	c.Assert(err, qt.IsNil)

	worker := services.NewTrashPurgeWorker(f.trash, services.WithTrashPurgeClock(func() time.Time { return now }))
	worker.Sweep(context.Background())

	_, err = f.regs.CommodityRegistry.GetTrashed(f.ctx, oldItem.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = f.regs.FileRegistry.GetTrashed(f.ctx, oldFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(f.blobExists(c, "old-manual.pdf"), qt.IsFalse)
	c.Assert(f.blobExists(c, "old-loose.pdf"), qt.IsFalse)

	// The entry deleted an hour ago waits out its 30 days.
	_ = must.Must(f.regs.CommodityRegistry.GetTrashed(f.ctx, newItem.ID))

	// A second sweep finds nothing left to do.
	worker.Sweep(context.Background())
	_ = must.Must(f.regs.CommodityRegistry.GetTrashed(f.ctx, newItem.ID))
}

func TestTrashService_NoLoanRemindersWhileTrashed(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	_ = must.Must(f.regs.CommodityLoanRegistry.Create(f.ctx, models.CommodityLoan{
		CommodityID:  drill.ID,
		BorrowerName: "Neighbor",
		LentAt:       models.Date("2026-10-01"),
		DueBackAt:    models.ToPDate("2026-10-10"),
	}))
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	loanReg := f.factorySet.CommodityLoanRegistryFactory.CreateServiceRegistry()

	pending := must.Must(loanReg.ListPendingReminders(f.ctx, registry.LoanReminderKindOverdue, now, 7))
	c.Assert(pending, qt.HasLen, 1)

	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	pending = must.Must(loanReg.ListPendingReminders(f.ctx, registry.LoanReminderKindOverdue, now, 7))
	c.Assert(pending, qt.HasLen, 0)
	loans, total := must.Must2(loanReg.ListPaginated(f.ctx, 0, 10, registry.LoanListOptions{}))
	c.Assert(loans, qt.HasLen, 0)
	c.Assert(total, qt.Equals, 0)

	// The loan is still the item's: it comes back with it.
	_, err = f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	pending = must.Must(loanReg.ListPendingReminders(f.ctx, registry.LoanReminderKindOverdue, now, 7))
	c.Assert(pending, qt.HasLen, 1)
}

func TestTrashService_NoWarrantyRemindersWhileTrashed(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	expires := models.Date(now.AddDate(0, 0, 30).Format("2006-01-02"))
	kettle := must.Must(f.regs.CommodityRegistry.Create(f.ctx, models.Commodity{
		Name:              "Kettle",
		ShortName:         "kettle",
		Type:              models.CommodityTypeOther,
		Status:            models.CommodityStatusInUse,
		Count:             1,
		WarrantyExpiresAt: &expires,
	}))
	emails := &recordingEmailService{}
	reminders := services.NewWarrantyReminderService(f.factorySet, emails, nil)

	_, err := f.trash.TrashCommodity(f.ctx, kettle.ID)
	c.Assert(err, qt.IsNil)
	stats, err := reminders.RemindOnce(f.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(emails.snapshot(), qt.HasLen, 0)

	// Restored, the item is due its 60- and 30-day reminders again.
	_, err = f.trash.RestoreCommodity(f.ctx, kettle.ID)
	c.Assert(err, qt.IsNil)
	stats, err = reminders.RemindOnce(f.ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 2)
}

func TestTrashService_NoMaintenanceRemindersWhileTrashed(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	_ = must.Must(f.regs.MaintenanceScheduleRegistry.Create(f.ctx, models.MaintenanceSchedule{
		CommodityID:  drill.ID,
		Title:        "Oil the chuck",
		IntervalDays: 30,
		NextDueAt:    models.Date("2026-11-02"),
	}))
	scheduleReg := f.factorySet.MaintenanceScheduleRegistryFactory.CreateServiceRegistry()
	c.Assert(must.Must(scheduleReg.List(f.ctx)), qt.HasLen, 1)

	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(must.Must(scheduleReg.List(f.ctx)), qt.HasLen, 0)

	// The per-item read still finds it, so the schedule is there after a
	// restore.
	c.Assert(must.Must(scheduleReg.ListByCommodity(f.ctx, drill.ID)), qt.HasLen, 1)
	_, err = f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(must.Must(scheduleReg.List(f.ctx)), qt.HasLen, 1)
}

func TestTrashService_StorageCountsTrashSeparately(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	manual := f.file(c, "drill-manual.pdf", drill.ID)
	manual.SizeBytes = 1000
	_ = must.Must(f.regs.FileRegistry.Update(f.ctx, *manual))

	before := must.Must(f.regs.FileRegistry.SumSizeBreakdown(f.ctx))
	c.Assert(before.Documents, qt.Equals, int64(1000))
	c.Assert(before.Trash, qt.Equals, int64(0))

	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	after := must.Must(f.regs.FileRegistry.SumSizeBreakdown(f.ctx))
	c.Assert(after.Documents, qt.Equals, int64(0))
	c.Assert(after.Trash, qt.Equals, int64(1000))
	c.Assert(after.Total(), qt.Equals, int64(1000))
}

// failingRestoreFiles fails Restore for one file, as a storage outage would.
type failingRestoreFiles struct {
	registry.FileRegistry
	failID string
}

func (r failingRestoreFiles) Restore(ctx context.Context, id string) error {
	if id == r.failID {
		return errors.New("storage unavailable")
	}
	return r.FileRegistry.Restore(ctx, id)
}

type failingRestoreFileFactory struct {
	registry.FileRegistryFactory
	failID string
}

func (f failingRestoreFileFactory) CreateUserRegistry(ctx context.Context) (registry.FileRegistry, error) {
	reg, err := f.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return failingRestoreFiles{FileRegistry: reg, failID: f.failID}, nil
}

// failingPurgeCommodities fails PurgeTrashed for one item.
type failingPurgeCommodities struct {
	registry.CommodityRegistry
	failID string
}

func (r failingPurgeCommodities) PurgeTrashed(ctx context.Context, id string) (bool, error) {
	if id == r.failID {
		return false, errors.New("row is locked")
	}
	return r.CommodityRegistry.PurgeTrashed(ctx, id)
}

type failingPurgeCommodityFactory struct {
	registry.CommodityRegistryFactory
	failID string
}

func (f failingPurgeCommodityFactory) CreateServiceRegistry() registry.CommodityRegistry {
	return failingPurgeCommodities{CommodityRegistry: f.CommodityRegistryFactory.CreateServiceRegistry(), failID: f.failID}
}

func TestTrashService_FailedFileRestoreLeavesItemInTrash(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	manual := f.file(c, "drill-manual.pdf", drill.ID)
	receipt := f.file(c, "drill-receipt.pdf", drill.ID)
	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	broken := *f.factorySet
	broken.FileRegistryFactory = failingRestoreFileFactory{FileRegistryFactory: f.factorySet.FileRegistryFactory, failID: receipt.ID}
	_, err = services.NewTrashService(&broken, f.uploadLocation).RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.ErrorMatches, `.*storage unavailable.*`)

	// The item stays in Trash, so the restore can be repeated.
	_ = must.Must(f.regs.CommodityRegistry.GetTrashed(f.ctx, drill.ID))
	_ = must.Must(f.regs.FileRegistry.GetTrashed(f.ctx, receipt.ID))

	result, err := f.trash.RestoreCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result.Commodity.IsTrashed(), qt.IsFalse)
	_ = must.Must(f.regs.FileRegistry.Get(f.ctx, manual.ID))
	_ = must.Must(f.regs.FileRegistry.Get(f.ctx, receipt.ID))
}

func TestTrashService_SweepRestoresFilesOfALiveItem(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	manual := f.file(c, "drill-manual.pdf", drill.ID)
	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	// The item is back while its file is still in Trash: a delete that
	// raced the restore leaves exactly this.
	_ = must.Must(f.regs.CommodityRegistry.Restore(f.ctx, drill.ID))
	_, err = f.regs.FileRegistry.Get(f.ctx, manual.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The file is not old enough to expire; the sweep restores it anyway.
	services.NewTrashPurgeWorker(f.trash).Sweep(context.Background())

	got := must.Must(f.regs.FileRegistry.Get(f.ctx, manual.ID))
	c.Assert(got.IsTrashed(), qt.IsFalse)
	c.Assert(got.LinkTo(models.FileLinkCommodity, drill.ID), qt.IsNotNil)
	c.Assert(f.blobExists(c, "drill-manual.pdf"), qt.IsTrue)
}

func TestTrashService_EmptyTrashSettlesFilesOfAPurgedItem(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	manual := f.file(c, "drill-manual.pdf", drill.ID)
	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	// A purge that stopped right after deleting the item row: the file
	// still links the item and is in neither Trash list.
	c.Assert(must.Must(f.regs.CommodityRegistry.PurgeTrashed(f.ctx, drill.ID)), qt.IsTrue)
	_, total, err := f.trash.ListTrashedFiles(f.ctx, 0, 10, services.DefaultTrashRetention)
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 0)

	items, files, err := f.trash.EmptyTrash(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(items, qt.Equals, 0)
	c.Assert(files, qt.Equals, 1)
	_, err = f.regs.FileRegistry.GetTrashed(f.ctx, manual.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(f.blobExists(c, "drill-manual.pdf"), qt.IsFalse)
}

func TestTrashService_PurgeLeavesLiveFilesLive(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	saw := f.item(c, "Saw", nil)
	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	// Files linked to the item after its delete listed its files, so they
	// stayed live.
	late := f.file(c, "late.pdf", drill.ID)
	shared := f.file(c, "shared.pdf", drill.ID, saw.ID)

	c.Assert(f.trash.PurgeCommodity(f.ctx, drill.ID), qt.IsNil)

	// Both stay live and lose only the link to the purged item.
	got, err := f.regs.FileRegistry.Get(f.ctx, late.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("a live file must survive the purge"))
	c.Assert(got.Links, qt.HasLen, 0)
	c.Assert(f.blobExists(c, "late.pdf"), qt.IsTrue)
	got = must.Must(f.regs.FileRegistry.Get(f.ctx, shared.ID))
	c.Assert(got.Links, qt.HasLen, 1)
	c.Assert(got.Links[0].EntityID, qt.Equals, saw.ID)
}

func TestTrashService_FileRestoredOnItsOwnSurvivesTheItemPurge(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	manual := f.file(c, "manual.pdf", drill.ID)

	// The manual is deleted on its own, then the drill; the manual comes
	// back by itself while the drill stays in Trash.
	c.Assert(f.trash.TrashFile(f.ctx, manual.ID), qt.IsNil)
	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(f.trash.RestoreFile(f.ctx, manual.ID), qt.IsNil)

	// The drill expires.
	cutoff := time.Now().Add(31*24*time.Hour - services.DefaultTrashRetention)
	items, files, err := f.trash.PurgeExpired(f.ctx, cutoff, 100)
	c.Assert(err, qt.IsNil)
	c.Assert(items, qt.Equals, 1)
	c.Assert(files, qt.Equals, 0)

	_, err = f.regs.CommodityRegistry.GetTrashed(f.ctx, drill.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	got, err := f.regs.FileRegistry.Get(f.ctx, manual.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("the manual restored on its own must stay live"))
	c.Assert(got.IsTrashed(), qt.IsFalse)
	c.Assert(got.Links, qt.HasLen, 0)
	c.Assert(f.blobExists(c, "manual.pdf"), qt.IsTrue)
}

func TestTrashService_UnknownLinkedItemKeepsTheFile(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	drill := f.item(c, "Drill", nil)
	// A link to an item this group cannot see. The link paths never create
	// one; the trash code must still not read it as "no live link".
	file := f.file(c, "manual.pdf", drill.ID, "item-the-group-cannot-see")

	_, err := f.trash.TrashCommodity(f.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	_ = must.Must(f.regs.FileRegistry.Get(f.ctx, file.ID))

	c.Assert(f.trash.PurgeCommodity(f.ctx, drill.ID), qt.IsNil)
	got := must.Must(f.regs.FileRegistry.Get(f.ctx, file.ID))
	c.Assert(got.Links, qt.HasLen, 1)
	c.Assert(got.Links[0].EntityID, qt.Equals, "item-the-group-cannot-see")
	c.Assert(f.blobExists(c, "manual.pdf"), qt.IsTrue)
}

func TestTrashService_PurgeExpiredGoesPastAFailingItem(t *testing.T) {
	c := qt.New(t)
	f := newTrashFixture(c)

	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	old := services.NewTrashService(f.factorySet, f.uploadLocation).WithClock(func() time.Time { return now.Add(-40 * 24 * time.Hour) })
	first := f.item(c, "First", nil)
	second := f.item(c, "Second", nil)
	_, err := old.TrashCommodity(f.ctx, first.ID)
	c.Assert(err, qt.IsNil)
	_, err = old.TrashCommodity(f.ctx, second.ID)
	c.Assert(err, qt.IsNil)

	broken := *f.factorySet
	broken.CommodityRegistryFactory = failingPurgeCommodityFactory{CommodityRegistryFactory: f.factorySet.CommodityRegistryFactory, failID: first.ID}
	items, _, err := services.NewTrashService(&broken, f.uploadLocation).PurgeExpired(context.Background(), now.Add(-30*24*time.Hour), 10)
	c.Assert(err, qt.ErrorMatches, `.*row is locked.*`)
	c.Assert(items, qt.Equals, 1)

	_ = must.Must(f.regs.CommodityRegistry.GetTrashed(f.ctx, first.ID))
	_, err = f.regs.CommodityRegistry.GetTrashed(f.ctx, second.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
