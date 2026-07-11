//go:build !legacy_xml_backup

package backup_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/memblob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/export"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/backupsign"
	_ "github.com/denisvmedia/inventario/internal/fileblob"
	"github.com/denisvmedia/inventario/internal/inb"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// inbUploadLocation uses the in-memory fileblob backend (memfs), which — unlike
// memblob — shares state across blob.OpenBucket calls keyed by the same path, so
// the export writer and the restore/test readers all see the same bucket.
const inbUploadLocation = "file://inb-test?memfs=1&create_dir=1"

func testSigner(c *qt.C) *backupsign.Signer {
	seed := make([]byte, backupsign.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 7)
	}
	s, err := backupsign.NewSigner(seed)
	c.Assert(err, qt.IsNil)
	return s
}

// inbFixture builds an in-memory factory seeded with a tenant/user/group and a
// location → area → commodity, returning the factory plus a user/group context.
type inbFixture struct {
	fs     *registry.FactorySet
	ctx    context.Context
	user   *models.User
	group  *models.LocationGroup
	locID  string
	areaID string
}

func newInbFixture(c *qt.C) *inbFixture {
	fs := memory.NewFactorySet()
	ctx := context.Background()

	user := must.Must(fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Email:               "a@example.com",
		Name:                "User A",
		IsActive:            true,
	}))
	must.Must(fs.TenantRegistry.Create(ctx, models.Tenant{
		EntityID: models.EntityID{ID: "tenant-a"},
		Name:     "Tenant A",
	}))
	group := must.Must(fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Group A",
		Status:              models.LocationGroupStatusActive,
		GroupCurrency:       "USD",
		CreatedBy:           user.ID,
	}))

	uctx := appctx.WithGroup(appctx.WithUser(ctx, user), group)

	locReg := must.Must(fs.LocationRegistryFactory.CreateUserRegistry(uctx))
	loc := must.Must(locReg.Create(uctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: group.ID, CreatedByUserID: user.ID},
		Name:                     "Home",
		Address:                  "1 Main St",
	}))
	areaReg := must.Must(fs.AreaRegistryFactory.CreateUserRegistry(uctx))
	area := must.Must(areaReg.Create(uctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: group.ID, CreatedByUserID: user.ID},
		Name:                     "Living Room",
		LocationID:               loc.ID,
	}))
	comReg := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(uctx))
	must.Must(comReg.Create(uctx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: group.ID, CreatedByUserID: user.ID},
		Name:                     "TV",
		ShortName:                "tv",
		Type:                     models.CommodityTypeElectronics,
		AreaID:                   new(area.ID),
		Count:                    1,
		Status:                   models.CommodityStatusInUse,
		OriginalPriceCurrency:    "USD",
		// Draft commodities skip the purchase-date / converted-price
		// conditional validations, keeping the fixture minimal while still
		// exercising the full export → restore round-trip.
		Draft: true,
	}))

	return &inbFixture{fs: fs, ctx: uctx, user: user, group: group, locID: loc.ID, areaID: area.ID}
}

// attachCommodityFile creates an image FileEntity linked to the fixture's first
// commodity (in the given bucket: images/invoices/manuals) and writes a blob of
// the given size to its tenant-namespaced key. Returns the file's UUID.
func (f *inbFixture) attachCommodityFile(c *qt.C, bucketMeta string, size int) string {
	return f.attachCommodityFileFull(c, bucketMeta, "photo", "photo", ".jpg", "image/jpeg", size)
}

// attachCommodityFileFull is the parameterized variant: it lets a test set the
// file's user-facing basename (path) distinct from its display title, plus the
// extension/MIME, so a round-trip can prove the filename and size survive.
func (f *inbFixture) attachCommodityFileFull(c *qt.C, bucketMeta, filePath, title, ext, mime string, size int) string {
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	commodities := must.Must(comReg.List(f.ctx))
	c.Assert(len(commodities) >= 1, qt.IsTrue)
	com := commodities[0]

	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	fileRow := models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Title:                    title,
		Type:                     models.FileTypeFromMIME(mime),
		Category:                 models.FileCategoryFromContext("commodity", bucketMeta, mime),
		Tags:                     models.StringSlice{},
		LinkedEntityType:         "commodity",
		LinkedEntityID:           com.ID,
		LinkedEntityMeta:         bucketMeta,
		File: &models.File{
			Path:      filePath,
			Ext:       ext,
			MIMEType:  mime,
			SizeBytes: int64(size),
		},
	}
	created := must.Must(fileReg.Create(f.ctx, fileRow))

	// Write the blob under the source tenant's namespace so the exporter can
	// stream it. The OriginalPath must match the key we write.
	blobKey := "t/tenant-a/files/" + created.UUID + ext
	created.OriginalPath = blobKey
	must.Must(fileReg.Update(f.ctx, *created))

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()
	body := bytes.Repeat([]byte("inventario-blob-chunk"), size/21+1)[:size]
	c.Assert(bucket.WriteAll(f.ctx, blobKey, body, nil), qt.IsNil)

	return created.UUID
}

// attachCommodityFileMissingBlob creates an image FileEntity linked to the
// fixture's first commodity that carries a SizeBytes hint (exactly as a real
// upload records) but deliberately writes NO blob. This models an orphan row:
// a manually-deleted blob, or seed fixtures that record file metadata without
// uploading the bytes. Returns the file's UUID. The exporter must DROP such a
// row rather than abort the whole backup when its member cannot be opened.
func (f *inbFixture) attachCommodityFileMissingBlob(c *qt.C, bucketMeta string, size int) string {
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	commodities := must.Must(comReg.List(f.ctx))
	c.Assert(len(commodities) >= 1, qt.IsTrue)
	com := commodities[0]

	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	fileRow := models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Title:                    "orphan",
		Type:                     models.FileTypeFromMIME("image/jpeg"),
		Category:                 models.FileCategoryFromContext("commodity", bucketMeta, "image/jpeg"),
		Tags:                     models.StringSlice{},
		LinkedEntityType:         "commodity",
		LinkedEntityID:           com.ID,
		LinkedEntityMeta:         bucketMeta,
		File: &models.File{
			Path:      "orphan",
			Ext:       ".jpg",
			MIMEType:  "image/jpeg",
			SizeBytes: int64(size), // the size hint is present…
		},
	}
	created := must.Must(fileReg.Create(f.ctx, fileRow))
	// …and the key is recorded, but no bucket.WriteAll happens — the blob the
	// key points at never lands, reproducing the seed/orphan condition.
	created.OriginalPath = "t/tenant-a/files/" + created.UUID + ".jpg"
	must.Must(fileReg.Update(f.ctx, *created))
	return created.UUID
}

// fileByUUID returns the (restored) FileEntity with the given immutable UUID, or
// fails the test if it is absent.
func (f *inbFixture) fileByUUID(c *qt.C, uuid string) *models.FileEntity {
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	files := must.Must(fileReg.List(f.ctx))
	for _, file := range files {
		if file != nil && file.UUID == uuid {
			return file
		}
	}
	c.Fatalf("file with UUID %s not found after restore", uuid)
	return nil
}

// runExport drives a full_database .inb export and returns the blob key + raw
// archive bytes.
func (f *inbFixture) runExport(c *qt.C, signer *backupsign.Signer) (string, []byte) {
	return f.runExportRow(c, signer, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusPending,
		Description:              "test backup",
	})
}

// runExportRow drives the .inb export for a caller-built export row (so a test
// can exercise a selected_items export) and returns the blob key + archive
// bytes.
func (f *inbFixture) runExportRow(c *qt.C, signer *backupsign.Signer, exportRow models.Export) (string, []byte) {
	svc := export.NewExportService(f.fs, inbUploadLocation, signer)

	expReg := must.Must(f.fs.ExportRegistryFactory.CreateUserRegistry(f.ctx))
	created := must.Must(expReg.Create(f.ctx, exportRow))

	err := svc.ProcessExport(f.ctx, created.ID)
	c.Assert(err, qt.IsNil)

	updated := must.Must(expReg.Get(f.ctx, created.ID))
	c.Assert(updated.Status, qt.Equals, models.ExportStatusCompleted)
	c.Assert(updated.FilePath, qt.Not(qt.Equals), "")

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()
	data := must.Must(bucket.ReadAll(f.ctx, updated.FilePath))
	return updated.FilePath, data
}

// innerMembers inflates an .inb archive's payload and returns the ordered list
// of inner-tar member names plus a name→JSON-bytes map of every .json member.
func innerMembers(c *qt.C, archive []byte) ([]string, map[string][]byte) {
	tr := tar.NewReader(bytes.NewReader(archive))
	_ = must.Must(tr.Next()) // signature member
	_ = must.Must(io.ReadAll(tr))
	_ = must.Must(tr.Next()) // payload member
	payload := must.Must(io.ReadAll(tr))

	gzr := must.Must(gzip.NewReader(bytes.NewReader(payload)))
	itr := tar.NewReader(gzr)
	var order []string
	jsons := map[string][]byte{}
	for {
		h, err := itr.Next()
		if err == io.EOF {
			break
		}
		c.Assert(err, qt.IsNil)
		order = append(order, h.Name)
		if strings.HasSuffix(h.Name, ".json") {
			jsons[h.Name] = must.Must(io.ReadAll(io.LimitReader(itr, 64<<20)))
		}
	}
	return order, jsons
}

func TestINBExport_ContainerStructure(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	_, archive := f.runExport(c, signer)

	// Outer tar: signature member first, then payload.
	tr := tar.NewReader(bytes.NewReader(archive))
	h1 := must.Must(tr.Next())
	c.Assert(h1.Name, qt.Equals, inb.SignatureName)
	sig := must.Must(io.ReadAll(tr))

	h2 := must.Must(tr.Next())
	c.Assert(h2.Name, qt.Equals, inb.PayloadName)
	payload := must.Must(io.ReadAll(tr))

	// Verify the signature over the streaming digest of the payload.
	digest := backupsign.NewDigest()
	_, _ = digest.Write(payload)
	c.Assert(signer.VerifyDigest(digest.Sum(nil), sig), qt.IsNil)

	// Inner gzip(tar) must contain manifest.json + a per-location JSON member.
	gzr := must.Must(gzip.NewReader(bytes.NewReader(payload)))
	itr := tar.NewReader(gzr)
	members := map[string]bool{}
	var manifestData []byte
	for {
		h, err := itr.Next()
		if err == io.EOF {
			break
		}
		c.Assert(err, qt.IsNil)
		members[h.Name] = true
		if h.Name == "manifest.json" {
			manifestData = must.Must(io.ReadAll(itr))
		}
	}
	c.Assert(members["manifest.json"], qt.IsTrue)

	hasLocationDoc := false
	for name := range members {
		if name != "manifest.json" {
			hasLocationDoc = true
		}
	}
	c.Assert(hasLocationDoc, qt.IsTrue, qt.Commentf("expected a per-location JSON member"))

	// Manifest records the format, signature info, and stats.
	var manifest map[string]any
	c.Assert(json.Unmarshal(manifestData, &manifest), qt.IsNil)
	c.Assert(manifest["version"], qt.Equals, "2.1")
	c.Assert(manifest["format"], qt.Equals, "json")
	sigBlock := manifest["signature"].(map[string]any)
	c.Assert(sigBlock["algorithm"], qt.Equals, backupsign.Algorithm)
	c.Assert(sigBlock["fingerprint"], qt.Equals, signer.Fingerprint())
}

func TestINBExport_GzipLevel3(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	_, archive := f.runExport(c, signer)

	// The payload member must be a valid gzip stream (level is internal but the
	// stream must inflate). We assert it inflates without error.
	tr := tar.NewReader(bytes.NewReader(archive))
	_ = must.Must(tr.Next()) // sig
	_ = must.Must(io.ReadAll(tr))
	_ = must.Must(tr.Next()) // payload
	payload := must.Must(io.ReadAll(tr))
	gzr, err := gzip.NewReader(bytes.NewReader(payload))
	c.Assert(err, qt.IsNil)
	// Bound the inflate so the decompression-bomb linter is satisfied; the test
	// fixture is tiny so 64 MiB is plenty.
	_, err = io.Copy(io.Discard, io.LimitReader(gzr, 64<<20))
	c.Assert(err, qt.IsNil)
}

// TestINBExport_DropsFileWithMissingBlob is the regression for the export
// aborting when a file row carries a SizeBytes hint but its blob was never
// written. Orphan rows like this are normal (manual blob deletes; seed fixtures
// that record file metadata without uploading bytes — the exact condition that
// turned every e2e docker lane red on #534). The export must COMPLETE and simply
// omit the orphan; it must never fail the whole backup over one missing member.
func TestINBExport_DropsFileWithMissingBlob(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	present := f.attachCommodityFile(c, "images", 256)            // blob exists
	missing := f.attachCommodityFileMissingBlob(c, "images", 512) // hint, no blob

	// runExport asserts Status==Completed internally, so reaching this line at
	// all proves the missing blob no longer aborts the export.
	_, archive := f.runExport(c, signer)

	order, jsons := innerMembers(c, archive)
	members := strings.Join(order, "\n")
	c.Assert(members, qt.Contains, present,
		qt.Commentf("present file's member must be archived; members=%v", order))
	c.Assert(members, qt.Not(qt.Contains), missing,
		qt.Commentf("missing-blob file's member must be dropped; members=%v", order))

	var locDocs strings.Builder
	for name, body := range jsons {
		if name != "manifest.json" {
			locDocs.Write(body)
		}
	}
	locDocsStr := locDocs.String()
	c.Assert(locDocsStr, qt.Contains, present,
		qt.Commentf("present file must be referenced in the location doc"))
	c.Assert(locDocsStr, qt.Not(qt.Contains), missing,
		qt.Commentf("orphan must not be referenced in the location doc"))

	// The manifest's image count reflects only the retained file (1, not 2).
	var manifest map[string]any
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	stats := manifest["statistics"].(map[string]any)
	c.Assert(stats["imageCount"], qt.Equals, float64(1),
		qt.Commentf("only the file with a real blob should be counted"))
}

func TestINBRestore_RejectsBadSignature(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	blobKey, archive := f.runExport(c, signer)

	// Flip a payload byte to invalidate the signature.
	tampered := tamperPayload(c, archive)

	// Write the tampered archive back to the bucket and run a restore.
	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	c.Assert(bucket.WriteAll(f.ctx, blobKey, tampered, nil), qt.IsNil)
	bucket.Close()

	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNotNil)
	// markRestoreFailed flattens the error to a string, but the signature
	// failure text (and the inb sentinel's message) survives in the chain and
	// the restore operation is marked failed.
	c.Assert(err.Error(), qt.Contains, "signature verification failed")
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)
}

func TestINBRestore_RejectsLegacyXML(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	// Feed a legacy XML blob (not a .inb container) to the restore.
	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	xmlKey := "t/tenant-a/restores/legacy.xml"
	c.Assert(bucket.WriteAll(f.ctx, xmlKey, []byte(`<?xml version="1.0"?><inventory></inventory>`), nil), qt.IsNil)
	bucket.Close()

	final, err := restoreInb(c, f, signer, xmlKey)
	c.Assert(err, qt.IsNotNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)
}

func TestINBRoundTrip_MultiMBFileReKeyed(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	// Attach a multi-MB image file to the fixture's commodity and write its
	// blob into the bucket so the exporter streams it into the archive.
	const blobSize = 3 << 20 // 3 MiB — large enough that buffering it whole would be obvious
	fileID := f.attachCommodityFile(c, "images", blobSize)

	blobKey, _ := f.runExport(c, signer)

	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Assert(final.ImageCount >= 1, qt.IsTrue)

	// The restored file blob must live under the importing tenant's namespace,
	// re-keyed to the importer's BuildFileBlobKey — NOT the archive path or the
	// source key.
	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	defer bucket.Close()
	expectedKey := "t/tenant-a/files/" + fileID + ".jpg"
	exists := must.Must(bucket.Exists(f.ctx, expectedKey))
	c.Assert(exists, qt.IsTrue, qt.Commentf("restored blob must be re-keyed to %s", expectedKey))
}

func TestINBRoundTrip_FullReplace(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	src := newInbFixture(c)
	blobKey, _ := src.runExport(c, signer)

	// Restore into the SAME factory using full_replace (idempotent re-create).
	final, err := restoreInb(c, src, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Assert(final.LocationCount >= 1, qt.IsTrue)
	c.Assert(final.AreaCount >= 1, qt.IsTrue)
	c.Assert(final.CommodityCount >= 1, qt.IsTrue, qt.Commentf("errors: %v", final.ErrorMessage))
}

// addUnassignedCommodity creates an area-less commodity (issue #1986) in the
// fixture's group and returns its immutable UUID.
func (f *inbFixture) addUnassignedCommodity(c *qt.C, name string) string {
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	com := must.Must(comReg.Create(f.ctx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Name:                     name,
		ShortName:                "ua",
		Type:                     models.CommodityTypeElectronics,
		// AreaID left nil — this is the unassigned commodity under test.
		Count:                 1,
		Status:                models.CommodityStatusInUse,
		OriginalPriceCurrency: "USD",
		Draft:                 true,
	}))
	c.Assert(com.AreaID, qt.IsNil)
	return com.UUID
}

// commodityByUUID lists commodities and returns the one with the given immutable
// UUID (restored rows get fresh DB IDs but keep their UUID).
func (f *inbFixture) commodityByUUID(c *qt.C, uuid string) *models.Commodity {
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	commodities := must.Must(comReg.List(f.ctx))
	for _, com := range commodities {
		if com != nil && com.UUID == uuid {
			return com
		}
	}
	c.Fatalf("commodity with UUID %s not found after restore", uuid)
	return nil
}

// TestINBRoundTrip_UnassignedCommodity is the issue #1986 fidelity test: an
// area-less commodity must export into its own member and restore with a nil
// area — without fabricating any synthetic location/area in the restored data.
func TestINBRoundTrip_UnassignedCommodity(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	looseUUID := f.addUnassignedCommodity(c, "Loose Gadget")

	blobKey, archive := f.runExport(c, signer)

	// The archive carries the dedicated unassigned member, and the manifest
	// records it.
	order, jsons := innerMembers(c, archive)
	c.Assert(order, qt.Contains, "unassigned-commodities.json")
	var manifest map[string]any
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	c.Assert(manifest["unassignedFile"], qt.Equals, "unassigned-commodities.json")

	// Count locations/areas BEFORE restore so we can prove restore adds none for
	// the unassigned commodity.
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	locsBefore := len(must.Must(locReg.List(f.ctx)))
	areasBefore := len(must.Must(areaReg.List(f.ctx)))

	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	// The unassigned commodity round-trips with a nil area. Restored rows keep
	// their immutable UUID but get fresh DB IDs, so look it up by UUID.
	restored := f.commodityByUUID(c, looseUUID)
	c.Assert(restored.AreaID, qt.IsNil)

	// full_replace re-creates the same rows; no NEW synthetic location/area was
	// fabricated to home the unassigned commodity.
	c.Assert(must.Must(locReg.List(f.ctx)), qt.HasLen, locsBefore)
	c.Assert(must.Must(areaReg.List(f.ctx)), qt.HasLen, areasBefore)
}

// TestINBExport_NoUnassignedMemberWhenNone proves an archive with zero area-less
// commodities omits the unassigned member entirely (byte-stability, issue #1986).
func TestINBExport_NoUnassignedMemberWhenNone(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c) // fixture's only commodity is area-bound

	_, archive := f.runExport(c, signer)
	order, jsons := innerMembers(c, archive)
	c.Assert(order, qt.Not(qt.Contains), "unassigned-commodities.json")
	var manifest map[string]any
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	_, present := manifest["unassignedFile"]
	c.Assert(present, qt.IsFalse)
}

// seedSecondTenant creates a second tenant (tenant-b) with its own user, group,
// and location in the SAME factory set as f, and returns tenant-b's RLS-scoped
// context plus the location ID. Used to prove a full_replace restore run as
// tenant A never touches another tenant's rows.
func seedSecondTenant(c *qt.C, f *inbFixture) (context.Context, string) {
	ctx := context.Background()
	user := must.Must(f.fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-b"},
		Email:               "b@example.com",
		Name:                "User B",
		IsActive:            true,
	}))
	must.Must(f.fs.TenantRegistry.Create(ctx, models.Tenant{
		EntityID: models.EntityID{ID: "tenant-b"},
		Name:     "Tenant B",
	}))
	group := must.Must(f.fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-b"},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Group B",
		Status:              models.LocationGroupStatusActive,
		GroupCurrency:       "USD",
		CreatedBy:           user.ID,
	}))

	bctx := appctx.WithGroup(appctx.WithUser(ctx, user), group)
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(bctx))
	loc := must.Must(locReg.Create(bctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-b", GroupID: group.ID, CreatedByUserID: user.ID},
		Name:                     "Tenant B Home",
		Address:                  "2 Other St",
	}))
	return bctx, loc.ID
}

// TestINBRestore_FullReplaceDoesNotWipeOtherTenant is the regression guard for
// the cross-tenant data-wipe bug (#534 review): clearExistingData under a
// full_replace restore MUST be RLS-scoped to the restoring user's tenant. A
// service registry would enumerate and recursively delete EVERY tenant's
// locations. Here tenant A runs a full_replace restore; tenant B's location
// must survive untouched.
func TestINBRestore_FullReplaceDoesNotWipeOtherTenant(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)

	src := newInbFixture(c)
	bctx, tenantBLocID := seedSecondTenant(c, src)

	blobKey, _ := src.runExport(c, signer)

	// Run the full_replace restore as tenant A (restoreInb uses src.ctx).
	final, err := restoreInb(c, src, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)

	// Tenant B's location must still exist — it was never in tenant A's scope.
	bLocReg := must.Must(src.fs.LocationRegistryFactory.CreateUserRegistry(bctx))
	bLoc, getErr := bLocReg.Get(bctx, tenantBLocID)
	c.Assert(getErr, qt.IsNil, qt.Commentf("tenant B location must survive a tenant A full_replace restore"))
	c.Assert(bLoc.Name, qt.Equals, "Tenant B Home")
}

// TestINBRoundTrip_FilePathAndSizePreserved guards the #534 review fixes: a
// restored commodity file must keep its original user-facing filename (not the
// display Title or the UUID) and its byte size (so per-group storage accounting
// isn't under-counted). The fixture deliberately uses a Path that differs from
// the Title so a regression collapsing one onto the other is caught.
func TestINBRoundTrip_FilePathAndSizePreserved(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	const size = 4096
	fileUUID := f.attachCommodityFileFull(c, "invoices", "invoice-2024-01", "January invoice", ".pdf", "application/pdf", size)

	blobKey, _ := f.runExport(c, signer)
	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)

	restored := f.fileByUUID(c, fileUUID)
	c.Assert(restored.File, qt.IsNotNil)
	c.Assert(restored.File.Path, qt.Equals, "invoice-2024-01") // original filename, not Title/UUID
	c.Assert(restored.Title, qt.Equals, "January invoice")
	c.Assert(restored.File.SizeBytes, qt.Equals, int64(size))
}

// restoreInb writes a RestoreOperation + export row pointing at blobKey and runs
// the restore service against it, returning the restore operation's final state.
func restoreInb(c *qt.C, f *inbFixture, signer *backupsign.Signer, blobKey string) (*models.RestoreOperation, error) {
	return restoreInbWithOptions(c, f, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyFullReplace),
	})
}

// restoreInbWithOptions is restoreInb with caller-supplied RestoreOptions, so a
// test can drive a dry-run (or other strategy) restore.
func restoreInbWithOptions(c *qt.C, f *inbFixture, signer *backupsign.Signer, blobKey string, opts models.RestoreOptions) (*models.RestoreOperation, error) {
	entityService := services.NewEntityService(f.fs, inbUploadLocation)
	svc := restore.NewRestoreService(f.fs, entityService, inbUploadLocation, signer)

	expReg := must.Must(f.fs.ExportRegistryFactory.CreateUserRegistry(f.ctx))
	exportRow := must.Must(expReg.Create(f.ctx, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusCompleted,
		Description:              "restore source",
		FilePath:                 blobKey,
	}))

	roReg := f.fs.RestoreOperationRegistryFactory.CreateServiceRegistry()
	restoreOp := must.Must(roReg.Create(f.ctx, models.RestoreOperation{
		ExportID: exportRow.ID,
		Status:   models.RestoreStatusPending,
		Options:  opts,
	}))

	err := svc.ProcessRestoreOperation(f.ctx, restoreOp.ID, inbUploadLocation)
	final := must.Must(roReg.Get(f.ctx, restoreOp.ID))
	return final, err
}

// TestINBRestore_DryRunWithFilesDoesNotErrorOnMembers guards the #534 review fix
// for missing-file-member detection: a dry-run restore never persists
// commodities, so file members can't link — but they ARE still delivered and
// consumed. The missing-member check must only flag refs whose member never
// appeared, so a dry-run of an archive that DOES carry every file member must
// still complete (not fail with ErrMissingFileMembers).
func TestINBRestore_DryRunWithFilesDoesNotErrorOnMembers(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	f.attachCommodityFile(c, "images", 2048)
	blobKey, _ := f.runExport(c, signer)

	final, err := restoreInbWithOptions(c, f, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyFullReplace),
		DryRun:   true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
}

// TestINBRestore_MissingFileMemberFails proves the positive case: an archive
// that DECLARES a commodity file reference in its location document but never
// carries the corresponding file member must fail the restore rather than
// silently lose the file. The inner tar is hand-built to omit the file body.
func TestINBRestore_MissingFileMemberFails(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	com := must.Must(comReg.List(f.ctx))[0]
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	area := must.Must(areaReg.List(f.ctx))[0]
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	loc := must.Must(locReg.Get(f.ctx, f.locID))

	// A location doc that references an image file member which we deliberately
	// do NOT write into the tar.
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
				ID:   "missing-file-uuid",
				Path: "files/home/" + com.UUID + "/images/ghost.jpg",
				Name: "ghost",
			}},
		}},
	}
	docJSON := must.Must(json.Marshal(doc))

	archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(`{"version":"2.0"}`))
		writeMember(c, tw, &tar.Header{Name: "location-home.json", Mode: 0o600, Typeflag: tar.TypeReg}, docJSON)
		// NOTE: the declared image member is intentionally absent.
	})

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	key := "t/tenant-a/restores/missing-member.inb"
	c.Assert(bucket.WriteAll(f.ctx, key, archive, nil), qt.IsNil)
	bucket.Close()

	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.IsNotNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)
}

// inbDocForFixture builds an INBLocationDoc for the fixture's location/area/
// commodity, letting the caller mutate the single commodity (e.g. to inject a
// malformed price) and optionally attach a file ref before it is serialized.
func inbDocForFixture(c *qt.C, f *inbFixture, mutate func(com *types.INBCommodity)) types.INBLocationDoc {
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	com := must.Must(comReg.List(f.ctx))[0]
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	area := must.Must(areaReg.List(f.ctx))[0]
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	loc := must.Must(locReg.Get(f.ctx, f.locID))

	inbCom := types.INBCommodity{
		ID:                    com.UUID,
		Name:                  com.Name,
		ShortName:             com.ShortName,
		Type:                  string(com.Type),
		AreaID:                area.UUID,
		Count:                 com.Count,
		Status:                string(com.Status),
		OriginalPriceCurrency: string(com.OriginalPriceCurrency),
		Draft:                 com.Draft,
	}
	if mutate != nil {
		mutate(&inbCom)
	}
	return types.INBLocationDoc{
		Location:    types.INBLocation{ID: loc.UUID, Name: loc.Name, Address: loc.Address},
		Areas:       []types.INBArea{{ID: area.UUID, Name: area.Name, LocationID: loc.UUID}},
		Commodities: []types.INBCommodity{inbCom},
	}
}

// signedArchiveFromDoc builds a fully-signed .inb archive containing a manifest
// plus a single hand-built location document.
func signedArchiveFromDoc(c *qt.C, signer *backupsign.Signer, doc types.INBLocationDoc) []byte {
	docJSON := must.Must(json.Marshal(doc))
	return signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(`{"version":"2.0"}`))
		writeMember(c, tw, &tar.Header{Name: "location-home.json", Mode: 0o600, Typeflag: tar.TypeReg}, docJSON)
	})
}

// TestINBRestore_MalformedPriceFails guards the Item-B fix: a commodity whose
// originalPrice is non-empty but unparseable must FAIL the restore with a clear
// error rather than being silently coerced to zero.
func TestINBRestore_MalformedPriceFails(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	doc := inbDocForFixture(c, f, func(com *types.INBCommodity) {
		com.OriginalPrice = "not-a-number"
	})
	archive := signedArchiveFromDoc(c, signer, doc)

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	key := "t/tenant-a/restores/bad-price.inb"
	c.Assert(bucket.WriteAll(f.ctx, key, archive, nil), qt.IsNil)
	bucket.Close()

	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.IsNotNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)
	c.Assert(final.ErrorMessage, qt.Contains, "originalPrice")
}

// TestINBRestore_MalformedTimestampFails guards the Item-B fix for file
// metadata: a file reference whose createdAt is non-empty but unparseable must
// FAIL the restore rather than being coerced to time.Now(). The file member IS
// delivered so the failure is the conversion, not a missing member.
func TestINBRestore_MalformedTimestampFails(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	com := must.Must(comReg.List(f.ctx))[0]
	filePath := "files/home/" + com.UUID + "/images/photo.jpg"

	doc := inbDocForFixture(c, f, func(inbCom *types.INBCommodity) {
		inbCom.Images = []types.INBFileRef{{
			ID:        "file-bad-ts",
			Path:      filePath,
			Name:      "photo",
			Extension: ".jpg",
			MimeType:  "image/jpeg",
			CreatedAt: "definitely-not-a-timestamp",
		}}
	})

	docJSON := must.Must(json.Marshal(doc))
	archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(`{"version":"2.0"}`))
		writeMember(c, tw, &tar.Header{Name: "location-home.json", Mode: 0o600, Typeflag: tar.TypeReg}, docJSON)
		writeMember(c, tw, &tar.Header{Name: filePath, Mode: 0o600, Typeflag: tar.TypeReg}, []byte("jpeg-bytes"))
	})

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	key := "t/tenant-a/restores/bad-timestamp.inb"
	c.Assert(bucket.WriteAll(f.ctx, key, archive, nil), qt.IsNil)
	bucket.Close()

	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.IsNotNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)
}

// TestINBRestore_ValidPriceAndTimestampRoundTrip is the Item-B happy-path twin:
// a commodity with a well-formed price and a file with a valid RFC3339 timestamp
// must still restore cleanly, so the stricter parsing did not break valid data.
func TestINBRestore_ValidPriceAndTimestampRoundTrip(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	f.attachCommodityFile(c, "images", 1024)

	// Give the commodity a real, parseable price so the strict parser exercises
	// the success path.
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	com := must.Must(comReg.List(f.ctx))[0]
	com.OriginalPrice = decimal.RequireFromString("199.99")
	must.Must(comReg.Update(f.ctx, *com))

	blobKey, _ := f.runExport(c, signer)
	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Assert(final.CommodityCount >= 1, qt.IsTrue)
	c.Assert(final.ImageCount >= 1, qt.IsTrue)
}

// signedArchiveFromInnerTar builds a fully-signed .inb archive whose inner tar
// is produced by buildInner. Used to craft hostile-but-correctly-signed
// archives so the restore guards (not the signature check) are what reject them.
func signedArchiveFromInnerTar(c *qt.C, signer *backupsign.Signer, buildInner func(tw *tar.Writer)) []byte {
	var payloadBuf bytes.Buffer
	gzw := gzip.NewWriter(&payloadBuf)
	tw := tar.NewWriter(gzw)
	buildInner(tw)
	c.Assert(tw.Close(), qt.IsNil)
	c.Assert(gzw.Close(), qt.IsNil)
	payload := payloadBuf.Bytes()

	digest := backupsign.NewDigest()
	_, _ = digest.Write(payload)
	sig := signer.SignDigest(digest.Sum(nil))

	var buf bytes.Buffer
	c.Assert(inb.WriteContainer(&buf, sig, bytes.NewReader(payload), int64(len(payload))), qt.IsNil)
	return buf.Bytes()
}

func writeMember(c *qt.C, tw *tar.Writer, hdr *tar.Header, body []byte) {
	hdr.Size = int64(len(body))
	c.Assert(tw.WriteHeader(hdr), qt.IsNil)
	if len(body) > 0 {
		_, err := tw.Write(body)
		c.Assert(err, qt.IsNil)
	}
}

func TestINBRestore_RejectsTraversalMember(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(`{"version":"2.0"}`))
		// Hostile member name with a parent-traversal segment.
		writeMember(c, tw, &tar.Header{Name: "files/../../escape", Mode: 0o600, Typeflag: tar.TypeReg}, []byte("x"))
	})

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	key := "t/tenant-a/restores/evil-traversal.inb"
	c.Assert(bucket.WriteAll(f.ctx, key, archive, nil), qt.IsNil)
	bucket.Close()

	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.IsNotNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)
}

func TestINBRestore_RejectsSymlinkMember(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(`{"version":"2.0"}`))
		// A symlink member must be rejected (non-regular file).
		c.Assert(tw.WriteHeader(&tar.Header{Name: "files/link", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}), qt.IsNil)
	})

	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	key := "t/tenant-a/restores/evil-symlink.inb"
	c.Assert(bucket.WriteAll(f.ctx, key, archive, nil), qt.IsNil)
	bucket.Close()

	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.IsNotNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)
}

// TestINBExport_ManifestIsFirstMember guards the Pass-1/Pass-2 refactor: the
// manifest is now written FIRST (with complete statistics), so the import
// metadata path reaches it without inflating the location bodies. The first
// inner-tar member must be manifest.json and it must carry the real statistics.
func TestINBExport_ManifestIsFirstMember(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	f.attachCommodityFile(c, "images", 2048)

	_, archive := f.runExport(c, signer)

	order, jsons := innerMembers(c, archive)
	c.Assert(len(order) >= 1, qt.IsTrue)
	c.Assert(order[0], qt.Equals, "manifest.json")

	var manifest map[string]any
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	stats := manifest["statistics"].(map[string]any)
	// Statistics must be fully populated even though the manifest is first —
	// Pass 1 accumulates them before the manifest is emitted.
	c.Assert(stats["locationCount"], qt.Equals, float64(1))
	c.Assert(stats["areaCount"], qt.Equals, float64(1))
	c.Assert(stats["commodityCount"], qt.Equals, float64(1))
	c.Assert(stats["imageCount"], qt.Equals, float64(1))
	c.Assert(stats["fileCount"], qt.Equals, float64(1))
	c.Assert(stats["totalFileSize"], qt.Equals, float64(2048))
}

// TestINBExport_ManifestFirstStillRoundTrips proves the manifest-first layout
// did not break restore: a full export (location → area → commodity + file)
// still restores end-to-end with the manifest emitted before the bodies.
func TestINBExport_ManifestFirstStillRoundTrips(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	f.attachCommodityFile(c, "images", 4096)

	blobKey, archive := f.runExport(c, signer)
	order, _ := innerMembers(c, archive)
	c.Assert(order[0], qt.Equals, "manifest.json")

	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Assert(final.LocationCount >= 1, qt.IsTrue)
	c.Assert(final.AreaCount >= 1, qt.IsTrue)
	c.Assert(final.CommodityCount >= 1, qt.IsTrue)
	c.Assert(final.ImageCount >= 1, qt.IsTrue)
}

// TestINBExport_SelectedItemsScopeSurvivesRefactor proves the scope filtering
// survived the preload/index refactor: a selected_items export that picks one of
// two commodities must emit only that commodity (plus its implied location +
// area), not the sibling commodity in another location.
func TestINBExport_SelectedItemsScopeSurvivesRefactor(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	// Seed a SECOND location → area → commodity in the same group, so the
	// scope filter has something to exclude.
	locReg := must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	loc2 := must.Must(locReg.Create(f.ctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Name:                     "Office",
		Address:                  "9 Side St",
	}))
	areaReg := must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	area2 := must.Must(areaReg.Create(f.ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Name:                     "Desk",
		LocationID:               loc2.ID,
	}))
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	excluded := must.Must(comReg.Create(f.ctx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Name:                     "Monitor",
		ShortName:                "mon",
		Type:                     models.CommodityTypeElectronics,
		AreaID:                   new(area2.ID),
		Count:                    1,
		Status:                   models.CommodityStatusInUse,
		OriginalPriceCurrency:    "USD",
		Draft:                    true,
	}))

	// The fixture's first commodity (the "TV") is the one we select.
	selected := must.Must(comReg.List(f.ctx))[0]
	for _, com := range must.Must(comReg.List(f.ctx)) {
		if com.Name == "TV" {
			selected = com
		}
	}

	_, archive := f.runExportRow(c, signer, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Type:                     models.ExportTypeSelectedItems,
		Status:                   models.ExportStatusPending,
		Description:              "selected backup",
		SelectedItems: models.ValuerSlice[models.ExportSelectedItem]{
			{ID: selected.ID, Type: models.ExportSelectedItemTypeCommodity, Name: selected.Name},
		},
	})

	// Concatenate every location JSON document and assert the selected
	// commodity's UUID is present while the excluded one's UUID is absent.
	_, jsons := innerMembers(c, archive)
	var docBuilder strings.Builder
	for name, body := range jsons {
		if name != "manifest.json" {
			docBuilder.Write(body)
		}
	}
	allDocs := docBuilder.String()
	c.Assert(allDocs, qt.Contains, selected.UUID, qt.Commentf("selected commodity must be exported"))
	c.Assert(allDocs, qt.Not(qt.Contains), excluded.UUID, qt.Commentf("non-selected commodity must NOT be exported"))
	// The excluded commodity's location (Office) must not appear either.
	c.Assert(allDocs, qt.Not(qt.Contains), loc2.UUID, qt.Commentf("non-selected location must NOT be exported"))
}

// tamperPayload returns a copy of an .inb archive with one byte of the payload
// member flipped so its signature no longer verifies.
func tamperPayload(c *qt.C, archive []byte) []byte {
	tr := tar.NewReader(bytes.NewReader(archive))
	h1 := must.Must(tr.Next())
	sig := must.Must(io.ReadAll(tr))
	_ = must.Must(tr.Next())
	payload := must.Must(io.ReadAll(tr))
	c.Assert(len(payload) > 32, qt.IsTrue)
	payload[len(payload)/2] ^= 0xFF

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	c.Assert(tw.WriteHeader(&tar.Header{Name: h1.Name, Mode: 0o600, Size: int64(len(sig)), Typeflag: tar.TypeReg}), qt.IsNil)
	must.Must(tw.Write(sig))
	c.Assert(tw.WriteHeader(&tar.Header{Name: inb.PayloadName, Mode: 0o600, Size: int64(len(payload)), Typeflag: tar.TypeReg}), qt.IsNil)
	must.Must(tw.Write(payload))
	c.Assert(tw.Close(), qt.IsNil)
	return buf.Bytes()
}
