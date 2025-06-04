package export

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/denisvmedia/inventario/models"
)

func TestNewExportWorker(t *testing.T) {
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	exportService := NewExportService(registrySet, tempDir, "/tmp/uploads")
	worker := NewExportWorker(exportService, registrySet)

	if worker == nil {
		t.Fatal("NewExportWorker returned nil")
	}
	if worker.exportService != exportService {
		t.Errorf("Expected exportService to be %v, got %v", exportService, worker.exportService)
	}
	if worker.registrySet != registrySet {
		t.Errorf("Expected registrySet to be %v, got %v", registrySet, worker.registrySet)
	}
	if worker.pollInterval != 10*time.Second {
		t.Errorf("Expected pollInterval to be 10s, got %v", worker.pollInterval)
	}
	if worker.stopCh == nil {
		t.Error("Expected stopCh to be initialized")
	}
	if worker.isRunning {
		t.Error("Expected worker to not be running initially")
	}
}

func TestExportWorkerStartStop(t *testing.T) {
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	exportService := NewExportService(registrySet, tempDir, "/tmp/uploads")
	worker := NewExportWorker(exportService, registrySet)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test initial state
	if worker.IsRunning() {
		t.Error("Worker should not be running initially")
	}

	// Start the worker
	worker.Start(ctx)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	if !worker.IsRunning() {
		t.Error("Worker should be running after Start()")
	}

	// Test starting again (should be no-op)
	worker.Start(ctx)
	if !worker.IsRunning() {
		t.Error("Worker should still be running after second Start()")
	}

	// Stop the worker
	worker.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	if worker.IsRunning() {
		t.Error("Worker should not be running after Stop()")
	}
}

func TestExportWorkerIsRunning(t *testing.T) {
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	exportService := NewExportService(registrySet, tempDir, "/tmp/uploads")
	worker := NewExportWorker(exportService, registrySet)

	// Test initial state
	if worker.IsRunning() {
		t.Error("Worker should not be running initially")
	}

	// Manually set running state to test the method
	worker.mu.Lock()
	worker.isRunning = true
	worker.mu.Unlock()

	if !worker.IsRunning() {
		t.Error("IsRunning() should return true when worker is running")
	}

	worker.mu.Lock()
	worker.isRunning = false
	worker.mu.Unlock()

	if worker.IsRunning() {
		t.Error("IsRunning() should return false when worker is not running")
	}
}

func TestExportWorkerProcessPendingExports(t *testing.T) {
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	exportService := NewExportService(registrySet, tempDir, "/tmp/uploads")
	worker := NewExportWorker(exportService, registrySet)

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
	if err != nil {
		t.Fatalf("Failed to create export1: %v", err)
	}

	createdExport2, err := registrySet.ExportRegistry.Create(ctx, export2)
	if err != nil {
		t.Fatalf("Failed to create export2: %v", err)
	}

	// Process pending exports
	worker.processPendingExports(ctx)

	// Give time for async processing to complete
	time.Sleep(100 * time.Millisecond)

	// Check that exports were processed (status should change from pending)
	updatedExport1, err := registrySet.ExportRegistry.Get(ctx, createdExport1.ID)
	if err != nil {
		t.Fatalf("Failed to get updated export1: %v", err)
	}

	updatedExport2, err := registrySet.ExportRegistry.Get(ctx, createdExport2.ID)
	if err != nil {
		t.Fatalf("Failed to get updated export2: %v", err)
	}

	if updatedExport1.Status == models.ExportStatusPending {
		t.Error("Export1 status should have changed from pending")
	}

	if updatedExport2.Status == models.ExportStatusPending {
		t.Error("Export2 status should have changed from pending")
	}
}

func TestExportWorkerProcessExport(t *testing.T) {
	registrySet := newTestRegistrySet()

	// Create a temporary directory for exports
	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	exportService := NewExportService(registrySet, tempDir, "/tmp/uploads")
	worker := NewExportWorker(exportService, registrySet)

	ctx := context.Background()

	// Create a test export
	export := models.Export{
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	if err != nil {
		t.Fatalf("Failed to create export: %v", err)
	}

	// Process the specific export
	worker.processExport(ctx, createdExport.ID)

	// Check that export was processed
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	if err != nil {
		t.Fatalf("Failed to get updated export: %v", err)
	}

	if updatedExport.Status == models.ExportStatusPending {
		t.Error("Export status should have changed from pending")
	}

	// Status should be either completed or failed
	if updatedExport.Status != models.ExportStatusCompleted && updatedExport.Status != models.ExportStatusFailed {
		t.Errorf("Expected export status to be completed or failed, got %s", updatedExport.Status)
	}
}

func TestExportWorkerConcurrentAccess(t *testing.T) {
	// Test concurrent access to worker methods
	registrySet := newTestRegistrySet()

	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	exportService := NewExportService(registrySet, tempDir, "/tmp/uploads")
	worker := NewExportWorker(exportService, registrySet)

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
	if worker.IsRunning() {
		t.Error("Worker should be stopped after concurrent operations")
	}
}

func TestExportWorkerContextCancellation(t *testing.T) {
	// Test that worker respects context cancellation
	registrySet := newTestRegistrySet()

	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	exportService := NewExportService(registrySet, tempDir, "/tmp/uploads")
	worker := NewExportWorker(exportService, registrySet)

	ctx, cancel := context.WithCancel(context.Background())

	// Start worker
	worker.Start(ctx)

	if !worker.IsRunning() {
		t.Error("Worker should be running after start")
	}

	// Cancel context
	cancel()

	// Give some time for worker to respond to cancellation
	time.Sleep(500 * time.Millisecond)

	// Worker should have stopped due to context cancellation
	if worker.IsRunning() {
		t.Error("Worker should have stopped after context cancellation")
	}
}