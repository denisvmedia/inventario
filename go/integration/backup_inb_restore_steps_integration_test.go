//go:build !legacy_xml_backup

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	exportpkg "go.5x5.cz/inventario/backup/export"
	restoretypes "go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/internal/backupsign"
	_ "go.5x5.cz/inventario/internal/fileblob" // register the file:// blob driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// TestINBRestoreRecordsStepsPostgres checks that a restore on postgres writes
// its steps, in the restore operation's tenant and group, so the group's
// members can read them. A step row needs a tenant and a group, and the
// restore worker writes steps without RLS, so nothing else fills them in.
func TestINBRestoreRecordsStepsPostgres(t *testing.T) {
	c := qt.New(t)
	fs, uctx, uploadLocation := newRestoreStepsFixture(c)
	user := appctx.UserFromContext(uctx)
	group := appctx.GroupFromContext(uctx)

	ids := models.TenantGroupAwareEntityID{TenantID: user.TenantID, GroupID: group.ID, CreatedByUserID: user.ID}
	loc := must.Must(must.Must(fs.LocationRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.Location{TenantGroupAwareEntityID: ids, Name: "Home"}))
	must.Must(must.Must(fs.AreaRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.Area{TenantGroupAwareEntityID: ids, Name: "Shelf", LocationID: loc.ID}))

	signer := must.Must(backupsign.NewSigner(make([]byte, backupsign.SeedSize)))
	srv := fs.CreateServiceRegistrySet()
	exportRec := must.Must(srv.ExportRegistry.Create(uctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", user.TenantID, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusPending,
		Description:              "restore steps export",
	}))
	c.Assert(exportpkg.NewExportService(fs, uploadLocation, signer).ProcessExport(uctx, exportRec.ID), qt.IsNil)
	exportRec = must.Must(srv.ExportRegistry.Get(uctx, exportRec.ID))
	c.Assert(exportRec.Status, qt.Equals, models.ExportStatusCompleted)

	final := runINBRestore(c, fs, signer, uploadLocation, uctx, exportRec.FilePath, models.RestoreOptions{Strategy: string(restoretypes.RestoreStrategyFullReplace)})
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("restore error: %s", final.ErrorMessage))

	results := restoreStepResults(c, final)
	c.Check(results["Initializing restore"], qt.Equals, models.RestoreStepResultSuccess)
	c.Check(results["Reading backup file"], qt.Equals, models.RestoreStepResultSuccess)
	c.Check(results["Restore completed successfully"], qt.Equals, models.RestoreStepResultSuccess)
	for _, step := range final.Steps {
		c.Check(step.TenantID, qt.Equals, final.TenantID, qt.Commentf("step %q", step.Name))
		c.Check(step.GroupID, qt.Equals, final.GroupID, qt.Commentf("step %q", step.Name))
		c.Check(step.CreatedByUserID, qt.Equals, final.CreatedByUserID, qt.Commentf("step %q", step.Name))
	}
}

// TestINBRestoreRecordsFailureStepPostgres checks that a restore that fails
// on postgres records the step that says why.
func TestINBRestoreRecordsFailureStepPostgres(t *testing.T) {
	c := qt.New(t)
	fs, uctx, uploadLocation := newRestoreStepsFixture(c)
	user := appctx.UserFromContext(uctx)
	signer := must.Must(backupsign.NewSigner(make([]byte, backupsign.SeedSize)))

	missing := "t/" + user.TenantID + "/exports/missing.inb"
	final := runINBRestore(c, fs, signer, uploadLocation, uctx, missing, models.RestoreOptions{Strategy: string(restoretypes.RestoreStrategyMergeAdd)})
	c.Assert(final.Status, qt.Equals, models.RestoreStatusFailed)

	results := restoreStepResults(c, final)
	c.Check(results["Initializing restore"], qt.Equals, models.RestoreStepResultInProgress)
	c.Check(results["Restore failed"], qt.Equals, models.RestoreStepResultError)
}

func newRestoreStepsFixture(c *qt.C) (*registry.FactorySet, context.Context, string) {
	c.Helper()
	dsn := testDSN(c)
	c.Assert(setupFreshDatabase(dsn), qt.IsNil)

	registrySetFunc, cleanup := postgres.NewPostgresRegistrySet()
	c.Cleanup(func() { _ = cleanup() })
	fs, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil)

	ctx := context.Background()
	uniq := fmt.Sprintf("%d", time.Now().UnixNano())
	_, user, group := seedFidelityTenant(c, fs, ctx, uniq)
	return fs, appctx.WithGroup(appctx.WithUser(ctx, user), group), "file://" + c.TempDir() + "?create_dir=1"
}

// restoreStepResults maps each step name of op to its result. The steps are
// the ones the registry loaded with op, through the group-scoped registry.
func restoreStepResults(c *qt.C, op *models.RestoreOperation) map[string]models.RestoreStepResult {
	c.Helper()
	c.Assert(op.Steps, qt.Not(qt.HasLen), 0, qt.Commentf("the restore must record its steps"))
	results := make(map[string]models.RestoreStepResult, len(op.Steps))
	for _, step := range op.Steps {
		results[step.Name] = step.Result
	}
	return results
}
