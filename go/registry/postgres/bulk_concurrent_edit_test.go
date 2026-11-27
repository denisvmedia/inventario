package postgres_test

import (
	"fmt"
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// bulkResult turns the outcome of a bulk action on the one item id into an
// error, so the tests can wait for it like for any other call.
func bulkResult(out services.BulkOutcome, id string) error {
	if !slices.Equal(out.Succeeded, []string{id}) {
		return fmt.Errorf("bulk action did not change %s: %+v", id, out)
	}
	return nil
}

func newBulkServiceForTagFixture(c *qt.C, fx tagPGFixture) *services.CommodityBulkService {
	c.Helper()
	return services.NewCommodityBulkService(
		services.NewTagService(fx.factorySet),
		services.NewCommodityEventService(fx.factorySet),
		services.NewTrashService(fx.factorySet, "file://"+c.TempDir()),
	)
}

// A member saves an item while a bulk action tags it. A third transaction
// holds the item's row, so the save queues on it first and the bulk action
// second. When the row is released, the bulk action must start from the
// saved item: the item keeps the new name and the tag the save added, and
// gains the bulk tag (#2891).
func TestCommodityBulkUpdate_Postgres_KeepsAnEditSavedMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := validationctx.WithGroupCurrency(fx.ctxA, "USD")
	items := fx.groupASet.CommodityRegistry
	bulk := newBulkServiceForTagFixture(c, fx)

	itemID := seedTagCommodity(c, fx.groupASet, ctx, fx.areaAID, "Drill", "garage")
	item, err := items.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	patch, err := services.ParseCommodityPatch(ctx, fx.groupASet.ItemTypeRegistry, services.CommodityPatchInput{AddTags: []string{"insured"}})
	c.Assert(err, qt.IsNil)
	c.Assert(bulk.EnsureTags(ctx, &patch), qt.IsNil)

	release := holdRowForUpdate(c, fx.dbx, "commodities", itemID)

	saveDone := make(chan error, 1)
	go func() {
		edit := *item
		edit.Name = "Cordless drill"
		edit.Tags = append(edit.Tags, "power-tools")
		_, err := items.Update(ctx, edit)
		saveDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, saveDone)

	bulkDone := make(chan error, 1)
	go func() {
		out := bulk.Update(ctx, items, []services.BulkTarget{{ID: itemID}}, patch)
		bulkDone <- bulkResult(out, itemID)
	}()
	waitForLockWaiters(c, fx.dbx, 2, saveDone, bulkDone)

	release()
	c.Assert(<-saveDone, qt.IsNil)
	c.Assert(<-bulkDone, qt.IsNil)

	got, err := items.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Name, qt.Equals, "Cordless drill")
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"garage", "power-tools", "insured"})
}

// A rename of one of the item's tags reaches the item's row before a bulk
// action that tags it. The bulk action must start from the renamed tag, not
// write the old one back (#2891).
func TestCommodityBulkUpdate_Postgres_KeepsATagRenamedMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := validationctx.WithGroupCurrency(fx.ctxA, "USD")
	items := fx.groupASet.CommodityRegistry
	bulk := newBulkServiceForTagFixture(c, fx)

	itemID := seedTagCommodity(c, fx.groupASet, ctx, fx.areaAID, "Drill", "garage")
	tag, err := fx.groupASet.TagRegistry.GetBySlug(ctx, models.TagKindCommodity, "garage")
	c.Assert(err, qt.IsNil)
	patch, err := services.ParseCommodityPatch(ctx, fx.groupASet.ItemTypeRegistry, services.CommodityPatchInput{AddTags: []string{"insured"}})
	c.Assert(err, qt.IsNil)
	c.Assert(bulk.EnsureTags(ctx, &patch), qt.IsNil)

	release := holdRowForUpdate(c, fx.dbx, "commodities", itemID)

	renameDone := make(chan error, 1)
	go func() {
		_, _, err := fx.groupASet.TagRegistry.RenameAtomic(ctx, tag.ID, "Workshop", "workshop", "", nil)
		renameDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, renameDone)

	bulkDone := make(chan error, 1)
	go func() {
		out := bulk.Update(ctx, items, []services.BulkTarget{{ID: itemID}}, patch)
		bulkDone <- bulkResult(out, itemID)
	}()
	waitForLockWaiters(c, fx.dbx, 2, renameDone, bulkDone)

	release()
	c.Assert(<-renameDone, qt.IsNil)
	c.Assert(<-bulkDone, qt.IsNil)

	got, err := items.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"workshop", "insured"})
	_, err = fx.groupASet.TagRegistry.GetBySlug(ctx, models.TagKindCommodity, "garage")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// The same race with a bulk move in place of the tag patch.
func TestCommodityBulkMove_Postgres_KeepsAnEditSavedMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := validationctx.WithGroupCurrency(fx.ctxA, "USD")
	items := fx.groupASet.CommodityRegistry
	bulk := newBulkServiceForTagFixture(c, fx)

	itemID := seedTagCommodity(c, fx.groupASet, ctx, fx.areaAID, "Drill", "garage")
	item, err := items.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	shedID := seedTagArea(c, fx.groupASet, ctx)

	release := holdRowForUpdate(c, fx.dbx, "commodities", itemID)

	saveDone := make(chan error, 1)
	go func() {
		edit := *item
		edit.Comments = "Battery replaced"
		_, err := items.Update(ctx, edit)
		saveDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, saveDone)

	bulkDone := make(chan error, 1)
	go func() {
		out := bulk.Move(ctx, items, []services.BulkTarget{{ID: itemID}}, shedID)
		bulkDone <- bulkResult(out, itemID)
	}()
	waitForLockWaiters(c, fx.dbx, 2, saveDone, bulkDone)

	release()
	c.Assert(<-saveDone, qt.IsNil)
	c.Assert(<-bulkDone, qt.IsNil)

	got, err := items.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Comments, qt.Equals, "Battery replaced")
	c.Assert(*got.AreaID, qt.Equals, shedID)
}

// A member saves a file while a bulk action tags it: the file keeps the new
// title and the tag the save added, and gains the bulk tag.
func TestFileBulkUpdateTags_Postgres_KeepsAnEditSavedMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := fx.ctxA
	files := fx.groupASet.FileRegistry
	bulk := services.NewFileBulkService(services.NewTagService(fx.factorySet))

	fileID := seedTagFile(c, fx.groupASet, ctx, "receipt", "paperwork")
	file, err := files.Get(ctx, fileID)
	c.Assert(err, qt.IsNil)
	patch, err := services.ParseFileTagPatch([]string{"receipts"}, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(bulk.EnsureTags(ctx, &patch), qt.IsNil)

	release := holdRowForUpdate(c, fx.dbx, "files", fileID)

	saveDone := make(chan error, 1)
	go func() {
		edit := *file
		edit.Title = "Drill receipt"
		edit.Tags = append(edit.Tags, "warranty")
		_, err := files.Update(ctx, edit)
		saveDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, saveDone)

	bulkDone := make(chan error, 1)
	go func() {
		out := bulk.UpdateTags(ctx, files, []string{fileID}, patch)
		bulkDone <- bulkResult(out, fileID)
	}()
	waitForLockWaiters(c, fx.dbx, 2, saveDone, bulkDone)

	release()
	c.Assert(<-saveDone, qt.IsNil)
	c.Assert(<-bulkDone, qt.IsNil)

	got, err := files.Get(ctx, fileID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Title, qt.Equals, "Drill receipt")
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"paperwork", "warranty", "receipts"})
}
