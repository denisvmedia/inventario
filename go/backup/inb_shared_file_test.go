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

	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
)

// sharedFileFixture is an inbFixture with a second item and one receipt
// attached to both items and to the location.
type sharedFileFixture struct {
	*inbFixture
	tvID, soundbarID string
	tvUUID           string
	soundbarUUID     string
	locUUID          string
	receiptUUID      string
}

const sharedReceiptSize = 3000

func newSharedFileFixture(c *qt.C) sharedFileFixture {
	f := newInbFixture(c)

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	tv := must.Must(comReg.List(f.ctx))[0]
	soundbar := must.Must(comReg.Create(f.ctx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Name:                     "Soundbar",
		ShortName:                "soundbar",
		Type:                     models.CommodityTypeElectronics,
		AreaID:                   new(f.areaID),
		Count:                    1,
		Status:                   models.CommodityStatusInUse,
		OriginalPriceCurrency:    "USD",
		Draft:                    true,
	}))
	loc := must.Must(must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx)).Get(f.ctx, f.locID))

	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	receipt := must.Must(fileReg.Create(f.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Title:                    "receipt",
		Type:                     models.FileTypeDocument,
		Category:                 models.FileCategoryDocuments,
		Tags:                     models.StringSlice{},
		Links: []models.FileLink{
			{EntityType: models.FileLinkCommodity, EntityID: tv.ID, Meta: models.FileLinkMetaDocuments},
			{EntityType: models.FileLinkCommodity, EntityID: soundbar.ID, Meta: models.FileLinkMetaDocuments},
			{EntityType: models.FileLinkLocation, EntityID: loc.ID, Meta: models.FileLinkMetaFiles},
		},
		File: &models.File{
			Path:      "receipt",
			Ext:       ".pdf",
			MIMEType:  "application/pdf",
			SizeBytes: sharedReceiptSize,
		},
	}))
	f.writeSourceBlob(c, receipt, ".pdf", sharedReceiptSize)

	return sharedFileFixture{
		inbFixture:   f,
		tvID:         tv.ID,
		soundbarID:   soundbar.ID,
		tvUUID:       tv.UUID,
		soundbarUUID: soundbar.UUID,
		locUUID:      loc.UUID,
		receiptUUID:  receipt.UUID,
	}
}

// restoredLinks describes the links of the restored receipt by the UUIDs of
// the entities they point at, so the assertion does not depend on the fresh
// database ids the restore assigns.
func (f sharedFileFixture) restoredLinks(c *qt.C) []string {
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	receipt := f.fileByUUID(c, f.receiptUUID)
	out := make([]string, 0, len(receipt.Links))
	for _, l := range receipt.Links {
		var uuid string
		switch l.EntityType {
		case models.FileLinkCommodity:
			uuid = must.Must(comReg.Get(f.ctx, l.EntityID)).UUID
		case models.FileLinkLocation:
			uuid = must.Must(locReg.Get(f.ctx, l.EntityID)).UUID
		}
		out = append(out, l.EntityType+"/"+uuid+"/"+l.Meta)
	}
	sort.Strings(out)
	return out
}

func (f sharedFileFixture) wantLinks() []string {
	want := []string{
		"commodity/" + f.tvUUID + "/documents",
		"commodity/" + f.soundbarUUID + "/documents",
		"location/" + f.locUUID + "/files",
	}
	sort.Strings(want)
	return want
}

// TestINBExport_SharedFileStoredOnce: a file attached to two items and a
// location is written to the archive once, every reference names that one
// member, and the statistics count the stored file once and the attachments
// per bucket.
func TestINBExport_SharedFileStoredOnce(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSharedFileFixture(c)

	_, archive := f.runExport(c, signer)
	order, jsons := innerMembers(c, archive)

	var byteMembers []string
	for _, name := range order {
		if strings.Contains(name, f.receiptUUID) {
			byteMembers = append(byteMembers, name)
		}
	}
	c.Assert(byteMembers, qt.HasLen, 1, qt.Commentf("members: %v", order))

	// Every JSON document that references the receipt names the same member.
	refs := 0
	for name, data := range jsons {
		if name == "manifest.json" {
			continue
		}
		for _, path := range archivePathsOf(c, data, f.receiptUUID) {
			c.Assert(path, qt.Equals, byteMembers[0], qt.Commentf("document %s", name))
			refs++
		}
	}
	c.Assert(refs, qt.Equals, 3)

	var manifest map[string]any
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	c.Assert(manifest["version"], qt.Equals, export.INBFormatVersion)
	stats := manifest["statistics"].(map[string]any)
	c.Assert(stats["fileCount"], qt.Equals, float64(1))
	c.Assert(stats["totalFileSize"], qt.Equals, float64(sharedReceiptSize))
}

// archivePathsOf walks a JSON document and returns the "path" of every file
// reference whose "id" (the file UUID) is fileUUID.
func archivePathsOf(c *qt.C, data []byte, fileUUID string) []string {
	var doc any
	c.Assert(json.Unmarshal(data, &doc), qt.IsNil)
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if t["id"] == fileUUID {
				if p, ok := t["path"].(string); ok {
					out = append(out, p)
				}
			}
			for _, child := range t {
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		}
	}
	walk(doc)
	return out
}

// TestINBRoundTrip_SharedFileKeepsAllLinks: restoring the archive recreates
// one file with all three links, pointing at the restored entities; restoring
// it again, with any strategy, adds no file and no link.
func TestINBRoundTrip_SharedFileKeepsAllLinks(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSharedFileFixture(c)

	blobKey, _ := f.runExport(c, signer)

	strategies := []types.RestoreStrategy{
		types.RestoreStrategyFullReplace,
		types.RestoreStrategyFullReplace,
		types.RestoreStrategyMergeAdd,
		types.RestoreStrategyMergeUpdate,
	}
	for i, strategy := range strategies {
		final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(strategy)})
		c.Assert(err, qt.IsNil, qt.Commentf("restore %d (%s)", i, strategy))
		c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("restore %d (%s): %v", i, strategy, final.ErrorMessage))
		c.Assert(final.ErrorCount, qt.Equals, 0)

		c.Assert(f.countFilesWithUUID(c, f.receiptUUID), qt.Equals, 1, qt.Commentf("restore %d (%s)", i, strategy))
		c.Assert(f.restoredLinks(c), qt.DeepEquals, f.wantLinks(), qt.Commentf("restore %d (%s)", i, strategy))
	}

	receipt := f.fileByUUID(c, f.receiptUUID)
	c.Assert(receipt.SizeBytes, qt.Equals, int64(sharedReceiptSize))
	c.Assert(receipt.SHA256, qt.Not(qt.Equals), "")
	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()
	c.Assert(must.Must(bucket.ReadAll(f.ctx, receipt.OriginalPath)), qt.DeepEquals, blobBody(sharedReceiptSize))
}
