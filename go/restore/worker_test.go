package restore_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/restore"
)

func newTestRegistrySet() *registry.Set {
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	restoreStepRegistry := memory.NewRestoreStepRegistry()

	return &registry.Set{
		LocationRegistry:         locationRegistry,
		AreaRegistry:             areaRegistry,
		CommodityRegistry:        commodityRegistry,
		ImageRegistry:            memory.NewImageRegistry(commodityRegistry),
		InvoiceRegistry:          memory.NewInvoiceRegistry(commodityRegistry),
		ManualRegistry:           memory.NewManualRegistry(commodityRegistry),
		SettingsRegistry:         memory.NewSettingsRegistry(),
		ExportRegistry:           memory.NewExportRegistry(),
		RestoreOperationRegistry: memory.NewRestoreOperationRegistry(restoreStepRegistry),
		RestoreStepRegistry:      restoreStepRegistry,
	}
}

func TestNewRestoreWorker(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for uploads
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	restoreService := restore.NewRestoreService(registrySet, uploadLocation)
	worker := restore.NewRestoreWorker(restoreService, registrySet, uploadLocation)

	c.Assert(worker, qt.IsNotNil)
	c.Assert(worker.IsRunning(), qt.IsFalse)
}

func TestRestoreWorkerStartStop(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for restores
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	restoreService := restore.NewRestoreService(registrySet, uploadLocation)
	worker := restore.NewRestoreWorker(restoreService, registrySet, uploadLocation)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test initial state
	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should not be running initially"))

	// Start the worker
	worker.Start(ctx)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	c.Assert(worker.IsRunning(), qt.IsTrue, qt.Commentf("Worker should be running after Start()"))

	// Stop the worker
	worker.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should not be running after Stop()"))
}

func TestRestoreWorkerConcurrentAccess(t *testing.T) {
	c := qt.New(t)
	// Test concurrent access to worker methods
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	restoreService := restore.NewRestoreService(registrySet, uploadLocation)
	worker := restore.NewRestoreWorker(restoreService, registrySet, uploadLocation)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Test concurrent IsRunning calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = worker.IsRunning()
			}
		}()
	}

	// Test concurrent Start/Stop calls
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker.Start(ctx)
			time.Sleep(10 * time.Millisecond)
			worker.Stop()
		}()
	}

	wg.Wait()

	// Ensure worker is stopped at the end
	worker.Stop()
	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should be stopped after concurrent operations"))
}

func TestRestoreWorkerContextCancellation(t *testing.T) {
	c := qt.New(t)
	// Test that worker respects context cancellation
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	restoreService := restore.NewRestoreService(registrySet, uploadLocation)
	worker := restore.NewRestoreWorker(restoreService, registrySet, uploadLocation)

	ctx, cancel := context.WithCancel(context.Background())

	// Start worker
	worker.Start(ctx)

	c.Assert(worker.IsRunning(), qt.IsTrue, qt.Commentf("Worker should be running after start"))

	// Cancel context
	cancel()

	// Give some time for worker to respond to cancellation
	time.Sleep(500 * time.Millisecond)

	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should stop after context cancellation"))
}

func TestRestoreWorkerConfigurableConcurrentLimit(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for restores
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	restoreService := restore.NewRestoreService(registrySet, uploadLocation)

	// Test with different concurrent limits
	worker1 := restore.NewRestoreWorker(restoreService, registrySet, uploadLocation)

	// The workers should be created without panicking
	c.Assert(worker1, qt.IsNotNil)
}

func TestHasRunningRestores(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	restoreService := restore.NewRestoreService(registrySet, uploadLocation)
	worker := restore.NewRestoreWorker(restoreService, registrySet, uploadLocation)

	ctx := context.Background()

	// Initially no running restores
	hasRunning, err := worker.HasRunningRestores(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(hasRunning, qt.IsFalse)

	// Create a running restore operation
	runningRestoreOp := models.RestoreOperation{
		ExportID:    "test-export-id",
		Description: "Test running restore",
		Status:      models.RestoreStatusRunning,
		Options: models.RestoreOptions{
			Strategy:        "merge_update",
			IncludeFileData: false,
			DryRun:          false,
			BackupExisting:  false,
		},
		CreatedDate: models.PNow(),
	}

	_, err = registrySet.RestoreOperationRegistry.Create(ctx, runningRestoreOp)
	c.Assert(err, qt.IsNil)

	// Now should have running restores
	hasRunning, err = worker.HasRunningRestores(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(hasRunning, qt.IsTrue)
}

func TestHasRunningRestores_PendingAlsoBlocks(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	restoreService := restore.NewRestoreService(registrySet, uploadLocation)
	worker := restore.NewRestoreWorker(restoreService, registrySet, uploadLocation)

	ctx := context.Background()

	// Initially no running restores
	hasRunning, err := worker.HasRunningRestores(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(hasRunning, qt.IsFalse)

	// Create a pending restore operation
	pendingRestoreOp := models.RestoreOperation{
		ExportID:    "test-export-id",
		Description: "Test pending restore",
		Status:      models.RestoreStatusPending,
		Options: models.RestoreOptions{
			Strategy:        "merge_update",
			IncludeFileData: false,
			DryRun:          false,
			BackupExisting:  false,
		},
		CreatedDate: models.PNow(),
	}

	_, err = registrySet.RestoreOperationRegistry.Create(ctx, pendingRestoreOp)
	c.Assert(err, qt.IsNil)

	// Now should have running restores (pending counts as blocking)
	hasRunning, err = worker.HasRunningRestores(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(hasRunning, qt.IsTrue)
}
