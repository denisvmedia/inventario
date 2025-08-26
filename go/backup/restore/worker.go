package restore

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

// NewRestoreWorker creates a new restore worker
func NewRestoreWorker(restoreService *RestoreService, registrySet *registry.Set, uploadLocation string) *RestoreWorker {
	const maxConcurrentRestores = 1
	return &RestoreWorker{
		restoreService: restoreService,
		registrySet:    registrySet,
		uploadLocation: uploadLocation,
		pollInterval:   defaultPollInterval, // Check for new restores every 10 seconds
		stopCh:         make(chan struct{}),
		semaphore:      semaphore.NewWeighted(int64(maxConcurrentRestores)),
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

	log.Print("Restore worker started")
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
		log.Print("Restore worker stopped")
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
	restoreOperations, err := w.registrySet.RestoreOperationRegistry.WithServiceAccount().List(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to get restore operations")
		return
	}

	for _, restoreOp := range restoreOperations {
		if restoreOp.Status != models.RestoreStatusPending {
			continue
		}

		// Attempt to acquire a semaphore slot to limit concurrent goroutines
		if !w.semaphore.TryAcquire(1) {
			log.Warn("Failed to acquire semaphore for restore, another restore is in progress, skipping...")
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
	log.WithField("restore_operation_id", restoreOperationID).Info("Processing restore operation")

	if err := w.restoreService.ProcessRestoreOperation(ctx, restoreOperationID, w.uploadLocation); err != nil {
		log.WithError(err).WithField("restore_operation_id", restoreOperationID).Error("Failed to process restore operation")
		return
	}
	log.WithField("restore_operation_id", restoreOperationID).Info("Successfully processed restore operation")
}

// HasRunningRestores checks if there are any restore operations currently running or pending
func (w *RestoreWorker) HasRunningRestores(ctx context.Context) (bool, error) {
	restoreOperations, err := w.registrySet.RestoreOperationRegistry.WithServiceAccount().List(ctx)
	if err != nil {
		return false, err
	}

	for _, restoreOp := range restoreOperations {
		if restoreOp.Status == models.RestoreStatusRunning || restoreOp.Status == models.RestoreStatusPending {
			return true, nil
		}
	}

	return false, nil
}
