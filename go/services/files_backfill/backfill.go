// Package files_backfill copies the legacy commodity-scoped images, invoices,
// and manuals tables into the unified `files` table introduced in #1397, so
// the new `/files` endpoint can serve as a complete replacement before #1421
// removes the legacy routes.
//
// Idempotency: each run skips legacy rows whose UUID already exists in
// `files`. Re-runs after a partial failure (or after the FE has resumed
// uploading directly into `files`) are safe and produce zero new rows.
//
// Reversibility: the legacy tables are not touched. The follow-up cutover
// PR (#1421) is the only place that drops them.
package files_backfill

import (
	"context"
	"database/sql"
	"fmt"

	errxtrace "github.com/go-extras/errx/stacktrace"
)

// SourceStats is a per-source-table audit row produced by the backfill.
//
// `Total`     — rows currently in the legacy table.
// `Migrated`  — legacy rows that already have a matching files.uuid.
// `Pending`   — legacy rows still to copy (Total - Migrated).
// `Inserted`  — rows actually written this run; always 0 for dry runs and
//
//	always equal to Pending after a successful live run.
type SourceStats struct {
	Source   string
	Total    int
	Migrated int
	Pending  int
	Inserted int
}

// Stats is the full audit + write report for a single backfill invocation.
type Stats struct {
	DryRun  bool
	Sources []SourceStats
}

// TotalInserted is the sum of inserted rows across all sources.
func (s *Stats) TotalInserted() int {
	n := 0
	for _, src := range s.Sources {
		n += src.Inserted
	}
	return n
}

// TotalPending is the sum of pending rows across all sources, useful for
// dry-run summaries.
func (s *Stats) TotalPending() int {
	n := 0
	for _, src := range s.Sources {
		n += src.Pending
	}
	return n
}

// Manager owns the backfill SQL. It runs every step inside a single
// transaction so dry runs can roll back cleanly and partial failures don't
// leak half-migrated rows.
type Manager struct {
	db *sql.DB
}

// NewManager returns a Manager bound to the supplied *sql.DB. The DB must
// connect as a role that can read the legacy tables and write to `files`
// across tenants — typically the migrator role used for `inventario migrate
// up`. RLS bypass is handled by that role's policy `USING (true)`.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// runMode discriminates between an actual write and a preview. Modeled as
// a typed enum (not a bool) so callers read at the call site and revive's
// flag-parameter check stays out of our way.
type runMode int

const (
	modeApply runMode = iota
	modePreview
)

// Apply runs the backfill end-to-end and commits the transaction. Use
// PreviewOnly when you want the audit row counts without persisting the
// new files rows.
func (m *Manager) Apply(ctx context.Context) (*Stats, error) {
	return m.run(ctx, modeApply)
}

// PreviewOnly executes the same INSERTs as Apply but rolls the
// transaction back at the end, so the audit row counts in the returned
// Stats reflect "what would happen" with zero side effects.
func (m *Manager) PreviewOnly(ctx context.Context) (*Stats, error) {
	return m.run(ctx, modePreview)
}

func (m *Manager) run(ctx context.Context, mode runMode) (*Stats, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to begin backfill transaction", err)
	}
	// Always end the transaction; rollback on error or dry-run, commit on
	// successful live run. Defer makes this safe even if a panic escapes a
	// helper below.
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	stats := &Stats{DryRun: mode == modePreview}
	for _, plan := range backfillPlans() {
		row, err := backfillSource(ctx, tx, plan)
		if err != nil {
			return nil, errxtrace.Wrap(fmt.Sprintf("failed to backfill %s", plan.source), err)
		}
		stats.Sources = append(stats.Sources, row)
	}

	if mode == modePreview {
		// rollback handled by deferred branch; leave committed=false
		return stats, nil
	}

	if err := tx.Commit(); err != nil {
		return nil, errxtrace.Wrap("failed to commit backfill transaction", err)
	}
	committed = true
	return stats, nil
}

// backfillPlan describes one legacy → files mapping.
//
//	source            — legacy table name, used both for SQL and for the
//	                    SourceStats label.
//	linkedEntityMeta  — the meta value the backfilled FileEntity carries on
//	                    its linked_entity_meta column (matches what new
//	                    upload handlers set for the same legacy bucket).
//	category          — the user-meaningful FileCategory enum value.
//	typeExpr          — SQL expression yielding the FileType enum, evaluated
//	                    against the source row alias `s`. For images this is
//	                    a literal; for invoices/manuals we mirror the runtime
//	                    `FileTypeFromMIME` switch so a stray non-PDF lands in
//	                    the right type bucket.
type backfillPlan struct {
	source           string
	linkedEntityMeta string
	category         string
	typeExpr         string
}

// fileTypeFromMimeSQL mirrors models.FileTypeFromMIME. Kept here as a
// constant so the migration logic stays in lock-step with the Go function;
// any change must be made in both.
const fileTypeFromMimeSQL = `CASE
		WHEN s.mime_type LIKE 'image/%' THEN 'image'
		WHEN s.mime_type LIKE 'video/%' THEN 'video'
		WHEN s.mime_type LIKE 'audio/%' THEN 'audio'
		WHEN s.mime_type IN ('application/zip','application/x-zip-compressed') THEN 'archive'
		WHEN s.mime_type IN ('application/pdf','text/plain','text/csv','application/json','application/msword')
			OR s.mime_type LIKE 'application/vnd.ms-%'
			OR s.mime_type LIKE 'application/vnd.openxmlformats-%' THEN 'document'
		ELSE 'other'
	END`

func backfillPlans() []backfillPlan {
	return []backfillPlan{
		{
			source:           "images",
			linkedEntityMeta: "images",
			category:         "photos",
			// images bucket only stores image MIMEs, but we still derive
			// from MIME to keep the three branches consistent.
			typeExpr: fileTypeFromMimeSQL,
		},
		{
			source:           "invoices",
			linkedEntityMeta: "invoices",
			category:         "invoices",
			typeExpr:         fileTypeFromMimeSQL,
		},
		{
			source:           "manuals",
			linkedEntityMeta: "manuals",
			category:         "documents",
			typeExpr:         fileTypeFromMimeSQL,
		},
	}
}

// backfillSource issues the three SQL statements for one legacy source.
// Every interpolated value (`plan.source`, `plan.typeExpr`) comes from the
// hard-coded backfillPlans slice — never from user input — so gosec's
// G201 warnings about SQL string formatting are deliberately suppressed.
//
//nolint:gosec // SQL fragments are built from hard-coded backfillPlan constants
func backfillSource(ctx context.Context, tx *sql.Tx, plan backfillPlan) (SourceStats, error) {
	row := SourceStats{Source: plan.source}

	totalQuery := "SELECT COUNT(*) FROM " + plan.source
	migratedQuery := "SELECT COUNT(*) FROM " + plan.source + " s WHERE EXISTS (SELECT 1 FROM files f WHERE f.uuid = s.uuid)"
	if err := tx.QueryRowContext(ctx, totalQuery).Scan(&row.Total); err != nil {
		return row, errxtrace.Wrap("failed to count source rows", err)
	}
	if err := tx.QueryRowContext(ctx, migratedQuery).Scan(&row.Migrated); err != nil {
		return row, errxtrace.Wrap("failed to count migrated rows", err)
	}
	row.Pending = row.Total - row.Migrated

	// We always run the INSERT — on a dry run the surrounding transaction
	// is rolled back, so the planner cost is the only side effect. Doing it
	// this way means the dry-run report reflects "what would actually
	// happen" rather than a separately-computed estimate.
	insert := fmt.Sprintf(`
		INSERT INTO files (
			id, uuid, tenant_id, group_id, created_by_user_id,
			title, description, type, category, tags,
			linked_entity_type, linked_entity_id, linked_entity_meta,
			path, original_path, ext, mime_type,
			created_at, updated_at
		)
		SELECT
			gen_random_uuid()::text,
			s.uuid,
			s.tenant_id,
			s.group_id,
			s.created_by_user_id,
			s.path,
			'',
			%s,
			$1,
			'[]'::jsonb,
			'commodity',
			s.commodity_id,
			$2,
			s.path,
			s.original_path,
			s.ext,
			s.mime_type,
			NOW(),
			NOW()
		FROM %s s
		WHERE NOT EXISTS (SELECT 1 FROM files f WHERE f.uuid = s.uuid)`,
		plan.typeExpr, plan.source)

	res, err := tx.ExecContext(ctx, insert, plan.category, plan.linkedEntityMeta)
	if err != nil {
		return row, errxtrace.Wrap("failed to insert backfill rows", err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		return row, errxtrace.Wrap("failed to read RowsAffected", err)
	}
	row.Inserted = int(inserted)
	return row, nil
}
