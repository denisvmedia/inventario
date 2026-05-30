package export

import (
	"context"
	"runtime"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services/workerpause"
)

// TestExportWorkerSoftPause is the #1308 acceptance test: a paused export
// worker does NOT claim a pending export, and resuming it does. It drives
// the controller's RefreshOnce + the worker's processPendingExports
// directly (no ticker) so the pause/resume transition is deterministic.
//
// "Not picked up while paused" is proved by the export's status staying
// ExportStatusPending after processPendingExports runs under a pause —
// contrast the baseline / resumed cases where the status leaves pending
// (the worker claimed it and the service ran to completed/failed).
func TestExportWorkerSoftPause(t *testing.T) {
	c := qt.New(t)
	factorySet := newTestFactorySet()
	registrySet := factorySet.CreateServiceRegistrySet()

	tempDir := c.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	exportService := NewExportService(factorySet, uploadLocation)

	// The controller polls the WorkerControlRegistry; the worker consults
	// the controller's lock-free IsPaused on its claim phase.
	ctrl := workerpause.NewController(factorySet.WorkerControlRegistry)
	worker := NewExportWorker(exportService, factorySet, 3, WithPauseController(ctrl))

	ctx := newTestContext()

	newPendingExport := func() string {
		c.Helper()
		export := models.Export{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				CreatedByUserID: testUserID,
				TenantID:        "test-tenant",
				GroupID:         testGroupID,
			},
			Type:            models.ExportTypeCommodities,
			Status:          models.ExportStatusPending,
			IncludeFileData: false,
		}
		created, err := registrySet.ExportRegistry.Create(ctx, export)
		c.Assert(err, qt.IsNil)
		return created.ID
	}

	// (1) BASELINE — no pause row. RefreshOnce sees nothing paused, the
	// worker claims the pending export, and its status leaves pending.
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)
	c.Assert(ctrl.IsPaused(models.WorkerTypeExport), qt.IsFalse)

	baselineID := newPendingExport()
	worker.processPendingExports(ctx)
	c.Assert(waitForStatusLeavesPending(ctx, registrySet, baselineID), qt.IsTrue,
		qt.Commentf("baseline: unpaused worker should claim the pending export"))

	// (2) PAUSED — write a pause row, refresh, and confirm the worker
	// leaves a fresh pending export untouched.
	_, err := factorySet.WorkerControlRegistry.Pause(ctx, string(models.WorkerTypeExport), "tester", "maint")
	c.Assert(err, qt.IsNil)
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)
	c.Assert(ctrl.IsPaused(models.WorkerTypeExport), qt.IsTrue)

	pausedID := newPendingExport()
	worker.processPendingExports(ctx)
	// A single sleep+check could let a late async claim slip through after
	// the assertion ran. Instead, poll over a window and require the export
	// to stay pending for the whole interval — any non-pending observation
	// means the paused worker wrongly claimed it.
	c.Assert(staysPending(ctx, registrySet, pausedID), qt.IsTrue,
		qt.Commentf("paused worker must NOT claim the pending export"))

	// (3) RESUMED — clear the pause, refresh, and confirm the once-blocked
	// export is now claimed.
	_, err = factorySet.WorkerControlRegistry.Resume(ctx, string(models.WorkerTypeExport))
	c.Assert(err, qt.IsNil)
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)
	c.Assert(ctrl.IsPaused(models.WorkerTypeExport), qt.IsFalse)

	worker.processPendingExports(ctx)
	c.Assert(waitForStatusLeavesPending(ctx, registrySet, pausedID), qt.IsTrue,
		qt.Commentf("resumed worker should now claim the previously-paused export"))
}

// waitForStatusLeavesPending polls the export row until its status is no
// longer ExportStatusPending (the worker claimed + processed it) or a
// short timeout elapses. Returns true on the leave-pending transition,
// false on timeout. The export worker processes claimed exports in a
// goroutine, so the assertion must give that goroutine a bounded chance
// to run rather than reading the status synchronously.
func waitForStatusLeavesPending(ctx context.Context, registrySet *registry.Set, exportID string) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := registrySet.ExportRegistry.Get(ctx, exportID)
		if err == nil && got.Status != models.ExportStatusPending {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// staysPending polls the export status every ~20ms over a ~400ms window
// and returns true only if the status remains ExportStatusPending for the
// whole window. It returns false on the first non-pending observation, so
// a late async claim by a worker that should be paused cannot slip past a
// single point-in-time check.
func staysPending(ctx context.Context, registrySet *registry.Set, exportID string) bool {
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		got, err := registrySet.ExportRegistry.Get(ctx, exportID)
		if err == nil && got.Status != models.ExportStatusPending {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}
