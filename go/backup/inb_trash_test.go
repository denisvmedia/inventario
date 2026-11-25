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
