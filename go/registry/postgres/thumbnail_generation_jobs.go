package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// ThumbnailGenerationJobRegistryFactory creates ThumbnailGenerationJobRegistry instances with proper context
type ThumbnailGenerationJobRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// ThumbnailGenerationJobRegistry is a context-aware registry that can only be created through the factory
type ThumbnailGenerationJobRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

var _ registry.ThumbnailGenerationJobRegistry = (*ThumbnailGenerationJobRegistry)(nil)
var _ registry.ThumbnailGenerationJobRegistryFactory = (*ThumbnailGenerationJobRegistryFactory)(nil)

func NewThumbnailGenerationJobRegistry(dbx *sqlx.DB) *ThumbnailGenerationJobRegistryFactory {
	return NewThumbnailGenerationJobRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewThumbnailGenerationJobRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *ThumbnailGenerationJobRegistryFactory {
	return &ThumbnailGenerationJobRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.ThumbnailGenerationJobRegistryFactory

func (f *ThumbnailGenerationJobRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.ThumbnailGenerationJobRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *ThumbnailGenerationJobRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.ThumbnailGenerationJobRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}

	return &ThumbnailGenerationJobRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     user.ID,
		tenantID:   user.TenantID,
		service:    false,
	}, nil
}

func (f *ThumbnailGenerationJobRegistryFactory) CreateServiceRegistry() registry.ThumbnailGenerationJobRegistry {
	return &ThumbnailGenerationJobRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     "",
		tenantID:   "",
		service:    true,
	}
}

// Get returns a thumbnail generation job by ID
func (r *ThumbnailGenerationJobRegistry) Get(ctx context.Context, id string) (*models.ThumbnailGenerationJob, error) {
	var job models.ThumbnailGenerationJob
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &job)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get thumbnail generation job")
	}

	return &job, nil
}

// List returns all thumbnail generation jobs
func (r *ThumbnailGenerationJobRegistry) List(ctx context.Context) ([]*models.ThumbnailGenerationJob, error) {
	var jobs []*models.ThumbnailGenerationJob
	reg := r.newSQLRegistry()

	for job, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list thumbnail generation jobs")
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// Count returns the number of thumbnail generation jobs
func (r *ThumbnailGenerationJobRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count thumbnail generation jobs")
	}

	return cnt, nil
}

// Create creates a new thumbnail generation job
func (r *ThumbnailGenerationJobRegistry) Create(ctx context.Context, job models.ThumbnailGenerationJob) (*models.ThumbnailGenerationJob, error) {
	reg := r.newSQLRegistry()

	result, err := reg.Create(ctx, job, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create thumbnail generation job")
	}

	return &result, nil
}

// Update updates a thumbnail generation job
func (r *ThumbnailGenerationJobRegistry) Update(ctx context.Context, job models.ThumbnailGenerationJob) (*models.ThumbnailGenerationJob, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, job, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update thumbnail generation job")
	}

	return &job, nil
}

// Delete deletes a thumbnail generation job
func (r *ThumbnailGenerationJobRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errkit.Wrap(err, "failed to delete thumbnail generation job")
	}

	return nil
}

// GetJobByFileID retrieves a thumbnail generation job by file ID
func (r *ThumbnailGenerationJobRegistry) GetJobByFileID(ctx context.Context, fileID string) (*models.ThumbnailGenerationJob, error) {
	var job models.ThumbnailGenerationJob
	reg := r.newSQLRegistry()

	// Get the most recent job for this file
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE file_id = $1
			ORDER BY created_at DESC
			LIMIT 1`, r.tableNames.ThumbnailGenerationJobs())

		err := tx.GetContext(ctx, &job, query, fileID)
		if err != nil {
			return errkit.Wrap(err, "failed to get thumbnail generation job by file ID")
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &job, nil
}

// ListPending retrieves all pending thumbnail generation jobs
func (r *ThumbnailGenerationJobRegistry) ListPending(ctx context.Context) ([]*models.ThumbnailGenerationJob, error) {
	var jobs []*models.ThumbnailGenerationJob
	reg := r.newSQLRegistry()

	for job, err := range reg.ScanByField(ctx, store.Pair("status", models.ThumbnailStatusPending)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list pending thumbnail generation jobs")
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// GetPendingJobs returns pending thumbnail generation jobs ordered by priority and creation time
func (r *ThumbnailGenerationJobRegistry) GetPendingJobs(ctx context.Context, limit int) ([]*models.ThumbnailGenerationJob, error) {
	var jobs []models.ThumbnailGenerationJob
	reg := r.newSQLRegistry()

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE status = $1
			ORDER BY created_at ASC
			LIMIT $2`, r.tableNames.ThumbnailGenerationJobs())

		err := tx.SelectContext(ctx, &jobs, query, models.ThumbnailStatusPending, limit)
		if err != nil {
			return errkit.Wrap(err, "failed to get pending thumbnail generation jobs")
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert to pointer slice
	var result []*models.ThumbnailGenerationJob
	for i := range jobs {
		result = append(result, &jobs[i])
	}

	return result, nil
}

// UpdateJobStatus updates the status of a thumbnail generation job
func (r *ThumbnailGenerationJobRegistry) UpdateJobStatus(ctx context.Context, jobID string, status models.ThumbnailGenerationStatus, errorMessage string) error {
	reg := r.newSQLRegistry()

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			UPDATE %s
			SET status = $2, error_message = $3, updated_at = $4
			WHERE id = $1`, r.tableNames.ThumbnailGenerationJobs())

		_, err := tx.ExecContext(ctx, query, jobID, status, errorMessage, time.Now())
		if err != nil {
			return errkit.Wrap(err, "failed to update thumbnail generation job status")
		}
		return nil
	})

	return err
}

// CleanupCompletedJobs removes completed/failed jobs older than the specified duration
func (r *ThumbnailGenerationJobRegistry) CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) error {
	reg := r.newSQLRegistry()
	cutoffTime := time.Now().Add(-olderThan)

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			DELETE FROM %s
			WHERE (status = $1 OR status = $2) AND updated_at < $3`, r.tableNames.ThumbnailGenerationJobs())

		_, err := tx.ExecContext(ctx, query, models.ThumbnailStatusCompleted, models.ThumbnailStatusFailed, cutoffTime)
		if err != nil {
			return errkit.Wrap(err, "failed to cleanup completed thumbnail generation jobs")
		}
		return nil
	})

	return err
}

func (r *ThumbnailGenerationJobRegistry) newSQLRegistry() *store.RLSRepository[models.ThumbnailGenerationJob, *models.ThumbnailGenerationJob] {
	if r.service {
		return store.NewServiceSQLRegistry[models.ThumbnailGenerationJob](r.dbx, r.tableNames.ThumbnailGenerationJobs())
	}
	return store.NewUserAwareSQLRegistry[models.ThumbnailGenerationJob](r.dbx, r.userID, r.tenantID, r.tableNames.ThumbnailGenerationJobs())
}
