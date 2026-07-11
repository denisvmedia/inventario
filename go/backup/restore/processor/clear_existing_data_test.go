package processor

import (
	"context"
	"runtime"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // Register file driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// clearDataUploadLocation builds a file:// upload-location URL for the given
// temp dir, matching the OS-specific scheme the rest of the tests use.
func clearDataUploadLocation(tempDir string) string {
	if runtime.GOOS == "windows" {
		return "file:///" + tempDir + "?create_dir=1"
	}
	return "file://" + tempDir + "?create_dir=1"
}

// TestClearExistingData_SweepsAreaAndLocationLinkedFiles pins #2119 for the
// restore full_replace path: clearExistingData must remove files attached to
// areas and locations — rows AND physical blobs — via DeleteLocationRecursive
// plus the second all-files sweep, while export-linked files are preserved
// (the sweep explicitly skips linked_entity_type='export'). A regression that
// re-introduces a LinkedEntityType skip in either pass would leave restore
// full_replace orphaning area/location attachments again.
func TestClearExistingData_SweepsAreaAndLocationLinkedFiles(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := clearDataUploadLocation(tempDir)

	factorySet := memory.NewFactorySet()
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}
	user := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))
	ctx := appctx.WithUser(context.Background(), user)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID}))

	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()

	// Files attached to the location and the area (rows + blobs) — must be
	// swept by full_replace.
	locationBlobKey := "loc-doc.pdf"
	c.Assert(b.WriteAll(ctx, locationBlobKey, []byte("loc pdf"), nil), qt.IsNil)
	locationFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		LinkedEntityType: "location",
		LinkedEntityID:   location.ID,
		LinkedEntityMeta: "images",
		File: &models.File{
			Path:         "loc-doc",
			OriginalPath: locationBlobKey,
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	areaBlobKey := "area-doc.pdf"
	c.Assert(b.WriteAll(ctx, areaBlobKey, []byte("area pdf"), nil), qt.IsNil)
	areaFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		LinkedEntityType: "area",
		LinkedEntityID:   area.ID,
		LinkedEntityMeta: "images",
		File: &models.File{
			Path:         "area-doc",
			OriginalPath: areaBlobKey,
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	// A standalone file (no linked entity) — the second sweep must take it.
	standaloneBlobKey := "standalone.pdf"
	c.Assert(b.WriteAll(ctx, standaloneBlobKey, []byte("standalone pdf"), nil), qt.IsNil)
	standaloneFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		File: &models.File{
			Path:         "standalone",
			OriginalPath: standaloneBlobKey,
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	// An export-linked file — must SURVIVE (backups are not user inventory).
	exportBlobKey := "export.xml"
	c.Assert(b.WriteAll(ctx, exportBlobKey, []byte("<export/>"), nil), qt.IsNil)
	exportFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		LinkedEntityType: "export",
		LinkedEntityID:   "test-export-id",
		LinkedEntityMeta: "xml-1.0",
		File: &models.File{
			Path:         "export",
			OriginalPath: exportBlobKey,
			Ext:          ".xml",
			MIMEType:     "application/xml",
		},
	}))

	entityService := services.NewEntityService(factorySet, uploadLocation)
	proc := NewRestoreOperationProcessor("test-restore-op", factorySet, entityService, uploadLocation, nil)

	c.Assert(proc.clearExistingData(ctx), qt.IsNil)

	// The location and area rows are gone.
	_, err := registrySet.LocationRegistry.Get(ctx, location.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = registrySet.AreaRegistry.Get(ctx, area.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The location-, area-linked and standalone files are gone — rows and blobs.
	for _, fileID := range []string{locationFile.ID, areaFile.ID, standaloneFile.ID} {
		_, err = registrySet.FileRegistry.Get(ctx, fileID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	}
	for _, key := range []string{locationBlobKey, areaBlobKey, standaloneBlobKey} {
		c.Assert(must.Must(b.Exists(ctx, key)), qt.IsFalse,
			qt.Commentf("blob %s must be swept by full_replace", key))
	}

	// The export-linked file survives — row and blob.
	c.Assert(must.Must(registrySet.FileRegistry.Get(ctx, exportFile.ID)), qt.IsNotNil)
	c.Assert(must.Must(b.Exists(ctx, exportBlobKey)), qt.IsTrue)
}

// TestClearExistingData_SweepsAreaLessCommodities pins #2236: commodity.area_id is
// nullable since #1986, so an area-less commodity is NOT reachable through the
// location → area recursion. Before the direct commodity enumeration, its row
// survived a full_replace wipe while the type-agnostic file sweep still deleted
// its attachments — leaving a zombie item with no files, whose preserved UUID then
// collided with the archive's own copy on re-create. full_replace must really mean
// full replace: after clearExistingData no commodity of any kind may remain.
func TestClearExistingData_SweepsAreaLessCommodities(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := clearDataUploadLocation(tempDir)

	factorySet := memory.NewFactorySet()
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}
	user := must.Must(factorySet.UserRegistry.Create(context.Background(), testUser))
	ctx := appctx.WithUser(context.Background(), user)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID}))

	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()

	// An area-BOUND commodity (reached by the location recursion) and an
	// area-LESS one (#1986 — reachable only by the direct enumeration).
	boundCommodity := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Bound TV",
		ShortName:             "btv",
		Type:                  models.CommodityTypeElectronics,
		AreaID:                new(area.ID),
		Count:                 1,
		Status:                models.CommodityStatusInUse,
		OriginalPriceCurrency: "USD",
		Draft:                 true,
	}))
	looseCommodity := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:      "Loose Gadget",
		ShortName: "lg",
		Type:      models.CommodityTypeElectronics,
		// AreaID left nil — the #2236 survivor.
		Count:                 1,
		Status:                models.CommodityStatusInUse,
		OriginalPriceCurrency: "USD",
		Draft:                 true,
	}))
	c.Assert(looseCommodity.AreaID, qt.IsNil)

	looseBlobKey := "loose-photo.jpg"
	c.Assert(b.WriteAll(ctx, looseBlobKey, []byte("loose jpeg"), nil), qt.IsNil)
	looseFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		Type:             models.FileTypeImage,
		Category:         models.FileCategoryImages,
		LinkedEntityType: "commodity",
		LinkedEntityID:   looseCommodity.ID,
		LinkedEntityMeta: "images",
		File: &models.File{
			Path:         "loose-photo",
			OriginalPath: looseBlobKey,
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))

	entityService := services.NewEntityService(factorySet, uploadLocation)
	proc := NewRestoreOperationProcessor("test-restore-op", factorySet, entityService, uploadLocation, nil)

	c.Assert(proc.clearExistingData(ctx), qt.IsNil)

	// No commodity of either kind survives.
	c.Assert(must.Must(registrySet.CommodityRegistry.List(ctx)), qt.HasLen, 0)
	_, err := registrySet.CommodityRegistry.Get(ctx, boundCommodity.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = registrySet.CommodityRegistry.Get(ctx, looseCommodity.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The area-less commodity's file went with it — row and blob.
	_, err = registrySet.FileRegistry.Get(ctx, looseFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(must.Must(b.Exists(ctx, looseBlobKey)), qt.IsFalse)
}
