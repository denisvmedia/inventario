package export

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
)

func TestNewExportWorker(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for uploads
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	exportService := NewExportService(registrySet, uploadLocation)
	worker := NewExportWorker(exportService, registrySet, 3)

	c.Assert(worker, qt.IsNotNil)
	c.Assert(worker.exportService, qt.Equals, exportService)
	c.Assert(worker.registrySet, qt.Equals, registrySet)
	c.Assert(worker.pollInterval, qt.Equals, 10*time.Second)
	c.Assert(worker.stopCh, qt.IsNotNil)
	c.Assert(worker.isRunning, qt.IsFalse)
}

func TestExportWorkerStartStop(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	exportService := NewExportService(registrySet, uploadLocation)
	worker := NewExportWorker(exportService, registrySet, 3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test initial state
	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should not be running initially"))

	// Start the worker
	worker.Start(ctx)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	c.Assert(worker.IsRunning(), qt.IsTrue, qt.Commentf("Worker should be running after Start()"))

	// Test starting again (should be no-op)
	worker.Start(ctx)
	c.Assert(worker.IsRunning(), qt.IsTrue, qt.Commentf("Worker should still be running after second Start()"))

	// Stop the worker
	worker.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should not be running after Stop()"))
}

func TestExportWorkerIsRunning(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	exportService := NewExportService(registrySet, uploadLocation)
	worker := NewExportWorker(exportService, registrySet, 3)

	// Test initial state
	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should not be running initially"))

	// Manually set running state to test the method
	worker.mu.Lock()
	worker.isRunning = true
	worker.mu.Unlock()

	c.Assert(worker.IsRunning(), qt.IsTrue, qt.Commentf("IsRunning() should return true when worker is running"))

	worker.mu.Lock()
	worker.isRunning = false
	worker.mu.Unlock()

	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("IsRunning() should return false when worker is not running"))
}

func TestExportWorkerProcessPendingExports(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	exportService := NewExportService(registrySet, uploadLocation)
	worker := NewExportWorker(exportService, registrySet, 3)

	ctx := context.Background()

	// Create some test exports
	export1 := models.Export{
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	export2 := models.Export{
		Type:            models.ExportTypeLocations,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	// Create exports in database
	createdExport1, err := registrySet.ExportRegistry.Create(ctx, export1)
	c.Assert(err, qt.IsNil)

	createdExport2, err := registrySet.ExportRegistry.Create(ctx, export2)
	c.Assert(err, qt.IsNil)

	// Process pending exports
	worker.processPendingExports(ctx)

	// Give time for async processing to complete
	time.Sleep(100 * time.Millisecond)

	// Check that exports were processed (status should change from pending)
	updatedExport1, err := registrySet.ExportRegistry.Get(ctx, createdExport1.ID)
	c.Assert(err, qt.IsNil)

	updatedExport2, err := registrySet.ExportRegistry.Get(ctx, createdExport2.ID)
	c.Assert(err, qt.IsNil)

	c.Assert(updatedExport1.Status, qt.Not(qt.Equals), models.ExportStatusPending, qt.Commentf("Export1 status should have changed from pending"))
	c.Assert(updatedExport2.Status, qt.Not(qt.Equals), models.ExportStatusPending, qt.Commentf("Export2 status should have changed from pending"))
}

func TestExportWorkerProcessExport(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	exportService := NewExportService(registrySet, uploadLocation)
	worker := NewExportWorker(exportService, registrySet, 3)

	ctx := context.Background()

	// Create a test export
	export := models.Export{
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	// Process the specific export
	worker.processExport(ctx, createdExport.ID)

	// Check that export was processed
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	c.Assert(updatedExport.Status, qt.Not(qt.Equals), models.ExportStatusPending, qt.Commentf("Export status should have changed from pending"))

	// Status should be either completed or failed
	c.Assert(updatedExport.Status == models.ExportStatusCompleted || updatedExport.Status == models.ExportStatusFailed, qt.IsTrue,
		qt.Commentf("Expected export status to be completed or failed, got %s", updatedExport.Status))
}

func TestExportWorkerConcurrentAccess(t *testing.T) {
	c := qt.New(t)
	// Test concurrent access to worker methods
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	exportService := NewExportService(registrySet, uploadLocation)
	worker := NewExportWorker(exportService, registrySet, 3)

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

func TestExportWorkerContextCancellation(t *testing.T) {
	c := qt.New(t)
	// Test that worker respects context cancellation
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	exportService := NewExportService(registrySet, uploadLocation)
	worker := NewExportWorker(exportService, registrySet, 3)

	ctx, cancel := context.WithCancel(context.Background())

	// Start worker
	worker.Start(ctx)

	c.Assert(worker.IsRunning(), qt.IsTrue, qt.Commentf("Worker should be running after start"))

	// Cancel context
	cancel()

	// Give some time for worker to respond to cancellation
	time.Sleep(500 * time.Millisecond)

	// Worker should have stopped due to context cancellation
	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should have stopped after context cancellation"))
}

func TestExportWorkerConfigurableConcurrentLimit(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	exportService := NewExportService(registrySet, uploadLocation)

	// Test with different concurrent limits
	worker1 := NewExportWorker(exportService, registrySet, 1)
	worker2 := NewExportWorker(exportService, registrySet, 5)

	// The semaphore capacity should match the configured limit
	// We can't directly access the semaphore capacity, but we can verify
	// that the workers are created without panicking and can acquire resources

	// Test that worker1 (limit 1) can acquire 1 but blocks on second
	c.Assert(worker1.semaphore.TryAcquire(1), qt.IsTrue)
	c.Assert(worker1.semaphore.TryAcquire(1), qt.IsFalse) // Should fail as limit is 1
	worker1.semaphore.Release(1)

	// Test that worker2 (limit 5) can acquire multiple resources
	c.Assert(worker2.semaphore.TryAcquire(3), qt.IsTrue)
	c.Assert(worker2.semaphore.TryAcquire(2), qt.IsTrue)  // Should succeed as limit is 5
	c.Assert(worker2.semaphore.TryAcquire(1), qt.IsFalse) // Should fail as we've hit the limit
	worker2.semaphore.Release(5)
}
