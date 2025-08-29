package importpkg_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	_ "gocloud.dev/blob/memblob"

	importpkg "github.com/denisvmedia/inventario/backup/import"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
)

func TestNewImportWorker(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for uploads
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	importService := importpkg.NewImportService(registrySet, uploadLocation)
	worker := importpkg.NewImportWorker(importService, registrySet, 3)

	c.Assert(worker, qt.IsNotNil)
	c.Assert(worker.IsRunning(), qt.IsFalse)
}

func TestImportWorkerStartStop(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for imports
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	importService := importpkg.NewImportService(registrySet, uploadLocation)
	worker := importpkg.NewImportWorker(importService, registrySet, 3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test initial state
	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should not be running initially"))

	// Start the worker
	worker.Start(ctx)

	for worker.IsRunning() == false {
		time.Sleep(1 * time.Microsecond)
	}

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

func TestImportWorkerIsRunning(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for imports
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	importService := importpkg.NewImportService(registrySet, uploadLocation)
	worker := importpkg.NewImportWorker(importService, registrySet, 3)

	// Test initial state
	c.Assert(worker.IsRunning(), qt.IsFalse, qt.Commentf("Worker should not be running initially"))
}

func TestImportWorkerConcurrentAccess(t *testing.T) {
	c := qt.New(t)
	// Test concurrent access to worker methods
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	importService := importpkg.NewImportService(registrySet, uploadLocation)
	worker := importpkg.NewImportWorker(importService, registrySet, 3)

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

func TestImportWorkerContextCancellation(t *testing.T) {
	c := qt.New(t)
	// Test that worker respects context cancellation
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	importService := importpkg.NewImportService(registrySet, uploadLocation)
	worker := importpkg.NewImportWorker(importService, registrySet, 3)

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

func TestImportWorkerConfigurableConcurrentLimit(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for imports
	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	importService := importpkg.NewImportService(registrySet, uploadLocation)

	// Test with different concurrent limits
	worker1 := importpkg.NewImportWorker(importService, registrySet, 1)
	worker2 := importpkg.NewImportWorker(importService, registrySet, 5)

	// The workers should be created without panicking
	c.Assert(worker1, qt.IsNotNil)
	c.Assert(worker2, qt.IsNotNil)
}

func TestImportWorkerIgnoresNonImportExports(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	importService := importpkg.NewImportService(registrySet, uploadLocation)
	worker := importpkg.NewImportWorker(importService, registrySet, 3)

	ctx := context.Background()

	// Create exports of different types - only "imported" type should be processed
	exports := []models.Export{
		{
			Type:        models.ExportTypeCommodities,
			Status:      models.ExportStatusPending,
			Description: "Regular commodities export",
		},
		{
			Type:        models.ExportTypeLocations,
			Status:      models.ExportStatusPending,
			Description: "Regular locations export",
		},
		{
			Type:        models.ExportTypeFullDatabase,
			Status:      models.ExportStatusPending,
			Description: "Regular full database export",
		},
		{
			Type:        models.ExportTypeImported,
			Status:      models.ExportStatusCompleted, // Not pending, should be ignored
			Description: "Completed import",
		},
	}

	var createdExports []*models.Export
	for _, export := range exports {
		created, err := registrySet.ExportRegistry.Create(ctx, export)
		c.Assert(err, qt.IsNil)
		createdExports = append(createdExports, created)
	}

	// Start worker briefly
	worker.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	worker.Stop()

	// Verify that none of the exports were processed (all should remain in their original state)
	for i, created := range createdExports {
		updated, err := registrySet.ExportRegistry.Get(ctx, created.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Status, qt.Equals, exports[i].Status, qt.Commentf("Export %d status should not have changed", i))
	}
}

func TestImportWorkerStopIdempotent(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"

	importService := importpkg.NewImportService(registrySet, uploadLocation)
	worker := importpkg.NewImportWorker(importService, registrySet, 3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test that calling Stop multiple times is safe
	c.Assert(worker.IsRunning(), qt.IsFalse)

	// Stop when not running should be safe
	worker.Stop()
	c.Assert(worker.IsRunning(), qt.IsFalse)

	// Start and then stop multiple times
	worker.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	c.Assert(worker.IsRunning(), qt.IsTrue)

	worker.Stop()
	worker.Stop() // Second stop should be safe
	worker.Stop() // Third stop should be safe

	time.Sleep(100 * time.Millisecond)
	c.Assert(worker.IsRunning(), qt.IsFalse)
}
