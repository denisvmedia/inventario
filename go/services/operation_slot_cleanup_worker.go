// OperationSlotCleanupWorker mirrors RefreshTokenCleanupWorker /
// EmailVerificationCleanupWorker by design — same Start/Stop/runCleanup/
// cleanupOnce lifecycle and the same soft-pause skip (#1308). Each worker
// still owns its own registry type, worker-type pause key, and log wording,
// so a shared generic base would erase those per-worker specifics for
// negligible LOC savings.
package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const defaultOperationSlotCleanupInterval = 5 * time.Minute

// OperationSlotCleanupWorker periodically deletes expired operation slots
// from the registry (#2122 F4). Operation slots are the DB-backed
// concurrency-limiter rows; without a scheduled sweep, expired rows linger
// and widen the window in which a stale slot can block the deletion of the
// user/job rows it references. For PostgreSQL this keeps the operation_slots
// table from growing unbounded; for the in-memory registry it reclaims
// memory during long-running test servers.
type OperationSlotCleanupWorker struct {
	registry        registry.OperationSlotRegistry
	cleanupInterval time.Duration
	pause           PauseChecker
	stopCh          chan struct{}
	stopOnce        sync.Once
	wg              sync.WaitGroup
}

// OperationSlotCleanupOption customizes an OperationSlotCleanupWorker created by NewOperationSlotCleanupWorker.
type OperationSlotCleanupOption func(*operationSlotCleanupOptions)

type operationSlotCleanupOptions struct {
	cleanupInterval time.Duration
	pause           PauseChecker
}

// WithOperationSlotCleanupInterval overrides the default cleanup interval.
// Non-positive values are ignored.
func WithOperationSlotCleanupInterval(d time.Duration) OperationSlotCleanupOption {
	return func(o *operationSlotCleanupOptions) {
		if d > 0 {
			o.cleanupInterval = d
		}
	}
}

// WithOperationSlotCleanupPauseController wires the soft-pause controller
// so the worker skips its cleanup while the operation-slot-cleanup worker
// type is paused (#1308). A nil checker leaves the worker unpaused.
func WithOperationSlotCleanupPauseController(pc PauseChecker) OperationSlotCleanupOption {
	return func(o *operationSlotCleanupOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// NewOperationSlotCleanupWorker creates a cleanup worker with the default five-minute interval,
// overridable via OperationSlotCleanupOption values (e.g., WithOperationSlotCleanupInterval).
func NewOperationSlotCleanupWorker(r registry.OperationSlotRegistry, opts ...OperationSlotCleanupOption) *OperationSlotCleanupWorker {
	options := operationSlotCleanupOptions{
		cleanupInterval: defaultOperationSlotCleanupInterval,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &OperationSlotCleanupWorker{
		registry:        r,
		cleanupInterval: options.cleanupInterval,
		pause:           options.pause,
		stopCh:          make(chan struct{}),
	}
}

// Start launches the background cleanup goroutine. It is a no-op if the registry is nil.
func (w *OperationSlotCleanupWorker) Start(ctx context.Context) {
	if w.registry == nil {
		slog.Warn("OperationSlotCleanupWorker: no registry configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.runCleanup(ctx)
	})
	slog.Info("Operation slot cleanup worker started", "interval", w.cleanupInterval)
}

// Stop signals the worker to stop and waits for it to finish.
func (w *OperationSlotCleanupWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Operation slot cleanup worker stopped")
}

func (w *OperationSlotCleanupWorker) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(w.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.cleanupOnce(ctx)
		}
	}
}

// cleanupOnce runs a single expired-slot sweep.
func (w *OperationSlotCleanupWorker) cleanupOnce(ctx context.Context) {
	// Soft-pause (#1308): skip the cleanup while paused. The ticker keeps
	// running so resuming takes effect on the next tick without a restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeOperationSlotCleanup) {
		return
	}

	deleted, err := w.registry.CleanupExpiredSlots(ctx)
	if err != nil {
		slog.Error("Failed to delete expired operation slots", "error", err)
		return
	}
	if deleted > 0 {
		slog.Debug("Expired operation slots cleaned up", "count", deleted)
	}
}
