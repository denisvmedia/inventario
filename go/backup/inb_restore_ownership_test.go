//go:build !legacy_xml_backup

package backup_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/restore"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// restoreAsOtherUser restores the archive at blobKey into a group of a second
// user of the fixture's tenant. The archive's items keep their UUIDs, and the
// fixture's user created those items, so each one fails the ownership check.
func restoreAsOtherUser(c *qt.C, f *inbFixture, blobKey string) *models.RestoreOperation {
	c.Helper()
	ctx := context.Background()
	other := must.Must(f.fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Email:               "other@example.com",
		Name:                "Other User",
		IsActive:            true,
	}))
	group := must.Must(f.fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Other Group",
		Status:              models.LocationGroupStatusActive,
		GroupCurrency:       "USD",
		CreatedBy:           other.ID,
	}))
	octx := appctx.WithGroup(appctx.WithUser(ctx, other), group)

	expReg := must.Must(f.fs.ExportRegistryFactory.CreateUserRegistry(octx))
	exportRow := must.Must(expReg.Create(octx, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: group.ID, CreatedByUserID: other.ID},
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusCompleted,
		Description:              "someone else's backup",
		FilePath:                 blobKey,
	}))
	roReg := f.fs.RestoreOperationRegistryFactory.CreateServiceRegistry()
	op := must.Must(roReg.Create(octx, models.RestoreOperation{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: group.ID, CreatedByUserID: other.ID},
		ExportID:                 exportRow.ID,
		Status:                   models.RestoreStatusPending,
		Options:                  models.RestoreOptions{Strategy: string(types.RestoreStrategyMergeAdd)},
	}))

	svc := restore.NewRestoreService(f.fs, services.NewEntityService(f.fs, inbUploadLocation), inbUploadLocation, testSigner(c))
	c.Assert(svc.ProcessRestoreOperation(octx, op.ID, inbUploadLocation), qt.IsNil)
	return must.Must(roReg.Get(octx, op.ID))
}

// An item another user created fails the ownership check, and the restore
// counts that failure once (#3075).
func TestINBRestore_ForeignItemIsOneError(t *testing.T) {
	c := qt.New(t)
	f := newInbFixture(c) // one area-bound item
	blobKey, _ := f.runExport(c, testSigner(c))

	final := restoreAsOtherUser(c, f, blobKey)

	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("error: %s", final.ErrorMessage))
	c.Assert(final.CommodityCount, qt.Equals, 0)
	c.Assert(final.ErrorCount, qt.Equals, 1)
}

// The same holds for an item without an area, which an archive keeps in its
// own member and the restore walks separately.
func TestINBRestore_ForeignUnassignedItemIsOneError(t *testing.T) {
	c := qt.New(t)
	f := newInbFixture(c)
	f.addUnassignedCommodity(c, "Loose Gadget")
	blobKey, _ := f.runExport(c, testSigner(c))

	final := restoreAsOtherUser(c, f, blobKey)

	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("error: %s", final.ErrorMessage))
	c.Assert(final.CommodityCount, qt.Equals, 0)
	// The area-bound item and the unassigned one, one error each.
	c.Assert(final.ErrorCount, qt.Equals, 2)
}
