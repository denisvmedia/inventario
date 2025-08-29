package importpkg

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
)

// ImportWorker processes import requests in the background
type ImportWorker struct {
	importService *ImportService
	registrySet   *registry.Set
	pollInterval  time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
	isRunning     bool
	mu            sync.RWMutex
	stopped       bool
	semaphore     *semaphore.Weighted
}

// NewImportWorker creates a new import worker
func NewImportWorker(importService *ImportService, registrySet *registry.Set, maxConcurrentImports int) *ImportWorker {
	return &ImportWorker{
		importService: importService,
		registrySet:   registrySet,
		pollInterval:  defaultPollInterval, // Check for new imports every 10 seconds
		stopCh:        make(chan struct{}),
		semaphore:     semaphore.NewWeighted(int64(maxConcurrentImports)),
	}
}

// NewImportWorkerWithPollInterval creates a new import worker with custom poll interval
func NewImportWorkerWithPollInterval(importService *ImportService, registrySet *registry.Set, maxConcurrentImports int, pollInterval time.Duration) *ImportWorker {
	return &ImportWorker{
		importService: importService,
		registrySet:   registrySet,
		pollInterval:  pollInterval,
		stopCh:        make(chan struct{}),
		semaphore:     semaphore.NewWeighted(int64(maxConcurrentImports)),
	}
}

// Start begins processing imports in the background
func (w *ImportWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.isRunning {
		return
	}

	w.isRunning = true
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		defer func() {
			w.mu.Lock()
			w.isRunning = false
			w.mu.Unlock()
		}()
		w.run(ctx)
	}()

	slog.Info("Import worker started")
}

// Stop stops the import worker
func (w *ImportWorker) Stop() {
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
		slog.Info("Import worker stopped")
	}()
}

// IsRunning returns whether the worker is currently running
func (w *ImportWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// run is the main worker loop
func (w *ImportWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processPendingImports(ctx)
		}
	}
}

// processPendingImports finds and processes pending import requests
func (w *ImportWorker) processPendingImports(ctx context.Context) {
	exports, err := w.registrySet.ExportRegistry.WithServiceAccount().List(ctx)
	if err != nil {
		slog.Error("Failed to get exports", "error", err)
		return
	}

	for _, export := range exports {
		// Only process imported exports that are pending
		pendingImport := export.Imported && export.Status == models.ExportStatusPending
		if !pendingImport {
			continue
		}

		// Block until we can acquire a semaphore slot to limit concurrent goroutines
		if err := w.semaphore.Acquire(ctx, 1); err != nil {
			slog.Error("Failed to acquire semaphore", "error", err)
			return
		}

		go func(exportID, sourceFilePath string) {
			defer w.semaphore.Release(1)
			w.processImport(ctx, exportID, sourceFilePath)
		}(export.ID, export.FilePath)
	}
}

// processImport processes a single import operation
func (w *ImportWorker) processImport(ctx context.Context, exportID, sourceFilePath string) {
	slog.Info("Processing import", "export_id", exportID)

	err := w.importService.ProcessImport(ctx, exportID, sourceFilePath)
	if err != nil {
		slog.Error("Failed to process import", "export_id", exportID, "error", err)
		return
	}

	slog.Info("Import processed successfully", "export_id", exportID)
}
