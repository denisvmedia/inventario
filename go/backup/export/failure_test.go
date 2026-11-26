package export_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
)

// A run that fails marks the export failed for good (#1307): the run is
// counted on the row, the row records when it ended, and the dead-letter
// counter goes up once.
func TestProcessExport_FailureIsCountedOnce(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	user, err := fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: personalTenant},
		Email:               "owner@example.com",
		Name:                "Owner",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	group, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: personalTenant},
		Slug:                "failing-export-group-0000000000",
		Name:                "Home",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
	})
	c.Assert(err, qt.IsNil)
	created, err := fs.ExportRegistryFactory.CreateServiceRegistry().Create(ctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("failing-export", personalTenant, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusPending,
		Description:              "Weekly backup",
	})
	c.Assert(err, qt.IsNil)

	counter := metrics.JobsDeadLetteredFor(string(models.WorkerTypeExport))
	before := testutil.ToFloat64(counter)

	// Neither build can export without a signer and a bucket.
	service := export.NewExportService(fs, "nowhere://bucket", nil)
	c.Assert(service.ProcessExport(ctx, created.ID), qt.IsNotNil)

	failed, err := fs.ExportRegistryFactory.CreateServiceRegistry().Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(failed.Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(failed.ErrorMessage, qt.Not(qt.Equals), "")
	c.Assert(failed.AttemptCount, qt.Equals, 1)
	c.Assert(failed.CompletedDate, qt.IsNotNil, qt.Commentf("the failed list orders by when the run ended"))
	c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, float64(1))
}
