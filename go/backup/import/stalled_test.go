package importpkg_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"

	importpkg "go.5x5.cz/inventario/backup/import"
	"go.5x5.cz/inventario/backup/stall"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// stalledImport stores an import whose run went quiet an hour ago, after
// runs runs, next to an export in the same state, which the import worker
// leaves to the export worker.
func stalledImport(c *qt.C, fs *registry.FactorySet, runs int) (imported, exported *models.Export) {
	c.Helper()
	reg := fs.ExportRegistryFactory.CreateServiceRegistry()
	create := func(id string, exportType models.ExportType) *models.Export {
		created, err := reg.Create(context.Background(), models.Export{
			TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID(id, "tenant-1", "group-1", "user-1"),
			Type:                     exportType,
			Imported:                 exportType == models.ExportTypeImported,
			Status:                   models.ExportStatusInProgress,
			Description:              "Uploaded backup",
			AttemptCount:             runs,
			CreatedDate:              models.NewPTimestamp(time.Now().Add(-2 * time.Hour)),
			HeartbeatAt:              models.NewPTimestamp(time.Now().Add(-time.Hour)),
		})
		c.Assert(err, qt.IsNil)
		return created
	}
	return create("stalled-import", models.ExportTypeImported), create("stalled-export", models.ExportTypeFullDatabase)
}

func TestImportService_RecoverStalled_RequeuesWhileRunsAreLeft(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	imported, exported := stalledImport(c, fs, 1)

	out, err := importpkg.NewImportService(fs, "nowhere://bucket", nil).RecoverStalled(ctx, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, registry.StalledJobs{Requeued: 1})

	reg := fs.ExportRegistryFactory.CreateServiceRegistry()
	got, err := reg.Get(ctx, imported.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.ExportStatusPending)
	got, err = reg.Get(ctx, exported.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.ExportStatusInProgress)
}

// The import worker sweeps for dead runs on its own: an import that used its
// runs fails with an error that says the worker stopped, counts once as
// dead-lettered and shows among the failed jobs.
func TestImportWorker_FailsAnImportWhoseWorkerStopped(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	imported, _ := stalledImport(c, fs, stall.MaxRuns)

	counter := metrics.JobsDeadLetteredFor(string(models.WorkerTypeImport))
	before := testutil.ToFloat64(counter)

	worker := importpkg.NewImportWorker(importpkg.NewImportService(fs, "nowhere://bucket", nil), fs, 1,
		importpkg.WithPollInterval(5*time.Millisecond))
	worker.Start(t.Context())
	defer worker.Stop()

	reg := fs.ExportRegistryFactory.CreateServiceRegistry()
	var got *models.Export
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		got, err = reg.Get(context.Background(), imported.ID)
		c.Assert(err, qt.IsNil)
		if got.Status != models.ExportStatusInProgress {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	c.Assert(got.Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(got.ErrorMessage, qt.Equals, importpkg.StalledImportError)
	c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, float64(1))

	failed, err := fs.FailedJobRegistry.ListFailed(context.Background(), models.WorkerTypeImport, registry.FailedJobScope{}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.HasLen, 1)
	c.Assert(failed[0].ID, qt.Equals, imported.ID)
}
