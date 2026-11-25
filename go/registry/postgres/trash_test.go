package postgres_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	_ "go.5x5.cz/inventario/internal/fileblob" // register file:// driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

type trashPGFixture struct {
	fileLinkPGFixture
	upload string
	bucket *blob.Bucket
	trash  *services.TrashService
	area   *models.Area
}

func newTrashPGFixture(t *testing.T) trashPGFixture {
	t.Helper()
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	upload := newPostgresFileUploadLocation(c)
	bucket, err := blob.OpenBucket(fx.ctx, upload)
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { _ = bucket.Close() })
	location := createTestLocation(c, fx.set)
	return trashPGFixture{
		fileLinkPGFixture: fx,
		upload:            upload,
		bucket:            bucket,
		trash:             services.NewTrashService(fx.factorySet, upload),
		area:              createTestArea(c, fx.set, location.ID),
	}
}

func (fx trashPGFixture) item(c *qt.C, name string) *models.Commodity {
	c.Helper()
	item := createTestCommodity(c, fx.set, fx.area.ID)
	item.Name = name
	updated, err := fx.set.CommodityRegistry.Update(fx.ctx, *item)
	c.Assert(err, qt.IsNil)
	return updated
}

// blobFile creates a file whose blob exists, linked to the given items.
func (fx trashPGFixture) blobFile(c *qt.C, name string, size int64, itemIDs ...string) *models.FileEntity {
	c.Helper()
	links := make([]models.FileLink, 0, len(itemIDs))
	for _, id := range itemIDs {
		links = append(links, pgCommodityLink(id, models.FileLinkMetaDocuments))
	}
	file := fx.file(c, name, size, links...)
	c.Assert(fx.bucket.WriteAll(fx.ctx, file.OriginalPath, []byte("bytes"), nil), qt.IsNil)
	return file
}

func (fx trashPGFixture) blobExists(c *qt.C, key string) bool {
	c.Helper()
	ok, err := fx.bucket.Exists(fx.ctx, key)
	c.Assert(err, qt.IsNil)
	return ok
}

func TestTrash_Postgres_LiveReadsLeaveTrashOut(t *testing.T) {
	c := qt.New(t)
	fx := newTrashPGFixture(t)
	reg := fx.set.CommodityRegistry

	drill := fx.item(c, "Drill")
	saw := fx.item(c, "Saw")
	manual := fx.blobFile(c, "manual", 300, drill.ID)
	receipt := fx.blobFile(c, "receipt", 200, drill.ID, saw.ID)

	_, err := fx.trash.TrashCommodity(fx.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	_, err = reg.Get(fx.ctx, drill.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	count, err := reg.Count(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
	page, total, err := reg.ListPaginated(fx.ctx, 0, 10, registry.CommodityListOptions{IncludeInactive: true})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(page[0].ID, qt.Equals, saw.ID)
	many, err := reg.GetMany(fx.ctx, []string{drill.ID, saw.ID})
	c.Assert(err, qt.IsNil)
	c.Assert(many, qt.HasLen, 1)
	_, err = reg.Update(fx.ctx, *drill)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	ids, err := fx.set.AreaRegistry.GetCommodities(fx.ctx, fx.area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(ids, qt.DeepEquals, []string{saw.ID})

	_, err = fx.set.FileRegistry.Get(fx.ctx, manual.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	files, err := fx.set.FileRegistry.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 1)
	c.Assert(files[0].ID, qt.Equals, receipt.ID)

	breakdown, err := fx.set.FileRegistry.SumSizeBreakdown(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(breakdown.Documents, qt.Equals, int64(200))
	c.Assert(breakdown.Trash, qt.Equals, int64(300))

	trashed, total, err := reg.ListTrashed(fx.ctx, 0, 10)
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 1)
	c.Assert(trashed[0].ID, qt.Equals, drill.ID)
	c.Assert(*trashed[0].DeletedBy, qt.Equals, fx.user.ID)
	c.Assert(*trashed[0].DeletedFromAreaID, qt.Equals, fx.area.ID)

	stats, err := fx.set.FileRegistry.CountTrashedWith(fx.ctx, []string{drill.ID})
	c.Assert(err, qt.IsNil)
	c.Assert(stats[drill.ID], qt.Equals, registry.TrashedFilesStats{Count: 1, Bytes: 300})
}

func TestTrash_Postgres_RestoreAfterAreaDeleted(t *testing.T) {
	c := qt.New(t)
	fx := newTrashPGFixture(t)

	drill := fx.item(c, "Drill")
	manual := fx.blobFile(c, "manual", 300, drill.ID)

	entities := services.NewEntityService(fx.factorySet, fx.upload)
	c.Assert(entities.DeleteAreaRecursive(fx.ctx, fx.area.ID), qt.IsNil)

	_, err := fx.set.AreaRegistry.Get(fx.ctx, fx.area.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	result, err := fx.trash.RestoreCommodity(fx.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result.AreaRemoved, qt.IsTrue)
	c.Assert(result.FilesRestored, qt.Equals, 1)
	c.Assert(result.Commodity.AreaID, qt.IsNil)

	got, err := fx.set.FileRegistry.Get(fx.ctx, manual.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.IsTrashed(), qt.IsFalse)
	c.Assert(fx.blobExists(c, manual.OriginalPath), qt.IsTrue)
}

func TestTrash_Postgres_PurgeKeepsSharedDocument(t *testing.T) {
	c := qt.New(t)
	fx := newTrashPGFixture(t)

	drill := fx.item(c, "Drill")
	saw := fx.item(c, "Saw")
	manual := fx.blobFile(c, "manual", 300, drill.ID)
	receipt := fx.blobFile(c, "receipt", 200, drill.ID, saw.ID)

	_, err := fx.trash.TrashCommodity(fx.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fx.trash.PurgeCommodity(fx.ctx, drill.ID), qt.IsNil)

	_, err = fx.set.CommodityRegistry.GetTrashed(fx.ctx, drill.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	got, err := fx.set.FileRegistry.Get(fx.ctx, receipt.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(pgLinkKeys(got), qt.DeepEquals, []string{"commodity/" + saw.ID + "/documents"})
	c.Assert(fx.blobExists(c, receipt.OriginalPath), qt.IsTrue)

	_, err = fx.set.FileRegistry.GetTrashed(fx.ctx, manual.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(fx.blobExists(c, manual.OriginalPath), qt.IsFalse)
}

func TestTrash_Postgres_PurgeExpiredAcrossGroups(t *testing.T) {
	c := qt.New(t)
	fx := newTrashPGFixture(t)

	now := time.Now().UTC()
	old := services.NewTrashService(fx.factorySet, fx.upload).WithClock(func() time.Time { return now.Add(-31 * 24 * time.Hour) })

	expired := fx.item(c, "Expired")
	manual := fx.blobFile(c, "manual", 300, expired.ID)
	fresh := fx.item(c, "Fresh")
	loose := fx.blobFile(c, "loose", 100)

	_, err := old.TrashCommodity(fx.ctx, expired.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(old.TrashFile(fx.ctx, loose.ID), qt.IsNil)
	_, err = fx.trash.TrashCommodity(fx.ctx, fresh.ID)
	c.Assert(err, qt.IsNil)

	items, files, err := fx.trash.PurgeExpired(fx.ctx, now.Add(-services.DefaultTrashRetention), 100)
	c.Assert(err, qt.IsNil)
	c.Assert(items, qt.Equals, 1)
	c.Assert(files, qt.Equals, 1)

	_, err = fx.set.CommodityRegistry.GetTrashed(fx.ctx, expired.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fx.set.FileRegistry.GetTrashed(fx.ctx, manual.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fx.set.FileRegistry.GetTrashed(fx.ctx, loose.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(fx.blobExists(c, manual.OriginalPath), qt.IsFalse)
	c.Assert(fx.blobExists(c, loose.OriginalPath), qt.IsFalse)

	_, err = fx.set.CommodityRegistry.GetTrashed(fx.ctx, fresh.ID)
	c.Assert(err, qt.IsNil)

	// Nothing left that expired: a repeat is a no-op.
	items, files, err = fx.trash.PurgeExpired(fx.ctx, now.Add(-services.DefaultTrashRetention), 100)
	c.Assert(err, qt.IsNil)
	c.Assert(items, qt.Equals, 0)
	c.Assert(files, qt.Equals, 0)
}

func TestTrash_Postgres_LoanRemindersSkipTrashedItems(t *testing.T) {
	c := qt.New(t)
	fx := newTrashPGFixture(t)

	drill := fx.item(c, "Drill")
	_, err := fx.set.CommodityLoanRegistry.Create(fx.ctx, models.CommodityLoan{
		CommodityID:  drill.ID,
		BorrowerName: "Neighbor",
		LentAt:       models.Date("2026-01-01"),
		DueBackAt:    models.ToPDate("2026-01-10"),
	})
	c.Assert(err, qt.IsNil)

	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	loans := fx.factorySet.CommodityLoanRegistryFactory.CreateServiceRegistry()
	pending, err := loans.ListPendingReminders(fx.ctx, registry.LoanReminderKindOverdue, now, 7)
	c.Assert(err, qt.IsNil)
	c.Assert(pending, qt.HasLen, 1)

	_, err = fx.trash.TrashCommodity(fx.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	pending, err = loans.ListPendingReminders(fx.ctx, registry.LoanReminderKindOverdue, now, 7)
	c.Assert(err, qt.IsNil)
	c.Assert(pending, qt.HasLen, 0)
	all, err := fx.set.CommodityLoanRegistry.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 0)
	own, err := fx.set.CommodityLoanRegistry.ListByCommodity(fx.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(own, qt.HasLen, 1)

	_, err = fx.trash.RestoreCommodity(fx.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	pending, err = loans.ListPendingReminders(fx.ctx, registry.LoanReminderKindOverdue, now, 7)
	c.Assert(err, qt.IsNil)
	c.Assert(pending, qt.HasLen, 1)
}

func TestTrash_Postgres_TagDeleteReachesTrashedItems(t *testing.T) {
	c := qt.New(t)
	fx := newTrashPGFixture(t)

	tag, err := fx.set.TagRegistry.Create(fx.ctx, models.Tag{Kind: models.TagKindCommodity, Slug: "tools", Label: "Tools"})
	c.Assert(err, qt.IsNil)
	drill := fx.item(c, "Drill")
	drill.Tags = models.ValuerSlice[string]{"tools"}
	_, err = fx.set.CommodityRegistry.Update(fx.ctx, *drill)
	c.Assert(err, qt.IsNil)

	_, err = fx.trash.TrashCommodity(fx.ctx, drill.ID)
	c.Assert(err, qt.IsNil)

	// No live item uses the tag, so it deletes without force; the trashed
	// item loses the slug too.
	usage, err := fx.set.TagRegistry.DeleteAtomic(fx.ctx, tag.ID, false)
	c.Assert(err, qt.IsNil)
	c.Assert(usage.Commodities, qt.Equals, 0)

	result, err := fx.trash.RestoreCommodity(fx.ctx, drill.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result.Commodity.Tags, qt.HasLen, 0)
}
