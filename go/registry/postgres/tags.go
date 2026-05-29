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

func (r *TagRegistry) GetBySlug(ctx context.Context, kind models.TagKind, slug string) (*models.Tag, error) {
	var tag models.Tag
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// RLS scopes the query to the current tenant+group; (kind, slug)
		// disambiguates the per-(group, kind) unique tuple.
		err := tx.QueryRowxContext(ctx,
			fmt.Sprintf(`SELECT * FROM %s WHERE kind = $1 AND slug = $2 LIMIT 1`, r.tableNames.Tags()),
			kind, slug).StructScan(&tag)
		if errors.Is(err, sql.ErrNoRows) {
			return registry.ErrNotFound
		}
		return err
	})
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return nil, err
		}
		return nil, errxtrace.Wrap("failed to get tag by slug", err)
	}
	return &tag, nil
}

// tableForKind returns the JSONB-bearing table whose `tags` column holds
// references for the given tag kind: commodities for commodity tags, files
// for file tags. Unknown/empty kind falls back to commodities (defence in
// depth — the write paths always pass a concrete kind).
func (r *TagRegistry) tableForKind(kind models.TagKind) store.TableName {
	if kind == models.TagKindFile {
		return r.tableNames.Files()
	}
	return r.tableNames.Commodities()
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

// kindUsageExpr returns the per-tag reference-count SQL expression for the
// given kind: commodity tags count commodity rows, file tags count file
// rows. The expression is evaluated against the outer `t` alias (the tags
// row), so callers must reference the tags table as `t`. The `tags @>`
// operand uses the JSONB containment operator backed by the existing GIN
// indexes (commodities_tags_gin_idx, files_tags_gin_idx).
//
// Empty/unknown kind falls back to the combined expression — only reached
// when listing across all kinds (the public handlers always pass a concrete
// kind).
func (r *TagRegistry) kindUsageExpr(kind models.TagKind) string {
	commodityExpr := fmt.Sprintf(
		`(SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array(t.slug))`,
		r.tableNames.Commodities())
	fileExpr := fmt.Sprintf(
		`(SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array(t.slug))`,
		r.tableNames.Files())
	switch kind {
	case models.TagKindCommodity:
		return commodityExpr
	case models.TagKindFile:
		return fileExpr
	default:
		return "(" + commodityExpr + " + " + fileExpr + ")"
	}
}

func (r *TagRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.TagListOptions) ([]*models.Tag, int, error) {
	var tags []*models.Tag
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var conditions []string
		var args []any
		argIdx := 1
		if opts.Search != "" {
			conditions = append(conditions, fmt.Sprintf("(t.label ILIKE $%d OR t.slug ILIKE $%d)", argIdx, argIdx+1))
			pattern := "%" + opts.Search + "%"
			args = append(args, pattern, pattern)
			argIdx += 2
		}
		// Kind filter: intrinsic. A tag passes ?kind=commodity iff its
		// stored kind is "commodity"; same for file. Zero kind adds no
		// condition (list across all kinds — internal callers only).
		if opts.Kind == models.TagKindCommodity || opts.Kind == models.TagKindFile {
			conditions = append(conditions, fmt.Sprintf("t.kind = $%d", argIdx))
			args = append(args, opts.Kind)
			argIdx++
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
			// Usage sort counts only the kind's own table (commodity tags →
			// commodities, file tags → files), so "Sort by usage" reflects
			// the tag's relevant usage.
			orderBy = fmt.Sprintf("ORDER BY %s %s, t.label ASC", r.kindUsageExpr(opts.Kind), dir)
		default:
			orderBy = fmt.Sprintf("ORDER BY LOWER(t.label) %s, t.id ASC", dir)
		}

		dataArgs := append([]any{}, args...)
		dataArgs = append(dataArgs, limit, offset)
		dataQuery := fmt.Sprintf(
			`SELECT t.* FROM %s t %s %s LIMIT $%d OFFSET $%d`,
			r.tableNames.Tags(), whereClause, orderBy, argIdx, argIdx+1,
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

func (r *TagRegistry) Search(ctx context.Context, q string, limit int, kind models.TagKind) ([]*models.Tag, error) {
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
		// Kind filter (intrinsic): the autocomplete dropdown on a commodity
		// input only ever offers commodity tags, and a file input only file
		// tags — they are separate entities.
		if kind == models.TagKindCommodity || kind == models.TagKindFile {
			conditions = append(conditions, fmt.Sprintf("t.kind = $%d", argIdx))
			args = append(args, kind)
			argIdx++
		}
		whereClause := ""
		if len(conditions) > 0 {
			whereClause = "WHERE " + strings.Join(conditions, " AND ")
		}

		// Rank by the kind's own usage desc, then created_at desc
		// (recency tiebreaker).
		kindUsageExpr := r.kindUsageExpr(kind)
		args = append(args, limit)
		query := fmt.Sprintf(
			`SELECT t.* FROM %s t %s ORDER BY %s DESC, t.created_at DESC LIMIT $%d`,
			r.tableNames.Tags(), whereClause, kindUsageExpr, argIdx,
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

func (r *TagRegistry) GetUsageBatch(ctx context.Context, kind models.TagKind, slugs []string) (map[string]registry.TagUsage, error) {
	out := make(map[string]registry.TagUsage, len(slugs))
	for _, s := range slugs {
		out[s] = registry.TagUsage{}
	}
	if len(slugs) == 0 {
		return out, nil
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Single-pass query over the kind's own table only (commodity tags
		// count commodity rows, file tags count file rows): filtered by
		// `tags ?| input` so the GIN index drives the lookup, unnest the
		// JSONB arrays once per row, COUNT(DISTINCT id) per slug to match
		// the @> containment semantics (a row with duplicate slugs still
		// counts as 1), then LEFT JOIN back to the requested input list so
		// missing slugs land as 0 instead of being dropped.
		query := fmt.Sprintf(`
			WITH input(slug) AS (SELECT unnest($1::text[])),
			counts AS (
				SELECT t.value AS slug, COUNT(DISTINCT id)::int AS cnt
				FROM %s, jsonb_array_elements_text(tags) AS t(value)
				WHERE tags ?| $1::text[]
				GROUP BY t.value
			)
			SELECT input.slug, COALESCE(c.cnt, 0) AS cnt
			FROM input
			LEFT JOIN counts c ON c.slug = input.slug
		`, r.tableForKind(kind))

		rows, err := tx.QueryxContext(ctx, query, slugs)
		if err != nil {
			return errxtrace.Wrap("failed to batch-compute tag usage", err)
		}
		defer rows.Close()
		for rows.Next() {
			var slug string
			var cnt int
			if err := rows.Scan(&slug, &cnt); err != nil {
				return errxtrace.Wrap("failed to scan tag usage", err)
			}
			out[slug] = usageForKind(kind, cnt)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to compute tag usage batch", err)
	}
	return out, nil
}

// usageForKind packs a single per-kind reference count into the relevant
// side of TagUsage (the other side stays zero).
func usageForKind(kind models.TagKind, cnt int) registry.TagUsage {
	if kind == models.TagKindFile {
		return registry.TagUsage{Files: cnt}
	}
	return registry.TagUsage{Commodities: cnt}
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
				(SELECT COUNT(*) FROM %s WHERE kind = 'commodity') AS commodity_tags_total,
				(SELECT COUNT(*) FROM %s WHERE kind = 'file') AS file_tags_total,
				(SELECT COUNT(*) FROM %s WHERE jsonb_array_length(COALESCE(tags, '[]'::jsonb)) > 0) AS items_tagged,
				(SELECT COUNT(*) FROM %s WHERE jsonb_array_length(COALESCE(tags, '[]'::jsonb)) = 0) AS items_untagged,
				(SELECT COUNT(*) FROM %s WHERE jsonb_array_length(COALESCE(tags, '[]'::jsonb)) > 0) AS files_tagged,
				(SELECT COUNT(*) FROM %s WHERE jsonb_array_length(COALESCE(tags, '[]'::jsonb)) = 0) AS files_untagged
		`,
			r.tableNames.Tags(),
			r.tableNames.Tags(), r.tableNames.Tags(),
			r.tableNames.Commodities(), r.tableNames.Commodities(),
			r.tableNames.Files(), r.tableNames.Files(),
		)
		return tx.QueryRowxContext(ctx, query).Scan(
			&stats.TagsTotal,
			&stats.CommodityTagsTotal, &stats.FileTagsTotal,
			&stats.ItemsTagged, &stats.ItemsUntagged,
			&stats.FilesTagged, &stats.FilesUntagged,
		)
	})
	if err != nil {
		return registry.TagStats{}, errxtrace.Wrap("failed to compute tag stats", err)
	}
	return stats, nil
}

func (r *TagRegistry) GetUsage(ctx context.Context, kind models.TagKind, slug string) (registry.TagUsage, error) {
	var cnt int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array($1::text)`,
			r.tableForKind(kind),
		)
		return tx.QueryRowxContext(ctx, query, slug).Scan(&cnt)
	})
	if err != nil {
		return registry.TagUsage{}, errxtrace.Wrap("failed to compute tag usage", err)
	}
	return usageForKind(kind, cnt), nil
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

func (r *TagRegistry) RewriteSlugReferences(ctx context.Context, kind models.TagKind, oldSlug, newSlug string) (commodityRows, fileRows int, err error) {
	if oldSlug == newSlug {
		return 0, 0, nil
	}

	var affected int
	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
			r.tableForKind(kind), jsonbReplaceSlugExpr(1, 2),
		)
		res, execErr := tx.ExecContext(ctx, query, oldSlug, newSlug)
		if execErr != nil {
			return errxtrace.Wrap("failed to rewrite tag references", execErr)
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows affected", raErr)
		}
		affected = int(n)
		return nil
	})
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to rewrite slug references", err)
	}
	return rowsForKind(kind, affected)
}

// rowsForKind splits a single affected-row count into the
// (commodityRows, fileRows) return tuple used by Rewrite/StripSlugReferences.
func rowsForKind(kind models.TagKind, affected int) (commodityRows, fileRows int, err error) {
	if kind == models.TagKindFile {
		return 0, affected, nil
	}
	return affected, 0, nil
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
func ensureTagRowsInTx(ctx context.Context, tx *sqlx.Tx, tableNames store.TableNames, tenantID, groupID, userID string, kind models.TagKind, slugs []string) error {
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
		`SELECT 1 FROM %s WHERE group_id = $1 AND kind = $2 AND slug = $3 FOR UPDATE`, tableNames.Tags())
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s (id, tenant_id, group_id, kind, slug, label, color, created_by_user_id)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (group_id, kind, slug) DO NOTHING`, tableNames.Tags())

	for _, slug := range cleaned {
		// Lock key is namespaced by kind so a commodity-tag insert and a
		// file-tag insert of the same slug don't serialize against each
		// other (they are separate rows).
		if err := acquireTagAdvisoryLock(ctx, tx, groupID, tagSlugLockKey(kind, slug)); err != nil {
			return err
		}
		// SELECT FOR UPDATE — if the row exists, take a row-level lock so
		// a concurrent DeleteAtomic (which also does FOR UPDATE on the
		// same row) blocks until our tx commits. ON CONFLICT DO NOTHING
		// alone is not sufficient: it skips the INSERT but doesn't lock
		// the existing row, so a concurrent delete that committed
		// before us would still leave us with an orphan reference.
		var dummy int
		err := tx.QueryRowContext(ctx, selectForUpdate, groupID, kind, slug).Scan(&dummy)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// Row gone (or never existed) — INSERT a fresh one. ON
			// CONFLICT DO NOTHING is the safety net for two concurrent
			// inserters racing on the same brand-new (kind, slug).
			if _, err := tx.ExecContext(ctx, insertQuery,
				tenantID, groupID, kind, slug, defaultTagLabelFromSlug(slug), models.DefaultTagColor, userID); err != nil {
				return errxtrace.Wrap("failed to insert tag row", err, errx.Attrs("slug", slug))
			}
		case err != nil:
			return errxtrace.Wrap("failed to lock existing tag row", err, errx.Attrs("slug", slug))
		}
	}
	return nil
}

// tagSlugLockKey builds the per-(kind, slug) advisory-lock key. The inserter
// (ensureTagRowsInTx) and the rename/delete paths (acquireSlugLocks) must
// agree on this key so they serialize on the same (kind, slug) tuple.
func tagSlugLockKey(kind models.TagKind, slug string) string {
	return "slug:" + string(kind) + ":" + slug
}

// peekTagSlug reads the tag's current slug WITHOUT taking a row lock.
// Safe to call under the (group, id) advisory lock alone: that lock
// serializes RenameAtomic / DeleteAtomic on this tag id with itself, so
// the slug we read won't change before we re-acquire the row with
// FOR UPDATE later in the same tx.
//
// Returning the slug without a row lock is the load-bearing piece for
// avoiding the AB-BA deadlock vs. ensureTagRowsInTx: both paths must
// take the (group, slug) advisory lock BEFORE they take the tag-row
// lock, otherwise a concurrent inserter holding (slug-lock + waiting
// for row-lock) deadlocks against a deleter holding (row-lock +
// waiting for slug-lock).
func (r *TagRegistry) peekTagSlug(ctx context.Context, tx *sqlx.Tx, id string) (slug string, kind models.TagKind, err error) {
	err = tx.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT slug, kind FROM %s WHERE id = $1`, r.tableNames.Tags()),
		id).Scan(&slug, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", errxtrace.Wrap("failed to look up tag", registry.ErrNotFound, errx.Attrs("id", id))
	}
	if err != nil {
		return "", "", errxtrace.Wrap("failed to look up tag", err)
	}
	return slug, kind, nil
}

// renameRewriteSlug runs the JSONB-rewrite half of RenameAtomic — slug
// clash check + the two table updates — inside the caller's tx.
// Slug-advisory locks must already be held by the orchestrator (taken
// before the tag-row FOR UPDATE so the lock acquisition order matches
// ensureTagRowsInTx). Lifted out of RenameAtomic so the orchestrator
// stays under gocognit's threshold without losing the rewrite step.
func (r *TagRegistry) renameRewriteSlug(ctx context.Context, tx *sqlx.Tx, id string, kind models.TagKind, oldSlug, newSlug string) error {
	// Pre-emptive slug-clash check, inside the lock — scoped to the same
	// kind, because (group, kind, slug) is the uniqueness tuple: the same
	// slug may legitimately exist under the other kind. Relying on the
	// unique index alone would still work, but yields a worse error
	// message (Postgres-level uniqueness violation vs. our domain
	// ErrAlreadyExists).
	var clashID string
	clashErr := tx.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id FROM %s WHERE kind = $1 AND slug = $2 AND id != $3 LIMIT 1`, r.tableNames.Tags()),
		kind, newSlug, id).Scan(&clashID)
	switch {
	case errors.Is(clashErr, sql.ErrNoRows):
		// no clash — proceed
	case clashErr != nil:
		return errxtrace.Wrap("failed to check slug availability", clashErr)
	default:
		return errxtrace.Wrap("target slug is already used by another tag",
			registry.ErrAlreadyExists, errx.Attrs("slug", newSlug))
	}

	// Rewrite references only on the kind's own table — a commodity-tag
	// rename must not touch files (those slugs belong to file tags).
	rewriteQuery := fmt.Sprintf(
		`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
		r.tableForKind(kind), jsonbReplaceSlugExpr(1, 2))
	if _, err := tx.ExecContext(ctx, rewriteQuery, oldSlug, newSlug); err != nil {
		return errxtrace.Wrap("failed to rewrite tag references", err)
	}
	return nil
}

// acquireSlugLocks takes (group, slug) advisory locks for the given
// slugs in canonical (sorted, deduped) order. Run before any tag-row
// FOR UPDATE so the lock acquisition order matches the inserter side
// in ensureTagRowsInTx — slug-advisory then row-lock, never the
// reverse, because the reverse interleaves with the inserter's order
// and deadlocks under concurrency (real failure mode seen in CI on
// PR #1491; see follow-up #1492).
func (r *TagRegistry) acquireSlugLocks(ctx context.Context, tx *sqlx.Tx, kind models.TagKind, slugs ...string) error {
	keys := slices.Clone(slugs)
	sort.Strings(keys)
	keys = slices.Compact(keys)
	for _, s := range keys {
		if s == "" {
			continue
		}
		if err := acquireTagAdvisoryLock(ctx, tx, r.groupID, tagSlugLockKey(kind, s)); err != nil {
			return err
		}
	}
	return nil
}

// RenameAtomic does the slug-clash check, JSONB rewrite, and tags-row
// update inside a single transaction held under a per-(group, tag id)
// advisory lock. Two parallel renames of the same tag id serialize on
// the lock; the second re-reads the row inside its lock and renames
// from whatever slug the first one settled on.
//
// Lock order: id-advisory → peek slug (no row lock) → slug-advisory
// (old + new, sorted) → tag-row FOR UPDATE → rewrite + update. The
// peek-then-slug-lock split is what avoids deadlocking against
// concurrent ensureTagRowsInTx (which takes slug-lock then row-lock).
func (r *TagRegistry) RenameAtomic(ctx context.Context, id, newLabel, newSlug string, newColor models.TagColor) (*models.Tag, error) {
	var final *models.Tag
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := acquireTagAdvisoryLock(ctx, tx, r.groupID, "id:"+id); err != nil {
			return err
		}

		// Peek the slug + kind without locking the row, so we can take the
		// slug-advisory lock(s) BEFORE the row lock. Kind is needed both to
		// namespace the locks and to scope the rewrite to the right table.
		oldSlug, kind, err := r.peekTagSlug(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := r.acquireSlugLocks(ctx, tx, kind, oldSlug, newSlug); err != nil {
			return err
		}

		// Now safe to take the row lock.
		var current models.Tag
		err = tx.QueryRowxContext(ctx,
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

		if newSlug != "" && newSlug != current.Slug {
			updated.Slug = newSlug
			if err := r.renameRewriteSlug(ctx, tx, id, kind, current.Slug, newSlug); err != nil {
				return err
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
// Lock order: id-advisory → peek slug (no row lock) → slug-advisory →
// tag-row FOR UPDATE → strip + delete. Same shape as RenameAtomic for
// the same reason — taking the row lock before the slug lock would
// deadlock against ensureTagRowsInTx, which acquires them in the
// reverse order (slug-lock then row-lock).
//
// When force=false and usage > 0, returns the populated TagUsage along
// with registry.ErrTagInUse and rolls back without mutating state.
//
//revive:disable-next-line:flag-parameter
func (r *TagRegistry) DeleteAtomic(ctx context.Context, id string, force bool) (registry.TagUsage, error) {
	var usage registry.TagUsage
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := acquireTagAdvisoryLock(ctx, tx, r.groupID, "id:"+id); err != nil {
			return err
		}

		// Peek the slug + kind without locking the row, so we can take the
		// slug-advisory lock BEFORE the row lock.
		slug, kind, err := r.peekTagSlug(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := r.acquireSlugLocks(ctx, tx, kind, slug); err != nil {
			return err
		}

		var current models.Tag
		err = tx.QueryRowxContext(ctx,
			fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.Tags()),
			id).StructScan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			return errxtrace.Wrap("failed to look up tag", registry.ErrNotFound, errx.Attrs("id", id))
		}
		if err != nil {
			return errxtrace.Wrap("failed to look up tag", err)
		}

		// Usage counts only the tag's own kind table — a commodity tag's
		// usage is its commodities, never files that happen to share the slug.
		var refCount int
		usageQuery := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE tags @> jsonb_build_array($1::text)`,
			r.tableForKind(current.Kind))
		if err := tx.QueryRowxContext(ctx, usageQuery, current.Slug).Scan(&refCount); err != nil {
			return errxtrace.Wrap("failed to compute tag usage", err)
		}
		usage = usageForKind(current.Kind, refCount)

		if refCount > 0 && !force {
			return registry.ErrTagInUse
		}
		if refCount > 0 {
			stripQuery := fmt.Sprintf(
				`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
				r.tableForKind(current.Kind), jsonbStripSlugExpr(1))
			if _, err := tx.ExecContext(ctx, stripQuery, current.Slug); err != nil {
				return errxtrace.Wrap("failed to strip tag references", err)
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

func (r *TagRegistry) StripSlugReferences(ctx context.Context, kind models.TagKind, slug string) (commodityRows, fileRows int, err error) {
	var affected int
	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET tags = %s WHERE tags @> jsonb_build_array($1::text)`,
			r.tableForKind(kind), jsonbStripSlugExpr(1),
		)
		res, execErr := tx.ExecContext(ctx, query, slug)
		if execErr != nil {
			return errxtrace.Wrap("failed to strip tag references", execErr)
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows affected", raErr)
		}
		affected = int(n)
		return nil
	})
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to strip slug references", err)
	}
	return rowsForKind(kind, affected)
}
