package processor_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"go.5x5.cz/inventario/backup/restore/processor"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// A restore that cannot run is marked failed for good (#1307), with when it
// ended, and the dead-letter counter goes up once.
func TestProcess_FailureIsCountedOnce(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	op, err := fs.RestoreOperationRegistryFactory.CreateServiceRegistry().Create(ctx, models.RestoreOperation{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("restore-1", "tenant-1", "group-1", "user-1"),
		ExportID:                 "missing-export",
		Description:              "Restore from Monday",
		Status:                   models.RestoreStatusPending,
		Options:                  models.RestoreOptions{Strategy: "merge_add"},
	})
	c.Assert(err, qt.IsNil)

	counter := metrics.JobsDeadLetteredFor(string(models.WorkerTypeRestore))
	before := testutil.ToFloat64(counter)

	const uploadLocation = "memory://restore-failure"
	proc := processor.NewRestoreOperationProcessor(op.ID, fs, services.NewEntityService(fs, uploadLocation), uploadLocation, nil)
	c.Assert(proc.Process(ctx), qt.IsNotNil)

	failed, err := fs.RestoreOperationRegistryFactory.CreateServiceRegistry().Get(ctx, op.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(failed.Status, qt.Equals, models.RestoreStatusFailed)
	c.Assert(failed.ErrorMessage, qt.Contains, "failed to get export")
	c.Assert(failed.CompletedDate, qt.IsNotNil)
	c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, float64(1))
}
