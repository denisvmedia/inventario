//go:build !legacy_xml_backup

package backup_test

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/backup/restore/processor"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/models"
)

// blobBody builds a deterministic body of exactly size bytes, matching the shape
// attachCommodityFileFull writes, so a round-trip can compare the restored bytes
// against the source bytes.
func blobBody(size int) []byte {
	return bytes.Repeat([]byte("inventario-blob-chunk"), size/21+1)[:size]
}

// attachEntityFile creates a FileEntity linked to a LOCATION or an AREA (issue
// #2235) and writes its blob under the source tenant's namespace. meta is the
// location/area bucket ("images" or "files"). Returns the file's immutable UUID.
func (f *inbFixture) attachEntityFile(c *qt.C, linkedType, entityID, meta, name, ext, mime string, size int) string {
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	created := must.Must(fileReg.Create(f.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Title:                    name,
		Type:                     models.FileTypeFromMIME(mime),
		Category:                 models.FileCategoryFromContext(linkedType, meta, mime),
		Tags:                     models.StringSlice{},
		LinkedEntityType:         linkedType,
		LinkedEntityID:           entityID,
		LinkedEntityMeta:         meta,
		File: &models.File{
			Path:      name,
			Ext:       ext,
			MIMEType:  mime,
			SizeBytes: int64(size),
		},
	}))
	f.writeSourceBlob(c, created, ext, size)
	return created.UUID
}

// attachStandaloneFile creates an unlinked FileEntity (all three link fields
// empty) and writes its blob. Returns the file's immutable UUID.
func (f *inbFixture) attachStandaloneFile(c *qt.C, name, ext, mime string, size int) string {
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	created := must.Must(fileReg.Create(f.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Title:                    name,
		Type:                     models.FileTypeFromMIME(mime),
		Category:                 models.FileCategoryFromMIME(mime),
		Tags:                     models.StringSlice{},
		File: &models.File{
			Path:      name,
			Ext:       ext,
			MIMEType:  mime,
			SizeBytes: int64(size),
		},
	}))
	f.writeSourceBlob(c, created, ext, size)
	return created.UUID
}

// writeSourceBlob points a freshly created file row at a tenant-namespaced blob
// key and writes size bytes there, so the exporter can stream it.
func (f *inbFixture) writeSourceBlob(c *qt.C, file *models.FileEntity, ext string, size int) {
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	blobKey := "t/tenant-a/files/" + file.UUID + ext
	file.OriginalPath = blobKey
	must.Must(fileReg.Update(f.ctx, *file))

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()
	c.Assert(bucket.WriteAll(f.ctx, blobKey, blobBody(size), nil), qt.IsNil)
}

// locationByUUID / areaByUUID look a restored row up by its immutable UUID (the
// DB id is regenerated on restore, the UUID is not).
func (f *inbFixture) locationByUUID(c *qt.C, uuid string) *models.Location {
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	for _, loc := range must.Must(locReg.List(f.ctx)) {
		if loc != nil && loc.UUID == uuid {
			return loc
		}
	}
	c.Fatalf("location with UUID %s not found after restore", uuid)
	return nil
}

func (f *inbFixture) areaByUUID(c *qt.C, uuid string) *models.Area {
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	for _, area := range must.Must(areaReg.List(f.ctx)) {
		if area != nil && area.UUID == uuid {
			return area
		}
	}
	c.Fatalf("area with UUID %s not found after restore", uuid)
	return nil
}

// TestINBRoundTrip_NonCommodityFiles is the #2235 fidelity test: files attached to
// a LOCATION or an AREA, and STANDALONE files, must survive a full export →
// full_replace restore — metadata, polymorphic link, and blob BYTES. Before the
// files member they were never exported at all, while full_replace swept them, so
// a restore permanently destroyed them.
func TestINBRoundTrip_NonCommodityFiles(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	srcLoc := must.Must(locReg.Get(f.ctx, f.locID))
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	srcArea := must.Must(areaReg.Get(f.ctx, f.areaID))

	const (
		locImageSize   = 300
		locFileSize    = 512
		areaImageSize  = 700
		areaFileSize   = 128
		standaloneSize = 4096
	)
	commodityImage := f.attachCommodityFile(c, "images", 256)
	locImage := f.attachEntityFile(c, "location", f.locID, "images", "front-door", ".jpg", "image/jpeg", locImageSize)
	locDoc := f.attachEntityFile(c, "location", f.locID, "files", "lease", ".pdf", "application/pdf", locFileSize)
	areaImage := f.attachEntityFile(c, "area", f.areaID, "images", "shelf", ".jpg", "image/jpeg", areaImageSize)
	areaDoc := f.attachEntityFile(c, "area", f.areaID, "files", "layout", ".pdf", "application/pdf", areaFileSize)
	standalone := f.attachStandaloneFile(c, "receipt-scan", ".pdf", "application/pdf", standaloneSize)

	blobKey, archive := f.runExport(c, signer)

	// The archive carries the files member, the manifest points at it, and the
	// document precedes its byte members (restore registers refs from the doc,
	// then matches each member against them).
	order, jsons := innerMembers(c, archive)
	var manifest map[string]any
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	c.Assert(manifest["filesFile"], qt.Equals, "files/_index.json")

	docIdx := memberIndex(c, order, "files/_index.json")
	for _, uuid := range []string{locImage, locDoc, areaImage, areaDoc, standalone} {
		byteIdx := memberIndexContaining(c, order, uuid)
		c.Assert(byteIdx > docIdx, qt.IsTrue,
			qt.Commentf("file %s bytes must follow files/_index.json; members=%v", uuid, order))
	}
	// Every location member precedes the files document: the entity links are
	// resolved through the ID mapping, which only fills as locations are applied.
	c.Assert(memberIndex(c, order, "location-home-"+srcLoc.UUID+".json") < docIdx, qt.IsTrue)

	// Statistics: the non-commodity files feed the unified fileCount and
	// totalFileSize only. imageCount stays a COMMODITY-scoped legacy counter, so
	// the location/area images must NOT inflate it.
	stats := manifest["statistics"].(map[string]any)
	c.Assert(stats["fileCount"], qt.Equals, float64(6))
	c.Assert(stats["imageCount"], qt.Equals, float64(1))
	c.Assert(stats["totalFileSize"], qt.Equals,
		float64(256+locImageSize+locFileSize+areaImageSize+areaFileSize+standaloneSize))

	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	// The restore-side statistics mirror the export-side rule: every restored file
	// feeds the unified FileCount, while ImageCount stays COMMODITY-scoped — the
	// location/area images must not inflate the count the operation reports.
	c.Assert(final.FileCount, qt.Equals, 6)
	c.Assert(final.ImageCount, qt.Equals, 1)
	c.Assert(final.InvoiceCount, qt.Equals, 0)
	c.Assert(final.ManualCount, qt.Equals, 0)

	// Links must point at the RESTORED location/area rows (fresh DB ids), never
	// at the archive's ids and never at nothing.
	restoredLoc := f.locationByUUID(c, srcLoc.UUID)
	restoredArea := f.areaByUUID(c, srcArea.UUID)

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()

	entityCases := []struct {
		uuid     string
		linked   string
		entityID string
		meta     string
		path     string
		ext      string
		category models.FileCategory
		fileType models.FileType
		size     int64
	}{
		{locImage, "location", restoredLoc.ID, "images", "front-door", ".jpg", models.FileCategoryImages, models.FileTypeImage, locImageSize},
		{locDoc, "location", restoredLoc.ID, "files", "lease", ".pdf", models.FileCategoryDocuments, models.FileTypeDocument, locFileSize},
		{areaImage, "area", restoredArea.ID, "images", "shelf", ".jpg", models.FileCategoryImages, models.FileTypeImage, areaImageSize},
		{areaDoc, "area", restoredArea.ID, "files", "layout", ".pdf", models.FileCategoryDocuments, models.FileTypeDocument, areaFileSize},
		{standalone, "", "", "", "receipt-scan", ".pdf", models.FileCategoryDocuments, models.FileTypeDocument, standaloneSize},
	}
	for _, tc := range entityCases {
		c.Run(tc.path, func(c *qt.C) {
			restored := f.fileByUUID(c, tc.uuid)
			c.Assert(restored.LinkedEntityType, qt.Equals, tc.linked)
			c.Assert(restored.LinkedEntityID, qt.Equals, tc.entityID)
			c.Assert(restored.LinkedEntityMeta, qt.Equals, tc.meta)
			c.Assert(restored.Category, qt.Equals, tc.category)
			c.Assert(restored.Type, qt.Equals, tc.fileType)
			c.Assert(restored.File, qt.IsNotNil)
			c.Assert(restored.File.Path, qt.Equals, tc.path)
			c.Assert(restored.File.Ext, qt.Equals, tc.ext)
			c.Assert(restored.File.SizeBytes, qt.Equals, tc.size)

			// The blob is re-minted under the importing tenant from the file's
			// immutable UUID, and carries the ORIGINAL bytes.
			key := "t/tenant-a/files/" + tc.uuid + tc.ext
			c.Assert(restored.File.OriginalPath, qt.Equals, key)
			c.Assert(must.Must(bucket.ReadAll(f.ctx, key)), qt.DeepEquals, blobBody(int(tc.size)))
		})
	}

	// The commodity attachment still round-trips through its nested ref — the new
	// member does not duplicate or displace it.
	c.Assert(f.fileByUUID(c, commodityImage).LinkedEntityType, qt.Equals, "commodity")
}

// memberIndex returns the position of an exact member name in the archive.
func memberIndex(c *qt.C, order []string, name string) int {
	for i, m := range order {
		if m == name {
			return i
		}
	}
	c.Fatalf("member %s not found; members=%v", name, order)
	return -1
}

// memberIndexContaining returns the position of the first member whose name
// contains the needle (used to find a file's byte member by its UUID segment).
func memberIndexContaining(c *qt.C, order []string, needle string) int {
	for i, m := range order {
		if m != "files/_index.json" && bytes.Contains([]byte(m), []byte(needle)) {
			return i
		}
	}
	c.Fatalf("no member containing %s; members=%v", needle, order)
	return -1
}

// TestINBExport_NoFilesMemberWhenNone proves an archive whose only files are
// commodity attachments omits the files member entirely (byte-stability, #2235) —
// so a backup of a group that never used location/area/standalone files is
// unchanged from the 2.0 layout apart from the version string.
func TestINBExport_NoFilesMemberWhenNone(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	f.attachCommodityFile(c, "images", 128)

	_, archive := f.runExport(c, signer)
	order, jsons := innerMembers(c, archive)
	c.Assert(order, qt.Not(qt.Contains), "files/_index.json")

	var manifest map[string]any
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	_, present := manifest["filesFile"]
	c.Assert(present, qt.IsFalse)
}

// TestINBExport_DropsEntityFileWithMissingBlob is the orphan-drop parity guard for
// the new member: a location-linked row whose blob was never written must be
// dropped from the files document and the statistics, exactly like a commodity
// attachment. Without it the archive would declare a member it never carries, and
// EVERY restore of that archive would hard-fail with ErrMissingFileMembers.
func TestINBExport_DropsEntityFileWithMissingBlob(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	present := f.attachEntityFile(c, "location", f.locID, "images", "front-door", ".jpg", "image/jpeg", 256)

	// A location file with a SizeBytes hint but no blob behind it (manual delete,
	// seed fixture) — the exporter must drop it.
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	orphan := must.Must(fileReg.Create(f.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Title:                    "orphan",
		Type:                     models.FileTypeImage,
		Category:                 models.FileCategoryImages,
		Tags:                     models.StringSlice{},
		LinkedEntityType:         "location",
		LinkedEntityID:           f.locID,
		LinkedEntityMeta:         "images",
		File:                     &models.File{Path: "ghost", Ext: ".jpg", MIMEType: "image/jpeg", SizeBytes: 512},
	}))
	orphan.OriginalPath = "t/tenant-a/files/" + orphan.UUID + ".jpg"
	must.Must(fileReg.Update(f.ctx, *orphan))

	blobKey, archive := f.runExport(c, signer)

	_, jsons := innerMembers(c, archive)
	filesDoc := string(jsons["files/_index.json"])
	c.Assert(filesDoc, qt.Contains, present)
	c.Assert(filesDoc, qt.Not(qt.Contains), orphan.UUID)

	var manifest map[string]any
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	stats := manifest["statistics"].(map[string]any)
	c.Assert(stats["fileCount"], qt.Equals, float64(1))

	// The archive is internally consistent, so it still restores cleanly.
	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Assert(final.ErrorCount, qt.Equals, 0)
}

// TestINBExport_SelectedItemsExcludesOutOfScopeEntityFiles pins the scope rule for
// the new member (legacy XML parity): a selected_items export carries the files of
// the locations/areas the user EXPLICITLY selected, excludes the files of every
// location/area it did not select, and excludes standalone files — which have no
// parent that could imply them into the selection.
func TestINBExport_SelectedItemsExcludesOutOfScopeEntityFiles(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	// A second location + area that are NOT selected, each with an attached file.
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	otherLoc := must.Must(locReg.Create(f.ctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Name:                     "Office",
		Address:                  "9 Side St",
	}))
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	otherArea := must.Must(areaReg.Create(f.ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Name:                     "Desk",
		LocationID:               otherLoc.ID,
	}))

	selectedLocFile := f.attachEntityFile(c, "location", f.locID, "files", "lease", ".pdf", "application/pdf", 64)
	selectedAreaFile := f.attachEntityFile(c, "area", f.areaID, "images", "shelf", ".jpg", "image/jpeg", 64)
	otherLocFile := f.attachEntityFile(c, "location", otherLoc.ID, "files", "other-lease", ".pdf", "application/pdf", 64)
	otherAreaFile := f.attachEntityFile(c, "area", otherArea.ID, "images", "other-shelf", ".jpg", "image/jpeg", 64)
	standalone := f.attachStandaloneFile(c, "receipt-scan", ".pdf", "application/pdf", 64)

	srcLoc := must.Must(locReg.Get(f.ctx, f.locID))
	srcArea := must.Must(areaReg.Get(f.ctx, f.areaID))
	_, archive := f.runExportRow(c, signer, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Type:                     models.ExportTypeSelectedItems,
		Status:                   models.ExportStatusPending,
		Description:              "selected backup",
		SelectedItems: models.ValuerSlice[models.ExportSelectedItem]{
			{ID: srcLoc.ID, Type: models.ExportSelectedItemTypeLocation, Name: srcLoc.Name},
			{ID: srcArea.ID, Type: models.ExportSelectedItemTypeArea, Name: srcArea.Name},
		},
	})

	_, jsons := innerMembers(c, archive)
	filesDoc := string(jsons["files/_index.json"])
	c.Assert(filesDoc, qt.Contains, selectedLocFile)
	c.Assert(filesDoc, qt.Contains, selectedAreaFile)
	c.Assert(filesDoc, qt.Not(qt.Contains), otherLocFile)
	c.Assert(filesDoc, qt.Not(qt.Contains), otherAreaFile)
	c.Assert(filesDoc, qt.Not(qt.Contains), standalone)
}

// TestINBExport_SelectedCommodityExcludesImpliedParentFiles is the other half of
// the scope rule: selecting a single COMMODITY forces its parent location and area
// documents to be emitted (the item needs a home), but their attached files must
// NOT ride along. Scoping the entity files on "was the parent emitted" would leak
// the location's lease and the area's floor plan into an archive the user built to
// share one item — and would diverge from the legacy XML exporter, which scopes
// files on the explicitly selected IDs alone.
func TestINBExport_SelectedCommodityExcludesImpliedParentFiles(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	com := must.Must(comReg.List(f.ctx))[0]

	commodityImage := f.attachCommodityFile(c, "images", 256)
	// Files of the PARENTS the selection merely implies — neither is selected.
	f.attachEntityFile(c, "location", f.locID, "files", "lease", ".pdf", "application/pdf", 64)
	f.attachEntityFile(c, "area", f.areaID, "images", "floor-plan", ".jpg", "image/jpeg", 64)

	_, archive := f.runExportRow(c, signer, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Type:                     models.ExportTypeSelectedItems,
		Status:                   models.ExportStatusPending,
		Description:              "one item",
		SelectedItems: models.ValuerSlice[models.ExportSelectedItem]{
			{ID: com.ID, Type: models.ExportSelectedItemTypeCommodity, Name: com.Name},
		},
	})

	order, jsons := innerMembers(c, archive)

	// No entity file is in scope, so the member is omitted entirely.
	c.Assert(order, qt.Not(qt.Contains), "files/_index.json")
	var manifest map[string]any
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	_, present := manifest["filesFile"]
	c.Assert(present, qt.IsFalse)

	// The selected commodity's own attachment still rides along, and the
	// statistics count it alone.
	stats := manifest["statistics"].(map[string]any)
	c.Assert(stats["fileCount"], qt.Equals, float64(1))
	c.Assert(memberIndexContaining(c, order, commodityImage) > 0, qt.IsTrue)
}

// TestINBRestore_EntityFileMissingMemberFails proves the missing-member guard
// covers the new refs too: an archive that DECLARES a location-linked file in the
// files document but never carries its bytes must fail the restore rather than
// silently lose the file.
func TestINBRestore_EntityFileMissingMemberFails(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	loc := must.Must(locReg.Get(f.ctx, f.locID))

	filesDoc := types.INBFilesDoc{Files: []types.INBEntityFileRef{{
		INBFileRef: types.INBFileRef{
			ID:        "ghost-file-uuid",
			Path:      "files/_entity/location/" + loc.UUID + "/files/ghost-file-uuid/ghost.pdf",
			Name:      "ghost",
			Extension: ".pdf",
			MimeType:  "application/pdf",
		},
		LinkedEntityType: "location",
		LinkedEntityID:   loc.UUID,
		LinkedEntityMeta: "files",
	}}}

	archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(`{"version":"2.1"}`))
		writeMember(c, tw, &tar.Header{Name: "location-home.json", Mode: 0o600, Typeflag: tar.TypeReg},
			must.Must(json.Marshal(inbLocationOnlyDoc(loc))))
		writeMember(c, tw, &tar.Header{Name: "files/_index.json", Mode: 0o600, Typeflag: tar.TypeReg},
			must.Must(json.Marshal(filesDoc)))
		// NOTE: the declared byte member is intentionally absent.
	})

	key := "t/tenant-a/restores/missing-entity-member.inb"
	writeArchive(c, f, key, archive)

	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.ErrorIs, processor.ErrMissingFileMembers)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)
}

// TestINBRestore_EntityFileUnmappedLinkIsCounted proves a files-document ref whose
// linked location never landed is DROPPED with a counted error rather than
// persisted with a dangling linked_entity_id. The restore itself still completes.
func TestINBRestore_EntityFileUnmappedLinkIsCounted(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	loc := must.Must(locReg.Get(f.ctx, f.locID))

	const memberPath = "files/_entity/location/no-such-location/files/dangling-uuid/x.pdf"
	filesDoc := types.INBFilesDoc{Files: []types.INBEntityFileRef{{
		INBFileRef: types.INBFileRef{
			ID:        "dangling-uuid",
			Path:      memberPath,
			Name:      "x",
			Extension: ".pdf",
			MimeType:  "application/pdf",
		},
		LinkedEntityType: "location",
		LinkedEntityID:   "no-such-location", // never emitted in this archive
		LinkedEntityMeta: "files",
	}}}

	archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(`{"version":"2.1"}`))
		writeMember(c, tw, &tar.Header{Name: "location-home.json", Mode: 0o600, Typeflag: tar.TypeReg},
			must.Must(json.Marshal(inbLocationOnlyDoc(loc))))
		writeMember(c, tw, &tar.Header{Name: "files/_index.json", Mode: 0o600, Typeflag: tar.TypeReg},
			must.Must(json.Marshal(filesDoc)))
		writeMember(c, tw, &tar.Header{Name: memberPath, Mode: 0o600, Typeflag: tar.TypeReg}, []byte("pdf-bytes"))
	})

	key := "t/tenant-a/restores/dangling-link.inb"
	writeArchive(c, f, key, archive)

	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Assert(final.ErrorCount, qt.Equals, 1)

	// No row was created for the dangling reference.
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	for _, file := range must.Must(fileReg.List(f.ctx)) {
		c.Assert(file.UUID, qt.Not(qt.Equals), "dangling-uuid")
	}
}

// versionGateArchive builds a minimal signed archive (manifest + one location
// document) whose manifest body is exactly manifestJSON, and stores it.
func versionGateArchive(c *qt.C, f *inbFixture, signer *backupsign.Signer, manifestJSON string) string {
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	loc := must.Must(locReg.Get(f.ctx, f.locID))

	archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(manifestJSON))
		writeMember(c, tw, &tar.Header{Name: "location-home.json", Mode: 0o600, Typeflag: tar.TypeReg},
			must.Must(json.Marshal(inbLocationOnlyDoc(loc))))
	})

	key := "t/tenant-a/restores/version-gate.inb"
	writeArchive(c, f, key, archive)
	return key
}

// TestINBRestore_SupportedFormatVersions is the compatibility half of the version
// contract (#2235): a reader accepts every archive whose MAJOR version it knows —
// including a 2.0 archive (no files member, no filesFile field) and an archive
// with no version at all. MINOR bumps are additive-only, so a future 2.x also
// restores here rather than being rejected.
func TestINBRestore_SupportedFormatVersions(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
	}{
		{name: "absent version", manifest: `{}`},
		{name: "2.0 archive without files member", manifest: `{"version":"2.0"}`},
		{name: "current 2.1 archive", manifest: `{"version":"2.1"}`},
		{name: "future minor of a known major", manifest: `{"version":"2.9"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			signer := testSigner(c)
			f := newInbFixture(c)

			key := versionGateArchive(c, f, signer, tt.manifest)

			final, err := restoreInb(c, f, signer, key)
			c.Assert(err, qt.IsNil)
			c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
		})
	}
}

// TestINBRestore_RejectsUnsupportedFormatMajor is the unhappy twin: an archive
// declaring a MAJOR version above what this build knows is rejected outright —
// and, crucially, BEFORE prepareRestore, so a full_replace leaves the existing
// data intact instead of failing on a database it has already wiped.
func TestINBRestore_RejectsUnsupportedFormatMajor(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	key := versionGateArchive(c, f, signer, `{"version":"3.0"}`)

	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.ErrorIs, processor.ErrUnsupportedFormatVersion)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)

	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(must.Must(locReg.List(f.ctx)), qt.HasLen, 1)
}

// TestINBRestore_OldArchiveWithCommodityFileStillRestores is the backward-compat
// contract in full: a 2.0 archive (no files member, no filesFile manifest field)
// restores its commodity attachment on the 2.1 reader exactly as before, with no
// counted errors.
func TestINBRestore_OldArchiveWithCommodityFileStillRestores(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	com := must.Must(comReg.List(f.ctx))[0]
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	area := must.Must(areaReg.Get(f.ctx, f.areaID))
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	loc := must.Must(locReg.Get(f.ctx, f.locID))

	const memberPath = "files/home/legacy/images/legacy-file-uuid/photo.jpg"
	body := []byte("legacy-jpeg-bytes")
	doc := types.INBLocationDoc{
		Location: types.INBLocation{ID: loc.UUID, Name: loc.Name, Address: loc.Address},
		Areas:    []types.INBArea{{ID: area.UUID, Name: area.Name, LocationID: loc.UUID}},
		Commodities: []types.INBCommodity{{
			ID:                    com.UUID,
			Name:                  com.Name,
			ShortName:             com.ShortName,
			Type:                  string(com.Type),
			AreaID:                area.UUID,
			Count:                 com.Count,
			Status:                string(com.Status),
			OriginalPriceCurrency: string(com.OriginalPriceCurrency),
			Draft:                 com.Draft,
			Images: []types.INBFileRef{{
				ID:        "legacy-file-uuid",
				Path:      memberPath,
				Name:      "photo",
				Extension: ".jpg",
				MimeType:  "image/jpeg",
			}},
		}},
	}

	archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(`{"version":"2.0"}`))
		writeMember(c, tw, &tar.Header{Name: "location-home.json", Mode: 0o600, Typeflag: tar.TypeReg}, must.Must(json.Marshal(doc)))
		writeMember(c, tw, &tar.Header{Name: memberPath, Mode: 0o600, Typeflag: tar.TypeReg}, body)
	})

	key := "t/tenant-a/restores/legacy-2-0.inb"
	writeArchive(c, f, key, archive)

	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	restored := f.fileByUUID(c, "legacy-file-uuid")
	c.Assert(restored.LinkedEntityType, qt.Equals, "commodity")
	c.Assert(restored.LinkedEntityMeta, qt.Equals, "images")
	c.Assert(restored.Category, qt.Equals, models.FileCategoryImages)

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()
	c.Assert(must.Must(bucket.ReadAll(f.ctx, "t/tenant-a/files/legacy-file-uuid.jpg")), qt.DeepEquals, body)
}

// TestINBRestore_FullReplaceReplacesAreaLessCommodity is the #2236 end-to-end
// guard. A pre-existing area-less commodity (#1986) is not reachable through the
// location → area recursion clearExistingData used to rely on, so it survived the
// wipe; full_replace then re-created the archive's copy with the SAME preserved
// UUID, colliding on the unique index — a counted error that dropped the whole
// item AND cascaded "references unmapped commodity" onto each of its files, while
// the restore still reported Completed. After the fix the survivor is swept, so the
// restore is clean and the item lands exactly once.
func TestINBRestore_FullReplaceReplacesAreaLessCommodity(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	looseUUID := f.addUnassignedCommodity(c, "Loose Gadget")
	blobKey, _ := f.runExport(c, signer)

	// The archive already contains the area-less commodity, and the target DB
	// still holds the original row with the same UUID — exactly the collision.
	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	var matches int
	for _, com := range must.Must(comReg.List(f.ctx)) {
		if com.UUID == looseUUID {
			matches++
		}
	}
	c.Assert(matches, qt.Equals, 1)
}

// TestINBRestore_EntityFilesUnderMergeStrategies pins the merge matrix for the new
// files: an archive restored over rows that already carry the same UUIDs must land
// each file exactly once, still linked to its location (or still standalone) — and
// with no counted errors, since merge seeds the ID mapping from the existing rows.
func TestINBRestore_EntityFilesUnderMergeStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy types.RestoreStrategy
	}{
		{name: "merge_add skips the existing file", strategy: types.RestoreStrategyMergeAdd},
		{name: "merge_update re-points the existing file", strategy: types.RestoreStrategyMergeUpdate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			signer := testSigner(c)
			f := newInbFixture(c)

			locFile := f.attachEntityFile(c, "location", f.locID, "images", "front-door", ".jpg", "image/jpeg", 256)
			standalone := f.attachStandaloneFile(c, "receipt-scan", ".pdf", "application/pdf", 128)

			blobKey, _ := f.runExport(c, signer)

			final, err := restoreInbWithOptions(c, f, signer, blobKey, models.RestoreOptions{
				Strategy: string(tt.strategy),
			})
			c.Assert(err, qt.IsNil)
			c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
			c.Assert(final.ErrorCount, qt.Equals, 0)

			c.Assert(f.fileByUUID(c, locFile).LinkedEntityType, qt.Equals, "location")
			c.Assert(f.fileByUUID(c, standalone).LinkedEntityType, qt.Equals, "")
			c.Assert(f.countFilesWithUUID(c, locFile), qt.Equals, 1)
			c.Assert(f.countFilesWithUUID(c, standalone), qt.Equals, 1)
		})
	}
}

// TestINBRestore_DryRunDoesNotPersistEntityFiles: a dry-run consumes the files
// member and its byte members (so the missing-member check stays satisfied) but
// creates no row. Note it DOES count "references unmapped location" errors, which
// is the pre-existing dry-run behaviour for every linked file — nothing is
// persisted, so there is no ID mapping to resolve a link against.
func TestINBRestore_DryRunDoesNotPersistEntityFiles(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	f.attachEntityFile(c, "location", f.locID, "images", "front-door", ".jpg", "image/jpeg", 256)
	f.attachStandaloneFile(c, "receipt-scan", ".pdf", "application/pdf", 128)

	blobKey, _ := f.runExport(c, signer)

	// Snapshot AFTER the export — it creates the archive's own artifact row.
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	before := len(must.Must(fileReg.List(f.ctx)))

	final, err := restoreInbWithOptions(c, f, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyFullReplace),
		DryRun:   true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)

	// No new file rows.
	c.Assert(must.Must(fileReg.List(f.ctx)), qt.HasLen, before)
}

// countFilesWithUUID counts the file rows carrying a given immutable UUID — 1 for
// a correct restore, >1 would mean the row was duplicated instead of matched.
func (f *inbFixture) countFilesWithUUID(c *qt.C, uuid string) int {
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	var n int
	for _, file := range must.Must(fileReg.List(f.ctx)) {
		if file != nil && file.UUID == uuid {
			n++
		}
	}
	return n
}

// inbLocationOnlyDoc builds a location document with no areas/commodities, for
// hand-built archives that only need the location to exist so an entity file can
// link to it.
func inbLocationOnlyDoc(loc *models.Location) types.INBLocationDoc {
	return types.INBLocationDoc{
		Location: types.INBLocation{ID: loc.UUID, Name: loc.Name, Address: loc.Address},
	}
}

// writeArchive stores a hand-built .inb archive in the fixture's bucket.
func writeArchive(c *qt.C, f *inbFixture, key string, archive []byte) {
	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()
	c.Assert(bucket.WriteAll(f.ctx, key, archive, nil), qt.IsNil)
}
