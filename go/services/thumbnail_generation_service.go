package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ThumbnailGenerationService manages thumbnail generation jobs and coordination
type ThumbnailGenerationService struct {
	factorySet         *registry.FactorySet
	fileService        *FileService
	concurrencyService *ThumbnailConcurrencyService
	rateLimitService   *ThumbnailRateLimitService
	uploadLocation     string
}

// ThumbnailGenerationConfig contains configuration for thumbnail generation
type ThumbnailGenerationConfig struct {
	MaxConcurrentPerUser int
	RateLimitPerMinute   int
	SlotDuration         time.Duration
}

// NewThumbnailGenerationService creates a new thumbnail generation service
func NewThumbnailGenerationService(factorySet *registry.FactorySet, uploadLocation string, config ThumbnailGenerationConfig) *ThumbnailGenerationService {
	fileService := NewFileService(factorySet, uploadLocation)
	concurrencyService := NewThumbnailConcurrencyService(factorySet, config.MaxConcurrentPerUser, config.SlotDuration)
	rateLimitService := NewThumbnailRateLimitService(config.RateLimitPerMinute)

	return &ThumbnailGenerationService{
		factorySet:         factorySet,
		fileService:        fileService,
		concurrencyService: concurrencyService,
		rateLimitService:   rateLimitService,
		uploadLocation:     uploadLocation,
	}
}

// RequestThumbnailGeneration requests thumbnail generation for a file with resource limiting
func (s *ThumbnailGenerationService) RequestThumbnailGeneration(ctx context.Context, fileID string) (*models.ThumbnailGenerationJob, error) {
	jobRegistry := s.factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()

	// Get file to determine user for rate limiting
	fileRegistry := s.factorySet.FileRegistryFactory.CreateServiceRegistry()
	file, err := fileRegistry.Get(ctx, fileID)
	if err != nil {
		return nil, stacktrace.Wrap("failed to get file for thumbnail generation", err)
	}

	// Check rate limit first
	err = s.rateLimitService.CheckRateLimit(ctx, file.UserID)
	if err != nil {
		if errors.Is(err, ErrRateLimitExceeded) {
			return nil, err // Return rate limit error as-is with stack trace
		}
		return nil, stacktrace.Wrap("failed to check rate limit", err)
	}

	// Check if a job already exists for this file
	existingJob, err := jobRegistry.GetJobByFileID(ctx, fileID)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, stacktrace.Wrap("failed to check for existing thumbnail job", err)
	}

	// If job exists and is not failed, return it
	if existingJob != nil && existingJob.Status != models.ThumbnailStatusFailed {
		slog.Debug("Thumbnail generation job already exists", "file_id", fileID, "job_id", existingJob.ID, "status", existingJob.Status)
		return existingJob, nil
	}

	// Create new job
	now := time.Now()
	job := models.ThumbnailGenerationJob{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: file.TenantID,
			UserID:   file.UserID,
		},
		FileID:       fileID,
		Status:       models.ThumbnailStatusPending,
		AttemptCount: 0,
		MaxAttempts:  3,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	createdJob, err := jobRegistry.Create(ctx, job)
	if err != nil {
		return nil, stacktrace.Wrap("failed to create thumbnail generation job", err)
	}

	slog.Info("Requested thumbnail generation", "file_id", fileID, "job_id", createdJob.ID, "user_id", file.UserID)
	return createdJob, nil
}

// GetRateLimitStatus returns the current rate limit status for a user
func (s *ThumbnailGenerationService) GetRateLimitStatus(ctx context.Context, userID string) int {
	return s.rateLimitService.GetRemainingRequests(ctx, userID)
}

// Stop stops the thumbnail generation service and cleans up resources
func (s *ThumbnailGenerationService) Stop() {
	s.rateLimitService.Stop()
}

// ProcessThumbnailGeneration processes a single thumbnail generation job with resource limiting
func (s *ThumbnailGenerationService) ProcessThumbnailGeneration(ctx context.Context, jobID string) error {
	jobRegistry := s.factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()

	// Get the job
	job, err := jobRegistry.Get(ctx, jobID)
	if err != nil {
		return stacktrace.Wrap("failed to get thumbnail generation job", err)
	}

	// Skip if job is not pending
	if job.Status != models.ThumbnailStatusPending {
		slog.Debug("Skipping non-pending thumbnail job", "job_id", jobID, "status", job.Status)
		return nil
	}

	// Check user's current generation count and enforce limits
	userSlots, err := s.concurrencyService.GetUserSlots(ctx, job.UserID)
	if err != nil {
		return stacktrace.Wrap("failed to get user concurrency slots", err)
	}

	// Count active generations for this user
	activeGenerations := len(userSlots)

	// Hard limit: deny if user has too many active generations
	const maxActiveGenerations = 50
	if activeGenerations >= maxActiveGenerations {
		slog.Warn("User has too many active thumbnail generations, denying request",
			"user_id", job.UserID, "active_count", activeGenerations, "max_allowed", maxActiveGenerations)
		s.markJobFailed(ctx, jobRegistry, job, fmt.Sprintf("too many active generations (%d/%d)", activeGenerations, maxActiveGenerations))
		return nil // Don't return error - this is expected behavior
	}

	// Soft limit: wait for resources if user has many active generations
	const maxConcurrentGenerations = 10
	if activeGenerations >= maxConcurrentGenerations {
		slog.Debug("User at concurrent generation limit, waiting for resources",
			"user_id", job.UserID, "active_count", activeGenerations, "max_concurrent", maxConcurrentGenerations)
		return nil // Return without error - will be retried later
	}

	// Try to acquire a concurrency slot
	slot, err := s.concurrencyService.AcquireSlot(ctx, job.UserID, job.ID)
	if err != nil {
		slog.Debug("Could not acquire concurrency slot for thumbnail generation", "job_id", jobID, "user_id", job.UserID, "error", err)
		return nil // Don't treat this as an error - will be retried
	}

	// Ensure we release the slot when done
	defer func() {
		if releaseErr := s.concurrencyService.ReleaseSlot(ctx, job.UserID, job.ID); releaseErr != nil {
			slog.Error("Failed to release concurrency slot", "job_id", jobID, "user_id", job.UserID, "error", releaseErr)
		}
	}()

	// Update job status to processing
	now := time.Now()
	err = jobRegistry.UpdateJobStatus(ctx, job.ID, models.ThumbnailStatusProcessing, "")
	if err != nil {
		return stacktrace.Wrap("failed to update job status to processing", err)
	}

	// Update processing started time
	job.ProcessingStartedAt = &now
	_, err = jobRegistry.Update(ctx, *job)
	if err != nil {
		slog.Error("Failed to update job processing started time", "job_id", jobID, "error", err)
		// Continue anyway
	}

	slog.Info("Starting thumbnail generation", "job_id", jobID, "file_id", job.FileID, "user_id", job.UserID, "slot_id", slot.ID)

	// Get the file
	fileRegistry := s.factorySet.FileRegistryFactory.CreateServiceRegistry()
	file, err := fileRegistry.Get(ctx, job.FileID)
	if err != nil {
		s.markJobFailed(ctx, jobRegistry, job, "failed to get file: "+err.Error())
		return stacktrace.Wrap("failed to get file for thumbnail generation", err)
	}

	// Generate thumbnails
	err = s.fileService.GenerateThumbnails(ctx, file)
	if err != nil {
		s.markJobFailed(ctx, jobRegistry, job, "failed to generate thumbnails: "+err.Error())
		return stacktrace.Wrap("failed to generate thumbnails", err)
	}

	// Mark job as completed
	completedAt := time.Now()
	job.ProcessingCompletedAt = &completedAt
	_, err = jobRegistry.Update(ctx, *job)
	if err != nil {
		slog.Error("Failed to update job completion time", "job_id", jobID, "error", err)
		// Continue anyway
	}

	err = jobRegistry.UpdateJobStatus(ctx, job.ID, models.ThumbnailStatusCompleted, "")
	if err != nil {
		slog.Error("Failed to mark job as completed", "job_id", jobID, "error", err)
		return stacktrace.Wrap("failed to mark job as completed", err)
	}

	slog.Info("Completed thumbnail generation", "job_id", jobID, "file_id", job.FileID, "user_id", job.UserID)
	return nil
}

// GetPendingJobs returns pending thumbnail generation jobs
func (s *ThumbnailGenerationService) GetPendingJobs(ctx context.Context, limit int) ([]*models.ThumbnailGenerationJob, error) {
	jobRegistry := s.factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()

	jobs, err := jobRegistry.GetPendingJobs(ctx, limit)
	if err != nil {
		return nil, stacktrace.Wrap("failed to get pending thumbnail jobs", err)
	}

	return jobs, nil
}

// GetJobByFileID returns the thumbnail generation job for a specific file
func (s *ThumbnailGenerationService) GetJobByFileID(ctx context.Context, fileID string) (*models.ThumbnailGenerationJob, error) {
	jobRegistry := s.factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()

	job, err := jobRegistry.GetJobByFileID(ctx, fileID)
	if err != nil {
		return nil, stacktrace.Wrap("failed to get thumbnail job by file ID", err)
	}

	return job, nil
}

// CleanupCompletedJobs removes completed/failed jobs older than the specified duration
func (s *ThumbnailGenerationService) CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) error {
	jobRegistry := s.factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()

	err := jobRegistry.CleanupCompletedJobs(ctx, olderThan)
	if err != nil {
		return stacktrace.Wrap("failed to cleanup completed thumbnail jobs", err)
	}

	return nil
}

// CleanupExpiredSlots removes expired concurrency slots
func (s *ThumbnailGenerationService) CleanupExpiredSlots(ctx context.Context) error {
	return s.concurrencyService.CleanupExpiredSlots(ctx)
}

// markJobFailed marks a job as failed and increments attempt count
func (s *ThumbnailGenerationService) markJobFailed(ctx context.Context, jobRegistry registry.ThumbnailGenerationJobRegistry, job *models.ThumbnailGenerationJob, errorMessage string) {
	job.AttemptCount++
	job.ErrorMessage = errorMessage

	// If we've exceeded max attempts, mark as failed permanently
	status := models.ThumbnailStatusFailed
	if job.AttemptCount < job.MaxAttempts {
		status = models.ThumbnailStatusPending // Allow retry
	}

	completedAt := time.Now()
	job.ProcessingCompletedAt = &completedAt

	_, updateErr := jobRegistry.Update(ctx, *job)
	if updateErr != nil {
		slog.Error("Failed to update job after failure", "job_id", job.ID, "error", updateErr)
	}

	statusErr := jobRegistry.UpdateJobStatus(ctx, job.ID, status, errorMessage)
	if statusErr != nil {
		slog.Error("Failed to update job status after failure", "job_id", job.ID, "error", statusErr)
	}

	slog.Error("Thumbnail generation job failed", "job_id", job.ID, "file_id", job.FileID, "attempt", job.AttemptCount, "max_attempts", job.MaxAttempts, "error", errorMessage)
}
