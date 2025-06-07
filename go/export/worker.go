package export

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	defaultPollInterval = 10 * time.Second
)

// ExportWorker processes export requests in the background
type ExportWorker struct {
	exportService *ExportService
	registrySet   *registry.Set
	pollInterval  time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
	isRunning     bool
	mu            sync.RWMutex
	stopped       bool
	semaphore     *semaphore.Weighted
}

// NewExportWorker creates a new export worker
func NewExportWorker(exportService *ExportService, registrySet *registry.Set, maxConcurrentExports int) *ExportWorker {
	return &ExportWorker{
		exportService: exportService,
		registrySet:   registrySet,
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

	log.Print("Export worker started")
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
		log.Print("Export worker stopped")
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
	exports, err := w.registrySet.ExportRegistry.List(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to get exports")
		return
	}

	for _, export := range exports {
		if export.Status != models.ExportStatusPending {
			continue
		}

		// Block until we can acquire a semaphore slot to limit concurrent goroutines
		if err := w.semaphore.Acquire(ctx, 1); err != nil {
			log.WithError(err).Error("Failed to acquire semaphore")
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
	log.WithField("export_id", exportID).Info("Processing export")

	if err := w.exportService.ProcessExport(ctx, exportID); err != nil {
		log.WithError(err).WithField("export_id", exportID).Error("Failed to process export")
		return
	}
	log.WithField("export_id", exportID).Info("Successfully processed export")
}
