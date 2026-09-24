package export

// White-box (package export) for the same reason as worker_pause_test.go:
// the claim/dispatch ordering is only observable from inside
// processPendingExports, and driving it through Start()'s ticker would be
// flaky.

import (
	"context"
	"runtime"
	"testing"

	qt "github.com/frankban/quicktest"

	_ "go.5x5.cz/inventario/internal/fileblob" // register fileblob driver
	"go.5x5.cz/inventario/models"
)

// TestExportWorkerLeavesRowPendingWithoutCapacity pins the order of the two
// steps that dispatch a job: take a worker slot, then claim the row. A claim
// that lands without a slot leaves the row in_progress with nobody working
// it, and every later poll filters on pending — so the export is stuck until
// someone edits the database.
//
// The canceled context is what makes this observable: it is the one input
// that fails the blocking Acquire. The memory registry does not consult the
// context, so the claim itself would still succeed — which is precisely the
// window this order closes.
func TestExportWorkerLeavesRowPendingWithoutCapacity(t *testing.T) {
	c := qt.New(t)
	factorySet := newTestFactorySet()
	registrySet := factorySet.CreateServiceRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	}

	worker := NewExportWorker(NewExportService(factorySet, uploadLocation, nil), factorySet, 1)

	ctx := newTestContext()
	created, err := registrySet.ExportRegistry.Create(ctx, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			CreatedByUserID: testUserID,
			TenantID:        "test-tenant",
			GroupID:         testGroupID,
		},
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	})
	c.Assert(err, qt.IsNil)

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	worker.processPendingExports(canceled)

	c.Assert(staysPending(ctx, registrySet, created.ID), qt.IsTrue,
		qt.Commentf("a worker that cannot take a slot must leave the row claimable"))
}
