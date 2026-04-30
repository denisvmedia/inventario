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

	createdFile, err := reg.Create(ctx, file, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file", err)
	}

	return &createdFile, nil
}

func (r *FileRegistry) Update(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, file, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update file", err)
	}

	return &file, nil
}

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
func buildSearchConditions(query string, fileType *models.FileType, fileCategory *models.FileCategory, tags []string, startIndex int) ([]string, []any, int) {
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

func (r *FileRegistry) Search(ctx context.Context, query string, fileType *models.FileType, fileCategory *models.FileCategory, tags []string) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		conditions, args, _ := buildSearchConditions(query, fileType, fileCategory, tags, 1)

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

func (r *FileRegistry) ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType, fileCategory *models.FileCategory) ([]*models.FileEntity, int, error) {
	var files []*models.FileEntity
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		conditions, args, _ := buildSearchConditions("", fileType, fileCategory, nil, 1)

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
// shape.
func (r *FileRegistry) CountByCategory(ctx context.Context, query string, fileType *models.FileType, tags []string) (map[models.FileCategory]int, error) {
	counts := map[models.FileCategory]int{
		models.FileCategoryPhotos:    0,
		models.FileCategoryInvoices:  0,
		models.FileCategoryDocuments: 0,
		models.FileCategoryOther:     0,
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		conditions, args, _ := buildSearchConditions(query, fileType, nil, tags, 1)

		whereClause := ""
		if len(conditions) > 0 {
			whereClause = "WHERE " + strings.Join(conditions, " AND ")
		}

		sqlQuery := fmt.Sprintf(`
			SELECT category, COUNT(*) FROM %s
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
			if err := rows.Scan(&category, &count); err != nil {
				return errxtrace.Wrap("failed to scan category count", err)
			}
			counts[category] = count
		}

		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count files by category", err)
	}

	return counts, nil
}
