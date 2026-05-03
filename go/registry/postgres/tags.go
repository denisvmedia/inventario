package postgres

import (
	"context"
	"fmt"
	"strings"

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
