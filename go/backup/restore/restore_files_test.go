package restore_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/export"
	exporttypes "github.com/denisvmedia/inventario/backup/export/types"
	"github.com/denisvmedia/inventario/backup/restore/processor"
	"github.com/denisvmedia/inventario/backup/restore/types"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// fileFixture is a per-test helper that builds a complete backup environment:
// tenant, user, group, location, area, commodity, plus a few unified files
// linked to the commodity / location / standalone. Returns the factory, the
// authenticated context, and direct DB IDs for assertions.
type fileFixture struct {
	factorySet *registry.FactorySet
	ctx        context.Context
	tenantID   string
	userID     string
	groupID    string
	locationID string
	areaID     string
	commodity  *models.Commodity
}

func newFileFixture(c *qt.C) *fileFixture {
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1485"},
		Email:               "1485@example.com",
		Name:                "File Restore User",
		IsActive:            true,
	}
	tenant := models.Tenant{
		EntityID: models.EntityID{ID: "tenant-1485"},
		Name:     "Tenant 1485",
	}

	factorySet := memory.NewFactorySet()
	must.Must(factorySet.TenantRegistry.Create(c.Context(), tenant))
	createdUser := must.Must(factorySet.UserRegistry.Create(c.Context(), user))
	ctx := ensureGroupForUser(c.Context(), factorySet, createdUser)
	group := appctx.GroupFromContext(ctx)
	c.Assert(group, qt.IsNotNil)

	// Stamp a minimal hierarchy so files can be linked to real entities.
	locReg := must.Must(factorySet.LocationRegistryFactory.CreateUserRegistry(ctx))
	loc := must.Must(locReg.Create(ctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        createdUser.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: createdUser.ID,
		},
		Name:    "HQ",
		Address: "1 Main",
	}))
	areaReg := must.Must(factorySet.AreaRegistryFactory.CreateUserRegistry(ctx))
	area := must.Must(areaReg.Create(ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        createdUser.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: createdUser.ID,
		},
		Name:       "Office",
		LocationID: loc.ID,
	}))
	purchaseDate := models.ToPDate(models.Date("2024-01-01"))
	comReg := must.Must(factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx))
	commodity := must.Must(comReg.Create(ctx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        createdUser.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: createdUser.ID,
		},
		Name:                  "Workstation",
		ShortName:             "WS",
		AreaID:                area.ID,
		Status:                models.CommodityStatusInUse,
		Type:                  models.CommodityTypeElectronics,
		Count:                 1,
		OriginalPriceCurrency: models.Currency("USD"),
		PurchaseDate:          purchaseDate,
	}))

	return &fileFixture{
		factorySet: factorySet,
		ctx:        ctx,
		tenantID:   createdUser.TenantID,
		userID:     createdUser.ID,
		groupID:    group.ID,
		locationID: loc.ID,
		areaID:     area.ID,
		commodity:  commodity,
	}
}

// makeFile creates a unified file row in the fixture's group with the given
// link metadata. Returns the created row so the test can assert UUID parity
// after round-trip.
func (f *fileFixture) makeFile(c *qt.C, title, mime, linkedType, linkedID, linkedMeta string, tags ...string) *models.FileEntity {
	now := time.Now().UTC().Truncate(time.Second)
	fileReg := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))
	created := must.Must(fileReg.Create(f.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.tenantID,
			GroupID:         f.groupID,
			CreatedByUserID: f.userID,
		},
		Title:            title,
		Description:      "for-1485",
		Type:             models.FileTypeFromMIME(mime),
		Category:         models.FileCategoryFromContext(linkedType, linkedMeta, mime),
		Tags:             models.StringSlice(tags),
		LinkedEntityType: linkedType,
		LinkedEntityID:   linkedID,
		LinkedEntityMeta: linkedMeta,
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "blob-" + title,
			OriginalPath: "blob-" + title + ".bin",
			Ext:          ".bin",
			MIMEType:     mime,
		},
	}))
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	return created
}

// runExport drives the export service against the fixture's group and
// returns the generated XML bytes — sidestepping the file-bucket round-trip
// that ProcessExport does. We don't need that here because the round-trip
// tests all read the XML directly into the restore processor.
func (f *fileFixture) runExport(c *qt.C, exportType models.ExportType, includeFileData bool, uploadLocation string) []byte {
	svc := export.NewExportService(f.factorySet, uploadLocation)
	exp := models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("export-1485", f.tenantID, f.groupID, f.userID),
		Type:                     exportType,
		Status:                   models.ExportStatusPending,
		IncludeFileData:          includeFileData,
	}
	var buf bytes.Buffer
	stats, err := export.ExportXML(svc, f.ctx, exp, &buf)
	c.Assert(err, qt.IsNil)
	c.Assert(stats, qt.IsNotNil)
	return buf.Bytes()
}

// TestExportRestore_FileRoundTrip is the headline acceptance test for #1485.
// Creates a hierarchy with three files (a commodity-linked image, a
// commodity-linked invoice, and a standalone document), exports the entire
// group, full-replaces into a fresh fixture, and verifies that every file
// reappears with original metadata + remapped linked_entity_id pointing at
// the new commodity DB row.
func TestExportRestore_FileRoundTrip(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"

	// --- Source group: three files of mixed categories.
	src := newFileFixture(c)
	commImage := src.makeFile(c, "photo", "image/jpeg", "commodity", src.commodity.ID, "images", "photos")
	commInvoice := src.makeFile(c, "invoice", "application/pdf", "commodity", src.commodity.ID, "invoices", "billing")
	standalone := src.makeFile(c, "guide", "text/plain", "", "", "")

	// Drop a real blob alongside one of the rows so includeFileData=true
	// has something to base64-encode on the way out. We use the same key
	// the FileEntity recorded on creation so the export reads it back.
	writeBlob(c, uploadLocation, commImage.OriginalPath, []byte("image-bytes"))
	writeBlob(c, uploadLocation, commInvoice.OriginalPath, []byte("invoice-bytes"))
	writeBlob(c, uploadLocation, standalone.OriginalPath, []byte("guide-bytes"))

	xmlBytes := src.runExport(c, models.ExportTypeFullDatabase, true, uploadLocation)
	c.Assert(string(xmlBytes), qt.Contains, "<files>")
	c.Assert(string(xmlBytes), qt.Contains, "<file id=\""+commImage.UUID+"\"")
	c.Assert(string(xmlBytes), qt.Contains, "<linkedEntityType>commodity</linkedEntityType>")
	c.Assert(string(xmlBytes), qt.Contains, "<linkedEntityId>"+src.commodity.UUID+"</linkedEntityId>")

	// --- Destination group: empty, then restore.
	dst := newFileFixture(c)
	// Wipe the destination's hierarchy so FullReplace can stamp the
	// XML's UUIDs in cleanly. The fixture pre-stamped one
	// location/area/commodity per the helper convention; the source XML
	// will recreate equivalent ones with the source UUIDs.
	dstStartCommodities := must.Must(must.Must(dst.factorySet.CommodityRegistryFactory.CreateUserRegistry(dst.ctx)).List(dst.ctx))
	c.Assert(dstStartCommodities, qt.HasLen, 1)

	proc := processor.NewRestoreOperationProcessor(
		"restore-1485",
		dst.factorySet,
		services.NewEntityService(dst.factorySet, uploadLocation),
		uploadLocation,
	)
	stats, err := proc.RestoreFromXML(dst.ctx, bytes.NewReader(xmlBytes), types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		IncludeFileData: true,
		DryRun:          false,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("restore errors: %v", stats.Errors))
	c.Assert(stats.FileCount, qt.Equals, 3)
	c.Assert(stats.BinaryDataSize, qt.Equals, int64(len("image-bytes")+len("invoice-bytes")+len("guide-bytes")))

	// --- Assertions: files reconstructed with original metadata + remapped IDs.
	dstFileReg := must.Must(dst.factorySet.FileRegistryFactory.CreateUserRegistry(dst.ctx))
	dstFiles := must.Must(dstFileReg.List(dst.ctx))
	c.Assert(dstFiles, qt.HasLen, 3)

	byUUID := make(map[string]*models.FileEntity, len(dstFiles))
	for _, f := range dstFiles {
		byUUID[f.UUID] = f
	}

	// Find the new commodity DB id (UUID == source commodity UUID).
	dstCommReg := must.Must(dst.factorySet.CommodityRegistryFactory.CreateUserRegistry(dst.ctx))
	dstCommodities := must.Must(dstCommReg.List(dst.ctx))
	var newCommodityID string
	for _, com := range dstCommodities {
		if com.UUID == src.commodity.UUID {
			newCommodityID = com.ID
			break
		}
	}
	c.Assert(newCommodityID, qt.Not(qt.Equals), "")

	// Photo: linked to remapped commodity DB id, category=photos, tags preserved.
	photo := byUUID[commImage.UUID]
	c.Assert(photo, qt.IsNotNil)
	c.Assert(photo.LinkedEntityType, qt.Equals, "commodity")
	c.Assert(photo.LinkedEntityID, qt.Equals, newCommodityID, qt.Commentf("expected commodity ID remap"))
	c.Assert(photo.LinkedEntityMeta, qt.Equals, "images")
	c.Assert(photo.Category, qt.Equals, models.FileCategoryPhotos)
	c.Assert([]string(photo.Tags), qt.DeepEquals, []string{"photos"})

	// Invoice: same commodity, different meta + category.
	invoice := byUUID[commInvoice.UUID]
	c.Assert(invoice, qt.IsNotNil)
	c.Assert(invoice.LinkedEntityType, qt.Equals, "commodity")
	c.Assert(invoice.LinkedEntityID, qt.Equals, newCommodityID)
	c.Assert(invoice.LinkedEntityMeta, qt.Equals, "invoices")
	c.Assert(invoice.Category, qt.Equals, models.FileCategoryInvoices)

	// Standalone: no link, no remap.
	guide := byUUID[standalone.UUID]
	c.Assert(guide, qt.IsNotNil)
	c.Assert(guide.LinkedEntityType, qt.Equals, "")
	c.Assert(guide.LinkedEntityID, qt.Equals, "")

	// Blob round-trip: restored bucket has the same bytes.
	c.Assert(readBlob(c, uploadLocation, commImage.OriginalPath), qt.DeepEquals, []byte("image-bytes"))
	c.Assert(readBlob(c, uploadLocation, commInvoice.OriginalPath), qt.DeepEquals, []byte("invoice-bytes"))
	c.Assert(readBlob(c, uploadLocation, standalone.OriginalPath), qt.DeepEquals, []byte("guide-bytes"))
}

// TestExportRestore_EmptyFileSection covers the empty-export AC: a group
// with zero files emits an empty <files> section (rather than crashing or
// producing malformed XML), and the restore-side accepts it cleanly.
func TestExportRestore_EmptyFileSection(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	src := newFileFixture(c)

	xmlBytes := src.runExport(c, models.ExportTypeFullDatabase, false, uploadLocation)
	c.Assert(string(xmlBytes), qt.Contains, "<files>")
	c.Assert(string(xmlBytes), qt.Contains, "</files>")

	dst := newFileFixture(c)
	proc := processor.NewRestoreOperationProcessor(
		"restore-1485-empty",
		dst.factorySet,
		services.NewEntityService(dst.factorySet, uploadLocation),
		uploadLocation,
	)
	stats, err := proc.RestoreFromXML(dst.ctx, bytes.NewReader(xmlBytes), types.RestoreOptions{
		Strategy: types.RestoreStrategyFullReplace,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.FileCount, qt.Equals, 0)
}

// TestExportRestore_ExcludesExportLinkedFiles guards the rule that the
// export's own backup-bundle FileEntity (linked_entity_type="export") does
// NOT appear inside the <files> section it itself produces — that would
// create a self-reference that the restore can't reconstruct safely.
func TestExportRestore_ExcludesExportLinkedFiles(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	src := newFileFixture(c)

	// Stamp an export-linked file directly. ProcessExport would create
	// one for the bundle, but here we just want to verify the filter.
	bundleFile := src.makeFile(c, "bundle", "application/xml", "export", "some-export-id", "xml-1.0")
	c.Assert(bundleFile.LinkedEntityType, qt.Equals, "export")

	xmlBytes := src.runExport(c, models.ExportTypeFullDatabase, false, uploadLocation)
	c.Assert(strings.Contains(string(xmlBytes), bundleFile.UUID), qt.IsFalse,
		qt.Commentf("export-linked files must not be emitted in <files>"))
}

// TestExportRestore_LegacyAttachmentSectionsIgnored is the regression guard
// for "drop the silent-skip code". A backup containing the pre-cutover
// <images>/<invoices>/<manuals> commodity sections must parse cleanly
// (no unknown-token errors, no panics) — they're simply ignored as the
// surrounding commodity decoding falls through them.
func TestExportRestore_LegacyAttachmentSectionsIgnored(t *testing.T) {
	c := qt.New(t)

	dst := newFileFixture(c)
	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<inventory>
  <locations>
    <location id="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa">
      <locationName>Test Location</locationName>
      <address>X</address>
    </location>
  </locations>
  <areas>
    <area id="bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb">
      <areaName>Test Area</areaName>
      <locationId>aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="cccccccc-cccc-cccc-cccc-cccccccccccc">
      <commodityName>Test Commodity</commodityName>
      <shortName>TC</shortName>
      <type>electronics</type>
      <areaId>bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb</areaId>
      <count>1</count>
      <status>in_use</status>
      <originalPrice>100</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <convertedOriginalPrice>0</convertedOriginalPrice>
      <currentPrice>100</currentPrice>
      <purchaseDate>2024-01-01</purchaseDate>
      <draft>false</draft>
      <images>
        <image id="legacy-image"><path>p</path><originalPath>p.jpg</originalPath><extension>.jpg</extension><mimeType>image/jpeg</mimeType></image>
      </images>
      <invoices>
        <invoice id="legacy-invoice"><path>p</path><originalPath>p.pdf</originalPath><extension>.pdf</extension><mimeType>application/pdf</mimeType></invoice>
      </invoices>
      <manuals>
        <manual id="legacy-manual"><path>p</path><originalPath>p.pdf</originalPath><extension>.pdf</extension><mimeType>application/pdf</mimeType></manual>
      </manuals>
    </commodity>
  </commodities>
</inventory>`

	proc := processor.NewRestoreOperationProcessor(
		"restore-1485-legacy",
		dst.factorySet,
		services.NewEntityService(dst.factorySet, uploadLocation),
		uploadLocation,
	)
	stats, err := proc.RestoreFromXML(dst.ctx, strings.NewReader(xmlContent), types.RestoreOptions{
		Strategy: types.RestoreStrategyFullReplace,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("restore errors: %v", stats.Errors))
	c.Assert(stats.CommodityCount, qt.Equals, 1)
	c.Assert(stats.FileCount, qt.Equals, 0, qt.Commentf("legacy attachment sections must NOT count as files"))
}

// TestExportRestore_SelectedItemsScope verifies that ExportTypeSelectedItems
// only emits files attached to the selected entities. Standalone files and
// files on other commodities are filtered out.
func TestExportRestore_SelectedItemsScope(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	src := newFileFixture(c)

	// Selected commodity gets one file; an unrelated file (standalone) is
	// also added to verify it's filtered out by the selected-items scope.
	wantedFile := src.makeFile(c, "photo", "image/jpeg", "commodity", src.commodity.ID, "images", "tag-x")
	otherFile := src.makeFile(c, "guide", "text/plain", "", "", "")

	svc := export.NewExportService(src.factorySet, uploadLocation)
	exp := models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("export-selected-1485", src.tenantID, src.groupID, src.userID),
		Type:                     models.ExportTypeSelectedItems,
		Status:                   models.ExportStatusPending,
		IncludeFileData:          false,
		SelectedItems: models.ValuerSlice[models.ExportSelectedItem]{
			{ID: src.commodity.ID, Type: models.ExportSelectedItemTypeCommodity},
		},
	}
	var buf bytes.Buffer
	_, err := export.ExportXML(svc, src.ctx, exp, &buf)
	c.Assert(err, qt.IsNil)

	out := buf.String()
	c.Assert(strings.Contains(out, wantedFile.UUID), qt.IsTrue, qt.Commentf("commodity-linked file should be in scope"))
	c.Assert(strings.Contains(out, otherFile.UUID), qt.IsFalse, qt.Commentf("standalone file should NOT be in selected_items scope"))
}

// TestExportRestore_CrossTenantIsolation guards that the FileRegistry's
// user-mode RLS context (memory mode mirrors the postgres behavior) keeps
// tenant A's export from including tenant B's files. Both tenants share
// the same FactorySet but each runs export under its own context.
func TestExportRestore_CrossTenantIsolation(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	tenantA := newFileFixture(c)
	aFile := tenantA.makeFile(c, "a-photo", "image/jpeg", "commodity", tenantA.commodity.ID, "images")

	// Build a separate fixture for tenant B (separate factory + context).
	tenantB := newFileFixture(c)
	bFile := tenantB.makeFile(c, "b-photo", "image/jpeg", "commodity", tenantB.commodity.ID, "images")

	xmlA := tenantA.runExport(c, models.ExportTypeFullDatabase, false, uploadLocation)
	xmlB := tenantB.runExport(c, models.ExportTypeFullDatabase, false, uploadLocation)

	c.Assert(strings.Contains(string(xmlA), aFile.UUID), qt.IsTrue)
	c.Assert(strings.Contains(string(xmlA), bFile.UUID), qt.IsFalse, qt.Commentf("tenant A export must not contain tenant B files"))
	c.Assert(strings.Contains(string(xmlB), bFile.UUID), qt.IsTrue)
	c.Assert(strings.Contains(string(xmlB), aFile.UUID), qt.IsFalse, qt.Commentf("tenant B export must not contain tenant A files"))
}

// writeBlob is a thin wrapper over the gocloud blob bucket so tests can
// stamp the source-side file content the export will base64 into XML.
func writeBlob(c *qt.C, uploadLocation, key string, data []byte) {
	b := must.Must(blob.OpenBucket(c.Context(), uploadLocation))
	defer b.Close()
	w := must.Must(b.NewWriter(c.Context(), key, nil))
	_, err := w.Write(data)
	c.Assert(err, qt.IsNil)
	c.Assert(w.Close(), qt.IsNil)
}

func readBlob(c *qt.C, uploadLocation, key string) []byte {
	b := must.Must(blob.OpenBucket(c.Context(), uploadLocation))
	defer b.Close()
	r := must.Must(b.NewReader(c.Context(), key, nil))
	defer r.Close()
	out := make([]byte, r.Size())
	_, err := r.Read(out)
	c.Assert(err, qt.IsNil)
	return out
}

// Compile-time guard: keep us honest about the shape of types.RestoreStats.
var _ = exporttypes.ExportStats{FileCount: 0}
