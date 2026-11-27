//go:build !legacy_xml_backup

package backup_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/internal/backupsign"
	"go.5x5.cz/inventario/models"
)

// An archive stores the bytes of a shared file once, after the first location
// document that references it. These tests break the item in that document so
// the restore refuses it, and check what becomes of the file.

// failedSharerFixture is an inbFixture with a receipt the TV in Home shares
// with one item in each of two more locations.
type failedSharerFixture struct {
	*inbFixture
	tvID    string
	tvUUID  string
	others  []string
	receipt string
}

const failedSharerReceiptSize = 2500

func newFailedSharerFixture(c *qt.C) *failedSharerFixture {
	f := newInbFixture(c)
	ids := models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID}
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	tv := must.Must(comReg.List(f.ctx))[0]

	links := []models.FileLink{{EntityType: models.FileLinkCommodity, EntityID: tv.ID, Meta: models.FileLinkMetaDocuments}}
	var others []string
	for _, name := range []string{"Office", "Garage"} {
		loc := must.Must(locReg.Create(f.ctx, models.Location{TenantGroupAwareEntityID: ids, Name: name}))
		area := must.Must(areaReg.Create(f.ctx, models.Area{TenantGroupAwareEntityID: ids, Name: name + " shelf", LocationID: loc.ID}))
		item := must.Must(comReg.Create(f.ctx, models.Commodity{
			TenantGroupAwareEntityID: ids,
			Name:                     name + " lamp",
			ShortName:                "lamp",
			Type:                     models.CommodityTypeFurniture,
			AreaID:                   new(area.ID),
			Count:                    1,
			Status:                   models.CommodityStatusInUse,
			OriginalPriceCurrency:    "USD",
			Draft:                    true,
		}))
		links = append(links, models.FileLink{EntityType: models.FileLinkCommodity, EntityID: item.ID, Meta: models.FileLinkMetaDocuments})
		others = append(others, item.UUID)
	}

	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	receipt := must.Must(fileReg.Create(f.ctx, models.FileEntity{
		TenantGroupAwareEntityID: ids,
		Title:                    "receipt",
		Type:                     models.FileTypeDocument,
		Category:                 models.FileCategoryDocuments,
		Tags:                     models.StringSlice{},
		Links:                    links,
		File:                     &models.File{Path: "receipt", Ext: ".pdf", MIMEType: "application/pdf", SizeBytes: failedSharerReceiptSize},
	}))
	f.writeSourceBlob(c, receipt, ".pdf", failedSharerReceiptSize)

	return &failedSharerFixture{inbFixture: f, tvID: tv.ID, tvUUID: tv.UUID, others: others, receipt: receipt.UUID}
}

// exportWithBrokenSharer exports the group and blanks the name of every item
// that references the receipt in the location document right before the
// receipt's bytes, so the restore refuses those items. It returns the key of
// the broken archive and the UUIDs of the items left intact.
func (f *failedSharerFixture) exportWithBrokenSharer(c *qt.C, signer *backupsign.Signer) (string, []string) {
	_, archive := f.runExport(c, signer)
	order, jsons := innerMembers(c, archive)
	var carrier string
	for _, name := range order {
		if strings.HasPrefix(name, "files/") && strings.Contains(name, f.receipt) {
			break
		}
		if strings.HasPrefix(name, "location-") {
			carrier = name
		}
	}
	c.Assert(carrier, qt.Not(qt.Equals), "", qt.Commentf("members: %v", order))
	c.Assert(archivePathsOf(c, jsons[carrier], f.receipt), qt.HasLen, 1, qt.Commentf("the receipt's bytes follow its first reference"))

	var broken []string
	archive = rewriteInnerMember(c, signer, archive, carrier, func(body []byte) []byte {
		var doc types.INBLocationDoc
		c.Assert(json.Unmarshal(body, &doc), qt.IsNil)
		for i := range doc.Commodities {
			com := &doc.Commodities[i]
			for _, ref := range com.Documents {
				if ref.ID == f.receipt {
					com.Name = ""
					broken = append(broken, com.ID)
				}
			}
		}
		return must.Must(json.Marshal(doc))
	})
	c.Assert(broken, qt.HasLen, 1)

	intact := []string{}
	for _, uuid := range append([]string{f.tvUUID}, f.others...) {
		if uuid != broken[0] {
			intact = append(intact, uuid)
		}
	}
	key := "t/tenant-a/restores/failed-sharer.inb"
	writeArchive(c, f.inbFixture, key, archive)
	return key, intact
}

// receiptItems returns the UUIDs of the items the restored receipt is linked
// to, sorted.
func (f *failedSharerFixture) receiptItems(c *qt.C) []string {
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	var out []string
	for _, l := range f.fileByUUID(c, f.receipt).Links {
		c.Assert(l.EntityType, qt.Equals, models.FileLinkCommodity)
		out = append(out, must.Must(comReg.Get(f.ctx, l.EntityID)).UUID)
	}
	sort.Strings(out)
	return out
}

// TestINBRestore_FailedItemLeavesSharedFileToOthers: when the item whose
// location document carries a shared file's bytes fails to restore, the items
// in later documents still get the file. Only the failed item counts as an
// error. A merge keeps the group's own item, so its link stays as well.
func TestINBRestore_FailedItemLeavesSharedFileToOthers(t *testing.T) {
	for _, strategy := range []types.RestoreStrategy{
		types.RestoreStrategyFullReplace,
		types.RestoreStrategyMergeAdd,
		types.RestoreStrategyMergeUpdate,
	} {
		t.Run(string(strategy), func(t *testing.T) {
			c := qt.New(t)
			signer := testSigner(c)
			f := newFailedSharerFixture(c)
			key, intact := f.exportWithBrokenSharer(c, signer)

			final, err := restoreInbWithOptions(c, f.inbFixture, signer, key, models.RestoreOptions{Strategy: string(strategy)})
			c.Assert(err, qt.IsNil)
			c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("error: %s", final.ErrorMessage))
			c.Check(final.ErrorCount, qt.Equals, 1, qt.Commentf("only the TV is an error"))

			want := intact
			if strategy != types.RestoreStrategyFullReplace {
				want = append([]string{f.tvUUID}, f.others...)
			}
			sort.Strings(want)
			c.Check(f.countFilesWithUUID(c, f.receipt), qt.Equals, 1)
			c.Check(f.receiptItems(c), qt.DeepEquals, want)

			receipt := f.fileByUUID(c, f.receipt)
			bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
			defer bucket.Close()
			c.Check(must.Must(bucket.ReadAll(f.ctx, receipt.OriginalPath)), qt.DeepEquals, blobBody(failedSharerReceiptSize))
		})
	}
}

// TestINBRestore_FileOfOnlyFailedItemsIsDropped: a file that no restored
// entity references is left out with a counted error, never kept as a file
// without links.
func TestINBRestore_FileOfOnlyFailedItemsIsDropped(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newFailedSharerFixture(c)
	photo := f.attachEntityFile(c, models.FileLinkCommodity, f.tvID, models.FileLinkMetaImages, "tv-photo", ".jpg", "image/jpeg", 300)
	key, _ := f.exportWithBrokenSharer(c, signer)

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, key, models.RestoreOptions{Strategy: string(types.RestoreStrategyFullReplace)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("error: %s", final.ErrorMessage))
	c.Assert(f.receiptItems(c), qt.Not(qt.Contains), f.tvUUID, qt.Commentf("the fixture breaks the TV"))

	c.Check(final.ErrorCount, qt.Equals, 2, qt.Commentf("the TV and its photo"))
	c.Check(f.countFilesWithUUID(c, photo), qt.Equals, 0)
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	for _, file := range must.Must(fileReg.List(f.ctx)) {
		c.Check(file.Links, qt.Not(qt.HasLen), 0, qt.Commentf("file %s %q has no links", file.UUID, file.Title))
	}
}

// TestINBRestore_DryRunPredictsFailedItemSharingAFile: a dry run of the
// archive ends the way the real restore does.
func TestINBRestore_DryRunPredictsFailedItemSharingAFile(t *testing.T) {
	for _, strategy := range []types.RestoreStrategy{
		types.RestoreStrategyFullReplace,
		types.RestoreStrategyMergeAdd,
		types.RestoreStrategyMergeUpdate,
	} {
		t.Run(string(strategy), func(t *testing.T) {
			c := qt.New(t)
			signer := testSigner(c)
			f := newFailedSharerFixture(c)
			key, _ := f.exportWithBrokenSharer(c, signer)

			dry, err := restoreInbWithOptions(c, f.inbFixture, signer, key, models.RestoreOptions{Strategy: string(strategy), DryRun: true})
			c.Assert(err, qt.IsNil)
			final, err := restoreInbWithOptions(c, f.inbFixture, signer, key, models.RestoreOptions{Strategy: string(strategy)})
			c.Assert(err, qt.IsNil)
			c.Check(dry.Status, qt.Equals, final.Status, qt.Commentf("dry run: %s; restore: %s", dry.ErrorMessage, final.ErrorMessage))
			c.Check(final.Status, qt.Equals, models.RestoreStatusCompleted)
		})
	}
}
