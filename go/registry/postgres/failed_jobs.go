package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

var _ registry.FailedJobRegistry = (*FailedJobRegistry)(nil)

// failedJobAdapter describes how one job type keeps its failed jobs in its
// own table (#1307). The queries alias the table as j; join adds the tables
// the columns need.
type failedJobAdapter struct {
	table func(store.TableNames) store.TableName
	// join is the FROM clause after "<table> j", empty when none.
	join func(store.TableNames) string
	// where holds the conditions that pick the type's rows of its table,
	// such as exports against imports. Nil when none.
	where func(store.TableNames) string
	// columns selects the failedJobRow fields.
	columns func(store.TableNames) string
	// groupColumn is the column that holds the job's group. Empty for a
	// type whose jobs belong to no group, which a group scope does not
	// reach.
	groupColumn string
	// retrySet is the SET clause of a retry, where @now stands for the time
	// of the retry. Empty for a type that cannot be retried.
	retrySet string
	// failedAt orders the list, newest failure first.
	failedAt string
	// asWorker reads a group scope under the background-worker role: the
	// table's row-level security is per user, not per group, so the app role
	// with a group context would see no rows. The group filter in the query
	// does the scoping.
	asWorker bool
}

// failedJobRow is the shape every adapter's columns produce.
type failedJobRow struct {
	ID        string     `db:"id"`
	TenantID  string     `db:"tenant_id"`
	GroupID   string     `db:"group_id"`
	UserID    string     `db:"user_id"`
	Title     string     `db:"title"`
	Error     string     `db:"error"`
	Attempts  int        `db:"attempts"`
	CreatedAt time.Time  `db:"created_at"`
	FailedAt  *time.Time `db:"failed_at"`
}

const failedJobExportColumns = `j.id, j.tenant_id, j.group_id, j.created_by_user_id AS user_id,
	COALESCE(j.description, '') AS title, COALESCE(j.error_message, '') AS error,
	GREATEST(j.attempt_count, 1) AS attempts, j.created_date AS created_at, j.completed_date AS failed_at`

// failedJobExportRetry puts an export or import back in the queue. The
// attempt count is kept: it counts every run.
const failedJobExportRetry = `status = 'pending', error_message = '', completed_date = NULL`

var failedJobAdapters = map[models.WorkerType]failedJobAdapter{
	models.WorkerTypeExport: {
		table:       func(t store.TableNames) store.TableName { return t.Exports() },
		where:       func(store.TableNames) string { return "j.type <> 'imported' AND j.deleted_at IS NULL" },
		columns:     func(store.TableNames) string { return failedJobExportColumns },
		groupColumn: "j.group_id",
		retrySet:    failedJobExportRetry,
		failedAt:    "j.completed_date",
	},
	models.WorkerTypeImport: {
		table:       func(t store.TableNames) store.TableName { return t.Exports() },
		where:       func(store.TableNames) string { return "j.type = 'imported' AND j.deleted_at IS NULL" },
		columns:     func(store.TableNames) string { return failedJobExportColumns },
		groupColumn: "j.group_id",
		retrySet:    failedJobExportRetry,
		failedAt:    "j.completed_date",
	},
	// A restore is not retried: it may have written part of its backup.
	models.WorkerTypeRestore: {
		table: func(t store.TableNames) store.TableName { return t.RestoreOperations() },
		columns: func(store.TableNames) string {
			return `j.id, j.tenant_id, j.group_id, j.created_by_user_id AS user_id,
				COALESCE(j.description, '') AS title, COALESCE(j.error_message, '') AS error,
				1 AS attempts, j.created_date AS created_at, j.completed_date AS failed_at`
		},
		groupColumn: "j.group_id",
		failedAt:    "j.completed_date",
	},
	models.WorkerTypeThumbnail: {
		table: func(t store.TableNames) store.TableName { return t.ThumbnailGenerationJobs() },
		join: func(t store.TableNames) string {
			return fmt.Sprintf("LEFT JOIN %s f ON f.id = j.file_id", t.Files())
		},
		// Only the newest job of a file counts: an older failure of a file
		// whose thumbnail was tried again since is history, not a job to
		// act on. The order is GetJobByFileID's.
		where: func(t store.TableNames) string {
			return fmt.Sprintf(`NOT EXISTS (SELECT 1 FROM %s n WHERE n.file_id = j.file_id
				AND (n.created_at > j.created_at OR (n.created_at = j.created_at AND n.id > j.id)))`,
				t.ThumbnailGenerationJobs())
		},
		columns: func(store.TableNames) string {
			return `j.id, j.tenant_id, COALESCE(f.group_id, '') AS group_id, j.user_id,
				COALESCE(NULLIF(f.title, ''), f.path, '') AS title, COALESCE(j.error_message, '') AS error,
				j.attempt_count AS attempts, j.created_at, j.processing_completed_at AS failed_at`
		},
		groupColumn: "f.group_id",
		// The attempt count gates the worker's own retries, so a manual
		// retry starts it over.
		retrySet: `status = 'pending', attempt_count = 0, error_message = '',
			processing_started_at = NULL, processing_completed_at = NULL, updated_at = @now`,
		failedAt: "j.processing_completed_at",
		asWorker: true,
	},
	// A failed scan is retried the way resuming its session retries it: back
	// in the queue, due now, with a fresh allowance of attempts. Attempts
	// itself never goes down.
	models.WorkerTypeIntake: {
		table: func(t store.TableNames) store.TableName { return t.IntakeSources() },
		columns: func(t store.TableNames) string {
			return fmt.Sprintf(`j.id, j.tenant_id, j.group_id, j.created_by_user_id AS user_id,
				COALESCE((SELECT COALESCE(NULLIF(f.title, ''), f.path) FROM %s l JOIN %s f ON f.id = l.file_id
					WHERE l.entity_type = '%s' AND l.entity_id = j.id ORDER BY l.created_at, f.id LIMIT 1),
					NULLIF(j.external_document_id, ''), '') AS title,
				j.error_code AS error, j.attempts, j.created_at, j.updated_at AS failed_at`,
				t.FileLinks(), t.Files(), models.FileLinkIntake)
		},
		groupColumn: "j.group_id",
		retrySet: `status = 'pending', retry_base = attempts, next_attempt_at = @now, claimed_at = NULL,
			wait_reason = '', error_code = '', updated_at = @now`,
		// A failed source is not changed again until it is retried, so its
		// last change is its failure.
		failedAt: "j.updated_at",
	},
	// A currency migration is not retried: its rate and totals were
	// confirmed in a preview for that moment. A new migration is started
	// instead.
	models.WorkerTypeCurrencyMigration: {
		table: func(t store.TableNames) store.TableName { return t.CurrencyMigrations() },
		columns: func(store.TableNames) string {
			return `j.id, j.tenant_id, j.group_id, j.created_by_user_id AS user_id,
				j.from_currency || ' → ' || j.to_currency AS title, COALESCE(j.error_message, '') AS error,
				1 AS attempts, j.created_at, j.completed_at AS failed_at`
		},
		groupColumn: "j.group_id",
		failedAt:    "j.completed_at",
	},
	// A personal data export belongs to a user, not to a group. Its title is
	// the user's email address, so the operator knows whose copy it is. A
	// retry builds it again from the start, and the new expiry keeps the row
	// until the build is done; the build sets the expiry again when it
	// finishes.
	models.FailedJobTypePersonalDataExport: {
		table: func(t store.TableNames) store.TableName { return t.PersonalDataExports() },
		join: func(t store.TableNames) string {
			return fmt.Sprintf("LEFT JOIN %s u ON u.id = j.user_id", t.Users())
		},
		columns: func(store.TableNames) string {
			return `j.id, j.tenant_id, '' AS group_id, j.user_id, COALESCE(u.email, '') AS title,
				j.error_message AS error, GREATEST(j.attempt_count, 1) AS attempts, j.created_at,
				j.completed_at AS failed_at`
		},
		retrySet: fmt.Sprintf(`status = 'pending', error_message = '', started_at = NULL, completed_at = NULL,
			blob_key = '', size_bytes = 0, expires_at = @now::timestamp + INTERVAL '%d seconds'`,
			int64(models.PersonalDataExportRetention/time.Second)),
		failedAt: "j.completed_at",
	},
}

// FailedJobRegistry is the postgres FailedJobRegistry.
type FailedJobRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewFailedJobRegistry returns the failed-jobs registry over dbx.
func NewFailedJobRegistry(dbx *sqlx.DB) *FailedJobRegistry {
	return &FailedJobRegistry{dbx: dbx, tableNames: store.DefaultTableNames}
}

// adapter returns the type's adapter for the scope. A group scope does not
// know a type whose jobs belong to no group.
func (r *FailedJobRegistry) adapter(jobType models.WorkerType, scope registry.FailedJobScope) (failedJobAdapter, error) {
	a, ok := failedJobAdapters[jobType]
	if !ok || (scope.GroupID != "" && a.groupColumn == "") {
		return failedJobAdapter{}, errxtrace.Classify(registry.ErrUnknownFailedJobType, errx.Attrs("job_type", string(jobType)))
	}
	return a, nil
}

// do runs fn in the transaction the scope calls for: across tenants under
// the admin role, which bypasses row-level security as the other
// system-admin reads do; for a group under the application role with the
// group's context, unless the adapter reads as the background worker.
func (r *FailedJobRegistry) do(ctx context.Context, a failedJobAdapter, scope registry.FailedJobScope, fn func(context.Context, *sqlx.Tx) error) error {
	switch {
	case scope.GroupID != "" && scope.TenantID == "":
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field", "tenant_id"))
	case scope.GroupID != "" && a.asWorker:
		return store.DoAsBackgroundWorker(ctx, r.dbx, fn)
	case scope.GroupID != "":
		return store.DoAsGroup(ctx, r.dbx, scope.TenantID, scope.GroupID, fn)
	default:
		return store.DoAsAdmin(ctx, r.dbx, fn)
	}
}

// conditions returns the WHERE clause that picks the type's failed,
// undiscarded jobs in the scope, with its arguments numbered from next.
func (r *FailedJobRegistry) conditions(a failedJobAdapter, scope registry.FailedJobScope, next int) (string, []any) {
	conds := []string{"j.status = 'failed'", "j.discarded_at IS NULL"}
	if a.where != nil {
		conds = append(conds, a.where(r.tableNames))
	}
	var args []any
	if scope.TenantID != "" {
		conds = append(conds, fmt.Sprintf("j.tenant_id = $%d", next+len(args)))
		args = append(args, scope.TenantID)
	}
	if scope.GroupID != "" {
		conds = append(conds, fmt.Sprintf("%s = $%d", a.groupColumn, next+len(args)))
		args = append(args, scope.GroupID)
	}
	return strings.Join(conds, " AND "), args
}

func (r *FailedJobRegistry) from(a failedJobAdapter) string {
	from := fmt.Sprintf("%s j", a.table(r.tableNames))
	if a.join != nil {
		from += " " + a.join(r.tableNames)
	}
	return from
}

func (r *FailedJobRegistry) ListFailed(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope, limit int) ([]*models.FailedJob, error) {
	a, err := r.adapter(jobType, scope)
	if err != nil {
		return nil, err
	}
	where, args := r.conditions(a, scope, 1)
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s DESC NULLS LAST, j.id",
		a.columns(r.tableNames), r.from(a), where, a.failedAt)
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	var rows []failedJobRow
	err = r.do(ctx, a, scope, func(ctx context.Context, tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, query, args...)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list failed jobs", err, errx.Attrs("job_type", string(jobType)))
	}
	jobs := make([]*models.FailedJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, row.job(jobType))
	}
	return jobs, nil
}

func (r *FailedJobRegistry) CountFailed(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope) (int, error) {
	a, err := r.adapter(jobType, scope)
	if err != nil {
		return 0, err
	}
	where, args := r.conditions(a, scope, 1)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", r.from(a), where)
	var n int
	err = r.do(ctx, a, scope, func(ctx context.Context, tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &n, query, args...)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count failed jobs", err, errx.Attrs("job_type", string(jobType)))
	}
	return n, nil
}

func (r *FailedJobRegistry) GetFailed(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string) (*models.FailedJob, error) {
	a, err := r.adapter(jobType, scope)
	if err != nil {
		return nil, err
	}
	where, args := r.conditions(a, scope, 2)
	query := fmt.Sprintf("SELECT %s FROM %s WHERE j.id = $1 AND %s", a.columns(r.tableNames), r.from(a), where)
	var row failedJobRow
	err = r.do(ctx, a, scope, func(ctx context.Context, tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, query, append([]any{id}, args...)...)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("job_type", string(jobType), "entity_id", id))
	}
	if err != nil {
		return nil, errxtrace.Wrap("failed to get failed job", err, errx.Attrs("job_type", string(jobType)))
	}
	return row.job(jobType), nil
}

func (r *FailedJobRegistry) Retry(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string, now time.Time) error {
	a, err := r.adapter(jobType, scope)
	if err != nil {
		return err
	}
	if a.retrySet == "" {
		return errxtrace.Classify(registry.ErrFailedJobNotRetryable, errx.Attrs("job_type", string(jobType)))
	}
	return r.update(ctx, a, jobType, scope, id, a.retrySet, now)
}

func (r *FailedJobRegistry) Discard(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string, now time.Time) error {
	a, err := r.adapter(jobType, scope)
	if err != nil {
		return err
	}
	return r.update(ctx, a, jobType, scope, id, "discarded_at = @now", now)
}

// update applies set to the job when it is a failed, undiscarded job of the
// type in the scope. The row is picked by the same conditions the list uses,
// so a job another call retried or discarded first is not changed twice.
func (r *FailedJobRegistry) update(ctx context.Context, a failedJobAdapter, jobType models.WorkerType, scope registry.FailedJobScope, id, set string, now time.Time) error {
	where, scopeArgs := r.conditions(a, scope, 2)
	args := append([]any{id}, scopeArgs...)
	if strings.Contains(set, "@now") {
		args = append(args, now.UTC())
		set = strings.ReplaceAll(set, "@now", fmt.Sprintf("$%d", len(args)))
	}
	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = (SELECT j.id FROM %s WHERE j.id = $1 AND %s FOR UPDATE OF j)",
		a.table(r.tableNames), set, r.from(a), where)
	var changed int64
	err := r.do(ctx, a, scope, func(ctx context.Context, tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		changed, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return errxtrace.Wrap("failed to update failed job", err, errx.Attrs("job_type", string(jobType)))
	}
	if changed == 0 {
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("job_type", string(jobType), "entity_id", id))
	}
	return nil
}

func (row failedJobRow) job(jobType models.WorkerType) *models.FailedJob {
	job := &models.FailedJob{
		Type:      jobType,
		ID:        row.ID,
		TenantID:  row.TenantID,
		GroupID:   row.GroupID,
		UserID:    row.UserID,
		Title:     row.Title,
		Error:     row.Error,
		Attempts:  row.Attempts,
		CreatedAt: row.CreatedAt.UTC(),
	}
	if row.FailedAt != nil {
		job.FailedAt = new(row.FailedAt.UTC())
	}
	return job
}
