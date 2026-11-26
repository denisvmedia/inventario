package memory

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var _ registry.FailedJobRegistry = (*FailedJobRegistry)(nil)

// FailedJobRegistry is the in-memory counterpart of postgres.FailedJobRegistry
// (#1307). Each job type's adapter reads and changes the rows of the factory
// that owns them, under that factory's lock.
type FailedJobRegistry struct {
	adapters map[models.WorkerType]memoryFailedJobAdapter
}

// memoryFailedJobAdapter is one job type over its store. retry is nil for a
// type that cannot be retried.
type memoryFailedJobAdapter struct {
	list    func(scope registry.FailedJobScope) []*models.FailedJob
	retry   func(scope registry.FailedJobScope, id string, now time.Time) bool
	discard func(scope registry.FailedJobScope, id string, now time.Time) bool
}

// FailedJobStores are the factories whose rows the failed-jobs adapters read.
type FailedJobStores struct {
	Exports            *ExportRegistryFactory
	Restores           *RestoreOperationRegistryFactory
	Thumbnails         *ThumbnailGenerationJobRegistryFactory
	Files              *FileRegistryFactory
	Intake             *IntakeRegistryFactory
	CurrencyMigrations *CurrencyMigrationRegistryFactory
}

// NewFailedJobRegistry wires the adapters to the stores. Every store is
// required.
func NewFailedJobRegistry(stores FailedJobStores) *FailedJobRegistry {
	exportAdapter := func(imported bool) memoryFailedJobAdapter {
		match := func(scope registry.FailedJobScope, e *models.Export) bool {
			return e.Status == models.ExportStatusFailed && e.DiscardedAt == nil && e.DeletedAt == nil &&
				(e.Type == models.ExportTypeImported) == imported &&
				inFailedJobScope(scope, e.TenantID, e.GroupID)
		}
		jobType := models.WorkerTypeExport
		if imported {
			jobType = models.WorkerTypeImport
		}
		base := stores.Exports.baseExportRegistry
		return memoryFailedJobAdapter{
			list: func(scope registry.FailedJobScope) []*models.FailedJob {
				return listFailedIn(base, func(e *models.Export) (*models.FailedJob, bool) {
					if !match(scope, e) {
						return nil, false
					}
					return &models.FailedJob{
						Type: jobType, ID: e.ID, TenantID: e.TenantID, GroupID: e.GroupID, UserID: e.CreatedByUserID,
						Title: e.Description, Error: e.ErrorMessage, Attempts: max(e.AttemptCount, 1),
						CreatedAt: timestampValue(e.CreatedDate), FailedAt: timestampPtr(e.CompletedDate),
					}, true
				})
			},
			retry: func(scope registry.FailedJobScope, id string, _ time.Time) bool {
				return updateFailedIn(base, id, func(e *models.Export) bool { return match(scope, e) }, func(e *models.Export) {
					e.Status = models.ExportStatusPending
					e.ErrorMessage = ""
					e.CompletedDate = nil
				})
			},
			discard: func(scope registry.FailedJobScope, id string, now time.Time) bool {
				return updateFailedIn(base, id, func(e *models.Export) bool { return match(scope, e) }, func(e *models.Export) {
					e.DiscardedAt = models.NewPTimestamp(now)
				})
			},
		}
	}

	restores := stores.Restores.baseRestoreOperationRegistry
	restoreMatch := func(scope registry.FailedJobScope, op *models.RestoreOperation) bool {
		return op.Status == models.RestoreStatusFailed && op.DiscardedAt == nil &&
			inFailedJobScope(scope, op.TenantID, op.GroupID)
	}

	migrations := stores.CurrencyMigrations.baseRegistry
	migrationMatch := func(scope registry.FailedJobScope, m *models.CurrencyMigration) bool {
		return m.Status == models.CurrencyMigrationStatusFailed && m.DiscardedAt == nil &&
			inFailedJobScope(scope, m.TenantID, m.GroupID)
	}

	return &FailedJobRegistry{adapters: map[models.WorkerType]memoryFailedJobAdapter{
		models.WorkerTypeExport: exportAdapter(false),
		models.WorkerTypeImport: exportAdapter(true),
		models.WorkerTypeRestore: {
			list: func(scope registry.FailedJobScope) []*models.FailedJob {
				return listFailedIn(restores, func(op *models.RestoreOperation) (*models.FailedJob, bool) {
					if !restoreMatch(scope, op) {
						return nil, false
					}
					return &models.FailedJob{
						Type: models.WorkerTypeRestore, ID: op.ID, TenantID: op.TenantID, GroupID: op.GroupID,
						UserID: op.CreatedByUserID, Title: op.Description, Error: op.ErrorMessage, Attempts: 1,
						CreatedAt: timestampValue(op.CreatedDate), FailedAt: timestampPtr(op.CompletedDate),
					}, true
				})
			},
			discard: func(scope registry.FailedJobScope, id string, now time.Time) bool {
				return updateFailedIn(restores, id, func(op *models.RestoreOperation) bool { return restoreMatch(scope, op) },
					func(op *models.RestoreOperation) { op.DiscardedAt = models.NewPTimestamp(now) })
			},
		},
		models.WorkerTypeThumbnail: newThumbnailFailedJobAdapter(stores.Thumbnails, stores.Files),
		models.WorkerTypeIntake:    newIntakeFailedJobAdapter(stores.Intake, stores.Files),
		models.WorkerTypeCurrencyMigration: {
			list: func(scope registry.FailedJobScope) []*models.FailedJob {
				return listFailedIn(migrations, func(m *models.CurrencyMigration) (*models.FailedJob, bool) {
					if !migrationMatch(scope, m) {
						return nil, false
					}
					return &models.FailedJob{
						Type: models.WorkerTypeCurrencyMigration, ID: m.ID, TenantID: m.TenantID, GroupID: m.GroupID,
						UserID: m.CreatedByUserID, Title: string(m.FromCurrency) + " → " + string(m.ToCurrency),
						Error: m.ErrorMessage, Attempts: 1, CreatedAt: m.CreatedAt, FailedAt: m.CompletedAt,
					}, true
				})
			},
			discard: func(scope registry.FailedJobScope, id string, now time.Time) bool {
				return updateFailedIn(migrations, id, func(m *models.CurrencyMigration) bool { return migrationMatch(scope, m) },
					func(m *models.CurrencyMigration) { m.DiscardedAt = new(now) })
			},
		},
	}}
}

func newThumbnailFailedJobAdapter(jobs *ThumbnailGenerationJobRegistryFactory, files *FileRegistryFactory) memoryFailedJobAdapter {
	base := jobs.baseThumbnailGenerationJobRegistry
	// fileOf returns the group and title of the job's file. A thumbnail job
	// has no group of its own.
	fileOf := func(fileID string) (groupID, title string) {
		files.baseFileRegistry.lock.RLock()
		defer files.baseFileRegistry.lock.RUnlock()
		f, ok := files.baseFileRegistry.items.Get(fileID)
		if !ok {
			return "", ""
		}
		return f.GroupID, fileTitle(f)
	}
	match := func(scope registry.FailedJobScope, j *models.ThumbnailGenerationJob, groupID string) bool {
		return j.Status == models.ThumbnailStatusFailed && j.DiscardedAt == nil &&
			inFailedJobScope(scope, j.TenantID, groupID)
	}
	// Read the job's file before taking the job store's lock, so the two
	// locks are never held together.
	matchByID := func(scope registry.FailedJobScope, id string) func(*models.ThumbnailGenerationJob) bool {
		groupID := ""
		base.lock.RLock()
		if j, ok := base.items.Get(id); ok {
			fileID := j.FileID
			base.lock.RUnlock()
			groupID, _ = fileOf(fileID)
		} else {
			base.lock.RUnlock()
		}
		return func(j *models.ThumbnailGenerationJob) bool { return match(scope, j, groupID) }
	}
	return memoryFailedJobAdapter{
		list: func(scope registry.FailedJobScope) []*models.FailedJob {
			candidates := listFailedIn(base, func(j *models.ThumbnailGenerationJob) (*models.FailedJob, bool) {
				if j.Status != models.ThumbnailStatusFailed || j.DiscardedAt != nil {
					return nil, false
				}
				return &models.FailedJob{
					Type: models.WorkerTypeThumbnail, ID: j.ID, TenantID: j.TenantID, UserID: j.UserID,
					Title: j.FileID, Error: j.ErrorMessage, Attempts: j.AttemptCount,
					CreatedAt: j.CreatedAt, FailedAt: j.ProcessingCompletedAt,
				}, true
			})
			jobs := candidates[:0]
			for _, job := range candidates {
				fileID := job.Title
				job.GroupID, job.Title = fileOf(fileID)
				if inFailedJobScope(scope, job.TenantID, job.GroupID) {
					jobs = append(jobs, job)
				}
			}
			return jobs
		},
		retry: func(scope registry.FailedJobScope, id string, now time.Time) bool {
			return updateFailedIn(base, id, matchByID(scope, id), func(j *models.ThumbnailGenerationJob) {
				j.Status = models.ThumbnailStatusPending
				j.AttemptCount = 0
				j.ErrorMessage = ""
				j.ProcessingStartedAt = nil
				j.ProcessingCompletedAt = nil
				j.UpdatedAt = now
			})
		},
		discard: func(scope registry.FailedJobScope, id string, now time.Time) bool {
			return updateFailedIn(base, id, matchByID(scope, id), func(j *models.ThumbnailGenerationJob) {
				j.DiscardedAt = new(now)
			})
		},
	}
}

func newIntakeFailedJobAdapter(intake *IntakeRegistryFactory, files *FileRegistryFactory) memoryFailedJobAdapter {
	state := intake.state
	match := func(scope registry.FailedJobScope, src *models.IntakeSource) bool {
		return src.Status == models.IntakeSourceFailed && src.DiscardedAt == nil &&
			inFailedJobScope(scope, src.TenantID, src.GroupID)
	}
	change := func(scope registry.FailedJobScope, id string, apply func(*models.IntakeSource)) bool {
		state.mu.Lock()
		defer state.mu.Unlock()
		src, ok := state.sources[id]
		if !ok || !match(scope, src) {
			return false
		}
		apply(src)
		return true
	}
	return memoryFailedJobAdapter{
		list: func(scope registry.FailedJobScope) []*models.FailedJob {
			names := intakeFileTitles(files)
			state.mu.Lock()
			defer state.mu.Unlock()
			var jobs []*models.FailedJob
			for _, src := range state.sources {
				if !match(scope, src) {
					continue
				}
				title := names[src.ID]
				if title == "" {
					title = src.ExternalDocumentID
				}
				jobs = append(jobs, &models.FailedJob{
					Type: models.WorkerTypeIntake, ID: src.ID, TenantID: src.TenantID, GroupID: src.GroupID,
					UserID: src.CreatedByUserID, Title: title, Error: src.ErrorCode, Attempts: src.Attempts,
					CreatedAt: src.CreatedAt, FailedAt: new(src.UpdatedAt),
				})
			}
			return jobs
		},
		retry: func(scope registry.FailedJobScope, id string, now time.Time) bool {
			return change(scope, id, func(src *models.IntakeSource) {
				src.Status = models.IntakeSourcePending
				src.RetryBase = src.Attempts
				src.NextAttemptAt = now
				src.ClaimedAt = nil
				src.WaitReason = ""
				src.ErrorCode = ""
				src.UpdatedAt = now
			})
		},
		discard: func(scope registry.FailedJobScope, id string, now time.Time) bool {
			return change(scope, id, func(src *models.IntakeSource) { src.DiscardedAt = new(now) })
		},
	}
}

// intakeFileTitles returns the title of the first file linked to each
// intake source, by source id.
func intakeFileTitles(files *FileRegistryFactory) map[string]string {
	files.baseFileRegistry.lock.RLock()
	defer files.baseFileRegistry.lock.RUnlock()
	first := make(map[string]models.FileLink)
	out := make(map[string]string)
	for fileID, links := range files.links.byFile {
		for _, l := range links {
			if l.EntityType != models.FileLinkIntake {
				continue
			}
			prev, seen := first[l.EntityID]
			if seen && (prev.CreatedAt.Before(l.CreatedAt) || (prev.CreatedAt.Equal(l.CreatedAt) && prev.FileID < fileID)) {
				continue
			}
			f, ok := files.baseFileRegistry.items.Get(fileID)
			if !ok {
				continue
			}
			first[l.EntityID] = models.FileLink{FileID: fileID, CreatedAt: l.CreatedAt}
			out[l.EntityID] = fileTitle(f)
		}
	}
	return out
}

func (r *FailedJobRegistry) adapter(jobType models.WorkerType) (memoryFailedJobAdapter, error) {
	a, ok := r.adapters[jobType]
	if !ok {
		return memoryFailedJobAdapter{}, errxtrace.Classify(registry.ErrUnknownFailedJobType, errx.Attrs("job_type", string(jobType)))
	}
	return a, nil
}

func checkFailedJobScope(scope registry.FailedJobScope) error {
	if scope.GroupID != "" && scope.TenantID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field", "tenant_id"))
	}
	return nil
}

func (r *FailedJobRegistry) ListFailed(_ context.Context, jobType models.WorkerType, scope registry.FailedJobScope, limit int) ([]*models.FailedJob, error) {
	a, err := r.adapter(jobType)
	if err != nil {
		return nil, err
	}
	if err := checkFailedJobScope(scope); err != nil {
		return nil, err
	}
	jobs := a.list(scope)
	slices.SortFunc(jobs, func(x, y *models.FailedJob) int {
		// Newest failure first, a job without a time last, as postgres
		// orders DESC NULLS LAST; the id breaks ties.
		switch {
		case x.FailedAt == nil && y.FailedAt != nil:
			return 1
		case x.FailedAt != nil && y.FailedAt == nil:
			return -1
		case x.FailedAt != nil && !x.FailedAt.Equal(*y.FailedAt):
			return y.FailedAt.Compare(*x.FailedAt)
		}
		return cmp.Compare(x.ID, y.ID)
	})
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

func (r *FailedJobRegistry) CountFailed(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope) (int, error) {
	jobs, err := r.ListFailed(ctx, jobType, scope, 0)
	if err != nil {
		return 0, err
	}
	return len(jobs), nil
}

func (r *FailedJobRegistry) GetFailed(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string) (*models.FailedJob, error) {
	jobs, err := r.ListFailed(ctx, jobType, scope, 0)
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		if job.ID == id {
			return job, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("job_type", string(jobType), "entity_id", id))
}

func (r *FailedJobRegistry) Retry(_ context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string, now time.Time) error {
	a, err := r.adapter(jobType)
	if err != nil {
		return err
	}
	if a.retry == nil {
		return errxtrace.Classify(registry.ErrFailedJobNotRetryable, errx.Attrs("job_type", string(jobType)))
	}
	if err := checkFailedJobScope(scope); err != nil {
		return err
	}
	if !a.retry(scope, id, now.UTC()) {
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("job_type", string(jobType), "entity_id", id))
	}
	return nil
}

func (r *FailedJobRegistry) Discard(_ context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string, now time.Time) error {
	a, err := r.adapter(jobType)
	if err != nil {
		return err
	}
	if err := checkFailedJobScope(scope); err != nil {
		return err
	}
	if !a.discard(scope, id, now.UTC()) {
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("job_type", string(jobType), "entity_id", id))
	}
	return nil
}

// inFailedJobScope reports whether a row of tenantID and groupID is in the
// scope.
func inFailedJobScope(scope registry.FailedJobScope, tenantID, groupID string) bool {
	return (scope.TenantID == "" || scope.TenantID == tenantID) && (scope.GroupID == "" || scope.GroupID == groupID)
}

// listFailedIn returns the jobs pick makes of the rows it accepts.
func listFailedIn[T any, P registry.PIDable[T]](reg *Registry[T, P], pick func(P) (*models.FailedJob, bool)) []*models.FailedJob {
	reg.lock.RLock()
	defer reg.lock.RUnlock()
	var jobs []*models.FailedJob
	for pair := reg.items.Oldest(); pair != nil; pair = pair.Next() {
		if job, ok := pick(pair.Value); ok {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// updateFailedIn applies apply to the row with the id when match accepts it,
// and reports whether it did.
func updateFailedIn[T any, P registry.PIDable[T]](reg *Registry[T, P], id string, match func(P) bool, apply func(P)) bool {
	reg.lock.Lock()
	defer reg.lock.Unlock()
	item, ok := reg.items.Get(id)
	if !ok || !match(item) {
		return false
	}
	apply(item)
	return true
}

// fileTitle is the name a file shows: its title, else its path.
func fileTitle(f *models.FileEntity) string {
	if f.Title != "" {
		return f.Title
	}
	if f.File != nil {
		return f.Path
	}
	return ""
}

func timestampValue(ts models.PTimestamp) time.Time {
	return ts.ToTime()
}

func timestampPtr(ts models.PTimestamp) *time.Time {
	if ts == nil || *ts == "" {
		return nil
	}
	return new(ts.ToTime())
}
