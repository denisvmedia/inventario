package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.FileRegistry = (*FileRegistry)(nil)

type FileRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
}

func NewFileRegistry(dbx *sqlx.DB) *FileRegistry {
	return NewFileRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewFileRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *FileRegistry {
	return &FileRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *FileRegistry) WithCurrentUser(ctx context.Context) (registry.FileRegistry, error) {
	tmp := *r

	userID, err := appctx.RequireUserIDFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}
	tmp.userID = userID
	return &tmp, nil
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
			return nil, errkit.Wrap(err, "failed to list files")
		}
		files = append(files, &file)
	}

	return files, nil
}

func (r *FileRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count files")
	}

	return cnt, nil
}

func (r *FileRegistry) Create(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	// Generate a new ID if one is not already provided
	if file.GetID() == "" {
		file.SetID(generateID())
	}

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, file, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create file")
	}

	return &file, nil
}

func (r *FileRegistry) Update(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, file, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update file")
	}

	return &file, nil
}

func (r *FileRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, nil)
	return err
}

func (r *FileRegistry) newSQLRegistry() *store.RLSRepository[models.FileEntity] {
	return store.NewUserAwareSQLRegistry[models.FileEntity](r.dbx, r.userID, r.tableNames.Files())
}

func (r *FileRegistry) get(ctx context.Context, id string) (*models.FileEntity, error) {
	var file models.FileEntity
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &file)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get file")
	}

	return &file, nil
}

func (r *FileRegistry) ListByType(ctx context.Context, fileType models.FileType) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	for file, err := range reg.ScanByField(ctx, store.Pair("type", fileType)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list files by type")
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
			return errkit.Wrap(err, "failed to list files by linked entity")
		}
		defer rows.Close()

		for rows.Next() {
			var file models.FileEntity
			err := rows.StructScan(&file)
			if err != nil {
				return errkit.Wrap(err, "failed to scan file")
			}
			files = append(files, &file)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list files by linked entity")
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
			return errkit.Wrap(err, "failed to list files by linked entity and meta")
		}
		defer rows.Close()

		for rows.Next() {
			var file models.FileEntity
			err := rows.StructScan(&file)
			if err != nil {
				return errkit.Wrap(err, "failed to scan file")
			}
			files = append(files, &file)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list files by linked entity and meta")
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
			return errkit.Wrap(err, "failed to search files")
		}
		defer rows.Close()

		for rows.Next() {
			var file models.FileEntity
			err := rows.StructScan(&file)
			if err != nil {
				return errkit.Wrap(err, "failed to scan file")
			}
			files = append(files, &file)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to search files")
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
			return errkit.Wrap(err, "failed to count files")
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
			return errkit.Wrap(err, "failed to list paginated files")
		}
		defer rows.Close()

		for rows.Next() {
			var file models.FileEntity
			err := rows.StructScan(&file)
			if err != nil {
				return errkit.Wrap(err, "failed to scan file")
			}
			files = append(files, &file)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, 0, errkit.Wrap(err, "failed to list paginated files")
	}

	return files, total, nil
}
