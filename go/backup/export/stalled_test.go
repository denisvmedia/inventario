package export_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/backup/stall"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// stalledExport stores an export whose run went quiet an hour ago, after
// runs runs.
func stalledExport(c *qt.C, fs *registry.FactorySet, id string, runs int) *models.Export {
	c.Helper()
	created, err := fs.ExportRegistryFactory.CreateServiceRegistry().Create(context.Background(), models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID(id, personalTenant, "group-1", "user-1"),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusInProgress,
		Description:              "Weekly backup",
		AttemptCount:             runs,
		CreatedDate:              models.NewPTimestamp(time.Now().Add(-2 * time.Hour)),
		HeartbeatAt:              models.NewPTimestamp(time.Now().Add(-time.Hour)),
	})
	c.Assert(err, qt.IsNil)
	return created
}

// An export whose worker died in the middle of a run goes back to the queue
// while it has runs left; that is not a failure.
func TestExportService_RecoverStalled_RequeuesWhileRunsAreLeft(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	stalled := stalledExport(c, fs, "stalled-export", 1)

	counter := metrics.JobsDeadLetteredFor(string(models.WorkerTypeExport))
	before := testutil.ToFloat64(counter)

	out, err := export.NewExportService(fs, "nowhere://bucket", nil).RecoverStalled(ctx, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, registry.StalledJobs{Requeued: 1})

	got, err := fs.ExportRegistryFactory.CreateServiceRegistry().Get(ctx, stalled.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.ExportStatusPending)
	c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, float64(0))
}

// An export whose worker died during its last run fails for good and counts
// once as dead-lettered.
func TestExportService_RecoverStalled_FailsAfterTheLastRun(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	stalled := stalledExport(c, fs, "stalled-export", stall.MaxRuns)

	counter := metrics.JobsDeadLetteredFor(string(models.WorkerTypeExport))
	before := testutil.ToFloat64(counter)

	out, err := export.NewExportService(fs, "nowhere://bucket", nil).RecoverStalled(ctx, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, registry.StalledJobs{Failed: 1})
	c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, float64(1))

	got, err := fs.ExportRegistryFactory.CreateServiceRegistry().Get(ctx, stalled.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.ExportStatusFailed)
}

// The export worker sweeps for dead runs on its own: an export that used its
// runs fails with an error that says the worker stopped and shows among the
// failed jobs. The service test above counts the metric: the worker records
// it after the row, and a worker of another test may still be running.
func TestExportWorker_FailsAnExportWhoseWorkerStopped(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	stalled := stalledExport(c, fs, "stalled-export", stall.MaxRuns)

	worker := export.NewExportWorker(export.NewExportService(fs, "nowhere://bucket", nil), fs, 1,
		export.WithPollInterval(5*time.Millisecond))
	worker.Start(t.Context())
	defer worker.Stop()

	reg := fs.ExportRegistryFactory.CreateServiceRegistry()
	var got *models.Export
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		got, err = reg.Get(context.Background(), stalled.ID)
		c.Assert(err, qt.IsNil)
		if got.Status != models.ExportStatusInProgress {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	c.Assert(got.Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(got.ErrorMessage, qt.Equals, export.StalledExportError)
	c.Assert(got.CompletedDate, qt.IsNotNil)

	failed, err := fs.FailedJobRegistry.ListFailed(context.Background(), models.WorkerTypeExport, registry.FailedJobScope{}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.HasLen, 1)
	c.Assert(failed[0].ID, qt.Equals, stalled.ID)
	c.Assert(failed[0].Attempts, qt.Equals, stall.MaxRuns)
}
