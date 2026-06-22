package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// FileRegistryFactory creates FileRegistry instances with proper context
type FileRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// FileRegistry is a context-aware registry that can only be created through the factory
type FileRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var _ registry.FileRegistry = (*FileRegistry)(nil)
var _ registry.FileRegistryFactory = (*FileRegistryFactory)(nil)

func NewFileRegistry(dbx *sqlx.DB) *FileRegistryFactory {
	return NewFileRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewFileRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *FileRegistryFactory {
	return &FileRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.FileRegistryFactory

func (f *FileRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.FileRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *FileRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.FileRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user ID from context", err)
	}

	return &FileRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
		service:         false,
	}, nil
}

func (f *FileRegistryFactory) CreateServiceRegistry() registry.FileRegistry {
	return &FileRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *FileRegistry) Get(ctx context.Context, id string) (*models.FileEntity, error) {
	return r.get(ctx, id)
}

func (r *FileRegistry) List(ctx context.Context) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()

	// Query the database for all files (atomic operation)
	for file, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list files", err)
		}
		files = append(files, &file)
	}

	return files, nil
}

func (r *FileRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count files", err)
	}

	return cnt, nil
}

func (r *FileRegistry) Create(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create

	reg := r.newSQLRegistry()

	createdFile, err := reg.Create(ctx, file, func(ctx context.Context, tx *sqlx.Tx) error {
		// Same orphan-prevention as in CommodityRegistry.Create — see
		// ensureTagRowsInTx in tags.go for the cross-tx invariant.
		return ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, models.TagKindFile, []string(file.Tags))
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file", err)
	}

	return &createdFile, nil
}

func (r *FileRegistry) Update(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, file, func(ctx context.Context, tx *sqlx.Tx, dbFile models.FileEntity) error {
		return ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, models.TagKindFile, []string(file.Tags))
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to update file", err)
	}

	return &file, nil
}

// Delete removes ONLY the file row. It does NOT touch the physical blob or its
// thumbnails, and it does NOT break the thumbnail_generation_jobs FK chain.
//
// INVARIANT (#2121): user-initiated file deletes MUST go through
// services.FileService.DeleteFileWithPhysical, which deletes the thumbnail-job
// chain, deletes this row, and then best-effort removes the blob + thumbnails.
// Calling this method directly orphans the physical blob (the bytes stay in the
// bucket with no row pointing at them). The only legitimate callers of the bare
// row delete are flows that delete the blobs separately and in bulk — e.g. the
// group/tenant purgers, which sweep every blob for the (tenant, group) up front
// via FileService.DeletePhysicalFilesForGroup / ...ForTenant before dropping the
// rows. New code that deletes a user's file should not call this.
func (r *FileRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, nil)
	return err
}

func (r *FileRegistry) newSQLRegistry() *store.RLSGroupRepository[models.FileEntity, *models.FileEntity] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.FileEntity](r.dbx, r.tableNames.Files())
	}
	return store.NewGroupAwareSQLRegistry[models.FileEntity](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.Files())
}

func (r *FileRegistry) get(ctx context.Context, id string) (*models.FileEntity, error) {
	var file models.FileEntity
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &file)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get file", err)
	}

	return &file, nil
}

func (r *FileRegistry) ListByType(ctx context.Context, fileType models.FileType) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	for file, err := range reg.ScanByField(ctx, store.Pair("type", fileType)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list files by type", err)
		}
		files = append(files, &file)
	}

	return files, nil
}

func (r *FileRegistry) ListByLinkedEntity(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE linked_entity_type = $1 AND linked_entity_id = $2
			ORDER BY created_at DESC`, r.tableNames.Files())

		rows, err := tx.QueryxContext(ctx, query, entityType, entityID)
		if err != nil {
			return errxtrace.Wrap("failed to list files by linked entity", err)
		}
		defer rows.Close()

		for rows.Next() {
			var file models.FileEntity
			err := rows.StructScan(&file)
			if err != nil {
				return errxtrace.Wrap("failed to scan file", err)
			}
			files = append(files, &file)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files by linked entity", err)
	}

	return files, nil
}

// ListByGroup returns every file belonging to the given (tenant_id, group_id)
// tuple. Used by GroupPurgeService to find physical blobs to delete before
// the dependent-row purge wipes the file table. Runs as a single indexed
// query so purge cost scales with per-group files rather than with the full
// tenant-wide file count (review comment on #1316).
//
// Called from the purge worker, which has no tenant context; must therefore
// be invoked on a service-mode registry. For user-mode callers the RLS
// policies already scope List() to a single tenant/group combination.
func (r *FileRegistry) ListByGroup(ctx context.Context, tenantID, groupID string) ([]*models.FileEntity, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE tenant_id = $1 AND group_id = $2
			ORDER BY created_at DESC`, r.tableNames.Files())

		rows, err := tx.QueryxContext(ctx, query, tenantID, groupID)
		if err != nil {
			return errxtrace.Wrap("failed to list files by group", err)
		}
		defer rows.Close()

		for rows.Next() {
			var file models.FileEntity
			if scanErr := rows.StructScan(&file); scanErr != nil {
				return errxtrace.Wrap("failed to scan file", scanErr)
			}
			files = append(files, &file)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files by group", err)
	}
	return files, nil
}

func (r *FileRegistry) ListByLinkedEntityAndMeta(ctx context.Context, entityType, entityID, meta string) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE linked_entity_type = $1 AND linked_entity_id = $2 AND linked_entity_meta = $3
			ORDER BY created_at DESC`, r.tableNames.Files())

		rows, err := tx.QueryxContext(ctx, query, entityType, entityID, meta)
		if err != nil {
			return errxtrace.Wrap("failed to list files by linked entity and meta", err)
		}
		defer rows.Close()

		for rows.Next() {
			var file models.FileEntity
			err := rows.StructScan(&file)
			if err != nil {
				return errxtrace.Wrap("failed to scan file", err)
			}
			files = append(files, &file)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files by linked entity and meta", err)
	}

	return files, nil
}

// buildSearchConditions assembles WHERE-clause fragments shared by Search,
// ListPaginated, and CountByCategory. Returns the conditions slice, the
// positional args, and the next parameter index so callers can append more
// filters without re-numbering.
func buildSearchConditions(query string, fileType *models.FileType, fileCategory *models.FileCategory, tags []string, linkedEntityType, linkedEntityID *string, startIndex int) ([]string, []any, int) {
	var conditions []string
	var args []any
	argIndex := startIndex

	if fileType != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIndex))
		args = append(args, *fileType)
		argIndex++
	}

	if fileCategory != nil {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argIndex))
		args = append(args, *fileCategory)
		argIndex++
	}

	// Linked-entity filter: both type+id must be supplied together or both
	// nil. Mismatched callers (one nil, one not) get no filter — the
	// interface contract documents this; mirroring the memory path.
	if linkedEntityType != nil && linkedEntityID != nil {
		conditions = append(conditions,
			fmt.Sprintf("linked_entity_type = $%d AND linked_entity_id = $%d", argIndex, argIndex+1))
		args = append(args, *linkedEntityType, *linkedEntityID)
		argIndex += 2
	}

	if len(tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("tags @> $%d", argIndex))
		tagsJSON, _ := json.Marshal(tags)
		args = append(args, tagsJSON)
		argIndex++
	}

	if query != "" {
		searchCondition := fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d OR path ILIKE $%d OR original_path ILIKE $%d)", argIndex, argIndex+1, argIndex+2, argIndex+3)
		conditions = append(conditions, searchCondition)
		searchPattern := "%" + query + "%"
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
		argIndex += 4
	}

	return conditions, args, argIndex
}

func (r *FileRegistry) Search(ctx context.Context, query string, fileType *models.FileType, fileCategory *models.FileCategory, tags []string, linkedEntityType, linkedEntityID *string) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		conditions, args, _ := buildSearchConditions(query, fileType, fileCategory, tags, linkedEntityType, linkedEntityID, 1)

		whereClause := ""
		if len(conditions) > 0 {
			whereClause = "WHERE " + strings.Join(conditions, " AND ")
		}

		sqlQuery := fmt.Sprintf(`
			SELECT * FROM %s
			%s
			ORDER BY created_at DESC`, r.tableNames.Files(), whereClause)

		rows, err := tx.QueryxContext(ctx, sqlQuery, args...)
		if err != nil {
			return errxtrace.Wrap("failed to search files", err)
		}
		defer rows.Close()

		for rows.Next() {
			var file models.FileEntity
			err := rows.StructScan(&file)
			if err != nil {
				return errxtrace.Wrap("failed to scan file", err)
			}
			files = append(files, &file)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to search files", err)
	}

	return files, nil
}

func (r *FileRegistry) ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType, fileCategory *models.FileCategory, linkedEntityType, linkedEntityID *string) ([]*models.FileEntity, int, error) {
	var files []*models.FileEntity
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		conditions, args, _ := buildSearchConditions("", fileType, fileCategory, nil, linkedEntityType, linkedEntityID, 1)

		whereClause := ""
		if len(conditions) > 0 {
			whereClause = "WHERE " + strings.Join(conditions, " AND ")
		}

		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, r.tableNames.Files(), whereClause)
		err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total)
		if err != nil {
			return errxtrace.Wrap("failed to count files", err)
		}

		dataArgs := append([]any{}, args...)
		dataArgs = append(dataArgs, limit, offset)
		dataQuery := fmt.Sprintf(`
			SELECT * FROM %s
			%s
			ORDER BY created_at DESC
			LIMIT $%d OFFSET $%d`, r.tableNames.Files(), whereClause, len(args)+1, len(args)+2)

		rows, err := tx.QueryxContext(ctx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list paginated files", err)
		}
		defer rows.Close()

		for rows.Next() {
			var file models.FileEntity
			err := rows.StructScan(&file)
			if err != nil {
				return errxtrace.Wrap("failed to scan file", err)
			}
			files = append(files, &file)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list paginated files", err)
	}

	return files, total, nil
}

// CountByCategory aggregates files matching the same filters as Search,
// grouped by Category. The four buckets are always present in the result
// (zero-filled when missing) so the FE tile renderer can rely on a stable
// shape. The second returned map carries the per-category sum of
// size_bytes — the FE consumes it to render the cumulative
// "{N} files · {Y} total" footer alongside the tile counts.
func (r *FileRegistry) CountByCategory(ctx context.Context, query string, fileType *models.FileType, tags []string) (map[models.FileCategory]int, map[models.FileCategory]int64, error) {
	counts := map[models.FileCategory]int{
		models.FileCategoryImages:    0,
		models.FileCategoryDocuments: 0,
		models.FileCategoryOther:     0,
	}
	bytes := map[models.FileCategory]int64{
		models.FileCategoryImages:    0,
		models.FileCategoryDocuments: 0,
		models.FileCategoryOther:     0,
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		conditions, args, _ := buildSearchConditions(query, fileType, nil, tags, nil, nil, 1)

		whereClause := ""
		if len(conditions) > 0 {
			whereClause = "WHERE " + strings.Join(conditions, " AND ")
		}

		// COALESCE keeps NULL sums (an entirely empty bucket on this
		// scoped query) from blowing up the int64 scan.
		sqlQuery := fmt.Sprintf(`
			SELECT category, COUNT(*), COALESCE(SUM(size_bytes), 0) FROM %s
			%s
			GROUP BY category`, r.tableNames.Files(), whereClause)

		rows, err := tx.QueryxContext(ctx, sqlQuery, args...)
		if err != nil {
			return errxtrace.Wrap("failed to count files by category", err)
		}
		defer rows.Close()

		for rows.Next() {
			var category models.FileCategory
			var count int
			var sizeBytes int64
			if err := rows.Scan(&category, &count, &sizeBytes); err != nil {
				return errxtrace.Wrap("failed to scan category count", err)
			}
			counts[category] = count
			bytes[category] = sizeBytes
		}

		return rows.Err()
	})
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to count files by category", err)
	}

	return counts, bytes, nil
}

// ListPendingSizeBackfill returns up to limit file rows whose
// size_bytes is still zero. Used by the boot-time backfill (#1388);
// runs in service mode (RLS bypass) across every tenant + group. Query
// is bounded by limit so a multi-million-row install can backfill in
// chunks instead of pulling the whole catalogue into memory.
func (r *FileRegistry) ListPendingSizeBackfill(ctx context.Context, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	var files []*models.FileEntity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE size_bytes = 0
			ORDER BY created_at ASC
			LIMIT $1`, r.tableNames.Files())
		rows, err := tx.QueryxContext(ctx, query, limit)
		if err != nil {
			return errxtrace.Wrap("failed to list pending size-backfill files", err)
		}
		defer rows.Close()
		for rows.Next() {
			var file models.FileEntity
			if scanErr := rows.StructScan(&file); scanErr != nil {
				return errxtrace.Wrap("failed to scan file", scanErr)
			}
			files = append(files, &file)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list pending size-backfill files", err)
	}
	return files, nil
}

// SumSizeBreakdownByGroup mirrors SumSizeBreakdown but for an explicit
// (tenant_id, group_id) tuple. Runs as background-worker (no RLS) so
// the storage quota warning worker (#1585) can compute usage per
// group while iterating every tenant. Same CASE-aggregate as
// SumSizeBreakdown, with the (tenant_id, group_id) tuple bolted onto
// the WHERE clause.
func (r *FileRegistry) SumSizeBreakdownByGroup(ctx context.Context, tenantID, groupID string) (registry.StorageBreakdown, error) {
	if tenantID == "" {
		return registry.StorageBreakdown{}, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if groupID == "" {
		return registry.StorageBreakdown{}, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	var breakdown registry.StorageBreakdown
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		sqlQuery := fmt.Sprintf(`
			SELECT
				COALESCE(SUM(CASE WHEN linked_entity_type = 'export' THEN size_bytes ELSE 0 END), 0) AS exports,
				COALESCE(SUM(CASE WHEN linked_entity_type IS DISTINCT FROM 'export' AND category = 'images' THEN size_bytes ELSE 0 END), 0) AS images,
				COALESCE(SUM(CASE WHEN linked_entity_type IS DISTINCT FROM 'export' AND category = 'documents' THEN size_bytes ELSE 0 END), 0) AS documents,
				COALESCE(SUM(CASE WHEN linked_entity_type IS DISTINCT FROM 'export' AND category = 'other' THEN size_bytes ELSE 0 END), 0) AS other
			FROM %s
			WHERE tenant_id = $1 AND group_id = $2`, r.tableNames.Files())

		row := tx.QueryRowxContext(ctx, sqlQuery, tenantID, groupID)
		if err := row.Scan(&breakdown.Exports, &breakdown.Images, &breakdown.Documents, &breakdown.Other); err != nil {
			return errxtrace.Wrap("failed to scan storage breakdown by group", err)
		}
		return nil
	})
	if err != nil {
		return registry.StorageBreakdown{}, errxtrace.Wrap("failed to sum size breakdown by group", err)
	}
	return breakdown, nil
}

// SumSizeBreakdown returns per-bucket byte totals for the current
// (tenant, group) scope (#1388). RLS handles the tenant+group filter;
// the SQL splits export bundles (linked_entity_type='export') out of
// FileCategoryOther so the FE can render a distinct "exports" row in
// the storage card without double-counting.
func (r *FileRegistry) SumSizeBreakdown(ctx context.Context) (registry.StorageBreakdown, error) {
	var breakdown registry.StorageBreakdown

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// One pass over the table, with CASE placing each row into its
		// bucket. COALESCE keeps an empty group from returning NULL.
		// #1622 dropped the `invoices` FileCategory; legacy rows are
		// reclassified to `documents` by the collapse_invoice_category
		// migration, so we only count three category buckets here.
		sqlQuery := fmt.Sprintf(`
			SELECT
				COALESCE(SUM(CASE WHEN linked_entity_type = 'export' THEN size_bytes ELSE 0 END), 0) AS exports,
				COALESCE(SUM(CASE WHEN linked_entity_type IS DISTINCT FROM 'export' AND category = 'images' THEN size_bytes ELSE 0 END), 0) AS images,
				COALESCE(SUM(CASE WHEN linked_entity_type IS DISTINCT FROM 'export' AND category = 'documents' THEN size_bytes ELSE 0 END), 0) AS documents,
				COALESCE(SUM(CASE WHEN linked_entity_type IS DISTINCT FROM 'export' AND category = 'other' THEN size_bytes ELSE 0 END), 0) AS other
			FROM %s`, r.tableNames.Files())

		row := tx.QueryRowxContext(ctx, sqlQuery)
		if err := row.Scan(&breakdown.Exports, &breakdown.Images, &breakdown.Documents, &breakdown.Other); err != nil {
			return errxtrace.Wrap("failed to scan storage breakdown", err)
		}
		return nil
	})
	if err != nil {
		return registry.StorageBreakdown{}, errxtrace.Wrap("failed to sum size breakdown", err)
	}

	return breakdown, nil
}
