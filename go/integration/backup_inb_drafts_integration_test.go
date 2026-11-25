//go:build !legacy_xml_backup

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/appctx"
	exportpkg "go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/backup/restore"
	restoretypes "go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/internal/backupsign"
	_ "go.5x5.cz/inventario/internal/fileblob" // register the file:// blob driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/services"
)

// TestINBDraftsRoundTripPostgres is the postgres half of the drafts round trip
// (#2812, format 3.5): an open draft with its area, seller, tags and a photo
// survives a full replace in its group, and a merge into another group of the
// tenant links the draft there to that group's copy of the photo and to the
// area and seller the restore made there, leaving the source group alone.
// Skips when POSTGRES_TEST_DSN is unset.
func TestINBDraftsRoundTripPostgres(t *testing.T) {
	c := qt.New(t)
	dsn := testDSN(t)
	c.Assert(setupFreshDatabase(dsn), qt.IsNil)

	registrySetFunc, cleanup := postgres.NewPostgresRegistrySet()
	defer cleanup()
	fs, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil)

	ctx := context.Background()
	uniq := fmt.Sprintf("%d", time.Now().UnixNano())
	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	tenant, user, group := seedFidelityTenant(c, fs, ctx, uniq)
	uctx := appctx.WithGroup(appctx.WithUser(ctx, user), group)

	src := seedDraftForBackup(c, fs, uctx, uploadLocation, tenant.ID, group.ID, user.ID)

	signer := must.Must(backupsign.NewSigner(make([]byte, backupsign.SeedSize)))
	exportSvc := exportpkg.NewExportService(fs, uploadLocation, signer)
	srv := fs.CreateServiceRegistrySet()
	exportRec := must.Must(srv.ExportRegistry.Create(ctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", tenant.ID, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusPending,
		Description:              "drafts export",
	}))
	c.Assert(exportSvc.ProcessExport(ctx, exportRec.ID), qt.IsNil)
	exportRec = must.Must(srv.ExportRegistry.Get(ctx, exportRec.ID))
	c.Assert(exportRec.Status, qt.Equals, models.ExportStatusCompleted)

	// Full replace in the same group.
	final := runDraftRestore(c, fs, signer, uploadLocation, uctx, exportRec.FilePath, restoretypes.RestoreStrategyFullReplace)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("restore error: %s", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	draftsA := must.Must(fs.CommodityDraftRegistryFactory.CreateUserRegistry(uctx))
	restored := must.Must(draftsA.GetByClientKey(uctx, "pg-draft"))
	c.Assert(restored.ID, qt.Not(qt.Equals), src.draft.ID)
	c.Assert(restored.Revision, qt.Equals, 2)
	c.Assert(restored.CreatedAt.Equal(src.draft.CreatedAt.Truncate(time.Second)), qt.IsTrue,
		qt.Commentf("created %v, want %v", restored.CreatedAt, src.draft.CreatedAt))
	areaA := areaByName(c, fs, uctx, "Workshop")
	c.Assert(restored.Values, qt.DeepEquals, models.CommodityDraftValues{
		"name":      "Cordless drill",
		"area_id":   areaA.ID,
		"seller_id": src.seller.ID,
		"tags":      []any{"power-tools"},
		"type":      "equipment",
		"count":     float64(2),
	})
	filesA := must.Must(fs.FileRegistryFactory.CreateUserRegistry(uctx))
	photosA := must.Must(filesA.ListByLinkedEntity(uctx, models.FileLinkDraft, restored.ID))
	c.Assert(photosA, qt.HasLen, 1)
	c.Assert(photosA[0].Title, qt.Equals, "drill photo")
	c.Assert(photosA[0].Links, qt.HasLen, 1)

	// Merge into another group of the tenant.
	groupB := must.Must(fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Second Group",
		Status:              models.LocationGroupStatusActive,
		GroupCurrency:       models.Currency("USD"),
		CreatedBy:           user.ID,
	}))
	bctx := appctx.WithGroup(appctx.WithUser(ctx, user), groupB)
	final = runDraftRestore(c, fs, signer, uploadLocation, bctx, exportRec.FilePath, restoretypes.RestoreStrategyMergeAdd)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("restore error: %s", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	inB := must.Must(must.Must(fs.CommodityDraftRegistryFactory.CreateUserRegistry(bctx)).GetByClientKey(bctx, "pg-draft"))
	c.Assert(inB.GroupID, qt.Equals, groupB.ID)
	areaB := areaByName(c, fs, bctx, "Workshop")
	c.Assert(areaB.ID, qt.Not(qt.Equals), areaA.ID)
	sellerB := must.Must(must.Must(fs.SellerRegistryFactory.CreateUserRegistry(bctx)).GetByName(bctx, "Tool Shop"))
	c.Assert(sellerB.ID, qt.Not(qt.Equals), src.seller.ID)
	c.Assert(inB.Values["area_id"], qt.Equals, areaB.ID)
	c.Assert(inB.Values["seller_id"], qt.Equals, sellerB.ID)
	photosB := must.Must(must.Must(fs.FileRegistryFactory.CreateUserRegistry(bctx)).ListByLinkedEntity(bctx, models.FileLinkDraft, inB.ID))
	c.Assert(photosB, qt.HasLen, 1)
	c.Assert(photosB[0].GroupID, qt.Equals, groupB.ID)
	c.Assert(photosB[0].ID, qt.Not(qt.Equals), photosA[0].ID)

	// The first group's draft and photo are as they were.
	again := must.Must(filesA.Get(uctx, photosA[0].ID))
	c.Assert(again.Links, qt.HasLen, 1)
	c.Assert(again.Links[0].EntityID, qt.Equals, restored.ID)
	stillA := must.Must(draftsA.Get(uctx, restored.ID))
	c.Assert(stillA.Values["area_id"], qt.Equals, areaA.ID)
}

type draftBackupSource struct {
	draft  *models.CommodityDraft
	seller *models.Seller
}

// seedDraftForBackup creates a location, an area and a seller, and a draft
// saved twice that names them, with a photo whose blob is written.
func seedDraftForBackup(c *qt.C, fs *registry.FactorySet, uctx context.Context, uploadLocation, tenantID, groupID, userID string) draftBackupSource {
	c.Helper()
	ids := models.TenantGroupAwareEntityID{TenantID: tenantID, GroupID: groupID, CreatedByUserID: userID}
	loc := must.Must(must.Must(fs.LocationRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.Location{TenantGroupAwareEntityID: ids, Name: "Garage"}))
	area := must.Must(must.Must(fs.AreaRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.Area{TenantGroupAwareEntityID: ids, Name: "Workshop", LocationID: loc.ID}))
	seller := must.Must(must.Must(fs.SellerRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.Seller{Name: "Tool Shop"}))

	drafts := must.Must(fs.CommodityDraftRegistryFactory.CreateUserRegistry(uctx))
	draft, _ := must.Must2(drafts.CreateOrGet(uctx, models.CommodityDraft{ClientKey: "pg-draft", Values: models.CommodityDraftValues{"name": "Cordless drill"}}))
	draft = must.Must(drafts.SaveValues(uctx, draft.ID, 1, models.CommodityDraftValues{
		"name":      "Cordless drill",
		"area_id":   area.ID,
		"seller_id": seller.ID,
		"tags":      []any{"Power Tools"},
		"type":      "equipment",
		"count":     2,
	}))

	fileReg := must.Must(fs.FileRegistryFactory.CreateUserRegistry(uctx))
	photo := must.Must(fileReg.Create(uctx, models.FileEntity{
		TenantGroupAwareEntityID: ids,
		Title:                    "drill photo",
		Type:                     models.FileTypeImage,
		Category:                 models.FileCategoryImages,
		Tags:                     models.StringSlice{},
		Links:                    []models.FileLink{{EntityType: models.FileLinkDraft, EntityID: draft.ID, Meta: models.FileLinkMetaImages}},
		File:                     &models.File{Path: "drill", Ext: ".jpg", MIMEType: "image/jpeg"},
	}))
	photo.OriginalPath = "t/" + tenantID + "/files/" + photo.UUID + ".jpg"
	photo.File.SizeBytes = 512
	must.Must(fileReg.Update(uctx, *photo))
	bucket := must.Must(blob.OpenBucket(uctx, uploadLocation))
	defer bucket.Close()
	c.Assert(bucket.WriteAll(uctx, photo.OriginalPath, make([]byte, 512), nil), qt.IsNil)

	return draftBackupSource{draft: draft, seller: seller}
}

func areaByName(c *qt.C, fs *registry.FactorySet, uctx context.Context, name string) *models.Area {
	c.Helper()
	for _, a := range must.Must(must.Must(fs.AreaRegistryFactory.CreateUserRegistry(uctx)).List(uctx)) {
		if a.Name == name {
			return a
		}
	}
	c.Fatalf("area %q not found", name)
	return nil
}

// runDraftRestore restores filePath into the group of uctx with strategy and
// returns the restore operation's final state.
func runDraftRestore(c *qt.C, fs *registry.FactorySet, signer *backupsign.Signer, uploadLocation string, uctx context.Context, filePath string, strategy restoretypes.RestoreStrategy) *models.RestoreOperation {
	c.Helper()
	user := appctx.UserFromContext(uctx)
	group := appctx.GroupFromContext(uctx)
	svc := restore.NewRestoreService(fs, services.NewEntityService(fs, uploadLocation), uploadLocation, signer)

	exportRow := must.Must(must.Must(fs.ExportRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", user.TenantID, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusCompleted,
		Description:              "drafts restore source",
		FilePath:                 filePath,
	}))
	roReg := must.Must(fs.RestoreOperationRegistryFactory.CreateUserRegistry(uctx))
	op := must.Must(roReg.Create(uctx, models.RestoreOperation{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", user.TenantID, group.ID, user.ID),
		ExportID:                 exportRow.ID,
		Status:                   models.RestoreStatusPending,
		Description:              "drafts restore",
		Options:                  models.RestoreOptions{Strategy: string(strategy)},
	}))
	c.Assert(svc.ProcessRestoreOperation(uctx, op.ID, uploadLocation), qt.IsNil)
	return must.Must(roReg.Get(uctx, op.ID))
}
