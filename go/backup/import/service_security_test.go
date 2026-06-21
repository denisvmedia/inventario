package importpkg_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	_ "gocloud.dev/blob/memblob" // register the memory:// blob driver

	importpkg "github.com/denisvmedia/inventario/backup/import"
	"github.com/denisvmedia/inventario/models"
)

// TestImportService_ProcessImport_ForeignTenantSourcePath_FailsFast is the
// defense-in-depth regression for the cross-tenant import gap: the import worker
// reads the source blob key WITHOUT RLS, and a signed `.inb` archive is verified
// against a tenant-AGNOSTIC server key. ProcessImport MUST fail fast — via
// markImportFailed, BEFORE opening the bucket — when sourceFilePath lives outside
// the export record's own tenant namespace.
//
// The export is seeded under "test-tenant"; the sourceFilePath points at another
// tenant's namespace. With the guard, the export is marked failed with the
// tenant-namespace message. Were the guard absent, the worker would instead read
// the bucket and fail with "failed to open uploaded backup file" — a distinct
// message — so this assertion proves the bucket is never touched.
func TestImportService_ProcessImport_ForeignTenantSourcePath_FailsFast(t *testing.T) {
	c := qt.New(t)

	factorySet, _, _ := newTestFactorySet()
	registrySet := factorySet.CreateServiceRegistrySet()
	service := importpkg.NewImportService(factorySet, "memory://test-bucket", nil)
	ctx := context.Background()

	createdExport := must.Must(registrySet.ExportRegistry.Create(ctx, models.Export{
		Type:                     models.ExportTypeImported,
		Status:                   models.ExportStatusPending,
		Description:              "Test import",
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "test-tenant"},
	}))

	// Another tenant's namespace — must be rejected without reading the bucket.
	err := service.ProcessImport(ctx, createdExport.ID, "t/other-tenant/exports/backup_full_database_20260101.inb")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "import source path is outside the export's tenant namespace")

	updatedExport := must.Must(registrySet.ExportRegistry.Get(ctx, createdExport.ID))
	c.Assert(updatedExport.Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(updatedExport.ErrorMessage, qt.Contains, "import source path is outside the export's tenant namespace")
}
