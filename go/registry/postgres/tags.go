package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// TagRegistryFactory creates TagRegistry instances with proper context.
type TagRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// TagRegistry is the postgres-backed group-scoped registry of tags.
type TagRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.TagRegistry        = (*TagRegistry)(nil)
	_ registry.TagRegistryFactory = (*TagRegistryFactory)(nil)
)

func NewTagRegistry(dbx *sqlx.DB) *TagRegistryFactory {
	return NewTagRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewTagRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *TagRegistryFactory {
	return &TagRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *TagRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.TagRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *TagRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.TagRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &TagRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
		service:         false,
	}, nil
}

func (f *TagRegistryFactory) CreateServiceRegistry() registry.TagRegistry {
	return &TagRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *TagRegistry) newSQLRegistry() *store.RLSGroupRepository[models.Tag, *models.Tag] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.Tag](r.dbx, r.tableNames.Tags())
	}
	return store.NewGroupAwareSQLRegistry[models.Tag](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.Tags())
}

func (r *TagRegistry) Get(ctx context.Context, id string) (*models.Tag, error) {
	var tag models.Tag
	err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &tag)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get tag", err)
	}
	return &tag, nil
}

func (r *TagRegistry) GetBySlug(ctx context.Context, slug string) (*models.Tag, error) {
	var tag models.Tag
	err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("slug", slug), &tag)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get tag by slug", err)
	}
	return &tag, nil
}

func (r *TagRegistry) List(ctx context.Context) ([]*models.Tag, error) {
	var tags []*models.Tag
	for tag, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list tags", err)
		}
		t := tag
		tags = append(tags, &t)
	}
	return tags, nil
}

func (r *TagRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count tags", err)
	}
	return cnt, nil
}

func (r *TagRegistry) Create(ctx context.Context, tag models.Tag) (*models.Tag, error) {
	created, err := r.newSQLRegistry().Create(ctx, tag, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create tag", err)
	}
	return &created, nil
}

func (r *TagRegistry) Update(ctx context.Context, tag models.Tag) (*models.Tag, error) {
	if err := r.newSQLRegistry().Update(ctx, tag, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update tag", err)
	}
	return &tag, nil
}

func (r *TagRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

// usageExpr is the SQL expression that yields the per-tag total reference
// count summed across commodities + files. The two `tags @>` operands use
// the @> JSONB containment operator backed by the existing GIN indexes
// (commodities_tags_gin_idx, files_tags_gin_idx).
func (r *TagRegistry) usageExpr() string {
	return fmt.Sprintf(
		`((SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array(t.slug))
		+ (SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array(t.slug)))`,
		r.tableNames.Commodities(),
		r.tableNames.Files(),
	)
}

func (r *TagRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.TagListOptions) ([]*models.Tag, int, error) {
	var tags []*models.Tag
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var conditions []string
		var args []any
		if opts.Search != "" {
			conditions = append(conditions, "(t.label ILIKE $1 OR t.slug ILIKE $2)")
			pattern := "%" + opts.Search + "%"
			args = append(args, pattern, pattern)
		}
		whereClause := ""
		if len(conditions) > 0 {
			whereClause = "WHERE " + strings.Join(conditions, " AND ")
		}

		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s t %s", r.tableNames.Tags(), whereClause)
		if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			return errxtrace.Wrap("failed to count tags", err)
		}

		sortField := opts.SortField
		if !sortField.IsValid() {
			sortField = registry.TagSortLabel
		}
		dir := "ASC"
		if opts.SortDesc {
			dir = "DESC"
		}
		var orderBy string
		switch sortField {
		case registry.TagSortCreatedAt:
			orderBy = fmt.Sprintf("ORDER BY t.created_at %s, t.label ASC", dir)
		case registry.TagSortUsage:
			orderBy = fmt.Sprintf("ORDER BY %s %s, t.label ASC", r.usageExpr(), dir)
		default:
			orderBy = fmt.Sprintf("ORDER BY LOWER(t.label) %s, t.id ASC", dir)
		}

		dataArgs := append([]any{}, args...)
		dataArgs = append(dataArgs, limit, offset)
		dataQuery := fmt.Sprintf(
			`SELECT t.* FROM %s t %s %s LIMIT $%d OFFSET $%d`,
			r.tableNames.Tags(), whereClause, orderBy, len(args)+1, len(args)+2,
		)
		rows, err := tx.QueryxContext(ctx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list tags", err)
		}
		defer rows.Close()
		for rows.Next() {
			var tag models.Tag
			if err := rows.StructScan(&tag); err != nil {
				return errxtrace.Wrap("failed to scan tag", err)
			}
			t := tag
			tags = append(tags, &t)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list paginated tags", err)
	}
	return tags, total, nil
}

func (r *TagRegistry) Search(ctx context.Context, q string, limit int) ([]*models.Tag, error) {
	var tags []*models.Tag

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var conditions []string
		var args []any
		argIdx := 1
		if q != "" {
			conditions = append(conditions, fmt.Sprintf("(t.label ILIKE $%d OR t.slug ILIKE $%d)", argIdx, argIdx+1))
			pattern := "%" + q + "%"
			args = append(args, pattern, pattern)
			argIdx += 2
		}
		whereClause := ""
		if len(conditions) > 0 {
			whereClause = "WHERE " + strings.Join(conditions, " AND ")
		}

		// Rank by usage desc, then created_at desc (recency tiebreaker).
		args = append(args, limit)
		query := fmt.Sprintf(
			`SELECT t.* FROM %s t %s ORDER BY %s DESC, t.created_at DESC LIMIT $%d`,
			r.tableNames.Tags(), whereClause, r.usageExpr(), argIdx,
		)

		rows, err := tx.QueryxContext(ctx, query, args...)
		if err != nil {
			return errxtrace.Wrap("failed to search tags", err)
		}
		defer rows.Close()
		for rows.Next() {
			var tag models.Tag
			if err := rows.StructScan(&tag); err != nil {
				return errxtrace.Wrap("failed to scan tag", err)
			}
			t := tag
			tags = append(tags, &t)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to search tags", err)
	}
	return tags, nil
}

func (r *TagRegistry) GetUsageBatch(ctx context.Context, slugs []string) (map[string]registry.TagUsage, error) {
	out := make(map[string]registry.TagUsage, len(slugs))
	for _, s := range slugs {
		out[s] = registry.TagUsage{}
	}
	if len(slugs) == 0 {
		return out, nil
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Single-pass query: scan commodities + files at most once each
		// (filtered by `tags ?| input` so the GIN index on the JSONB tags
		// column drives the lookup), unnest the JSONB arrays once per row,
		// COUNT(DISTINCT id) per slug to match the @> containment semantics
		// (a row with duplicate slugs in its tags array still counts as 1),
		// then LEFT JOIN back to the requested input list so missing slugs
		// land as 0/0 instead of being dropped.
		//
		// Compared to the per-slug correlated-subquery approach, this is
		// O(rows scanned once per table) vs O(slugs × rows) — important
		// for the upcoming Tags page which fetches usage for the whole
		// page (default per_page=50).
		query := fmt.Sprintf(`
			WITH input(slug) AS (SELECT unnest($1::text[])),
			commodity_counts AS (
				SELECT t.value AS slug, COUNT(DISTINCT id)::int AS cnt
				FROM %s, jsonb_array_elements_text(tags) AS t(value)
				WHERE tags ?| $1::text[]
				GROUP BY t.value
			),
			file_counts AS (
				SELECT t.value AS slug, COUNT(DISTINCT id)::int AS cnt
				FROM %s, jsonb_array_elements_text(tags) AS t(value)
				WHERE tags ?| $1::text[]
				GROUP BY t.value
			)
			SELECT input.slug,
				COALESCE(c.cnt, 0) AS commodities,
				COALESCE(f.cnt, 0) AS files
			FROM input
			LEFT JOIN commodity_counts c ON c.slug = input.slug
			LEFT JOIN file_counts f ON f.slug = input.slug
		`, r.tableNames.Commodities(), r.tableNames.Files())

		rows, err := tx.QueryxContext(ctx, query, slugs)
		if err != nil {
			return errxtrace.Wrap("failed to batch-compute tag usage", err)
		}
		defer rows.Close()
		for rows.Next() {
			var slug string
			var u registry.TagUsage
			if err := rows.Scan(&slug, &u.Commodities, &u.Files); err != nil {
				return errxtrace.Wrap("failed to scan tag usage", err)
			}
			out[slug] = u
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to compute tag usage batch", err)
	}
	return out, nil
}

func (r *TagRegistry) GetStats(ctx context.Context) (registry.TagStats, error) {
	var stats registry.TagStats

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// One round-trip: each subquery is a sealed COUNT under the current
		// RLS scope. `jsonb_array_length(COALESCE(tags, '[]'::jsonb)) > 0`
		// treats NULL the same as []: untagged.
		query := fmt.Sprintf(`
			SELECT
				(SELECT COUNT(*) FROM %s) AS tags_total,
				(SELECT COUNT(*) FROM %s WHERE jsonb_array_length(COALESCE(tags, '[]'::jsonb)) > 0) AS items_tagged,
				(SELECT COUNT(*) FROM %s WHERE jsonb_array_length(COALESCE(tags, '[]'::jsonb)) = 0) AS items_untagged,
				(SELECT COUNT(*) FROM %s WHERE jsonb_array_length(COALESCE(tags, '[]'::jsonb)) > 0) AS files_tagged,
				(SELECT COUNT(*) FROM %s WHERE jsonb_array_length(COALESCE(tags, '[]'::jsonb)) = 0) AS files_untagged
		`,
			r.tableNames.Tags(),
			r.tableNames.Commodities(), r.tableNames.Commodities(),
			r.tableNames.Files(), r.tableNames.Files(),
		)
		return tx.QueryRowxContext(ctx, query).Scan(
			&stats.TagsTotal,
			&stats.ItemsTagged, &stats.ItemsUntagged,
			&stats.FilesTagged, &stats.FilesUntagged,
		)
	})
	if err != nil {
		return registry.TagStats{}, errxtrace.Wrap("failed to compute tag stats", err)
	}
	return stats, nil
}

func (r *TagRegistry) GetUsage(ctx context.Context, slug string) (registry.TagUsage, error) {
	var usage registry.TagUsage

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT
				(SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array($1::text)),
				(SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array($1::text))`,
			r.tableNames.Commodities(), r.tableNames.Files(),
		)
		return tx.QueryRowxContext(ctx, query, slug).Scan(&usage.Commodities, &usage.Files)
	})
	if err != nil {
		return registry.TagUsage{}, errxtrace.Wrap("failed to compute tag usage", err)
	}
	return usage, nil
}

// jsonbReplaceSlugExpr emits the SQL expression that returns the rewritten
// JSONB array for a single row, deduplicating after the substitution so a
// rename onto an already-present slug doesn't produce a duplicate.
func jsonbReplaceSlugExpr(oldSlugParam, newSlugParam int) string {
	return fmt.Sprintf(
		`COALESCE(
			(SELECT jsonb_agg(DISTINCT (CASE WHEN value = $%d THEN $%d ELSE value END))
			 FROM jsonb_array_elements_text(tags) value),
			'[]'::jsonb
		)`,
		oldSlugParam, newSlugParam,
	)
}

// jsonbStripSlugExpr emits the SQL expression that returns the JSONB array
// with every occurrence of the given slug removed.
func jsonbStripSlugExpr(slugParam int) string {
	return fmt.Sprintf(
		`COALESCE(
			(SELECT jsonb_agg(value) FROM jsonb_array_elements_text(tags) value WHERE value <> $%d),
			'[]'::jsonb
		)`,
		slugParam,
	)
}

func (r *TagRegistry) RewriteSlugReferences(ctx context.Context, oldSlug, newSlug string) (commodityRows, fileRows int, err error) {
	if oldSlug == newSlug {
		return 0, 0, nil
	}

	var commodityCount, fileCount int
	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		commQuery := fmt.Sprintf(
			`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
			r.tableNames.Commodities(), jsonbReplaceSlugExpr(1, 2),
		)
		commRes, execErr := tx.ExecContext(ctx, commQuery, oldSlug, newSlug)
		if execErr != nil {
			return errxtrace.Wrap("failed to rewrite commodity tags", execErr)
		}
		commAffected, raErr := commRes.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read commodity rows affected", raErr)
		}
		commodityCount = int(commAffected)

		fileQuery := fmt.Sprintf(
			`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
			r.tableNames.Files(), jsonbReplaceSlugExpr(1, 2),
		)
		fileRes, execErr := tx.ExecContext(ctx, fileQuery, oldSlug, newSlug)
		if execErr != nil {
			return errxtrace.Wrap("failed to rewrite file tags", execErr)
		}
		fileAffected, raErr := fileRes.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read file rows affected", raErr)
		}
		fileCount = int(fileAffected)
		return nil
	})
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to rewrite slug references", err)
	}
	return commodityCount, fileCount, nil
}

// acquireTagAdvisoryLock takes a transaction-scoped postgres advisory lock
// keyed on (group, key). The two-int4 form lets us pack a per-group +
// per-tag-attribute pair into one lock space without a hash collision
// across groups. xact-scoped means we don't have to release explicitly —
// COMMIT/ROLLBACK does it.
func acquireTagAdvisoryLock(ctx context.Context, tx *sqlx.Tx, groupID, key string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, groupID, key)
	if err != nil {
		return errxtrace.Wrap("failed to acquire tag advisory lock", err, errx.Attrs("group_id", groupID, "key", key))
	}
	return nil
}

// defaultTagLabelFromSlug mirrors services.defaultLabelFromSlug — split a
// kebab-case slug on '-' and Title-case each word. Duplicated here so the
// registry's own auto-create path doesn't import services (which would be
// a cycle: services imports registry).
func defaultTagLabelFromSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

// ensureTagRowsInTx is the cross-tx safety net for the orphan-reference
// race surfaced by #1488: a commodity / file insert that references a
// slug must not commit if a concurrent DeleteTag already removed (or is
// removing) the tags row for that slug.
//
// For each (group, slug) we (a) take a per-(group, slug) xact advisory
// lock — which DeleteAtomic also takes — so we serialize against an
// in-flight delete on the same slug, and (b) run INSERT ... ON CONFLICT
// DO NOTHING. If the delete already committed, our INSERT creates a
// fresh tag row; if it hadn't yet, the deleter blocks until our tx
// commits, then re-checks usage (which now includes our new row) and
// either refuses (force=false) or re-strips (force=true).
//
// Slugs are deduplicated and sorted before locking so the lock
// acquisition order is deterministic across writers — eliminates
// deadlock potential when two writers reference overlapping slug sets.
func ensureTagRowsInTx(ctx context.Context, tx *sqlx.Tx, tableNames store.TableNames, tenantID, groupID, userID string, slugs []string) error {
	if len(slugs) == 0 || groupID == "" {
		return nil
	}
	cleaned := make([]string, 0, len(slugs))
	seen := make(map[string]struct{}, len(slugs))
	for _, s := range slugs {
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		cleaned = append(cleaned, s)
	}
	if len(cleaned) == 0 {
		return nil
	}
	sort.Strings(cleaned)

	selectForUpdate := fmt.Sprintf(
		`SELECT 1 FROM %s WHERE group_id = $1 AND slug = $2 FOR UPDATE`, tableNames.Tags())
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s (id, tenant_id, group_id, slug, label, color, created_by_user_id)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		ON CONFLICT (group_id, slug) DO NOTHING`, tableNames.Tags())

	for _, slug := range cleaned {
		if err := acquireTagAdvisoryLock(ctx, tx, groupID, "slug:"+slug); err != nil {
			return err
		}
		// SELECT FOR UPDATE — if the row exists, take a row-level lock so
		// a concurrent DeleteAtomic (which also does FOR UPDATE on the
		// same row) blocks until our tx commits. ON CONFLICT DO NOTHING
		// alone is not sufficient: it skips the INSERT but doesn't lock
		// the existing row, so a concurrent delete that committed
		// before us would still leave us with an orphan reference.
		var dummy int
		err := tx.QueryRowContext(ctx, selectForUpdate, groupID, slug).Scan(&dummy)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// Row gone (or never existed) — INSERT a fresh one. ON
			// CONFLICT DO NOTHING is the safety net for two concurrent
			// inserters racing on the same brand-new slug.
			if _, err := tx.ExecContext(ctx, insertQuery,
				tenantID, groupID, slug, defaultTagLabelFromSlug(slug), models.DefaultTagColor, userID); err != nil {
				return errxtrace.Wrap("failed to insert tag row", err, errx.Attrs("slug", slug))
			}
		case err != nil:
			return errxtrace.Wrap("failed to lock existing tag row", err, errx.Attrs("slug", slug))
		}
	}
	return nil
}

// RenameAtomic does the slug-clash check, JSONB rewrite, and tags-row
// update inside a single transaction held under a per-(group, tag id)
// advisory lock. Two parallel renames of the same tag id serialize on
// the lock; the second re-reads the row inside its lock and renames
// from whatever slug the first one settled on.
func (r *TagRegistry) RenameAtomic(ctx context.Context, id, newLabel, newSlug string, newColor models.TagColor) (*models.Tag, error) {
	var final *models.Tag
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := acquireTagAdvisoryLock(ctx, tx, r.groupID, "id:"+id); err != nil {
			return err
		}

		var current models.Tag
		err := tx.QueryRowxContext(ctx,
			fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.Tags()),
			id).StructScan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			return errxtrace.Wrap("failed to look up tag", registry.ErrNotFound, errx.Attrs("id", id))
		}
		if err != nil {
			return errxtrace.Wrap("failed to look up tag", err)
		}

		updated := current
		updated.UpdatedAt = time.Now()
		if strings.TrimSpace(newLabel) != "" {
			updated.Label = newLabel
		}
		if newColor != "" {
			updated.Color = newColor
		}

		slugChanged := newSlug != "" && newSlug != current.Slug
		if slugChanged {
			// Lock both old and new slugs in canonical order — coordinates
			// with concurrent commodity/file inserts that might want to
			// reference either side of the rename.
			slugLocks := []string{current.Slug, newSlug}
			sort.Strings(slugLocks)
			slugLocks = slices.Compact(slugLocks)
			for _, s := range slugLocks {
				if err := acquireTagAdvisoryLock(ctx, tx, r.groupID, "slug:"+s); err != nil {
					return err
				}
			}

			// Pre-emptive slug-clash check, inside the lock — relying on
			// the unique index alone would still work, but yields a worse
			// error message (Postgres-level uniqueness violation vs.
			// our domain ErrAlreadyExists).
			var clashID string
			clashErr := tx.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT id FROM %s WHERE slug = $1 AND id != $2 LIMIT 1`, r.tableNames.Tags()),
				newSlug, id).Scan(&clashID)
			switch {
			case errors.Is(clashErr, sql.ErrNoRows):
				// no clash — proceed
			case clashErr != nil:
				return errxtrace.Wrap("failed to check slug availability", clashErr)
			default:
				return errxtrace.Wrap("target slug is already used by another tag",
					registry.ErrAlreadyExists, errx.Attrs("slug", newSlug))
			}

			updated.Slug = newSlug
			rewriteCommQuery := fmt.Sprintf(
				`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
				r.tableNames.Commodities(), jsonbReplaceSlugExpr(1, 2))
			if _, err := tx.ExecContext(ctx, rewriteCommQuery, current.Slug, newSlug); err != nil {
				return errxtrace.Wrap("failed to rewrite commodity tags", err)
			}
			rewriteFileQuery := fmt.Sprintf(
				`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
				r.tableNames.Files(), jsonbReplaceSlugExpr(1, 2))
			if _, err := tx.ExecContext(ctx, rewriteFileQuery, current.Slug, newSlug); err != nil {
				return errxtrace.Wrap("failed to rewrite file tags", err)
			}
		}

		var result models.Tag
		err = tx.QueryRowxContext(ctx, fmt.Sprintf(
			`UPDATE %s SET slug = $1, label = $2, color = $3, updated_at = $4 WHERE id = $5 RETURNING *`,
			r.tableNames.Tags()),
			updated.Slug, updated.Label, updated.Color, updated.UpdatedAt, id).StructScan(&result)
		if err != nil {
			return errxtrace.Wrap("failed to update tag row", err)
		}
		final = &result
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to rename tag atomically", err)
	}
	return final, nil
}

// DeleteAtomic checks usage, strips JSONB references (when force=true),
// and deletes the tags row inside one tx held under per-(group, id) +
// per-(group, slug) xact advisory locks. The slug lock is what
// coordinates with concurrent commodity / file inserts via
// ensureTagRowsInTx — they take the same lock and either see the deleted
// row gone (and re-INSERT a fresh one via ON CONFLICT DO NOTHING) or
// block until our tx commits.
//
// When force=false and usage > 0, returns the populated TagUsage along
// with registry.ErrTagInUse and rolls back without mutating state.
func (r *TagRegistry) DeleteAtomic(ctx context.Context, id string, force bool) (registry.TagUsage, error) {
	var usage registry.TagUsage
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := acquireTagAdvisoryLock(ctx, tx, r.groupID, "id:"+id); err != nil {
			return err
		}

		var current models.Tag
		err := tx.QueryRowxContext(ctx,
			fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.Tags()),
			id).StructScan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			return errxtrace.Wrap("failed to look up tag", registry.ErrNotFound, errx.Attrs("id", id))
		}
		if err != nil {
			return errxtrace.Wrap("failed to look up tag", err)
		}

		if err := acquireTagAdvisoryLock(ctx, tx, r.groupID, "slug:"+current.Slug); err != nil {
			return err
		}

		usageQuery := fmt.Sprintf(
			`SELECT (SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array($1::text)),
			        (SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array($1::text))`,
			r.tableNames.Commodities(), r.tableNames.Files())
		if err := tx.QueryRowxContext(ctx, usageQuery, current.Slug).Scan(&usage.Commodities, &usage.Files); err != nil {
			return errxtrace.Wrap("failed to compute tag usage", err)
		}

		if usage.Commodities+usage.Files > 0 && !force {
			return registry.ErrTagInUse
		}
		if usage.Commodities+usage.Files > 0 {
			stripCommQuery := fmt.Sprintf(
				`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
				r.tableNames.Commodities(), jsonbStripSlugExpr(1))
			if _, err := tx.ExecContext(ctx, stripCommQuery, current.Slug); err != nil {
				return errxtrace.Wrap("failed to strip commodity tags", err)
			}
			stripFileQuery := fmt.Sprintf(
				`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
				r.tableNames.Files(), jsonbStripSlugExpr(1))
			if _, err := tx.ExecContext(ctx, stripFileQuery, current.Slug); err != nil {
				return errxtrace.Wrap("failed to strip file tags", err)
			}
		}

		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, r.tableNames.Tags()), id); err != nil {
			return errxtrace.Wrap("failed to delete tag row", err)
		}
		return nil
	})
	if err != nil {
		return usage, err
	}
	return usage, nil
}

func (r *TagRegistry) StripSlugReferences(ctx context.Context, slug string) (commodityRows, fileRows int, err error) {
	var commodityCount, fileCount int
	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		commQuery := fmt.Sprintf(
			`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
			r.tableNames.Commodities(), jsonbStripSlugExpr(1),
		)
		commRes, execErr := tx.ExecContext(ctx, commQuery, slug)
		if execErr != nil {
			return errxtrace.Wrap("failed to strip commodity tags", execErr)
		}
		commAffected, raErr := commRes.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read commodity rows affected", raErr)
		}
		commodityCount = int(commAffected)

		fileQuery := fmt.Sprintf(
			`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
			r.tableNames.Files(), jsonbStripSlugExpr(1),
		)
		fileRes, execErr := tx.ExecContext(ctx, fileQuery, slug)
		if execErr != nil {
			return errxtrace.Wrap("failed to strip file tags", execErr)
		}
		fileAffected, raErr := fileRes.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read file rows affected", raErr)
		}
		fileCount = int(fileAffected)
		return nil
	})
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to strip slug references", err)
	}
	return commodityCount, fileCount, nil
}
