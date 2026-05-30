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

	exportpkg "github.com/denisvmedia/inventario/backup/export"
	importpkg "github.com/denisvmedia/inventario/backup/import"
	"github.com/denisvmedia/inventario/internal/backupsign"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register the file:// blob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// TestINBBackupRoundTripPostgres is a regression test for the postgres-only
// import defect fixed in PR #1951 (#534, signed `.inb` backups): the import
// worker created the imported backup's FileEntity without a group in context,
// so the group-scoped PostgreSQL file registry rejected the insert with
// "group ID is required" and the imported export was marked Failed.
//
// The in-memory registry does NOT enforce group_id, so every memory-backed unit
// test passed while the postgres CI/e2e lanes failed — only a postgres-backed
// round trip catches it. It runs a real export -> re-import on PostgreSQL and
// asserts the imported export reaches Completed (pre-fix: Failed). It skips when
// POSTGRES_TEST_DSN is unset (see issue #1953 for making this run in-process).
func TestINBBackupRoundTripPostgres(t *testing.T) {
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

	// --- Tenant + user + group (the FileEntity the import creates is
	// group-scoped, so a real group must exist for the import to succeed). ---
	tenant, err := factorySet.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "INB RoundTrip " + uniq,
		Slug:   "inb-roundtrip-" + uniq,
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	user, err := factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "inb-" + uniq + "@test.com",
		Name:                "INB User",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	group, err := factorySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "INB Group",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
	})
	c.Assert(err, qt.IsNil)

	// --- Signing key + services (export and import must share the key so the
	// import verifies the signature the export produced). ---
	signer, err := backupsign.NewSigner(make([]byte, backupsign.SeedSize))
	c.Assert(err, qt.IsNil)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	exportSvc := exportpkg.NewExportService(factorySet, uploadLocation, signer)
	importSvc := importpkg.NewImportService(factorySet, uploadLocation, signer)

	srv := factorySet.CreateServiceRegistrySet()

	// --- Produce a signed `.inb` via the export worker path. ---
	exportRec, err := srv.ExportRegistry.Create(ctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", tenant.ID, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusPending,
		Description:              "inb round-trip export",
	})
	c.Assert(err, qt.IsNil)

	c.Assert(exportSvc.ProcessExport(ctx, exportRec.ID), qt.IsNil)

	exportRec, err = srv.ExportRegistry.Get(ctx, exportRec.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(exportRec.Status, qt.Equals, models.ExportStatusCompleted)
	c.Assert(exportRec.FilePath, qt.Not(qt.Equals), "")

	// --- Re-import that exact `.inb`. This is the regression point: before the
	// fix the import worker created the backing FileEntity with no group in
	// context and the postgres registry rejected it ("group ID is required"),
	// marking the import Failed. ---
	importRec, err := srv.ExportRegistry.Create(ctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", tenant.ID, group.ID, user.ID),
		Type:                     models.ExportTypeImported,
		Status:                   models.ExportStatusPending,
		Imported:                 true,
		FilePath:                 exportRec.FilePath,
		Description:              "inb round-trip import",
	})
	c.Assert(err, qt.IsNil)

	err = importSvc.ProcessImport(ctx, importRec.ID, exportRec.FilePath)
	c.Assert(err, qt.IsNil, qt.Commentf("import must succeed on postgres (group injected into context)"))

	importRec, err = srv.ExportRegistry.Get(ctx, importRec.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(importRec.Status, qt.Equals, models.ExportStatusCompleted,
		qt.Commentf("imported export must reach Completed; Failed here means the group-scoped FileEntity insert was rejected"))
	c.Assert(importRec.FileID, qt.IsNotNil, qt.Commentf("a backing FileEntity must have been created for the import"))
}
