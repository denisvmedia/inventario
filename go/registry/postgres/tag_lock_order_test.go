package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
)

// holdCurrencyMigrationLock takes the lock a running currency migration of
// groupID holds while it converts the group's items, and returns the function
// that commits its transaction.
func holdCurrencyMigrationLock(c *qt.C, dbx *sqlx.DB, groupID string) (release func()) {
	c.Helper()
	tx, err := dbx.BeginTxx(context.Background(), nil)
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(context.Background(),
		`SELECT pg_advisory_xact_lock(hashtext('currency_migration'), hashtext($1))`, groupID)
	c.Assert(err, qt.IsNil)
	return func() { c.Assert(tx.Commit(), qt.IsNil) }
}

// An item save and a rename of one of the item's tags meet on the item's
// row. A third transaction holds the row, so the save queues on it first and
// the rename second. When the row is released, the save must not wait for a
// lock the rename holds: both finish, and the item keeps the saved name under
// the renamed tag (#2874).
func TestTagRename_Postgres_DuringAnItemSave(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := fx.ctxA
	items := fx.groupASet.CommodityRegistry

	itemID := seedTagCommodity(c, fx.groupASet, ctx, fx.areaAID, "Drill", "garage")
	item, err := items.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	tag, err := fx.groupASet.TagRegistry.GetBySlug(ctx, models.TagKindCommodity, "garage")
	c.Assert(err, qt.IsNil)

	release := holdRowForUpdate(c, fx.dbx, "commodities", itemID)

	saveDone := make(chan error, 1)
	go func() {
		edit := *item
		edit.Name = "Cordless drill"
		_, err := items.Update(ctx, edit)
		saveDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, saveDone)

	renameDone := make(chan error, 1)
	go func() {
		_, _, err := fx.groupASet.TagRegistry.RenameAtomic(ctx, tag.ID, "Workshop", "workshop", "", nil)
		renameDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 2, saveDone, renameDone)

	release()
	c.Assert(<-saveDone, qt.IsNil)
	c.Assert(<-renameDone, qt.IsNil)

	got, err := items.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Name, qt.Equals, "Cordless drill")
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"workshop"})
}

// The same race with a forced tag delete in place of the rename.
func TestTagDelete_Postgres_DuringAnItemSave(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := fx.ctxA
	items := fx.groupASet.CommodityRegistry

	itemID := seedTagCommodity(c, fx.groupASet, ctx, fx.areaAID, "Drill", "garage")
	item, err := items.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	tag, err := fx.groupASet.TagRegistry.GetBySlug(ctx, models.TagKindCommodity, "garage")
	c.Assert(err, qt.IsNil)

	release := holdRowForUpdate(c, fx.dbx, "commodities", itemID)

	saveDone := make(chan error, 1)
	go func() {
		edit := *item
		edit.Name = "Cordless drill"
		_, err := items.Update(ctx, edit)
		saveDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, saveDone)

	deleteDone := make(chan error, 1)
	go func() {
		_, err := fx.groupASet.TagRegistry.DeleteAtomic(ctx, tag.ID, true)
		deleteDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 2, saveDone, deleteDone)

	release()
	c.Assert(<-saveDone, qt.IsNil)
	c.Assert(<-deleteDone, qt.IsNil)

	got, err := items.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Name, qt.Equals, "Cordless drill")
	c.Assert([]string(got.Tags), qt.HasLen, 0)
}

// A file save races a rename of one of the file's tags the same way.
func TestTagRename_Postgres_DuringAFileSave(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := fx.ctxA
	files := fx.groupASet.FileRegistry

	fileID := seedTagFile(c, fx.groupASet, ctx, "receipt", "paperwork")
	file, err := files.Get(ctx, fileID)
	c.Assert(err, qt.IsNil)
	tag, err := fx.groupASet.TagRegistry.GetBySlug(ctx, models.TagKindFile, "paperwork")
	c.Assert(err, qt.IsNil)

	release := holdRowForUpdate(c, fx.dbx, "files", fileID)

	saveDone := make(chan error, 1)
	go func() {
		edit := *file
		edit.Title = "Drill receipt"
		_, err := files.Update(ctx, edit)
		saveDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, saveDone)

	renameDone := make(chan error, 1)
	go func() {
		_, _, err := fx.groupASet.TagRegistry.RenameAtomic(ctx, tag.ID, "Receipts", "receipts", "", nil)
		renameDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 2, saveDone, renameDone)

	release()
	c.Assert(<-saveDone, qt.IsNil)
	c.Assert(<-renameDone, qt.IsNil)

	got, err := files.Get(ctx, fileID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Title, qt.Equals, "Drill receipt")
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"receipts"})
}

// A running currency migration updates the group's items one by one under
// its lock. A tag rename that rewrites those items waits for it instead of
// locking them in another order.
func TestTagRename_Postgres_WaitsForACurrencyMigration(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := fx.ctxA

	itemID := seedTagCommodity(c, fx.groupASet, ctx, fx.areaAID, "Drill", "garage")
	tag, err := fx.groupASet.TagRegistry.GetBySlug(ctx, models.TagKindCommodity, "garage")
	c.Assert(err, qt.IsNil)

	release := holdCurrencyMigrationLock(c, fx.dbx, fx.groupAID)

	done := make(chan error, 1)
	go func() {
		_, _, err := fx.groupASet.TagRegistry.RenameAtomic(ctx, tag.ID, "Workshop", "workshop", "", nil)
		done <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, done)

	release()
	c.Assert(<-done, qt.IsNil)
	got, err := fx.groupASet.CommodityRegistry.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"workshop"})
}

// A forced tag delete strips the tag from the group's items, so it waits for
// a running currency migration too.
func TestTagDelete_Postgres_WaitsForACurrencyMigration(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := fx.ctxA

	itemID := seedTagCommodity(c, fx.groupASet, ctx, fx.areaAID, "Drill", "garage")
	tag, err := fx.groupASet.TagRegistry.GetBySlug(ctx, models.TagKindCommodity, "garage")
	c.Assert(err, qt.IsNil)

	release := holdCurrencyMigrationLock(c, fx.dbx, fx.groupAID)

	done := make(chan error, 1)
	go func() {
		_, err := fx.groupASet.TagRegistry.DeleteAtomic(ctx, tag.ID, true)
		done <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, done)

	release()
	c.Assert(<-done, qt.IsNil)
	got, err := fx.groupASet.CommodityRegistry.Get(ctx, itemID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(got.Tags), qt.HasLen, 0)
}

// Deleting an item type that moves its items to another type waits for a
// running currency migration as well.
func TestItemTypeDelete_Postgres_MovingItemsWaitsForACurrencyMigration(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := fx.ctxA
	types := fx.groupASet.ItemTypeRegistry

	vehicle, err := types.Create(ctx, models.ItemType{Name: "Vehicle"})
	c.Assert(err, qt.IsNil)
	other, err := types.GetBySlug(ctx, models.CommodityTypeOther)
	c.Assert(err, qt.IsNil)
	created, err := fx.groupASet.CommodityRegistry.Create(ctx, models.Commodity{
		Name: "Car", ShortName: "Car", Type: vehicle.Slug,
		AreaID: new(fx.areaAID), Count: 1, Status: models.CommodityStatusInUse,
	})
	c.Assert(err, qt.IsNil)

	release := holdCurrencyMigrationLock(c, fx.dbx, fx.groupAID)

	done := make(chan error, 1)
	go func() {
		_, err := types.DeleteReassigning(ctx, vehicle.ID, other.ID)
		done <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, done)

	release()
	c.Assert(<-done, qt.IsNil)
	got, err := fx.groupASet.CommodityRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Type, qt.Equals, models.CommodityTypeOther)
}
