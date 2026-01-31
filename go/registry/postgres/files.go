package postgres

import (
	"context"
	"encoding/json"
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

// FileRegistryFactory creates FileRegistry instances with proper context
type FileRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// FileRegistry is a context-aware registry that can only be created through the factory
type FileRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
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
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     user.ID,
		tenantID:   user.TenantID,
		service:    false,
	}, nil
}

func (f *FileRegistryFactory) CreateServiceRegistry() registry.FileRegistry {
	return &FileRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     "",
		tenantID:   "",
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

func (r *FileRegistry) newSQLRegistry() *store.RLSRepository[models.FileEntity, *models.FileEntity] {
	if r.service {
		return store.NewServiceSQLRegistry[models.FileEntity](r.dbx, r.tableNames.Files())
	}
	return store.NewUserAwareSQLRegistry[models.FileEntity](r.dbx, r.userID, r.tenantID, r.tableNames.Files())
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

func (r *FileRegistry) Search(ctx context.Context, query string, fileType *models.FileType, tags []string) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var conditions []string
		var args []any
		argIndex := 1

		// Add type filter if specified
		if fileType != nil {
			conditions = append(conditions, fmt.Sprintf("type = $%d", argIndex))
			args = append(args, *fileType)
			argIndex++
		}

		// Add tags filter if specified
		if len(tags) > 0 {
			conditions = append(conditions, fmt.Sprintf("tags @> $%d", argIndex))
			tagsJSON, _ := json.Marshal(tags)
			args = append(args, tagsJSON)
			argIndex++
		}

		// Add text search if specified
		if query != "" {
			searchCondition := fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d OR path ILIKE $%d OR original_path ILIKE $%d)", argIndex, argIndex+1, argIndex+2, argIndex+3)
			conditions = append(conditions, searchCondition)
			searchPattern := "%" + query + "%"
			args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
		}

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

func (r *FileRegistry) ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType) ([]*models.FileEntity, int, error) {
	var files []*models.FileEntity
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// First get the total count
		var countQuery string
		var countArgs []any

		if fileType != nil {
			countQuery = fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE type = $1`, r.tableNames.Files())
			countArgs = []any{*fileType}
		} else {
			countQuery = fmt.Sprintf(`SELECT COUNT(*) FROM %s`, r.tableNames.Files())
		}

		err := tx.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
		if err != nil {
			return errxtrace.Wrap("failed to count files", err)
		}

		// Then get the paginated results
		var dataQuery string
		var dataArgs []any

		if fileType != nil {
			dataQuery = fmt.Sprintf(`
				SELECT * FROM %s
				WHERE type = $1
				ORDER BY created_at DESC
				LIMIT $2 OFFSET $3`, r.tableNames.Files())
			dataArgs = []any{*fileType, limit, offset}
		} else {
			dataQuery = fmt.Sprintf(`
				SELECT * FROM %s
				ORDER BY created_at DESC
				LIMIT $1 OFFSET $2`, r.tableNames.Files())
			dataArgs = []any{limit, offset}
		}

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
