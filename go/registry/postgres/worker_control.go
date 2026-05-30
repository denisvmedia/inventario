package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.WorkerControlRegistry = (*WorkerControlRegistry)(nil)

// WorkerControlRegistry is the postgres-backed background-worker
// soft-pause control store (#1308). The table is NOT RLS-enabled —
// worker pause state is a platform-operator control orthogonal to
// tenants (same posture as system_admin_grants / audit_logs). All
// operations run against r.dbx directly: Pause is a single ON CONFLICT
// upsert and Resume is a single UPDATE, so neither needs a multi-
// statement transaction. The unique index on worker_type guarantees at
// most one control row per worker and backs the upsert's ON CONFLICT
// target.
type WorkerControlRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewWorkerControlRegistry creates a new WorkerControlRegistry.
func NewWorkerControlRegistry(dbx *sqlx.DB) *WorkerControlRegistry {
	return NewWorkerControlRegistryWithTableNames(dbx, store.DefaultTableNames)
}

// NewWorkerControlRegistryWithTableNames is the test-friendly constructor
// that lets a caller override the table-name mapping.
func NewWorkerControlRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *WorkerControlRegistry {
	return &WorkerControlRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// List returns every worker_control row ordered by worker_type. An
// absent worker type means that worker is running — the caller treats
// "no row" as not-paused.
func (r *WorkerControlRegistry) List(ctx context.Context) ([]*models.WorkerControl, error) {
	query := fmt.Sprintf(
		`SELECT * FROM %s ORDER BY worker_type`,
		r.tableNames.WorkerControl(),
	)
	rows, err := r.dbx.QueryxContext(ctx, query)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list worker_control", err)
	}
	defer rows.Close()

	var controls []*models.WorkerControl
	for rows.Next() {
		var wc models.WorkerControl
		if scanErr := rows.StructScan(&wc); scanErr != nil {
			return nil, errxtrace.Wrap("failed to scan worker_control row", scanErr)
		}
		controls = append(controls, &wc)
	}
	if err := rows.Err(); err != nil {
		return nil, errxtrace.Wrap("failed during worker_control iteration", err)
	}
	return controls, nil
}

// Pause idempotently marks workerType paused. pausedBy and reason are
// stored as NULL when empty. Re-pausing an already-paused type updates
// paused_by/reason but PRESERVES the original paused_at via the CASE
// expression below — the pause timestamp must reflect when the worker
// first stopped, not the most recent operator note edit. The unique
// index on worker_type backs the ON CONFLICT upsert so concurrent pause
// calls collapse onto a single row.
func (r *WorkerControlRegistry) Pause(ctx context.Context, workerType, pausedBy, reason string) (*models.WorkerControl, error) {
	if workerType == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired)
	}

	// Empty strings map to NULL columns so a paused-without-reason row
	// reads as reason IS NULL rather than an empty-string sentinel.
	var pausedByArg, reasonArg any
	if pausedBy != "" {
		pausedByArg = pausedBy
	}
	if reason != "" {
		reasonArg = reason
	}

	query := fmt.Sprintf(
		`INSERT INTO %s (id, uuid, worker_type, paused, paused_by, paused_at, reason, updated_at)
		 VALUES ($1, $2, $3, true, $4, now(), $5, now())
		 ON CONFLICT (worker_type) DO UPDATE SET
		   paused = true,
		   paused_by = excluded.paused_by,
		   reason = excluded.reason,
		   paused_at = CASE WHEN %s.paused THEN %s.paused_at ELSE now() END,
		   updated_at = now()
		 RETURNING *`,
		r.tableNames.WorkerControl(),
		r.tableNames.WorkerControl(),
		r.tableNames.WorkerControl(),
	)

	var wc models.WorkerControl
	if err := r.dbx.QueryRowxContext(ctx, query,
		uuid.New().String(), uuid.New().String(), workerType, pausedByArg, reasonArg,
	).StructScan(&wc); err != nil {
		return nil, errxtrace.Wrap("failed to pause worker", err)
	}
	return &wc, nil
}

// Resume idempotently marks workerType not paused, clearing
// paused_at/paused_by/reason. When no row exists it is a no-op and
// returns a synthetic not-paused WorkerControl (no INSERT) — the worker
// was already running, so there is nothing to persist.
func (r *WorkerControlRegistry) Resume(ctx context.Context, workerType string) (*models.WorkerControl, error) {
	if workerType == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired)
	}

	query := fmt.Sprintf(
		`UPDATE %s SET
		   paused = false,
		   paused_by = NULL,
		   paused_at = NULL,
		   reason = NULL,
		   updated_at = now()
		 WHERE worker_type = $1
		 RETURNING *`,
		r.tableNames.WorkerControl(),
	)

	var wc models.WorkerControl
	switch err := r.dbx.QueryRowxContext(ctx, query, workerType).StructScan(&wc); {
	case err == nil:
		return &wc, nil
	case errors.Is(err, sql.ErrNoRows):
		// No control row — the worker is already running. Return a
		// synthetic not-paused state without inserting a row.
		return &models.WorkerControl{WorkerType: models.WorkerType(workerType)}, nil
	default:
		return nil, errxtrace.Wrap("failed to resume worker", err)
	}
}
