//go:build !legacy_xml_backup

package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	exportpkg "github.com/denisvmedia/inventario/backup/export"
	"github.com/denisvmedia/inventario/backup/restore"
	restoretypes "github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/backupsign"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register the file:// blob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
)

// TestINBFieldFidelityRoundTripPostgres is the lossless-commodity-round-trip
// guard for #534: a commodity with EVERY extended field populated — warranty
// (#1554), terminal-status metadata (#1611), the write-once acquisition pair
// (#202), and the user-picked cover photo (#1451) — must survive an
// export → restore cycle byte-for-byte.
//
// It runs against a REAL PostgreSQL instance because the acquisition columns are
// only writable through the restore-only registry.WithRestoreAcquisition context
// seam (and guarded by a both-or-neither CHECK), and the cover_file_id FK only
// exists in the SQL schema — the memory backend can't exercise either faithfully.
//
// The test deliberately wipes the source data and restores via full_replace into
// the same tenant/group (a clean target), then re-reads the restored commodity
// by its immutable UUID and asserts every field round-trips. It is designed to
// FAIL loudly if any single field is dropped on either the export or restore
// side. Skips when POSTGRES_TEST_DSN is unset.
func TestINBFieldFidelityRoundTripPostgres(t *testing.T) {
	c := qt.New(t)

	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
		return
	}

	c.Assert(setupFreshDatabase(dsn), qt.IsNil, qt.Commentf("failed to set up fresh database"))

	registrySetFunc, cleanup := postgres.NewPostgresRegistrySet()
	defer cleanup()
	factorySet, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil)

	ctx := context.Background()
	uniq := fmt.Sprintf("%d", time.Now().UnixNano())
	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"

	tenant, user, group := seedFidelityTenant(c, factorySet, ctx, uniq)
	uctx := appctx.WithGroup(appctx.WithUser(ctx, user), group)

	// --- Seed a SOURCE commodity with EVERY field populated. ---
	src := seedFullCommodity(c, factorySet, uctx, tenant.ID, group.ID, user.ID)
	attachFidelityCover(c, factorySet, uctx, uploadLocation, tenant.ID, group.ID, user.ID, src)

	// Capture the SOURCE state by name before the wipe, so the assertions compare
	// against a snapshot independent of DB ids / regenerated UUIDs.
	comReg := must.Must(factorySet.CommodityRegistryFactory.CreateUserRegistry(uctx))
	srcReloaded := must.Must(comReg.Get(uctx, src.ID))
	c.Assert(srcReloaded.AcquisitionPrice, qt.IsNotNil, qt.Commentf("seed must set acquisition via the restore context seam"))
	c.Assert(srcReloaded.CoverFileID, qt.IsNotNil, qt.Commentf("seed must set the cover photo"))

	// --- Export (full_database) the signed .inb. ---
	signer := must.Must(backupsign.NewSigner(make([]byte, backupsign.SeedSize)))
	exportSvc := exportpkg.NewExportService(factorySet, uploadLocation, signer)
	srv := factorySet.CreateServiceRegistrySet()

	exportRec := must.Must(srv.ExportRegistry.Create(ctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", tenant.ID, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusPending,
		Description:              "field-fidelity export",
	}))
	c.Assert(exportSvc.ProcessExport(ctx, exportRec.ID), qt.IsNil)
	exportRec = must.Must(srv.ExportRegistry.Get(ctx, exportRec.ID))
	c.Assert(exportRec.Status, qt.Equals, models.ExportStatusCompleted)
	c.Assert(exportRec.FilePath, qt.Not(qt.Equals), "")

	// --- Restore via full_replace into the same tenant/group (a clean target:
	// full_replace wipes the source data first, then recreates everything from
	// the archive). ---
	final := runFidelityRestore(c, factorySet, signer, uploadLocation, uctx, tenant, group, user, exportRec.FilePath)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("restore error: %s", final.ErrorMessage))
	// --- Re-read the restored commodity and assert every field round-tripped.
	// full_replace wiped the source and recreated exactly one commodity. The
	// postgres registry mints a fresh server-side id/UUID on create, so the
	// lookup is by the (unique-in-this-test) name, not the source UUID. ---
	restored := commodityByName(c, comReg, uctx, srcReloaded.Name)

	// Core fields.
	c.Assert(restored.ShortName, qt.Equals, srcReloaded.ShortName)
	c.Assert(restored.Type, qt.Equals, srcReloaded.Type)
	c.Assert(restored.Count, qt.Equals, srcReloaded.Count)
	c.Assert(restored.SerialNumber, qt.Equals, srcReloaded.SerialNumber)
	c.Assert(restored.Status, qt.Equals, srcReloaded.Status)
	c.Assert(restored.OriginalPriceCurrency, qt.Equals, srcReloaded.OriginalPriceCurrency)
	c.Assert(restored.OriginalPrice.Equal(srcReloaded.OriginalPrice), qt.IsTrue)
	c.Assert(restored.CurrentPrice.Equal(srcReloaded.CurrentPrice), qt.IsTrue)
	c.Assert(restored.Comments, qt.Equals, srcReloaded.Comments)
	c.Assert([]string(restored.ExtraSerialNumbers), qt.DeepEquals, []string(srcReloaded.ExtraSerialNumbers))
	c.Assert([]string(restored.PartNumbers), qt.DeepEquals, []string(srcReloaded.PartNumbers))
	c.Assert(restored.PurchaseDate, qt.IsNotNil)
	c.Assert(string(*restored.PurchaseDate), qt.Equals, string(*srcReloaded.PurchaseDate))

	// Extended scalar fields (#534).
	c.Assert(restored.WarrantyExpiresAt, qt.IsNotNil)
	c.Assert(string(*restored.WarrantyExpiresAt), qt.Equals, string(*srcReloaded.WarrantyExpiresAt))
	c.Assert(restored.WarrantyNotes, qt.Equals, srcReloaded.WarrantyNotes)
	c.Assert(restored.StatusDate, qt.IsNotNil)
	c.Assert(string(*restored.StatusDate), qt.Equals, string(*srcReloaded.StatusDate))
	c.Assert(restored.StatusNote, qt.Equals, srcReloaded.StatusNote)
	c.Assert(restored.SalePrice, qt.IsNotNil, qt.Commentf("salePrice must round-trip"))
	c.Assert(restored.SalePrice.Equal(*srcReloaded.SalePrice), qt.IsTrue)

	// Acquisition pair (#202) — restored verbatim via the seam.
	c.Assert(restored.AcquisitionPrice, qt.IsNotNil, qt.Commentf("acquisitionPrice must round-trip via the seam"))
	c.Assert(restored.AcquisitionPrice.Equal(*srcReloaded.AcquisitionPrice), qt.IsTrue)
	c.Assert(restored.AcquisitionCurrency, qt.IsNotNil)
	c.Assert(*restored.AcquisitionCurrency, qt.Equals, *srcReloaded.AcquisitionCurrency)

	// Cover photo (#1451) — restored to a NEW file DB id. The id resolves to a
	// real image file linked to THIS restored commodity (the cross-reference was
	// re-resolved through the restore's id mapping after the file was recreated).
	c.Assert(restored.CoverFileID, qt.IsNotNil, qt.Commentf("cover photo must round-trip"))
	fileReg := must.Must(factorySet.FileRegistryFactory.CreateUserRegistry(uctx))
	restoredCover := must.Must(fileReg.Get(uctx, *restored.CoverFileID))
	c.Assert(restoredCover.LinkedEntityType, qt.Equals, "commodity")
	c.Assert(restoredCover.LinkedEntityID, qt.Equals, restored.ID,
		qt.Commentf("restored cover must be linked to the restored commodity"))
	c.Assert(restoredCover.LinkedEntityMeta, qt.Equals, "images")
}

// seedFidelityTenant creates a tenant + active user + USD group. GroupCurrency is
// stamped explicitly so commodity validation (which reads it off the context)
// passes regardless of DB defaults.
func seedFidelityTenant(c *qt.C, fs *registry.FactorySet, ctx context.Context, uniq string) (*models.Tenant, *models.User, *models.LocationGroup) {
	c.Helper()
	tenant := must.Must(fs.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "INB Fidelity " + uniq,
		Slug:   "inb-fidelity-" + uniq,
		Status: models.TenantStatusActive,
	}))
	user := must.Must(fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "fidelity-" + uniq + "@test.com",
		Name:                "Fidelity User",
		IsActive:            true,
	}))
	group := must.Must(fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Fidelity Group",
		Status:              models.LocationGroupStatusActive,
		GroupCurrency:       models.Currency("USD"),
		CreatedBy:           user.ID,
	}))
	return tenant, user, group
}

// seedFullCommodity creates a location → area → commodity with every commodity
// field populated, then sets the write-once acquisition pair via the restore-only
// seam (the only legitimate writer reachable from a test). Returns the commodity.
func seedFullCommodity(c *qt.C, fs *registry.FactorySet, uctx context.Context, tenantID, groupID, userID string) *models.Commodity {
	c.Helper()
	locReg := must.Must(fs.LocationRegistryFactory.CreateUserRegistry(uctx))
	loc := must.Must(locReg.Create(uctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: tenantID, GroupID: groupID, CreatedByUserID: userID},
		Name:                     "Fidelity Home",
		Address:                  "1 Fidelity St",
	}))
	areaReg := must.Must(fs.AreaRegistryFactory.CreateUserRegistry(uctx))
	area := must.Must(areaReg.Create(uctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: tenantID, GroupID: groupID, CreatedByUserID: userID},
		Name:                     "Fidelity Room",
		LocationID:               loc.ID,
	}))

	comReg := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(uctx))
	salePrice := decimal.RequireFromString("120.00")
	// Acquisition pair (#202) is server-managed; the source row gets it the same
	// way the restore does — through the trusted WithRestoreAcquisition context
	// seam at create time (there is no public registry setter to bypass).
	acqPrice := decimal.RequireFromString("499.99")
	acqCurrency := models.Currency("USD")
	seedCtx := registry.WithRestoreAcquisition(uctx, acqPrice, acqCurrency)
	com := must.Must(comReg.Create(seedCtx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: tenantID, GroupID: groupID, CreatedByUserID: userID},
		Name:                     "Vintage Camera",
		ShortName:                "camera",
		Type:                     models.CommodityTypeElectronics,
		AreaID:                   area.ID,
		Count:                    1,
		OriginalPrice:            decimal.RequireFromString("499.99"),
		OriginalPriceCurrency:    models.Currency("USD"),
		ConvertedOriginalPrice:   decimal.Zero, // USD == group currency → must be zero
		CurrentPrice:             decimal.RequireFromString("250.00"),
		SerialNumber:             "SN-FIDELITY-1",
		ExtraSerialNumbers:       models.ValuerSlice[string]{"SN-EXTRA-1"},
		PartNumbers:              models.ValuerSlice[string]{"PN-1"},
		Status:                   models.CommodityStatusSold,
		PurchaseDate:             models.ToPDate("2020-01-15"),
		RegisteredDate:           models.ToPDate("2020-01-16"),
		Comments:                 "fully populated commodity",
		// Terminal-status metadata (#1611): status=sold so SalePrice + StatusDate
		// are valid.
		StatusDate: models.ToPDate("2024-06-01"),
		StatusNote: "Sold to a collector",
		SalePrice:  &salePrice,
		// Warranty (#1554): allowed because Count == 1.
		WarrantyExpiresAt: models.ToPDate("2026-12-31"),
		WarrantyNotes:     "Two-year manufacturer warranty",
	}))

	return com
}

// attachFidelityCover creates an image FileEntity linked to the commodity, writes
// its blob, and patches the commodity's CoverFileID to that file so the cover
// cross-reference is exercised by the round-trip.
func attachFidelityCover(c *qt.C, fs *registry.FactorySet, uctx context.Context, uploadLocation, tenantID, groupID, userID string, com *models.Commodity) {
	c.Helper()
	fileReg := must.Must(fs.FileRegistryFactory.CreateUserRegistry(uctx))
	created := must.Must(fileReg.Create(uctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: tenantID, GroupID: groupID, CreatedByUserID: userID},
		Title:                    "Cover photo",
		Type:                     models.FileTypeFromMIME("image/jpeg"),
		Category:                 models.FileCategoryFromContext("commodity", "images", "image/jpeg"),
		Tags:                     models.StringSlice{},
		LinkedEntityType:         "commodity",
		LinkedEntityID:           com.ID,
		LinkedEntityMeta:         "images",
		File: &models.File{
			Path:     "cover-photo",
			Ext:      ".jpg",
			MIMEType: "image/jpeg",
		},
	}))

	blobKey := "t/" + tenantID + "/files/" + created.UUID + ".jpg"
	created.OriginalPath = blobKey
	created.File.SizeBytes = 2048
	must.Must(fileReg.Update(uctx, *created))

	bucket := must.Must(blob.OpenBucket(uctx, uploadLocation))
	defer bucket.Close()
	c.Assert(bucket.WriteAll(uctx, blobKey, make([]byte, 2048), nil), qt.IsNil)

	// Patch the commodity's cover to this file's DB id (normal Update path —
	// CoverFileID is an ordinary column).
	comReg := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(uctx))
	reloaded := must.Must(comReg.Get(uctx, com.ID))
	coverID := created.ID
	reloaded.CoverFileID = &coverID
	must.Must(comReg.Update(uctx, *reloaded))
}

// commodityByName returns the single commodity with the given (unique-in-test)
// name from the user-scoped registry, failing the test if absent.
func commodityByName(c *qt.C, comReg registry.CommodityRegistry, uctx context.Context, name string) *models.Commodity {
	c.Helper()
	for _, com := range must.Must(comReg.List(uctx)) {
		if com != nil && com.Name == name {
			return com
		}
	}
	c.Fatalf("commodity %q not found after restore", name)
	return nil
}

// runFidelityRestore drives a full_replace restore of the given .inb file and
// returns the restore operation's final state. The export + restore-operation
// rows are created through USER-context registries (uctx carries the user/group)
// so the postgres registry stamps the real tenant — the service registry would
// override tenant_id to "" and violate fk_entity_tenant.
func runFidelityRestore(c *qt.C, fs *registry.FactorySet, signer *backupsign.Signer, uploadLocation string, uctx context.Context, tenant *models.Tenant, group *models.LocationGroup, user *models.User, filePath string) *models.RestoreOperation {
	c.Helper()
	entityService := services.NewEntityService(fs, uploadLocation)
	svc := restore.NewRestoreService(fs, entityService, uploadLocation, signer)

	expReg := must.Must(fs.ExportRegistryFactory.CreateUserRegistry(uctx))
	exportRow := must.Must(expReg.Create(uctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", tenant.ID, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusCompleted,
		Description:              "restore source",
		FilePath:                 filePath,
	}))
	roReg := must.Must(fs.RestoreOperationRegistryFactory.CreateUserRegistry(uctx))
	restoreOp := must.Must(roReg.Create(uctx, models.RestoreOperation{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", tenant.ID, group.ID, user.ID),
		ExportID:                 exportRow.ID,
		Status:                   models.RestoreStatusPending,
		Description:              "field-fidelity restore",
		Options:                  models.RestoreOptions{Strategy: string(restoretypes.RestoreStrategyFullReplace)},
	}))

	c.Assert(svc.ProcessRestoreOperation(uctx, restoreOp.ID, uploadLocation), qt.IsNil)
	return must.Must(roReg.Get(uctx, restoreOp.ID))
}
