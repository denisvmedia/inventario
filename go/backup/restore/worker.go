package restore

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
	defaultPollInterval          = 10 * time.Second
	defaultMaxConcurrentRestores = 1
)

// RestoreWorker processes restore requests in the background
type RestoreWorker struct {
	restoreService *RestoreService
	registrySet    *registry.Set
	uploadLocation string
	pollInterval   time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
	isRunning      bool
	mu             sync.RWMutex
	stopped        bool
	semaphore      *semaphore.Weighted
}

// WorkerOption customizes a RestoreWorker constructed via NewRestoreWorker.
type WorkerOption func(*restoreWorkerOptions)

type restoreWorkerOptions struct {
	pollInterval  time.Duration
	maxConcurrent int
}

// WithPollInterval overrides the default interval between restore queue polls.
// Non-positive values are ignored.
func WithPollInterval(d time.Duration) WorkerOption {
	return func(o *restoreWorkerOptions) {
		if d > 0 {
			o.pollInterval = d
		}
	}
}

// WithMaxConcurrent overrides the default number of restores processed in parallel.
// Non-positive values are ignored.
func WithMaxConcurrent(n int) WorkerOption {
	return func(o *restoreWorkerOptions) {
		if n > 0 {
			o.maxConcurrent = n
		}
	}
}

// NewRestoreWorker creates a new restore worker
func NewRestoreWorker(restoreService *RestoreService, registrySet *registry.Set, uploadLocation string, opts ...WorkerOption) *RestoreWorker {
	options := restoreWorkerOptions{
		pollInterval:  defaultPollInterval,
		maxConcurrent: defaultMaxConcurrentRestores,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &RestoreWorker{
		restoreService: restoreService,
		registrySet:    registrySet,
		uploadLocation: uploadLocation,
		pollInterval:   options.pollInterval,
		stopCh:         make(chan struct{}),
		semaphore:      semaphore.NewWeighted(int64(options.maxConcurrent)),
	}
}

// Start begins processing restores in the background
func (w *RestoreWorker) Start(ctx context.Context) {
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

	slog.Info("Restore worker started")
}

// Stop stops the restore worker
func (w *RestoreWorker) Stop() {
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
		slog.Info("Restore worker stopped")
	}()
}

// IsRunning returns whether the worker is currently running
func (w *RestoreWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// run is the main worker loop
func (w *RestoreWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processPendingRestores(ctx)
		}
	}
}

// processPendingRestores finds and processes pending restore requests
func (w *RestoreWorker) processPendingRestores(ctx context.Context) {
	restoreOperations, err := w.registrySet.RestoreOperationRegistry.List(ctx)
	if err != nil {
		slog.Error("Failed to get restore operations", "error", err)
		return
	}

	for _, restoreOp := range restoreOperations {
		if restoreOp.Status != models.RestoreStatusPending {
			continue
		}

		// Attempt to acquire a semaphore slot to limit concurrent goroutines
		if !w.semaphore.TryAcquire(1) {
			slog.Warn("Failed to acquire semaphore for restore, another restore is in progress, skipping...")
			return
		}

		go func(restoreOperationID string) {
			defer w.semaphore.Release(1)
			w.processRestore(ctx, restoreOperationID)
		}(restoreOp.ID)
	}
}

// processRestore processes a single restore request
func (w *RestoreWorker) processRestore(ctx context.Context, restoreOperationID string) {
	slog.Info("Processing restore operation", "restore_operation_id", restoreOperationID)

	if err := w.restoreService.ProcessRestoreOperation(ctx, restoreOperationID, w.uploadLocation); err != nil {
		slog.Error("Failed to process restore operation", "restore_operation_id", restoreOperationID, "error", err)
		return
	}
	slog.Info("Successfully processed restore operation", "restore_operation_id", restoreOperationID)
}

// HasRunningRestores checks if there are any restore operations currently
// running or pending. It delegates to RegistryStatusQuerier so that query-only
// callers (for example the HTTP API in an API-only deployment) can reuse the
// same implementation without a running worker.
func (w *RestoreWorker) HasRunningRestores(ctx context.Context) (bool, error) {
	return NewRegistryStatusQuerier(w.registrySet).HasRunningRestores(ctx)
}
