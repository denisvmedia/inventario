package export

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	defaultPollInterval = 10 * time.Second
	// Note: Cleanup interval removed - exports now use immediate hard delete with file entities
)

// ExportWorker processes export requests in the background
type ExportWorker struct {
	exportService *ExportService
	factorySet    *registry.FactorySet
	pollInterval  time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
	isRunning     bool
	mu            sync.RWMutex
	stopped       bool
	semaphore     *semaphore.Weighted
}

// NewExportWorker creates a new export worker
func NewExportWorker(exportService *ExportService, factorySet *registry.FactorySet, maxConcurrentExports int) *ExportWorker {
	return &ExportWorker{
		exportService: exportService,
		factorySet:    factorySet,
		pollInterval:  defaultPollInterval, // Check for new exports every 10 seconds
		stopCh:        make(chan struct{}),
		semaphore:     semaphore.NewWeighted(int64(maxConcurrentExports)),
	}
}

// Start begins processing exports in the background
func (w *ExportWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.isRunning {
		return
	}

	w.isRunning = true

	w.wg.Go(func() {
		defer func() {
			w.mu.Lock()
			w.isRunning = false
			w.mu.Unlock()
		}()
		w.run(ctx)
	})

	slog.Info("Export worker started")
}

// Stop stops the export worker
func (w *ExportWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isRunning || w.stopped {
		return
	}

	w.stopped = true
	close(w.stopCh)
	w.isRunning = false

	go func() {
		w.wg.Wait()
		slog.Info("Export worker stopped")
	}()
}

// IsRunning returns whether the worker is currently running
func (w *ExportWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// run is the main worker loop
func (w *ExportWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processPendingExports(ctx)
		}
	}
}

// processPendingExports finds and processes pending export requests
func (w *ExportWorker) processPendingExports(ctx context.Context) {
	reg := w.factorySet.ExportRegistryFactory.CreateServiceRegistry()
	exports, err := reg.List(ctx)
	if err != nil {
		slog.Error("Failed to get exports", "error", err)
		return
	}

	for _, export := range exports {
		if export.Status != models.ExportStatusPending {
			continue
		}

		// Skip imported exports - they are handled by the import worker
		if export.Type == models.ExportTypeImported {
			continue
		}

		// Block until we can acquire a semaphore slot to limit concurrent goroutines
		if err := w.semaphore.Acquire(ctx, 1); err != nil {
			slog.Error("Failed to acquire semaphore", "error", err)
			return
		}

		go func(exportID string) {
			defer w.semaphore.Release(1)
			w.processExport(ctx, exportID)
		}(export.ID)
	}
}

// processExport processes a single export request
func (w *ExportWorker) processExport(ctx context.Context, exportID string) {
	slog.Info("Processing export", "export_id", exportID)

	if err := w.exportService.ProcessExport(ctx, exportID); err != nil {
		slog.Error("Failed to process export", "export_id", exportID, "error", err)
		return
	}
	slog.Info("Successfully processed export", "export_id", exportID)
}

// cleanupDeletedExports is deprecated - exports now use immediate hard delete with file entities
// This method is kept for backward compatibility but is no longer used
func (w *ExportWorker) cleanupDeletedExports(ctx context.Context) {
	// No-op: cleanup is now handled immediately during export deletion
	slog.Info("Export cleanup called but is no longer needed - exports use immediate deletion")
}
