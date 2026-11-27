package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// fileLinkColumns is the column list of an INSERT into file_links.
const fileLinkColumns = "id, uuid, tenant_id, group_id, created_by_user_id, file_id, entity_type, entity_id, meta, created_at"

// loadLinks fills the Links of every file in one query. It runs in the
// caller's transaction, so the links are read under the same RLS context as
// the files.
func (r *FileRegistry) loadLinks(ctx context.Context, tx *sqlx.Tx, files []*models.FileEntity) error {
	if len(files) == 0 {
		return nil
	}
	ids := make([]string, 0, len(files))
	byID := make(map[string][]*models.FileEntity, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		f.Links = []models.FileLink{}
		if _, seen := byID[f.ID]; !seen {
			ids = append(ids, f.ID)
		}
		byID[f.ID] = append(byID[f.ID], f)
	}

	query := fmt.Sprintf(`SELECT * FROM %s WHERE file_id = ANY($1) ORDER BY created_at, id`, r.tableNames.FileLinks())
	rows, err := tx.QueryxContext(ctx, query, ids)
	if err != nil {
		return errxtrace.Wrap("failed to load file links", err)
	}
	defer rows.Close()
	for rows.Next() {
		var link models.FileLink
		if err := rows.StructScan(&link); err != nil {
			return errxtrace.Wrap("failed to scan file link", err)
		}
		for _, f := range byID[link.FileID] {
			f.Links = append(f.Links, link)
		}
	}
	return rows.Err()
}

// queryFiles runs a SELECT of file rows and loads their links, in tx.
func (r *FileRegistry) queryFiles(ctx context.Context, tx *sqlx.Tx, query string, args ...any) ([]*models.FileEntity, error) {
	rows, err := tx.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var files []*models.FileEntity
	for rows.Next() {
		var file models.FileEntity
		if err := rows.StructScan(&file); err != nil {
			rows.Close()
			return nil, errxtrace.Wrap("failed to scan file", err)
		}
		files = append(files, &file)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if err := r.loadLinks(ctx, tx, files); err != nil {
		return nil, err
	}
	return files, nil
}

// insertLink writes one link of file. The link takes the file's tenant and
// group, so it is visible wherever the file is. created is false when the file
// already had a link to that entity; the stored link is returned either way.
func (r *FileRegistry) insertLink(ctx context.Context, tx *sqlx.Tx, file *models.FileEntity, link models.FileLink) (*models.FileLink, bool, error) {
	createdBy := r.createdByUserID
	if createdBy == "" {
		createdBy = file.CreatedByUserID
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now()
	}
	insert := fmt.Sprintf(`
		INSERT INTO %s (%s)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (file_id, entity_type, entity_id) DO NOTHING
		RETURNING *`, r.tableNames.FileLinks(), fileLinkColumns)

	var stored models.FileLink
	err := tx.QueryRowxContext(ctx, insert,
		uuid.NewString(), uuid.NewString(), file.TenantID, file.GroupID, createdBy,
		file.ID, link.EntityType, link.EntityID, link.Meta, link.CreatedAt,
	).StructScan(&stored)
	if err == nil {
		return &stored, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, errxtrace.Wrap("failed to insert file link", err)
	}

	existing := fmt.Sprintf(`SELECT * FROM %s WHERE file_id = $1 AND entity_type = $2 AND entity_id = $3`, r.tableNames.FileLinks())
	if err := tx.QueryRowxContext(ctx, existing, file.ID, link.EntityType, link.EntityID).StructScan(&stored); err != nil {
		return nil, false, errxtrace.Wrap("failed to read existing file link", err)
	}
	return &stored, false, nil
}

// lockFileForShare reads the file row and holds a share lock on it until the
// transaction ends, so DeleteIfUnlinked cannot delete the file while a link to
// it is being written.
func (r *FileRegistry) lockFileForShare(ctx context.Context, tx *sqlx.Tx, fileID string) (*models.FileEntity, error) {
	var file models.FileEntity
	query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR SHARE`, r.tableNames.Files())
	if err := tx.QueryRowxContext(ctx, query, fileID).StructScan(&file); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", fileID))
		}
		return nil, errxtrace.Wrap("failed to read file", err)
	}
	return &file, nil
}

// AddLink attaches a live file to an entity; see the interface doc.
func (r *FileRegistry) AddLink(ctx context.Context, link models.FileLink) (*models.FileLink, bool, error) {
	return r.addLink(ctx, link, isLiveFile)
}

// AddLinkIncludingTrashed attaches a file in either state; see the interface
// doc.
func (r *FileRegistry) AddLinkIncludingTrashed(ctx context.Context, link models.FileLink) (*models.FileLink, bool, error) {
	return r.addLink(ctx, link, anyFileState)
}

func isLiveFile(f *models.FileEntity) bool { return !f.IsTrashed() }

func anyFileState(*models.FileEntity) bool { return true }

// addLink writes the link when inState holds for the file. The file is read
// under the share lock, which waits for a concurrent Trash to commit, so the
// state it checks is the one the link is written against.
func (r *FileRegistry) addLink(ctx context.Context, link models.FileLink, inState func(*models.FileEntity) bool) (*models.FileLink, bool, error) {
	if link.FileID == "" {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "FileID"))
	}
	var (
		stored  *models.FileLink
		created bool
	)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		file, err := r.lockFileForShare(ctx, tx, link.FileID)
		if err != nil {
			return err
		}
		if !inState(file) {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", link.FileID))
		}
		stored, created, err = r.insertLink(ctx, tx, file, link)
		return err
	})
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to add file link", err)
	}
	return stored, created, nil
}

// RemoveLink removes one link of a file; see the interface doc.
func (r *FileRegistry) RemoveLink(ctx context.Context, fileID, linkID string) (*models.FileLink, error) {
	var removed models.FileLink
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1 AND file_id = $2 RETURNING *`, r.tableNames.FileLinks())
		err := tx.QueryRowxContext(ctx, query, linkID, fileID).StructScan(&removed)
		if errors.Is(err, sql.ErrNoRows) {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file_link", "entity_id", linkID))
		}
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to remove file link", err)
	}
	return &removed, nil
}

// RemoveLinksToEntity removes every link to an entity; see the interface doc.
func (r *FileRegistry) RemoveLinksToEntity(ctx context.Context, entityType, entityID string) ([]string, error) {
	var fileIDs []string
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE entity_type = $1 AND entity_id = $2 RETURNING file_id`, r.tableNames.FileLinks())
		rows, err := tx.QueryxContext(ctx, query, entityType, entityID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			fileIDs = append(fileIDs, id)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to remove links to entity", err)
	}
	return fileIDs, nil
}

// DeleteIfUnlinked deletes the file row when no link points at it; see the
// interface doc. The row is locked before the link check, and the check runs
// as its own statement, so it sees every link committed before the lock was
// granted: a concurrent AddLink either finishes first and keeps the file, or
// waits and then finds the file gone.
func (r *FileRegistry) DeleteIfUnlinked(ctx context.Context, id, ignoreEntityType, ignoreEntityID string) (bool, error) {
	var deleted bool
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		lock := fmt.Sprintf(`SELECT id FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.Files())
		var locked string
		if err := tx.QueryRowxContext(ctx, lock, id).Scan(&locked); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
			}
			return err
		}

		var linked bool
		exists := fmt.Sprintf(`
			SELECT EXISTS (
				SELECT 1 FROM %s
				WHERE file_id = $1 AND NOT (entity_type = $2 AND entity_id = $3)
			)`, r.tableNames.FileLinks())
		if err := tx.QueryRowxContext(ctx, exists, id, ignoreEntityType, ignoreEntityID).Scan(&linked); err != nil {
			return err
		}
		if linked {
			return nil
		}

		del := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, r.tableNames.Files())
		if _, err := tx.ExecContext(ctx, del, id); err != nil {
			return err
		}
		deleted = true
		return nil
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to delete unlinked file", err)
	}
	return deleted, nil
}

// ListByChecksum returns the files with the given SHA-256; see the interface
// doc.
func (r *FileRegistry) ListByChecksum(ctx context.Context, sha256 string) ([]*models.FileEntity, error) {
	if sha256 == "" {
		return nil, nil
	}
	var files []*models.FileEntity
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE sha256 = $1 AND %s ORDER BY created_at DESC, id DESC`, r.tableNames.Files(), liveRow)
		var err error
		files, err = r.queryFiles(ctx, tx, query, sha256)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files by checksum", err)
	}
	return files, nil
}

// ListPendingChecksum returns files that still need a checksum; see the
// interface doc.
func (r *FileRegistry) ListPendingChecksum(ctx context.Context, afterID string, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	var files []*models.FileEntity
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE sha256 = '' AND original_path <> '' AND id > $1
			ORDER BY id
			LIMIT $2`, r.tableNames.Files())
		rows, err := tx.QueryxContext(ctx, query, afterID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var file models.FileEntity
			if err := rows.StructScan(&file); err != nil {
				return errxtrace.Wrap("failed to scan file", err)
			}
			files = append(files, &file)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files pending checksum", err)
	}
	return files, nil
}

// SetChecksum stores a file's SHA-256 when it has none; see the interface doc.
func (r *FileRegistry) SetChecksum(ctx context.Context, id, sha256 string) error {
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`UPDATE %s SET sha256 = $1 WHERE id = $2 AND sha256 = ''`, r.tableNames.Files())
		_, err := tx.ExecContext(ctx, query, sha256, id)
		return err
	})
	if err != nil {
		return errxtrace.Wrap("failed to set file checksum", err)
	}
	return nil
}

// ListPendingThumbnailOrientation returns JPEG files whose thumbnails are not
// marked oriented; see the interface doc.
func (r *FileRegistry) ListPendingThumbnailOrientation(ctx context.Context, afterID string, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	var files []*models.FileEntity
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE NOT thumbnails_oriented AND mime_type = 'image/jpeg' AND original_path <> '' AND id > $1
			ORDER BY id
			LIMIT $2`, r.tableNames.Files())
		rows, err := tx.QueryxContext(ctx, query, afterID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var file models.FileEntity
			if err := rows.StructScan(&file); err != nil {
				return errxtrace.Wrap("failed to scan file", err)
			}
			files = append(files, &file)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files pending thumbnail orientation", err)
	}
	return files, nil
}

// SetThumbnailsOriented marks a file's thumbnails oriented; see the interface
// doc.
func (r *FileRegistry) SetThumbnailsOriented(ctx context.Context, id string) error {
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`UPDATE %s SET thumbnails_oriented = TRUE WHERE id = $1 AND NOT thumbnails_oriented`, r.tableNames.Files())
		_, err := tx.ExecContext(ctx, query, id)
		return err
	})
	if err != nil {
		return errxtrace.Wrap("failed to mark thumbnails oriented", err)
	}
	return nil
}

// MoveLegacyLinks moves single links from the legacy columns into file_links;
// see the interface doc. One statement per batch: the rows are locked with
// SKIP LOCKED so two instances starting together split the work instead of
// waiting on each other, each moved link is inserted with ON CONFLICT DO
// NOTHING, and the legacy columns of the same rows are emptied in the same
// statement. PostgreSQL runs every data-modifying CTE to completion whether
// or not the outer query reads it.
func (r *FileRegistry) MoveLegacyLinks(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	var moved int
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			WITH batch AS (
				SELECT id, tenant_id, group_id, created_by_user_id,
					linked_entity_type, linked_entity_id, COALESCE(linked_entity_meta, '') AS linked_entity_meta,
					created_at
				FROM %[1]s
				WHERE COALESCE(linked_entity_type, '') <> ''
				ORDER BY id
				LIMIT $1
				FOR UPDATE SKIP LOCKED
			),
			inserted AS (
				INSERT INTO %[2]s (%[3]s)
				SELECT gen_random_uuid()::text, gen_random_uuid()::text, tenant_id, group_id, created_by_user_id,
					id, linked_entity_type, linked_entity_id, linked_entity_meta, created_at
				FROM batch
				WHERE COALESCE(linked_entity_id, '') <> ''
				ON CONFLICT (file_id, entity_type, entity_id) DO NOTHING
			),
			cleared AS (
				UPDATE %[1]s f
				SET linked_entity_type = '', linked_entity_id = '', linked_entity_meta = ''
				FROM batch
				WHERE f.id = batch.id
				RETURNING 1
			)
			SELECT COUNT(*) FROM cleared`,
			r.tableNames.Files(), r.tableNames.FileLinks(), fileLinkColumns)
		return tx.QueryRowxContext(ctx, query, limit).Scan(&moved)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to move legacy file links", err)
	}
	return moved, nil
}

// insertLinksForNewFile writes the links a file is created with.
func (r *FileRegistry) insertLinksForNewFile(ctx context.Context, tx *sqlx.Tx, file *models.FileEntity, links []models.FileLink) error {
	file.Links = make([]models.FileLink, 0, len(links))
	for _, link := range links {
		stored, _, err := r.insertLink(ctx, tx, file, link)
		if err != nil {
			return err
		}
		if file.LinkTo(stored.EntityType, stored.EntityID) == nil {
			file.Links = append(file.Links, *stored)
		}
	}
	return nil
}

// createFile inserts the file row and its links in one transaction. The
// identifiers are set the way the group store sets them for every other
// group-scoped row.
func (r *FileRegistry) createFile(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	if !r.service {
		if r.tenantID == "" {
			return nil, store.ErrTenantIDRequired
		}
		if r.groupID == "" {
			return nil, store.ErrGroupIDRequired
		}
		if r.createdByUserID == "" {
			return nil, store.ErrCreatedByUserIDRequired
		}
	}

	links := file.Links
	file.Links = nil
	file.ClearTrashState()
	file.SetID(uuid.NewString())
	file.SetUUID(uuid.NewString())
	if !r.service {
		file.SetTenantID(r.tenantID)
		file.SetGroupID(r.groupID)
		file.SetCreatedByUserID(r.createdByUserID)
	}

	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Same orphan-prevention as in CommodityRegistry.Create — see
		// ensureTagRowsInTx in tags.go for the cross-tx invariant.
		if err := ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, models.TagKindFile, []string(file.Tags)); err != nil {
			return err
		}
		if err := store.NewTxRegistry[models.FileEntity](tx, r.tableNames.Files()).Insert(ctx, file); err != nil {
			return err
		}
		return r.insertLinksForNewFile(ctx, tx, &file, links)
	})
	if err != nil {
		return nil, err
	}
	return &file, nil
}
