package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.FileRegistry = (*FileRegistry)(nil)

type FileRegistry struct {
	db *sqlx.DB
}

func NewFileRegistry(db *sqlx.DB) *FileRegistry {
	return &FileRegistry{db: db}
}

func (r *FileRegistry) Create(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	if file.ID == "" {
		file.SetID(generateID())
	}

	// Use InsertEntity like other registries to automatically handle all db-tagged fields including tenant_id
	err := InsertEntity(ctx, r.db, "files", file)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create file")
	}

	return &file, nil
}

func (r *FileRegistry) Get(ctx context.Context, id string) (*models.FileEntity, error) {
	var file models.FileEntity
	file.File = &models.File{}
	var tagsJSON []byte

	query := `
		SELECT id, title, description, type, tags, path, original_path, ext, mime_type, linked_entity_type, linked_entity_id, linked_entity_meta, created_at, updated_at
		FROM files
		WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&file.ID, &file.Title, &file.Description, &file.Type, &tagsJSON,
		&file.Path, &file.OriginalPath, &file.Ext, &file.MIMEType,
		&file.LinkedEntityType, &file.LinkedEntityID, &file.LinkedEntityMeta,
		&file.CreatedAt, &file.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, registry.ErrNotFound
		}
		return nil, errkit.Wrap(err, "failed to get file")
	}

	if len(tagsJSON) > 0 {
		err = json.Unmarshal(tagsJSON, &file.Tags)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to unmarshal tags")
		}
	}

	return &file, nil
}

func (r *FileRegistry) List(ctx context.Context) ([]*models.FileEntity, error) {
	query := `
		SELECT id, title, description, type, tags, path, original_path, ext, mime_type, linked_entity_type, linked_entity_id, linked_entity_meta, created_at, updated_at
		FROM files
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list files")
	}
	defer rows.Close()

	var files []*models.FileEntity
	for rows.Next() {
		var file models.FileEntity
		file.File = &models.File{}
		var tagsJSON []byte

		err := rows.Scan(
			&file.ID, &file.Title, &file.Description, &file.Type, &tagsJSON,
			&file.Path, &file.OriginalPath, &file.Ext, &file.MIMEType,
			&file.LinkedEntityType, &file.LinkedEntityID, &file.LinkedEntityMeta,
			&file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan file")
		}

		if len(tagsJSON) > 0 {
			err = json.Unmarshal(tagsJSON, &file.Tags)
			if err != nil {
				return nil, errkit.Wrap(err, "failed to unmarshal tags")
			}
		}

		files = append(files, &file)
	}

	return files, nil
}

// ListByLinkedEntity returns files linked to a specific entity
func (r *FileRegistry) ListByLinkedEntity(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error) {
	query := `
		SELECT id, title, description, type, tags, path, original_path, ext, mime_type, linked_entity_type, linked_entity_id, linked_entity_meta, created_at, updated_at
		FROM files
		WHERE linked_entity_type = $1 AND linked_entity_id = $2
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, entityType, entityID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list files by linked entity")
	}
	defer rows.Close()

	var files []*models.FileEntity
	for rows.Next() {
		var file models.FileEntity
		file.File = &models.File{}
		var tagsJSON []byte

		err := rows.Scan(
			&file.ID, &file.Title, &file.Description, &file.Type, &tagsJSON,
			&file.Path, &file.OriginalPath, &file.Ext, &file.MIMEType,
			&file.LinkedEntityType, &file.LinkedEntityID, &file.LinkedEntityMeta,
			&file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan file")
		}

		if len(tagsJSON) > 0 {
			err = json.Unmarshal(tagsJSON, &file.Tags)
			if err != nil {
				return nil, errkit.Wrap(err, "failed to unmarshal tags")
			}
		}

		files = append(files, &file)
	}

	if err = rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "failed to iterate over files")
	}

	return files, nil
}

// ListByLinkedEntityAndMeta returns files linked to a specific entity with specific metadata
func (r *FileRegistry) ListByLinkedEntityAndMeta(ctx context.Context, entityType, entityID, meta string) ([]*models.FileEntity, error) {
	query := `
		SELECT id, title, description, type, tags, path, original_path, ext, mime_type, linked_entity_type, linked_entity_id, linked_entity_meta, created_at, updated_at
		FROM files
		WHERE linked_entity_type = $1 AND linked_entity_id = $2 AND linked_entity_meta = $3
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, entityType, entityID, meta)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list files by linked entity and meta")
	}
	defer rows.Close()

	var files []*models.FileEntity
	for rows.Next() {
		var file models.FileEntity
		file.File = &models.File{}

		var tagsJSON []byte

		err := rows.Scan(
			&file.ID, &file.Title, &file.Description, &file.Type, &tagsJSON,
			&file.Path, &file.OriginalPath, &file.Ext, &file.MIMEType,
			&file.LinkedEntityType, &file.LinkedEntityID, &file.LinkedEntityMeta,
			&file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan file")
		}

		if len(tagsJSON) > 0 {
			err = json.Unmarshal(tagsJSON, &file.Tags)
			if err != nil {
				return nil, errkit.Wrap(err, "failed to unmarshal tags")
			}
		}

		// Initialize File struct
		files = append(files, &file)
	}

	if err = rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "failed to iterate over files")
	}

	return files, nil
}

func (r *FileRegistry) Update(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	tagsJSON, err := json.Marshal(file.Tags)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal tags")
	}

	query := `
		UPDATE files
		SET title = $2, description = $3, type = $4, tags = $5, path = $6, linked_entity_type = $7, linked_entity_id = $8, linked_entity_meta = $9, updated_at = $10
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		file.ID, file.Title, file.Description, file.Type, tagsJSON, file.Path,
		file.LinkedEntityType, file.LinkedEntityID, file.LinkedEntityMeta, file.UpdatedAt,
	)

	if err != nil {
		return nil, errkit.Wrap(err, "failed to update file")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return nil, registry.ErrNotFound
	}

	return &file, nil
}

func (r *FileRegistry) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM files WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete file")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errkit.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return registry.ErrNotFound
	}

	return nil
}

func (r *FileRegistry) Count(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM files`

	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count files")
	}

	return count, nil
}

func (r *FileRegistry) ListByType(ctx context.Context, fileType models.FileType) ([]*models.FileEntity, error) {
	query := `
		SELECT id, title, description, type, tags, path, original_path, ext, mime_type, linked_entity_type, linked_entity_id, linked_entity_meta, created_at, updated_at
		FROM files
		WHERE type = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, fileType)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list files by type")
	}
	defer rows.Close()

	var files []*models.FileEntity
	for rows.Next() {
		var file models.FileEntity
		file.File = &models.File{}
		var tagsJSON []byte

		err := rows.Scan(
			&file.ID, &file.Title, &file.Description, &file.Type, &tagsJSON,
			&file.Path, &file.OriginalPath, &file.Ext, &file.MIMEType,
			&file.LinkedEntityType, &file.LinkedEntityID, &file.LinkedEntityMeta,
			&file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan file")
		}

		if len(tagsJSON) > 0 {
			err = json.Unmarshal(tagsJSON, &file.Tags)
			if err != nil {
				return nil, errkit.Wrap(err, "failed to unmarshal tags")
			}
		}

		files = append(files, &file)
	}

	return files, nil
}

func (r *FileRegistry) Search(ctx context.Context, query string, fileType *models.FileType, tags []string) ([]*models.FileEntity, error) {
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
		argIndex += 4
	}
	_ = argIndex // unused for now

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	sqlQuery := fmt.Sprintf(`
		SELECT id, title, description, type, tags, path, original_path, ext, mime_type, linked_entity_type, linked_entity_id, linked_entity_meta, created_at, updated_at
		FROM files
		%s
		ORDER BY created_at DESC`, whereClause)

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to search files")
	}
	defer rows.Close()

	var files []*models.FileEntity
	for rows.Next() {
		var file models.FileEntity
		file.File = &models.File{}
		var tagsJSON []byte

		err := rows.Scan(
			&file.ID, &file.Title, &file.Description, &file.Type, &tagsJSON,
			&file.Path, &file.OriginalPath, &file.Ext, &file.MIMEType,
			&file.LinkedEntityType, &file.LinkedEntityID, &file.LinkedEntityMeta,
			&file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan file")
		}

		if len(tagsJSON) > 0 {
			err = json.Unmarshal(tagsJSON, &file.Tags)
			if err != nil {
				return nil, errkit.Wrap(err, "failed to unmarshal tags")
			}
		}

		files = append(files, &file)
	}

	return files, nil
}

func (r *FileRegistry) ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType) ([]*models.FileEntity, int, error) {
	// First get the total count
	var countQuery string
	var countArgs []any

	if fileType != nil {
		countQuery = `SELECT COUNT(*) FROM files WHERE type = $1`
		countArgs = []any{*fileType}
	} else {
		countQuery = `SELECT COUNT(*) FROM files`
	}

	var total int
	err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, errkit.Wrap(err, "failed to count files")
	}

	// Then get the paginated results
	var dataQuery string
	var dataArgs []any

	if fileType != nil {
		dataQuery = `
			SELECT id, title, description, type, tags, path, original_path, ext, mime_type, linked_entity_type, linked_entity_id, linked_entity_meta, created_at, updated_at
			FROM files
			WHERE type = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3`
		dataArgs = []any{*fileType, limit, offset}
	} else {
		dataQuery = `
			SELECT id, title, description, type, tags, path, original_path, ext, mime_type, linked_entity_type, linked_entity_id, linked_entity_meta, created_at, updated_at
			FROM files
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2`
		dataArgs = []any{limit, offset}
	}

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, errkit.Wrap(err, "failed to list paginated files")
	}
	defer rows.Close()

	var files []*models.FileEntity
	for rows.Next() {
		var file models.FileEntity
		file.File = &models.File{}
		var tagsJSON []byte

		err := rows.Scan(
			&file.ID, &file.Title, &file.Description, &file.Type, &tagsJSON,
			&file.Path, &file.OriginalPath, &file.Ext, &file.MIMEType,
			&file.LinkedEntityType, &file.LinkedEntityID, &file.LinkedEntityMeta,
			&file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, 0, errkit.Wrap(err, "failed to scan file")
		}

		if len(tagsJSON) > 0 {
			err = json.Unmarshal(tagsJSON, &file.Tags)
			if err != nil {
				return nil, 0, errkit.Wrap(err, "failed to unmarshal tags")
			}
		}

		files = append(files, &file)
	}

	return files, total, nil
}
