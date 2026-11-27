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

const stalledUploadLocation = "memory://stalled-restore"

// runningRestore stores a running restore whose last heartbeat was at
// heartbeat.
func runningRestore(c *qt.C, fs *registry.FactorySet, id string, heartbeat time.Time) *models.RestoreOperation {
	c.Helper()
	op, err := fs.RestoreOperationRegistryFactory.CreateServiceRegistry().Create(context.Background(), models.RestoreOperation{
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

func newRestoreService(fs *registry.FactorySet) *restore.RestoreService {
	return restore.NewRestoreService(fs, services.NewEntityService(fs, stalledUploadLocation), stalledUploadLocation, nil)
}

// A restore whose worker stopped is never queued again, since it may have
// written part of its backup: it fails and counts once as dead-lettered. A
// restore whose worker is alive is left alone.
func TestRestoreService_RecoverStalled_FailsARestoreWhoseWorkerStopped(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	stalled := runningRestore(c, fs, "stalled-restore", time.Now().Add(-time.Hour))
	alive := runningRestore(c, fs, "live-restore", time.Now())

	counter := metrics.JobsDeadLetteredFor(string(models.WorkerTypeRestore))
	before := testutil.ToFloat64(counter)

	failed, err := newRestoreService(fs).RecoverStalled(ctx, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.Equals, 1)
	c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, float64(1))

	reg := fs.RestoreOperationRegistryFactory.CreateServiceRegistry()
	got, err := reg.Get(ctx, stalled.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.RestoreStatusFailed)
	live, err := reg.Get(ctx, alive.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(live.Status, qt.Equals, models.RestoreStatusRunning)
}

// The restore worker sweeps for dead runs on its own: a restore whose worker
// stopped fails with an error that says so and shows among the failed jobs,
// and a restore whose worker is alive is left alone. The service test above
// counts the metric: the worker records it after the row, and a worker of
// another test may still be running.
func TestRestoreWorker_FailsARestoreWhoseWorkerStopped(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	reg := fs.RestoreOperationRegistryFactory.CreateServiceRegistry()
	stalled := runningRestore(c, fs, "stalled-restore", time.Now().Add(-time.Hour))
	alive := runningRestore(c, fs, "live-restore", time.Now())

	worker := restore.NewRestoreWorker(newRestoreService(fs), fs.CreateServiceRegistrySet(), stalledUploadLocation,
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

	live, err := reg.Get(ctx, alive.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(live.Status, qt.Equals, models.RestoreStatusRunning)

	failed, err := fs.FailedJobRegistry.ListFailed(ctx, models.WorkerTypeRestore, registry.FailedJobScope{}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.HasLen, 1)
	c.Assert(failed[0].ID, qt.Equals, stalled.ID)
}
