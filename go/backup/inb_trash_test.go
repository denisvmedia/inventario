//go:build !legacy_xml_backup

package backup_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

func (f sharedFileFixture) trashService() *services.TrashService {
	return services.NewTrashService(f.fs, inbUploadLocation)
}

func (f sharedFileFixture) countItemsWithUUID(c *qt.C, uuid string) (live, trashed int) {
	c.Helper()
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	for _, item := range must.Must(comReg.List(f.ctx)) {
		if item.UUID == uuid {
			live++
		}
	}
	page, _ := must.Must2(comReg.ListTrashed(f.ctx, 0, 100))
	for _, item := range page {
		if item.UUID == uuid {
			trashed++
		}
	}
	return live, trashed
}

// TestINBExport_LeavesTrashOut: an item in Trash is not in the backup. The
// receipt it shares with a live item and the location is, without the link
// to the trashed item.
func TestINBExport_LeavesTrashOut(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSharedFileFixture(c)

	_, err := f.trashService().TrashCommodity(f.ctx, f.soundbarID)
	c.Assert(err, qt.IsNil)

	_, archive := f.runExport(c, signer)
	_, jsons := innerMembers(c, archive)
	for name, data := range jsons {
		c.Assert(string(data), qt.Not(qt.Contains), f.soundbarUUID, qt.Commentf("document %s mentions the trashed item", name))
	}
	refs := 0
	for name, data := range jsons {
		if name == "manifest.json" {
			continue
		}
		refs += len(archivePathsOf(c, data, f.receiptUUID))
	}
	// One reference under the TV, one under the location.
	c.Assert(refs, qt.Equals, 2)
}

// TestINBRestore_MergeLeavesTrashedItemInTrash: a merge restore of a backup
// taken before the item was deleted does not bring a second copy and does
// not touch the item in Trash.
func TestINBRestore_MergeLeavesTrashedItemInTrash(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSharedFileFixture(c)

	blobKey, _ := f.runExport(c, signer)
	_, err := f.trashService().TrashCommodity(f.ctx, f.soundbarID)
	c.Assert(err, qt.IsNil)

	for _, strategy := range []types.RestoreStrategy{types.RestoreStrategyMergeAdd, types.RestoreStrategyMergeUpdate} {
		final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(strategy)})
		c.Assert(err, qt.IsNil, qt.Commentf("%s", strategy))
		c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("%s: %v", strategy, final.ErrorMessage))
		c.Assert(final.ErrorCount, qt.Equals, 0, qt.Commentf("%s", strategy))

		live, trashed := f.countItemsWithUUID(c, f.soundbarUUID)
		c.Assert(live, qt.Equals, 0, qt.Commentf("%s", strategy))
		c.Assert(trashed, qt.Equals, 1, qt.Commentf("%s", strategy))
		c.Assert(f.countFilesWithUUID(c, f.receiptUUID), qt.Equals, 1, qt.Commentf("%s", strategy))
	}

	// The receipt kept its link to the trashed item, so restoring the item
	// brings it back where it was.
	result, err := f.trashService().RestoreCommodity(f.ctx, f.soundbarID)
	c.Assert(err, qt.IsNil)
	c.Assert(result.Commodity.UUID, qt.Equals, f.soundbarUUID)
	c.Assert(f.restoredLinks(c), qt.DeepEquals, f.wantLinks())
}

// TestINBRestore_FullReplaceEmptiesTrash: replacing the group's data also
// empties its Trash, so the archive's copy of a trashed item lands as a live
// item.
func TestINBRestore_FullReplaceEmptiesTrash(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSharedFileFixture(c)

	blobKey, _ := f.runExport(c, signer)
	_, err := f.trashService().TrashCommodity(f.ctx, f.soundbarID)
	c.Assert(err, qt.IsNil)

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(types.RestoreStrategyFullReplace)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("%v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	live, trashed := f.countItemsWithUUID(c, f.soundbarUUID)
	c.Assert(live, qt.Equals, 1)
	c.Assert(trashed, qt.Equals, 0)
	c.Assert(f.restoredLinks(c), qt.DeepEquals, f.wantLinks())

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	_, err = comReg.GetTrashed(f.ctx, f.soundbarID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// fileOf creates a document file with a blob, linked to the given items.
func (f sharedFileFixture) fileOf(c *qt.C, title string, itemIDs ...string) *models.FileEntity {
	c.Helper()
	links := make([]models.FileLink, 0, len(itemIDs))
	for _, id := range itemIDs {
		links = append(links, models.FileLink{EntityType: models.FileLinkCommodity, EntityID: id, Meta: models.FileLinkMetaDocuments})
	}
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	file := must.Must(fileReg.Create(f.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Title:                    title,
		Type:                     models.FileTypeDocument,
		Category:                 models.FileCategoryDocuments,
		Tags:                     models.StringSlice{},
		Links:                    links,
		File:                     &models.File{Path: title, Ext: ".pdf", MIMEType: "application/pdf", SizeBytes: 100},
	}))
	f.writeSourceBlob(c, file, ".pdf", 100)
	return file
}

// trashedWith returns the file with the UUID among the files in Trash with
// the item, or nil.
func (f sharedFileFixture) trashedWith(c *qt.C, itemID, fileUUID string) *models.FileEntity {
	c.Helper()
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	for _, file := range must.Must(fileReg.ListTrashedLinkedTo(f.ctx, models.FileLinkCommodity, itemID)) {
		if file.UUID == fileUUID {
			return file
		}
	}
	return nil
}

// TestINBRestore_MergeBringsAFileOfATrashedItemIntoTrash: a merge that brings
// a file the group no longer has, whose only item waits in Trash, creates it
// in Trash with that item, so restoring the item brings the file back.
func TestINBRestore_MergeBringsAFileOfATrashedItemIntoTrash(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSharedFileFixture(c)
	manual := f.fileOf(c, "manual", f.soundbarID)

	blobKey, _ := f.runExport(c, signer)
	_, err := f.trashService().TrashCommodity(f.ctx, f.soundbarID)
	c.Assert(err, qt.IsNil)
	// The group loses the manual's row while the item waits in Trash.
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(must.Must(fileReg.PurgeTrashed(f.ctx, manual.ID)), qt.IsTrue)

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(types.RestoreStrategyMergeAdd)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("%v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	c.Assert(f.countFilesWithUUID(c, manual.UUID), qt.Equals, 0, qt.Commentf("no live copy"))
	parked := f.trashedWith(c, f.soundbarID, manual.UUID)
	c.Assert(parked, qt.IsNotNil)
	c.Assert(parked.DeletedWithCommodityID, qt.DeepEquals, new(f.soundbarID))

	result, err := f.trashService().RestoreCommodity(f.ctx, f.soundbarID)
	c.Assert(err, qt.IsNil)
	// The receipt never left: the TV keeps it live.
	c.Assert(result.FilesRestored, qt.Equals, 1)
	c.Assert(f.countFilesWithUUID(c, manual.UUID), qt.Equals, 1)
}

// TestINBRestore_MergeKeepsAFileSharedWithALiveItemLive: a file the group no
// longer has, shared by an item in Trash and a live item in another location,
// lands live with both links, whichever reference the archive lists first.
func TestINBRestore_MergeKeepsAFileSharedWithALiveItemLive(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSharedFileFixture(c)

	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	cabin := must.Must(locReg.Create(f.ctx, models.Location{Name: "Cabin", Address: "Lake"}))
	porch := must.Must(areaReg.Create(f.ctx, models.Area{Name: "Porch", LocationID: cabin.ID}))
	lamp := must.Must(comReg.Create(f.ctx, models.Commodity{
		Name:                  "Lamp",
		ShortName:             "lamp",
		Type:                  models.CommodityTypeElectronics,
		AreaID:                new(porch.ID),
		Count:                 1,
		Status:                models.CommodityStatusInUse,
		OriginalPriceCurrency: "USD",
		Draft:                 true,
	}))
	warranty := f.fileOf(c, "warranty", f.soundbarID, lamp.ID)

	blobKey, _ := f.runExport(c, signer)
	_, err := f.trashService().TrashCommodity(f.ctx, f.soundbarID)
	c.Assert(err, qt.IsNil)
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	_ = must.Must(fileReg.Get(f.ctx, warranty.ID)) // the lamp keeps it live
	c.Assert(fileReg.Delete(f.ctx, warranty.ID), qt.IsNil)

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(types.RestoreStrategyMergeAdd)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("%v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	restored := f.fileByUUID(c, warranty.UUID)
	c.Assert(restored.IsTrashed(), qt.IsFalse)
	c.Assert(restored.LinkTo(models.FileLinkCommodity, lamp.ID), qt.IsNotNil)
	c.Assert(restored.LinkTo(models.FileLinkCommodity, f.soundbarID), qt.IsNotNil)
}
