package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	defaultThumbnailPollInterval    = 5 * time.Second  // Check for new jobs every 5 seconds
	defaultJobBatchSize             = 10               // Process up to 10 jobs per batch
	defaultCleanupInterval          = 5 * time.Minute  // Cleanup every 5 minutes
	defaultJobRetentionPeriod       = 24 * time.Hour   // Keep completed jobs for 24 hours
	defaultThumbnailJobBatchTimeout = 30 * time.Second // Wait this long for a batch before polling again
	defaultDetachedThumbnailTimeout = 2 * time.Minute  // Bound each detached thumbnail job
)

// PauseChecker reports whether a worker type is currently soft-paused
// (#1308). Declared locally in the services package so the workers depend
// only on models, not on the workerpause controller — avoiding an import
// cycle while still being satisfied by *workerpause.Controller. Shared by
// every go/services background worker.
type PauseChecker interface {
	IsPaused(models.WorkerType) bool
}

// ThumbnailGenerationWorker processes thumbnail generation requests in the background
type ThumbnailGenerationWorker struct {
	thumbnailService   *ThumbnailGenerationService
	factorySet         *registry.FactorySet
	pollInterval       time.Duration
	jobBatchSize       int
	cleanupInterval    time.Duration
	retentionPeriod    time.Duration
	jobBatchTimeout    time.Duration
	detachedJobTimeout time.Duration
	pause              PauseChecker
	stopCh             chan struct{}
	wg                 sync.WaitGroup
	isRunning          bool
	mu                 sync.RWMutex
	stopped            bool
}

// ThumbnailWorkerOption customizes a ThumbnailGenerationWorker constructed via NewThumbnailGenerationWorker.
type ThumbnailWorkerOption func(*thumbnailWorkerOptions)

type thumbnailWorkerOptions struct {
	pollInterval       time.Duration
	jobBatchSize       int
	cleanupInterval    time.Duration
	retentionPeriod    time.Duration
	jobBatchTimeout    time.Duration
	detachedJobTimeout time.Duration
	pause              PauseChecker
}

// WithThumbnailPollInterval overrides the default interval between job queue polls.
// Non-positive values are ignored.
func WithThumbnailPollInterval(d time.Duration) ThumbnailWorkerOption {
	return func(o *thumbnailWorkerOptions) {
		if d > 0 {
			o.pollInterval = d
		}
	}
}

// WithThumbnailBatchSize overrides the maximum number of jobs processed per batch.
// Non-positive values are ignored.
func WithThumbnailBatchSize(n int) ThumbnailWorkerOption {
	return func(o *thumbnailWorkerOptions) {
		if n > 0 {
			o.jobBatchSize = n
		}
	}
}

// WithThumbnailCleanupInterval overrides the default interval between completed-job cleanups.
// Non-positive values are ignored.
func WithThumbnailCleanupInterval(d time.Duration) ThumbnailWorkerOption {
	return func(o *thumbnailWorkerOptions) {
		if d > 0 {
			o.cleanupInterval = d
		}
	}
}

// WithThumbnailJobRetentionPeriod overrides the default retention period for completed jobs.
// Non-positive values are ignored.
func WithThumbnailJobRetentionPeriod(d time.Duration) ThumbnailWorkerOption {
	return func(o *thumbnailWorkerOptions) {
		if d > 0 {
			o.retentionPeriod = d
		}
	}
}

// WithThumbnailJobBatchTimeout overrides how long the worker waits for an in-flight batch
// before polling again. Non-positive values are ignored.
func WithThumbnailJobBatchTimeout(d time.Duration) ThumbnailWorkerOption {
	return func(o *thumbnailWorkerOptions) {
		if d > 0 {
			o.jobBatchTimeout = d
		}
	}
}

// WithDetachedThumbnailJobTimeout overrides the per-job timeout applied to each detached
// thumbnail generation goroutine. Non-positive values are ignored.
func WithDetachedThumbnailJobTimeout(d time.Duration) ThumbnailWorkerOption {
	return func(o *thumbnailWorkerOptions) {
		if d > 0 {
			o.detachedJobTimeout = d
		}
	}
}

// WithThumbnailPauseController wires the soft-pause controller so the
// worker skips its claim phase while the thumbnail worker type is paused
// (#1308). A nil checker leaves the worker unpaused.
func WithThumbnailPauseController(pc PauseChecker) ThumbnailWorkerOption {
	return func(o *thumbnailWorkerOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// NewThumbnailGenerationWorker creates a new thumbnail generation worker
func NewThumbnailGenerationWorker(factorySet *registry.FactorySet, uploadLocation string, config ThumbnailGenerationConfig, opts ...ThumbnailWorkerOption) *ThumbnailGenerationWorker {
	thumbnailService := NewThumbnailGenerationService(factorySet, uploadLocation, config)

	options := thumbnailWorkerOptions{
		pollInterval:       defaultThumbnailPollInterval,
		jobBatchSize:       defaultJobBatchSize,
		cleanupInterval:    defaultCleanupInterval,
		retentionPeriod:    defaultJobRetentionPeriod,
		jobBatchTimeout:    defaultThumbnailJobBatchTimeout,
		detachedJobTimeout: defaultDetachedThumbnailTimeout,
	}
	for _, opt := range opts {
		opt(&options)
	}

	return &ThumbnailGenerationWorker{
		thumbnailService:   thumbnailService,
		factorySet:         factorySet,
		pollInterval:       options.pollInterval,
		jobBatchSize:       options.jobBatchSize,
		cleanupInterval:    options.cleanupInterval,
		retentionPeriod:    options.retentionPeriod,
		jobBatchTimeout:    options.jobBatchTimeout,
		detachedJobTimeout: options.detachedJobTimeout,
		pause:              options.pause,
		stopCh:             make(chan struct{}),
	}
}

// Start begins processing thumbnail generation jobs in the background
func (w *ThumbnailGenerationWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.isRunning {
		return
	}

	w.isRunning = true
	w.wg.Add(2) // One for job processing, one for cleanup

	// Start job processing goroutine
	go func() {
		defer w.wg.Done()
		defer func() {
			w.mu.Lock()
			w.isRunning = false
			w.mu.Unlock()
		}()
		w.runJobProcessor(ctx)
	}()

	// Start cleanup goroutine
	go func() {
		defer w.wg.Done()
		w.runCleanup(ctx)
	}()

	slog.Info("Thumbnail generation worker started")
}

// Stop stops the thumbnail generation worker
func (w *ThumbnailGenerationWorker) Stop() {
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
		slog.Info("Thumbnail generation worker stopped")
	}()
}

// IsRunning returns whether the worker is currently running
func (w *ThumbnailGenerationWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// runJobProcessor is the main job processing loop
func (w *ThumbnailGenerationWorker) runJobProcessor(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processPendingJobs(ctx)
		}
	}
}

// runCleanup is the cleanup loop
func (w *ThumbnailGenerationWorker) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(w.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.performCleanup(ctx)
		}
	}
}

// processPendingJobs finds and processes pending thumbnail generation jobs
func (w *ThumbnailGenerationWorker) processPendingJobs(ctx context.Context) {
	// Soft-pause (#1308): skip the claim phase while paused. The ticker
	// keeps running so resuming takes effect on the next tick without a
	// restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeThumbnail) {
		return
	}

	jobs, err := w.thumbnailService.GetPendingJobs(ctx, w.jobBatchSize)
	if err != nil {
		slog.Error("Failed to get pending thumbnail jobs", "error", err)
		return
	}

	if len(jobs) == 0 {
		return // No jobs to process
	}

	slog.Debug("Processing thumbnail generation jobs", "count", len(jobs))

	// Process jobs concurrently, but let the concurrency service handle the limits
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		jobCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), w.detachedJobTimeout)
		go func(jobID string, jobCtx context.Context, cancel context.CancelFunc) {
			defer wg.Done()
			defer cancel()

			err := w.thumbnailService.ProcessThumbnailGeneration(jobCtx, jobID)
			if err != nil {
				slog.Error("Failed to process thumbnail generation job", "job_id", jobID, "error", err)
			}
		}(job.ID, jobCtx, cancel)
	}

	// Wait for all jobs in this batch to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All jobs completed
	case <-ctx.Done():
		// Context cancelled, but let jobs continue in background
		slog.Info("Context cancelled during job processing, jobs will continue in background")
	case <-time.After(w.jobBatchTimeout):
		// Timeout, but let jobs continue in background
		slog.Warn("Job processing batch timed out, jobs will continue in background")
	}
}

// performCleanup cleans up expired concurrency slots and old jobs.
//
// Order matters (#2122 F5): expired concurrency slots are deleted FIRST so
// that the user_concurrency_slots.job_id -> thumbnail_generation_jobs(id) FK
// (NO ACTION) is already broken before CleanupCompletedJobs tries to delete
// the job rows. The two run in separate transactions, so doing it the other
// way round let an orphan slot still reference a completed job and block its
// deletion.
func (w *ThumbnailGenerationWorker) performCleanup(ctx context.Context) {
	// Cleanup expired concurrency slots first to free the FK referencing
	// completed jobs.
	if err := w.thumbnailService.CleanupExpiredSlots(ctx); err != nil {
		slog.Error("Failed to cleanup expired concurrency slots", "error", err)
	}

	// Cleanup completed jobs now that no expired slot references them.
	if err := w.thumbnailService.CleanupCompletedJobs(ctx, w.retentionPeriod); err != nil {
		slog.Error("Failed to cleanup completed thumbnail jobs", "error", err)
	}
}

// GetStats returns worker statistics
func (w *ThumbnailGenerationWorker) GetStats(ctx context.Context) (map[string]any, error) {
	jobRegistry := w.factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()
	slotRegistry := w.factorySet.UserConcurrencySlotRegistryFactory.CreateServiceRegistry()

	// Get job counts by status
	jobs, err := jobRegistry.List(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]any{
		"worker_running": w.IsRunning(),
		"poll_interval":  w.pollInterval.String(),
		"batch_size":     w.jobBatchSize,
	}

	// Count jobs by status
	statusCounts := make(map[string]int)
	for _, job := range jobs {
		statusCounts[string(job.Status)]++
	}
	stats["job_counts"] = statusCounts

	// Get active slots count
	slots, err := slotRegistry.List(ctx)
	if err != nil {
		slog.Error("Failed to get slots for stats", "error", err)
	} else {
		stats["active_slots"] = len(slots)
	}

	return stats, nil
}
