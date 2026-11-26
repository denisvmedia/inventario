package export

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

const (
	defaultPollInterval = 10 * time.Second
	// Note: Cleanup interval removed - exports now use immediate hard delete with file entities

	// personalExpireInterval and personalSweepInterval space out the
	// deletion of expired personal data exports and the sweep for archives
	// whose row is gone (#2177).
	personalExpireInterval = time.Minute
	personalSweepInterval  = time.Hour
)

// PauseChecker reports whether a worker type is currently soft-paused
// (#1308). Declared locally so this package depends only on models, not
// on the workerpause controller — avoiding an import cycle while still
// being satisfied by *workerpause.Controller.
type PauseChecker interface {
	IsPaused(models.WorkerType) bool
}

// ExportWorker processes export requests in the background
type ExportWorker struct {
	exportService *ExportService
	factorySet    *registry.FactorySet
	pollInterval  time.Duration
	pause         PauseChecker
	stopCh        chan struct{}
	wg            sync.WaitGroup
	isRunning     bool
	mu            sync.RWMutex
	stopped       bool
	semaphore     *semaphore.Weighted
	// inFlight skips a row this process has already dispatched, saving the
	// round trip the claim below would otherwise make on every tick (#2131).
	//
	// It is no longer what makes the guarantee. ClaimPending moves the row
	// from pending under a condition the database evaluates, so two replicas
	// racing for it get one winner (#2472); this set is an optimisation on
	// top of that, not the thing standing between one artifact and two.
	inFlight sync.Map

	// personal builds and expires personal data exports (#2177); nil
	// leaves them alone.
	personal        *PersonalDataExportService
	lastExpiry      time.Time
	lastOrphanSweep time.Time
}

// WorkerOption customizes an ExportWorker constructed via NewExportWorker.
type WorkerOption func(*exportWorkerOptions)

type exportWorkerOptions struct {
	pollInterval time.Duration
	pause        PauseChecker
	personal     *PersonalDataExportService
}

// WithPollInterval overrides the default interval between export queue polls.
// Non-positive values are ignored.
func WithPollInterval(d time.Duration) WorkerOption {
	return func(o *exportWorkerOptions) {
		if d > 0 {
			o.pollInterval = d
		}
	}
}

// WithPauseController wires the soft-pause controller so the worker skips
// its claim phase while the export worker type is paused (#1308). A nil
// checker leaves the worker unpaused.
func WithPauseController(pc PauseChecker) WorkerOption {
	return func(o *exportWorkerOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// WithPersonalDataExports makes the worker build the pending personal data
// exports and delete the expired ones (#2177), sharing the group exports'
// concurrency limit.
func WithPersonalDataExports(s *PersonalDataExportService) WorkerOption {
	return func(o *exportWorkerOptions) {
		o.personal = s
	}
}

// NewExportWorker creates a new export worker
func NewExportWorker(exportService *ExportService, factorySet *registry.FactorySet, maxConcurrentExports int, opts ...WorkerOption) *ExportWorker {
	options := exportWorkerOptions{
		pollInterval: defaultPollInterval,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &ExportWorker{
		exportService: exportService,
		factorySet:    factorySet,
		pollInterval:  options.pollInterval,
		pause:         options.pause,
		personal:      options.personal,
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

	slog.InfoContext(ctx, "Export worker started")
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
			w.processPersonalDataExports(ctx)
		}
	}
}

// processPendingExports finds and processes pending export requests
func (w *ExportWorker) processPendingExports(ctx context.Context) {
	// Soft-pause (#1308): skip the claim phase while paused. The ticker
	// keeps running so resuming takes effect on the next tick without a
	// restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeExport) {
		return
	}

	reg := w.factorySet.ExportRegistryFactory.CreateServiceRegistry()
	exports, err := reg.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get exports", "error", err)
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

		if _, running := w.inFlight.LoadOrStore(export.ID, struct{}{}); running {
			continue
		}

		// Take the slot before the claim. A claim that lands without one
		// leaves the row running with nobody running it, and the next tick
		// only looks at pending rows. This blocks until a slot frees up.
		if err := w.semaphore.Acquire(ctx, 1); err != nil {
			w.inFlight.Delete(export.ID)
			slog.ErrorContext(ctx, "Failed to acquire semaphore", "error", err)
			return
		}

		// Claim the row before dispatching: the condition is in the write, so
		// exactly one caller wins it however many replicas are polling (#2472).
		claimed, claimErr := reg.ClaimPending(ctx, export.ID)
		if claimErr != nil {
			w.semaphore.Release(1)
			w.inFlight.Delete(export.ID)
			slog.ErrorContext(ctx, "Failed to claim export", "export_id", export.ID, "error", claimErr)
			continue
		}
		if !claimed {
			// Another worker got there first, or the row moved on.
			w.semaphore.Release(1)
			w.inFlight.Delete(export.ID)
			continue
		}

		go func(exportID string) {
			defer w.semaphore.Release(1)
			defer w.inFlight.Delete(exportID)
			w.processExport(ctx, exportID)
		}(export.ID)
	}
}

// processExport processes a single export request
func (w *ExportWorker) processExport(ctx context.Context, exportID string) {
	slog.InfoContext(ctx, "Processing export", "export_id", exportID)

	if err := w.exportService.ProcessExport(ctx, exportID); err != nil {
		slog.ErrorContext(ctx, "Failed to process export", "export_id", exportID, "error", err)
		return
	}
	slog.InfoContext(ctx, "Successfully processed export", "export_id", exportID)
}

// processPersonalDataExports dispatches the pending personal data exports and,
// at its own interval, deletes the expired ones (#2177). Soft-pause stops
// both, like the group exports.
func (w *ExportWorker) processPersonalDataExports(ctx context.Context) {
	if w.personal == nil || (w.pause != nil && w.pause.IsPaused(models.WorkerTypeExport)) {
		return
	}

	w.personalHousekeeping(ctx, time.Now())

	reg := w.factorySet.PersonalDataExportRegistry
	pending, err := reg.ListPending(ctx)
	if err != nil {
		slog.Error("Failed to list pending personal data exports", "error", err)
		return
	}
	for _, export := range pending {
		key := "personal:" + export.ID
		if _, running := w.inFlight.LoadOrStore(key, struct{}{}); running {
			continue
		}
		if err := w.semaphore.Acquire(ctx, 1); err != nil {
			w.inFlight.Delete(key)
			slog.Error("Failed to acquire semaphore", "error", err)
			return
		}
		claimed, err := w.personal.Claim(ctx, export.ID)
		if err != nil || !claimed {
			w.semaphore.Release(1)
			w.inFlight.Delete(key)
			if err != nil {
				slog.Error("Failed to claim personal data export", "export_id", export.ID, "error", err)
			}
			continue
		}
		go func(exportID string) {
			defer w.semaphore.Release(1)
			defer w.inFlight.Delete(key)
			slog.Info("Processing personal data export", "export_id", exportID)
			if err := w.personal.Process(ctx, exportID); err != nil {
				slog.Error("Failed to process personal data export", "export_id", exportID, "error", err)
				return
			}
			slog.Info("Successfully processed personal data export", "export_id", exportID)
		}(export.ID)
	}
}

// personalHousekeeping queues stalled builds again and deletes expired
// exports every personalExpireInterval, and sweeps archives without a row
// every personalSweepInterval.
func (w *ExportWorker) personalHousekeeping(ctx context.Context, now time.Time) {
	if now.Sub(w.lastExpiry) >= personalExpireInterval {
		w.lastExpiry = now
		if n, err := w.personal.RequeueStalled(ctx); err != nil {
			slog.Error("Failed to queue stalled personal data exports again", "error", err)
		} else if n > 0 {
			slog.Warn("Queued stalled personal data exports again", "requeued", n)
		}
		if n, err := w.personal.ExpireDue(ctx); err != nil {
			slog.Error("Failed to delete expired personal data exports", "deleted", n, "error", err)
		} else if n > 0 {
			slog.Info("Deleted expired personal data exports", "deleted", n)
		}
	}
	if now.Sub(w.lastOrphanSweep) >= personalSweepInterval {
		w.lastOrphanSweep = now
		if n, err := w.personal.SweepOrphanedArchives(ctx); err != nil {
			slog.Error("Failed to sweep orphaned personal data export archives", "deleted", n, "error", err)
		} else if n > 0 {
			slog.Info("Deleted orphaned personal data export archives", "deleted", n)
		}
	}
}
