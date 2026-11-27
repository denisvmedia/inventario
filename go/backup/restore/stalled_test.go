package restore_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"go.5x5.cz/inventario/backup/restore"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// The restore worker sweeps for dead runs on its own. A restore whose worker
// stopped is never queued again, since it may have written part of its
// backup: it fails with an error that says the worker stopped, counts once as
// dead-lettered and shows among the failed jobs. A restore whose worker is
// alive is left alone.
func TestRestoreWorker_FailsARestoreWhoseWorkerStopped(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	reg := fs.RestoreOperationRegistryFactory.CreateServiceRegistry()
	create := func(id string, heartbeat time.Time) *models.RestoreOperation {
		op, err := reg.Create(ctx, models.RestoreOperation{
			TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID(id, "tenant-1", "group-1", "user-1"),
			ExportID:                 "export-1",
			Description:              "Restore from Monday",
			Status:                   models.RestoreStatusRunning,
			Options:                  models.RestoreOptions{Strategy: "merge_add"},
			CreatedDate:              models.NewPTimestamp(time.Now().Add(-2 * time.Hour)),
			HeartbeatAt:              models.NewPTimestamp(heartbeat),
		})
		c.Assert(err, qt.IsNil)
		return op
	}
	stalled := create("stalled-restore", time.Now().Add(-time.Hour))
	alive := create("live-restore", time.Now())

	counter := metrics.JobsDeadLetteredFor(string(models.WorkerTypeRestore))
	before := testutil.ToFloat64(counter)

	const uploadLocation = "memory://stalled-restore"
	service := restore.NewRestoreService(fs, services.NewEntityService(fs, uploadLocation), uploadLocation, nil)
	worker := restore.NewRestoreWorker(service, fs.CreateServiceRegistrySet(), uploadLocation,
		restore.WithPollInterval(5*time.Millisecond))
	worker.Start(t.Context())
	defer worker.Stop()

	var got *models.RestoreOperation
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		got, err = reg.Get(ctx, stalled.ID)
		c.Assert(err, qt.IsNil)
		if got.Status != models.RestoreStatusRunning {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	c.Assert(got.Status, qt.Equals, models.RestoreStatusFailed)
	c.Assert(got.ErrorMessage, qt.Equals, restore.StalledRestoreError)
	c.Assert(got.CompletedDate, qt.IsNotNil)
	c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, float64(1))

	live, err := reg.Get(ctx, alive.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(live.Status, qt.Equals, models.RestoreStatusRunning)

	failed, err := fs.FailedJobRegistry.ListFailed(ctx, models.WorkerTypeRestore, registry.FailedJobScope{}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.HasLen, 1)
	c.Assert(failed[0].ID, qt.Equals, stalled.ID)
}
