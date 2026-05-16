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
	c.Assert(photo.Category, qt.Equals, models.FileCategoryImages)
	c.Assert([]string(photo.Tags), qt.DeepEquals, []string{"photos"})

	// Invoice: same commodity + linked-entity-meta still "invoices", but
	// post-#1622 the category folds into `documents` and the file
	// carries the conventional `invoice` tag.
	invoice := byUUID[commInvoice.UUID]
	c.Assert(invoice, qt.IsNotNil)
	c.Assert(invoice.LinkedEntityType, qt.Equals, "commodity")
	c.Assert(invoice.LinkedEntityID, qt.Equals, newCommodityID)
	c.Assert(invoice.LinkedEntityMeta, qt.Equals, "invoices")
	c.Assert(invoice.Category, qt.Equals, models.FileCategoryDocuments)

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
	c.Assert(string(xmlBytes), qt.Not(qt.Contains), bundleFile.UUID,
		qt.Commentf("export-linked files must not be emitted in <files>"))
}

// TestExportRestore_LegacyAttachmentSectionsRecorded is the regression
// guard for the loud-fail policy on pre-cutover archives. A backup
// carrying the legacy <images>/<invoices>/<manuals> commodity sections
// must parse without panics, but each occurrence has to surface as a
// stats.Errors entry with stats.ErrorCount incremented — silently
// dropping their data with ErrorCount=0 misled operators into thinking
// the legacy backup was restored intact (Copilot review on PR #1493).
//
// The surrounding commodity is still created (we don't abort the
// commodity row over a legacy attachment section) so the rest of the
// restore continues; only the attachment data is dropped, with the
// operator informed.
func TestExportRestore_LegacyAttachmentSectionsRecorded(t *testing.T) {
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

	// One commodity, three legacy attachment sections → ErrorCount==3.
	c.Assert(stats.ErrorCount, qt.Equals, 3,
		qt.Commentf("expected one error per legacy <images>/<invoices>/<manuals> section, got %v", stats.Errors))
	c.Assert(stats.CommodityCount, qt.Equals, 1, qt.Commentf("commodity itself still restored"))
	c.Assert(stats.FileCount, qt.Equals, 0, qt.Commentf("legacy attachment sections must NOT count as files"))

	// Each error message points at the section name AND the commodity ID
	// so an operator can find the offending row in the source backup.
	joined := strings.Join(stats.Errors, "\n")
	c.Assert(joined, qt.Contains, "<images>")
	c.Assert(joined, qt.Contains, "<invoices>")
	c.Assert(joined, qt.Contains, "<manuals>")
	c.Assert(joined, qt.Contains, "cccccccc-cccc-cccc-cccc-cccccccccccc")
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
	c.Assert(out, qt.Contains, wantedFile.UUID, qt.Commentf("commodity-linked file should be in scope"))
	c.Assert(out, qt.Not(qt.Contains), otherFile.UUID, qt.Commentf("standalone file should NOT be in selected_items scope"))
}

// TestExportRestore_CrossTenantIsolation guards that the FileRegistry's
// user-mode RLS context (memory mode mirrors the postgres behavior) keeps
// tenant A's export from including tenant B's files.
//
// Both tenants share a single FactorySet so the test actually exercises
// the registry's per-user filtering (the previous version used two
// separate `memory.NewFactorySet()` instances, which gave each tenant
// its own store and would have masked an RLS regression — flagged by
// Copilot on PR #1493). Each user gets their own tenant + group; the
// commodities, files, and export streams all run under a context
// carrying the user's identity, and the resulting XML is asserted not
// to leak rows belonging to the other tenant.
func TestExportRestore_CrossTenantIsolation(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	factorySet := memory.NewFactorySet()

	tenantA, ctxA := stampTenantWithCommodityFile(c, factorySet, "tenant-A", "a-photo")
	tenantB, ctxB := stampTenantWithCommodityFile(c, factorySet, "tenant-B", "b-photo")

	svc := export.NewExportService(factorySet, uploadLocation)
	exportXMLForTenant := func(ctx context.Context, t *crossTenantHandle) []byte {
		exp := models.Export{
			TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("xt-export-"+t.tenantID, t.tenantID, t.groupID, t.userID),
			Type:                     models.ExportTypeFullDatabase,
			Status:                   models.ExportStatusPending,
			IncludeFileData:          false,
		}
		var buf bytes.Buffer
		_, err := export.ExportXML(svc, ctx, exp, &buf)
		c.Assert(err, qt.IsNil)
		return buf.Bytes()
	}

	xmlA := exportXMLForTenant(ctxA, tenantA)
	xmlB := exportXMLForTenant(ctxB, tenantB)

	// Tenant A's export must contain only its own file UUID; tenant B's
	// must contain only its own. A regression where the file registry
	// stops filtering by tenant context (RLS bypass, accidental
	// service-mode list, etc.) would surface here as a leaked UUID.
	c.Assert(string(xmlA), qt.Contains, tenantA.fileUUID)
	c.Assert(string(xmlA), qt.Not(qt.Contains), tenantB.fileUUID,
		qt.Commentf("tenant A export must not contain tenant B files"))
	c.Assert(string(xmlB), qt.Contains, tenantB.fileUUID)
	c.Assert(string(xmlB), qt.Not(qt.Contains), tenantA.fileUUID,
		qt.Commentf("tenant B export must not contain tenant A files"))
}

// crossTenantHandle bundles the IDs needed to drive an export under one
// tenant's context inside the cross-tenant isolation test. Lives next
// to the test that uses it; not exported.
type crossTenantHandle struct {
	tenantID    string
	userID      string
	groupID     string
	commodityID string
	fileUUID    string
}

// stampTenantWithCommodityFile creates a tenant + user + group + the
// minimal location/area/commodity hierarchy needed for a commodity-
// linked file to round-trip through export, in the supplied (shared)
// FactorySet. Returns a handle with the IDs the test cares about and a
// context carrying the user + group so the user-aware registries
// produced via CreateUserRegistry are properly scoped.
func stampTenantWithCommodityFile(c *qt.C, factorySet *registry.FactorySet, tenantSlug, fileTitle string) (*crossTenantHandle, context.Context) {
	tenantID := tenantSlug
	must.Must(factorySet.TenantRegistry.Create(c.Context(), models.Tenant{
		EntityID: models.EntityID{ID: tenantID},
		Name:     tenantSlug,
	}))
	user := must.Must(factorySet.UserRegistry.Create(c.Context(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               tenantSlug + "@example.com",
		Name:                tenantSlug,
		IsActive:            true,
	}))
	ctx := ensureGroupForUser(c.Context(), factorySet, user)
	group := appctx.GroupFromContext(ctx)
	c.Assert(group, qt.IsNotNil)

	purchaseDate := models.ToPDate(models.Date("2024-01-01"))
	locReg := must.Must(factorySet.LocationRegistryFactory.CreateUserRegistry(ctx))
	loc := must.Must(locReg.Create(ctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        tenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Name: "HQ-" + tenantSlug,
	}))
	areaReg := must.Must(factorySet.AreaRegistryFactory.CreateUserRegistry(ctx))
	area := must.Must(areaReg.Create(ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        tenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Name:       "Office-" + tenantSlug,
		LocationID: loc.ID,
	}))
	comReg := must.Must(factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx))
	commodity := must.Must(comReg.Create(ctx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        tenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Name:                  "Workstation-" + tenantSlug,
		ShortName:             "WS",
		AreaID:                area.ID,
		Status:                models.CommodityStatusInUse,
		Type:                  models.CommodityTypeElectronics,
		Count:                 1,
		OriginalPriceCurrency: models.Currency("USD"),
		PurchaseDate:          purchaseDate,
	}))
	fileReg := must.Must(factorySet.FileRegistryFactory.CreateUserRegistry(ctx))
	now := time.Now().UTC().Truncate(time.Second)
	created := must.Must(fileReg.Create(ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        tenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Title:            fileTitle,
		Type:             models.FileTypeImage,
		Category:         models.FileCategoryImages,
		Tags:             models.StringSlice{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   commodity.ID,
		LinkedEntityMeta: "images",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         fileTitle,
			OriginalPath: fileTitle + ".jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))

	return &crossTenantHandle{
		tenantID:    tenantID,
		userID:      user.ID,
		groupID:     group.ID,
		commodityID: commodity.ID,
		fileUUID:    created.UUID,
	}, ctx
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

// TestExportRestore_LargeBlobStreamingRoundTrip exercises the chunked
// base64 encode/decode path on a blob that's deliberately larger than the
// 32 KiB chunk size used by streamBase64Content / xmlChardataReader.
//
// What this proves:
//   - Export streams the blob through xmlBase64Writer in chunks (the
//     `<data>` element doesn't crash when the chardata size exceeds one
//     CharData token's natural size).
//   - Restore's xmlChardataReader reassembles arbitrary chardata-token
//     boundaries (Go's xml.Decoder may split chardata across tokens at
//     internal buffer boundaries — pretty-print whitespace also splits).
//   - The base64 round-trip is byte-identical for non-trivial payloads.
//
// What this DOES NOT prove (and would need a separate benchmark/profile):
//   - That memory residency stays bounded at chunk-size during the
//     transfer. The streaming path's *correctness* is testable here;
//     its *memory profile* is best verified with `go test -benchmem`
//     against a multi-MB blob, which is out of scope for this unit.
func TestExportRestore_LargeBlobStreamingRoundTrip(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	src := newFileFixture(c)

	// 200 KiB of pseudo-random-looking bytes — large enough to span
	// multiple 32 KiB chunks AND multiple xml.Decoder Token() reads.
	// Using a byte ramp so any ordering corruption is visible.
	const blobSize = 200 * 1024
	bigBlob := make([]byte, blobSize)
	for i := range bigBlob {
		bigBlob[i] = byte(i % 251) // 251 is prime → avoids alignment with 32 KiB
	}

	bigFile := src.makeFile(c, "big-photo", "image/jpeg", "commodity", src.commodity.ID, "images")
	writeBlob(c, uploadLocation, bigFile.OriginalPath, bigBlob)

	xmlBytes := src.runExport(c, models.ExportTypeFullDatabase, true, uploadLocation)
	c.Assert(string(xmlBytes), qt.Contains, "<data>")
	c.Assert(string(xmlBytes), qt.Contains, "</data>")

	// Restore on a fresh fixture using a SEPARATE bucket so we can verify
	// the blob lands at the same OriginalPath as the source.
	restoreLocation := "file://" + c.TempDir() + "?create_dir=1"
	dst := newFileFixture(c)
	proc := processor.NewRestoreOperationProcessor(
		"restore-1485-large",
		dst.factorySet,
		services.NewEntityService(dst.factorySet, restoreLocation),
		restoreLocation,
	)
	stats, err := proc.RestoreFromXML(dst.ctx, bytes.NewReader(xmlBytes), types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		IncludeFileData: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("restore errors: %v", stats.Errors))
	c.Assert(stats.FileCount, qt.Equals, 1)
	c.Assert(stats.BinaryDataSize, qt.Equals, int64(blobSize))

	got := readBlob(c, restoreLocation, bigFile.OriginalPath)
	c.Assert(got, qt.HasLen, blobSize)
	c.Assert(bytes.Equal(got, bigBlob), qt.IsTrue, qt.Commentf("blob bytes corrupted across streaming round-trip"))
}

// TestExportRestore_DryRunSkipsBlobWrite covers the DryRun branch on the
// restore side: the streaming decoder still drains the <data> chardata
// (so BinaryDataSize is reported for preview) but the destination bucket
// stays empty.
func TestExportRestore_DryRunSkipsBlobWrite(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	src := newFileFixture(c)
	srcFile := src.makeFile(c, "preview", "image/jpeg", "commodity", src.commodity.ID, "images")
	writeBlob(c, uploadLocation, srcFile.OriginalPath, []byte("preview-bytes"))

	xmlBytes := src.runExport(c, models.ExportTypeFullDatabase, true, uploadLocation)

	// Restore into a fresh empty bucket with DryRun=true. We use MergeAdd
	// so the existing fixture commodity hierarchy is treated as already-
	// present and validation falls through cleanly.
	restoreLocation := "file://" + c.TempDir() + "?create_dir=1"
	dst := newFileFixture(c)
	proc := processor.NewRestoreOperationProcessor(
		"restore-1485-dryrun",
		dst.factorySet,
		services.NewEntityService(dst.factorySet, restoreLocation),
		restoreLocation,
	)
	stats, err := proc.RestoreFromXML(dst.ctx, bytes.NewReader(xmlBytes), types.RestoreOptions{
		Strategy:        types.RestoreStrategyMergeAdd,
		IncludeFileData: true,
		DryRun:          true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.BinaryDataSize, qt.Equals, int64(len("preview-bytes")), qt.Commentf("dry-run should still surface decoded byte count"))

	// Bucket must be empty — no row was created, no blob was written.
	bucket, err := blob.OpenBucket(c.Context(), restoreLocation)
	c.Assert(err, qt.IsNil)
	defer bucket.Close()
	exists, err := bucket.Exists(c.Context(), srcFile.OriginalPath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse, qt.Commentf("DryRun must not write blobs to the destination bucket"))
}

// TestExportRestore_FullReplaceClearsOrphanBlobs guards the cleanup
// behavior flagged on PR #1493: a FullReplace restore must wipe both
// the row AND the physical blob for non-export files in the
// destination, otherwise repeated restores leak storage and stale
// thumbnails. Commodity-linked files cascade through
// EntityService.DeleteCommodityRecursive (covered by existing tests);
// this case verifies the second-pass sweep that catches
// location-/area-linked + standalone files.
func TestExportRestore_FullReplaceClearsOrphanBlobs(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	dst := newFileFixture(c)

	// Stamp pre-existing data in the destination: a standalone file row
	// + its physical blob. After FullReplace these must both be gone.
	preExisting := dst.makeFile(c, "stale", "image/jpeg", "", "", "")
	writeBlob(c, uploadLocation, preExisting.OriginalPath, []byte("stale-bytes"))

	// And keep an export-linked file around so we can verify it's NOT
	// touched (DeleteFileWithPhysical would race with the restore-input
	// FK if we deleted the bundle the restore is reading from).
	bundleFile := dst.makeFile(c, "bundle", "application/xml", "export", "stash-export", "xml-1.0")
	writeBlob(c, uploadLocation, bundleFile.OriginalPath, []byte("bundle-bytes"))

	// Empty-files XML (we just want to exercise clearExistingData via
	// the FullReplace strategy entry point — no <files> to restore).
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<inventory exportType="full_database">
  <locations></locations>
  <areas></areas>
  <commodities></commodities>
  <files></files>
</inventory>`

	proc := processor.NewRestoreOperationProcessor(
		"restore-1485-clear",
		dst.factorySet,
		services.NewEntityService(dst.factorySet, uploadLocation),
		uploadLocation,
	)
	stats, err := proc.RestoreFromXML(dst.ctx, strings.NewReader(xmlContent), types.RestoreOptions{
		Strategy: types.RestoreStrategyFullReplace,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("restore errors: %v", stats.Errors))

	// Pre-existing standalone row + blob both gone.
	dstFileReg := must.Must(dst.factorySet.FileRegistryFactory.CreateUserRegistry(dst.ctx))
	_, err = dstFileReg.Get(dst.ctx, preExisting.ID)
	c.Assert(err, qt.IsNotNil, qt.Commentf("standalone row should be deleted"))
	bucket := must.Must(blob.OpenBucket(c.Context(), uploadLocation))
	defer bucket.Close()
	exists, err := bucket.Exists(c.Context(), preExisting.OriginalPath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse, qt.Commentf("orphan standalone blob should be deleted from bucket"))

	// Bundle (export-linked) file untouched: row still present, blob still there.
	got, err := dstFileReg.Get(dst.ctx, bundleFile.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("export-linked bundle row must NOT be deleted by FullReplace"))
	c.Assert(got.UUID, qt.Equals, bundleFile.UUID)
	exists, err = bucket.Exists(c.Context(), bundleFile.OriginalPath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue, qt.Commentf("export-linked bundle blob must NOT be deleted by FullReplace"))
}

// TestExportRestore_MergeAddDoesNotOverwriteExistingBlob covers the
// blob-write-vs-decide ordering called out by Copilot on PR #1493.
// Before the fix, decodeFileElement streamed <data> into the bucket
// at xmlFile.OriginalPath BEFORE the strategy handler decided whether
// to skip the row; in MergeAdd, an already-present file UUID would
// have its blob clobbered even though the row is skipped.
//
// This test stamps a file row + custom blob bytes in the destination
// matching the source's UUID + OriginalPath, then runs MergeAdd. The
// row count must stay at 1 (skip), and the destination blob bytes
// must be the destination's original payload — not the source's.
func TestExportRestore_MergeAddDoesNotOverwriteExistingBlob(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	src := newFileFixture(c)

	// Source side: a commodity-linked file with known bytes.
	srcFile := src.makeFile(c, "shared", "image/jpeg", "commodity", src.commodity.ID, "images")
	writeBlob(c, uploadLocation, srcFile.OriginalPath, []byte("source-bytes"))

	xmlBytes := src.runExport(c, models.ExportTypeFullDatabase, true, uploadLocation)

	// Destination side: pre-stamp a file with the SAME UUID +
	// OriginalPath but DIFFERENT bytes. The dst fixture's
	// commodity has a different DB ID but the same UUID lifecycle
	// is irrelevant here — we're testing the file dedup path.
	dst := newFileFixture(c)
	dstFileReg := must.Must(dst.factorySet.FileRegistryFactory.CreateUserRegistry(dst.ctx))
	preExisting := must.Must(dstFileReg.Create(dst.ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID:        models.EntityID{UUID: srcFile.UUID},
			TenantID:        dst.tenantID,
			GroupID:         dst.groupID,
			CreatedByUserID: dst.userID,
		},
		Title:            "destination-original",
		Type:             models.FileTypeImage,
		Category:         models.FileCategoryImages,
		Tags:             models.StringSlice{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   dst.commodity.ID,
		LinkedEntityMeta: "images",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		File: &models.File{
			Path:         srcFile.Path,
			OriginalPath: srcFile.OriginalPath,
			Ext:          srcFile.Ext,
			MIMEType:     srcFile.MIMEType,
		},
	}))
	c.Assert(preExisting.UUID, qt.Equals, srcFile.UUID)
	writeBlob(c, uploadLocation, srcFile.OriginalPath, []byte("destination-bytes"))

	proc := processor.NewRestoreOperationProcessor(
		"restore-1485-mergeadd-dedup",
		dst.factorySet,
		services.NewEntityService(dst.factorySet, uploadLocation),
		uploadLocation,
	)
	stats, err := proc.RestoreFromXML(dst.ctx, bytes.NewReader(xmlBytes), types.RestoreOptions{
		Strategy:        types.RestoreStrategyMergeAdd,
		IncludeFileData: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("restore errors: %v", stats.Errors))

	// MergeAdd must have classified the duplicate as Skipped (not
	// Created or Updated) — and the bytes on disk must still be the
	// destination's, not the source's.
	c.Assert(stats.SkippedCount > 0, qt.IsTrue, qt.Commentf("file with existing UUID should classify as skipped"))
	c.Assert(readBlob(c, uploadLocation, srcFile.OriginalPath), qt.DeepEquals, []byte("destination-bytes"),
		qt.Commentf("MergeAdd dedup must not clobber the destination's existing blob"))
}

// TestExportRestore_DropsBlobOnUnresolvedLinkedEntity covers the second
// half of Copilot's blob-eager-write concern: when a file references a
// commodity / location / area that wasn't included in the same export
// (so the IDMapping has nothing to resolve to), the row will be
// rejected as `failed to process file: file ... references unknown
// commodity ...`. Pre-fix, decodeFileElement still wrote the blob to
// the bucket before that rejection — leaving an orphan blob with no
// row pointing at it. The fix routes the <data> chardata into
// io.Discard for that case.
func TestExportRestore_DropsBlobOnUnresolvedLinkedEntity(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	dst := newFileFixture(c)

	// Synthesize a <files>-only XML referencing a commodity UUID that
	// isn't in the destination. The restore will reject the file row
	// with an "unknown commodity" error, but the bucket must stay
	// empty — the blob must NOT be written before the rejection.
	const orphanFileUUID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	const unknownCommodityUUID = "00000000-0000-0000-0000-deadbeefdead"
	const orphanBlobKey = "orphan-blob-1745678901.jpg"
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<inventory exportType="full_database">
  <files>
    <file id="` + orphanFileUUID + `">
      <linkedEntityType>commodity</linkedEntityType>
      <linkedEntityId>` + unknownCommodityUUID + `</linkedEntityId>
      <linkedEntityMeta>images</linkedEntityMeta>
      <type>image</type>
      <category>photos</category>
      <path>orphan-blob</path>
      <originalPath>` + orphanBlobKey + `</originalPath>
      <extension>.jpg</extension>
      <mimeType>image/jpeg</mimeType>
      <data>b3JwaGFu</data>
    </file>
  </files>
</inventory>`

	proc := processor.NewRestoreOperationProcessor(
		"restore-1485-orphan",
		dst.factorySet,
		services.NewEntityService(dst.factorySet, uploadLocation),
		uploadLocation,
	)
	stats, err := proc.RestoreFromXML(dst.ctx, strings.NewReader(xmlContent), types.RestoreOptions{
		Strategy:        types.RestoreStrategyMergeAdd,
		IncludeFileData: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount > 0, qt.IsTrue, qt.Commentf("expected an error for the unresolved linked-entity reference"))

	// Bucket must NOT contain the orphan blob.
	bucket := must.Must(blob.OpenBucket(c.Context(), uploadLocation))
	defer bucket.Close()
	exists, err := bucket.Exists(c.Context(), orphanBlobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse, qt.Commentf("blob must not be written when linked-entity resolution fails"))
}

// Compile-time guard: keep us honest about the shape of types.RestoreStats.
var _ = exporttypes.ExportStats{FileCount: 0}
