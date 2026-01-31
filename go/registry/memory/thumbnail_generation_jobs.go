package memory

import (
	"context"
	"slices"
	"time"

	"github.com/go-extras/errx"
	"github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ThumbnailGenerationJobRegistry = (*ThumbnailGenerationJobRegistry)(nil)

type baseThumbnailGenerationJobRegistry = Registry[models.ThumbnailGenerationJob, *models.ThumbnailGenerationJob]

type ThumbnailGenerationJobRegistry struct {
	*baseThumbnailGenerationJobRegistry
}

// ThumbnailGenerationJobRegistryFactory creates ThumbnailGenerationJobRegistry instances with proper context
type ThumbnailGenerationJobRegistryFactory struct {
	baseThumbnailGenerationJobRegistry *Registry[models.ThumbnailGenerationJob, *models.ThumbnailGenerationJob]
}

func NewThumbnailGenerationJobRegistryFactory() *ThumbnailGenerationJobRegistryFactory {
	return &ThumbnailGenerationJobRegistryFactory{
		baseThumbnailGenerationJobRegistry: NewRegistry[models.ThumbnailGenerationJob, *models.ThumbnailGenerationJob](),
	}
}

func (f *ThumbnailGenerationJobRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.ThumbnailGenerationJobRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, stacktrace.Wrap("failed to get user from context", err)
	}

	// Create a new registry with user context already set
	userRegistry := &Registry[models.ThumbnailGenerationJob, *models.ThumbnailGenerationJob]{
		items:  f.baseThumbnailGenerationJobRegistry.items, // Share the data map
		lock:   f.baseThumbnailGenerationJobRegistry.lock,  // Share the mutex pointer
		userID: user.ID,                                    // Set user-specific userID
	}

	return &ThumbnailGenerationJobRegistry{
		baseThumbnailGenerationJobRegistry: userRegistry,
	}, nil
}

func (f *ThumbnailGenerationJobRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.ThumbnailGenerationJobRegistry {
	reg, err := f.CreateUserRegistry(ctx)
	if err != nil {
		panic(err)
	}
	return reg
}

func (f *ThumbnailGenerationJobRegistryFactory) CreateServiceRegistry() registry.ThumbnailGenerationJobRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.ThumbnailGenerationJob, *models.ThumbnailGenerationJob]{
		items:  f.baseThumbnailGenerationJobRegistry.items, // Share the data map
		lock:   f.baseThumbnailGenerationJobRegistry.lock,  // Share the mutex pointer
		userID: "",                                         // Clear userID to bypass user filtering
	}

	return &ThumbnailGenerationJobRegistry{
		baseThumbnailGenerationJobRegistry: serviceRegistry,
	}
}

// GetPendingJobs returns pending thumbnail generation jobs ordered by creation time
func (r *ThumbnailGenerationJobRegistry) GetPendingJobs(ctx context.Context, limit int) ([]*models.ThumbnailGenerationJob, error) {
	jobs, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	// Filter pending jobs
	var pendingJobs []*models.ThumbnailGenerationJob
	for _, job := range jobs {
		if job.Status == models.ThumbnailStatusPending {
			pendingJobs = append(pendingJobs, job)
		}
	}

	// Sort by creation time (ascending) - first come, first served
	slices.SortFunc(pendingJobs, func(a, b *models.ThumbnailGenerationJob) int {
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1 // Earlier creation time first
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		return 0
	})

	// Apply limit
	if limit > 0 && len(pendingJobs) > limit {
		pendingJobs = pendingJobs[:limit]
	}

	return pendingJobs, nil
}

// GetJobByFileID returns the thumbnail generation job for a specific file
func (r *ThumbnailGenerationJobRegistry) GetJobByFileID(ctx context.Context, fileID string) (*models.ThumbnailGenerationJob, error) {
	jobs, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		if job.FileID == fileID {
			return job, nil
		}
	}

	return nil, registry.ErrNotFound
}

// UpdateJobStatus updates the status of a thumbnail generation job
func (r *ThumbnailGenerationJobRegistry) UpdateJobStatus(ctx context.Context, jobID string, status models.ThumbnailGenerationStatus, errorMessage string) error {
	job, err := r.Get(ctx, jobID)
	if err != nil {
		return err
	}

	job.Status = status
	job.ErrorMessage = errorMessage
	job.UpdatedAt = time.Now()

	_, err = r.Update(ctx, *job)
	return err
}

// CleanupCompletedJobs removes completed/failed jobs older than the specified duration
func (r *ThumbnailGenerationJobRegistry) CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) error {
	jobs, err := r.List(ctx)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-olderThan)
	var toDelete []string

	for _, job := range jobs {
		// Only cleanup completed or failed jobs
		if (job.Status == models.ThumbnailStatusCompleted || job.Status == models.ThumbnailStatusFailed) &&
			job.ProcessingCompletedAt != nil &&
			job.ProcessingCompletedAt.Before(cutoff) {
			toDelete = append(toDelete, job.ID)
		}
	}

	// Delete the jobs
	for _, jobID := range toDelete {
		err := r.Delete(ctx, jobID)
		if err != nil {
			return stacktrace.Wrap("failed to delete completed job", err, errx.Attrs("job_id", jobID))
		}
	}

	return nil
}
