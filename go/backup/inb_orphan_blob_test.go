//go:build !legacy_xml_backup

package backup_test

import (
	"bytes"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/models"
)

// attachCommodityFileAtKey is the divergent-key variant of attachCommodityFile:
// it stores the blob under a caller-chosen OriginalPath that is NOT the canonical
// BuildFileBlobKey(tenant, uuid, ext) shape (e.g. an upload-time basename key, or
// a legacy flat key). The export streams from OriginalPath, but on restore the
// importer always re-keys to the canonical t/<tenant>/files/<uuid><ext>. The two
// keys therefore differ, which is exactly the condition under which the
// merge-restore orphan-blob bug (#2125) is observable: a MergeAdd-skip would
// otherwise write the canonical key with no row to clean it, and a MergeUpdate
// would re-point the row to the canonical key and strand the old one.
//
// Returns the file UUID and the custom (source) blob key it was stored under.
func (f *inbFixture) attachCommodityFileAtKey(c *qt.C, bucketMeta, customKey, ext, mime string, size int) (fileUUID, sourceKey string) {
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	commodities := must.Must(comReg.List(f.ctx))
	c.Assert(len(commodities) >= 1, qt.IsTrue)
	com := commodities[0]

	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	created := must.Must(fileReg.Create(f.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Title:                    "photo",
		Type:                     models.FileTypeFromMIME(mime),
		Category:                 models.FileCategoryFromContext("commodity", bucketMeta, mime),
		Tags:                     models.StringSlice{},
		LinkedEntityType:         "commodity",
		LinkedEntityID:           com.ID,
		LinkedEntityMeta:         bucketMeta,
		File: &models.File{
			Path:      "photo",
			Ext:       ext,
			MIMEType:  mime,
			SizeBytes: int64(size),
		},
	}))

	// Point the row at the custom (non-canonical) key and write the blob there.
	created.OriginalPath = customKey
	must.Must(fileReg.Update(f.ctx, *created))

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()
	body := bytes.Repeat([]byte("inventario-blob-chunk"), size/21+1)[:size]
	c.Assert(bucket.WriteAll(f.ctx, customKey, body, nil), qt.IsNil)

	return created.UUID, customKey
}

// blobExists is a small helper around bucket.Exists for the orphan-blob tests.
func (f *inbFixture) blobExists(c *qt.C, key string) bool {
	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()
	return must.Must(bucket.Exists(f.ctx, key))
}

// TestINBRestore_MergeAddSkip_NoOrphanBlob is a #2125 regression: a MergeAdd
// restore SKIPS files that already exist, but the `.inb` walker used to stream
// the member bytes into a fresh canonical blob key BEFORE making the skip
// decision — leaving a blob with no owning row. The fix decides skip first and
// drains the bytes instead of writing them. Here the existing file's blob lives
// under a custom (non-canonical) key, so the canonical key the broken walker
// would have written is distinguishable: it must NOT exist after the skip.
func TestINBRestore_MergeAddSkip_NoOrphanBlob(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	const size = 4096
	customKey := "t/tenant-a/files/legacy-upload-basename.jpg"
	fileUUID, srcKey := f.attachCommodityFileAtKey(c, "images", customKey, ".jpg", "image/jpeg", size)

	blobKey, _ := f.runExport(c, signer)

	// The canonical key the importer will compute for this file — the key the
	// broken walker would orphan on skip.
	canonicalKey := blobkeys.BuildFileBlobKey("tenant-a", fileUUID, ".jpg")
	c.Assert(canonicalKey, qt.Not(qt.Equals), srcKey)
	// Pre-condition: the canonical key does not exist yet (only the custom key does).
	c.Assert(f.blobExists(c, canonicalKey), qt.IsFalse)

	// Restore into the SAME factory with MergeAdd — the file already exists, so
	// it is skipped.
	final, err := restoreInbWithOptions(c, f, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyMergeAdd),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	// The skip must NOT have written the canonical blob key — no orphan.
	c.Assert(f.blobExists(c, canonicalKey), qt.IsFalse,
		qt.Commentf("MergeAdd skip must not orphan a blob at the canonical key %s", canonicalKey))
	// The existing file's original blob is untouched.
	c.Assert(f.blobExists(c, srcKey), qt.IsTrue)
}

// TestINBRestore_MergeUpdate_DeletesStaleBlob is a #2125 regression: a
// MergeUpdate restore re-points the existing file row to a freshly-computed
// canonical blob key. When the existing row's blob lived under a different key
// (an upload basename key, a different ext) the old blob used to dangle. The fix
// best-effort deletes the superseded key after the row is re-pointed. Here the
// existing blob is under a custom key; after the update the canonical key must
// hold the bytes and the custom (stale) key must be gone.
func TestINBRestore_MergeUpdate_DeletesStaleBlob(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	const size = 4096
	customKey := "t/tenant-a/files/legacy-upload-basename.jpg"
	fileUUID, srcKey := f.attachCommodityFileAtKey(c, "images", customKey, ".jpg", "image/jpeg", size)

	blobKey, _ := f.runExport(c, signer)

	canonicalKey := blobkeys.BuildFileBlobKey("tenant-a", fileUUID, ".jpg")
	c.Assert(canonicalKey, qt.Not(qt.Equals), srcKey)

	// Restore into the SAME factory with MergeUpdate — the file exists, so it is
	// updated and re-pointed to the canonical key.
	final, err := restoreInbWithOptions(c, f, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyMergeUpdate),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	// The restored row points at the canonical key, and the bytes are there.
	restored := f.fileByUUID(c, fileUUID)
	c.Assert(restored.File, qt.IsNotNil)
	c.Assert(restored.File.OriginalPath, qt.Equals, canonicalKey)
	c.Assert(f.blobExists(c, canonicalKey), qt.IsTrue)

	// The superseded (stale) blob must have been cleaned up — no dangling blob.
	c.Assert(f.blobExists(c, srcKey), qt.IsFalse,
		qt.Commentf("MergeUpdate must delete the superseded blob at the old key %s", srcKey))
}

// MergeUpdate must NOT delete the superseded blob when ANOTHER live file row
// shares that stale key (#2250).
//
// The stale key is the EXISTING row's pre-#2241 key, which another row can
// legitimately share (two same-named uploads in one second). Re-pointing the
// restored row must not take the sharer's bytes with it — `files` has no
// soft-delete.
//
// Without a test like this the guard is invisible: removing the whole
// BlobSharedByOtherRows check from deleteSupersededBlob leaves every other
// backup test green, because none of them plant a sharer on the stale key.
func TestINBRestore_MergeUpdate_KeepsStaleBlobSharedByAnotherRow(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	const size = 4096
	staleKey := "t/tenant-a/files/receipt-1783824560.jpg"
	fileUUID, _ := f.attachCommodityFileAtKey(c, "images", staleKey, ".jpg", "image/jpeg", size)

	// A SECOND live file row sharing the exact same blob key — the sharer whose
	// bytes must survive. Service registry so it can carry a different group.
	sharerReg := f.fs.FileRegistryFactory.CreateServiceRegistry()
	sharer := must.Must(sharerReg.Create(f.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-a", GroupID: "another-group", CreatedByUserID: f.user.ID,
		},
		Title: "sharer", Type: models.FileTypeImage,
		File: &models.File{Path: "receipt", OriginalPath: staleKey, Ext: ".jpg", MIMEType: "image/jpeg"},
	}))

	blobKey, _ := f.runExport(c, signer)

	final, err := restoreInbWithOptions(c, f, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyMergeUpdate),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	// The restored row moved to its canonical key…
	canonicalKey := blobkeys.BuildFileBlobKey("tenant-a", fileUUID, ".jpg")
	restored := f.fileByUUID(c, fileUUID)
	c.Assert(restored.File.OriginalPath, qt.Equals, canonicalKey)

	// …but the stale key's bytes SURVIVE, because the sharer still points there.
	c.Assert(f.blobExists(c, staleKey), qt.IsTrue,
		qt.Commentf("MergeUpdate destroyed the bytes of a live file sharing the stale key %s", staleKey))
	c.Assert(must.Must(sharerReg.Get(f.ctx, sharer.ID)).File.OriginalPath, qt.Equals, staleKey)
}
