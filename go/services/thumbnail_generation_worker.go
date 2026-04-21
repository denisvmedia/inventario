package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

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

// performCleanup cleans up old jobs and expired slots
func (w *ThumbnailGenerationWorker) performCleanup(ctx context.Context) {
	// Cleanup completed jobs
	err := w.thumbnailService.CleanupCompletedJobs(ctx, w.retentionPeriod)
	if err != nil {
		slog.Error("Failed to cleanup completed thumbnail jobs", "error", err)
	}

	// Cleanup expired concurrency slots
	err = w.thumbnailService.CleanupExpiredSlots(ctx)
	if err != nil {
		slog.Error("Failed to cleanup expired concurrency slots", "error", err)
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
