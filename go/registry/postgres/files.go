package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	sqlb "github.com/huandu/go-sqlbuilder"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
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

// List returns every live file.
func (r *FileRegistry) List(ctx context.Context) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE %s`, r.tableNames.Files(), liveRow)
		var err error
		files, err = r.queryFiles(ctx, tx, query)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files", err)
	}
	return files, nil
}

// Count returns the number of live files.
func (r *FileRegistry) Count(ctx context.Context) (int, error) {
	var cnt int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var err error
		cnt, err = countRows(ctx, tx, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, r.tableNames.Files(), liveRow))
		return err
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count files", err)
	}

	return cnt, nil
}

// Create inserts the file and the links it carries in one transaction. The
// registry sets the ids, tenant, group and creator of the file and of every
// link.
func (r *FileRegistry) Create(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	createdFile, err := r.createFile(ctx, file)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file", err)
	}
	return createdFile, nil
}

// Update writes the file's own columns. It leaves the links alone, keeps the
// stored checksum when file carries none, and never writes the legacy link
// columns: those belong to MoveLegacyLinks, and the row is locked before they
// are read so a concurrent move cannot be undone.
func (r *FileRegistry) Update(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	reg := r.newSQLRegistry()

	err := reg.UpdateWithPrepare(ctx, file, func(ctx context.Context, tx *sqlx.Tx, stored models.FileEntity, next *models.FileEntity) error {
		// The tag locks come before the file's row lock, the order a tag
		// rename or delete takes them in (see ensureTagRowsInTx).
		if err := ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, models.TagKindFile, []string(next.Tags)); err != nil {
			return err
		}
		var current struct {
			LegacyType string `db:"linked_entity_type"`
			LegacyID   string `db:"linked_entity_id"`
			LegacyMeta string `db:"linked_entity_meta"`
			SHA256     string `db:"sha256"`
			Trashed    bool   `db:"trashed"`
		}
		lock := fmt.Sprintf(`
			SELECT COALESCE(linked_entity_type, '') AS linked_entity_type,
				COALESCE(linked_entity_id, '') AS linked_entity_id,
				COALESCE(linked_entity_meta, '') AS linked_entity_meta,
				sha256,
				deleted_at IS NOT NULL AS trashed
			FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.Files())
		if err := tx.QueryRowxContext(ctx, lock, next.ID).StructScan(&current); err != nil {
			return errxtrace.Wrap("failed to lock file", err)
		}
		// A trashed file is read-only until it is restored. stored was read
		// without a lock, so the check is made under this one. The trash
		// columns themselves belong to Trash and Restore.
		if current.Trashed {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", stored.ID))
		}
		next.ClearTrashState()
		next.LegacyFileLink = models.LegacyFileLink{
			LegacyLinkedEntityType: current.LegacyType,
			LegacyLinkedEntityID:   current.LegacyID,
			LegacyLinkedEntityMeta: current.LegacyMeta,
		}
		if next.File != nil && next.SHA256 == "" {
			next.SHA256 = current.SHA256
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to update file", err)
	}

	updated, err := r.get(ctx, file.ID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to read updated file", err)
	}
	return updated, nil
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

// get returns a live file with its links.
func (r *FileRegistry) get(ctx context.Context, id string) (*models.FileEntity, error) {
	return r.getInState(ctx, id, liveRow)
}

// getInState returns a file with its links when its row matches state, which
// is liveRow or trashedRow.
func (r *FileRegistry) getInState(ctx context.Context, id, state string) (*models.FileEntity, error) {
	var files []*models.FileEntity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 AND %s LIMIT 1`, r.tableNames.Files(), state)
		var err error
		files, err = r.queryFiles(ctx, tx, query, id)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to get file", err)
	}
	if len(files) == 0 {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
	}
	return files[0], nil
}

// ListByType returns the live files of one type.
func (r *FileRegistry) ListByType(ctx context.Context, fileType models.FileType) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE type = $1 AND %s`, r.tableNames.Files(), liveRow)
		var err error
		files, err = r.queryFiles(ctx, tx, query, fileType)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files by type", err)
	}
	return files, nil
}

func (r *FileRegistry) ListByLinkedEntity(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT f.* FROM %s f
			WHERE f.deleted_at IS NULL AND EXISTS (
				SELECT 1 FROM %s l
				WHERE l.file_id = f.id AND l.entity_type = $1 AND l.entity_id = $2
			)
			ORDER BY f.created_at DESC, f.id DESC`, r.tableNames.Files(), r.tableNames.FileLinks())
		var err error
		files, err = r.queryFiles(ctx, tx, query, entityType, entityID)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files by linked entity", err)
	}

	return files, nil
}

// CountLinkedTo returns how many live files are linked to each entity.
func (r *FileRegistry) CountLinkedTo(ctx context.Context, entityType string, entityIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(entityIDs))
	if len(entityIDs) == 0 {
		return out, nil
	}
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT l.entity_id, COUNT(*)
			FROM %s l
			JOIN %s f ON f.id = l.file_id
			WHERE f.deleted_at IS NULL AND l.entity_type = $1 AND l.entity_id = ANY($2)
			GROUP BY l.entity_id`, r.tableNames.FileLinks(), r.tableNames.Files())
		rows, err := tx.QueryxContext(ctx, query, entityType, entityIDs)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var n int
			if err := rows.Scan(&id, &n); err != nil {
				return err
			}
			out[id] = n
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count files by linked entity", err)
	}
	return out, nil
}

// ListByGroup returns every file belonging to the given (tenant_id, group_id)
// tuple, trashed files included. Used by GroupPurgeService to find physical blobs to delete before
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

		var err error
		files, err = r.queryFiles(ctx, tx, query, tenantID, groupID)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files by group", err)
	}
	return files, nil
}

// ListByTenant returns every file of the tenant, trashed files included.
func (r *FileRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.FileEntity, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	var files []*models.FileEntity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE tenant_id = $1 ORDER BY created_at DESC`, r.tableNames.Files())
		var err error
		files, err = r.queryFiles(ctx, tx, query, tenantID)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files by tenant", err)
	}
	return files, nil
}

func (r *FileRegistry) ListByLinkedEntityAndMeta(ctx context.Context, entityType, entityID, meta string) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT f.* FROM %s f
			WHERE f.deleted_at IS NULL AND EXISTS (
				SELECT 1 FROM %s l
				WHERE l.file_id = f.id AND l.entity_type = $1 AND l.entity_id = $2 AND l.meta = $3
			)
			ORDER BY f.created_at DESC, f.id DESC`, r.tableNames.Files(), r.tableNames.FileLinks())
		var err error
		files, err = r.queryFiles(ctx, tx, query, entityType, entityID, meta)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files by linked entity and meta", err)
	}

	return files, nil
}

// GetMany returns the live files among ids, with their links, in one read.
func (r *FileRegistry) GetMany(ctx context.Context, ids []string) ([]*models.FileEntity, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var files []*models.FileEntity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE id = ANY($1) AND %s`, r.tableNames.Files(), liveRow)
		var err error
		files, err = r.queryFiles(ctx, tx, query, ids)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to get files", err)
	}
	return files, nil
}

// CommodityIDsWithPhotos returns the items of the group that have a photo
// in the sense of FirstPhotos. Row-level security keeps it to the group.
func (r *FileRegistry) CommodityIDsWithPhotos(ctx context.Context) ([]string, error) {
	var ids []string
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT DISTINCT l.entity_id
			FROM %s l
			JOIN %s f ON f.id = l.file_id
			WHERE l.entity_type = $1 AND l.meta = $2
				AND f.deleted_at IS NULL AND f.type = $3 AND f.category = $4`,
			r.tableNames.FileLinks(), r.tableNames.Files())
		return tx.SelectContext(ctx, &ids, query,
			models.FileLinkCommodity, models.FileLinkMetaImages, models.FileTypeImage, models.FileCategoryImages)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list items with photos", err)
	}
	return ids, nil
}

// FirstPhotos returns the oldest photo of each item among commodityIDs that
// is not in Trash. DISTINCT ON keeps the first row per item in the ORDER BY, so the photo
// filter runs before the pick and a non-photo cannot shadow a photo.
func (r *FileRegistry) FirstPhotos(ctx context.Context, commodityIDs []string) (map[string]*models.FileEntity, error) {
	out := make(map[string]*models.FileEntity, len(commodityIDs))
	if len(commodityIDs) == 0 {
		return out, nil
	}
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		pickQuery := fmt.Sprintf(`
			SELECT DISTINCT ON (l.entity_id) l.entity_id, f.id
			FROM %s l
			JOIN %s f ON f.id = l.file_id
			WHERE l.entity_type = $1 AND l.meta = $2 AND l.entity_id = ANY($3)
				AND f.deleted_at IS NULL AND f.type = $4 AND f.category = $5
				AND NOT EXISTS (
					SELECT 1 FROM %s c WHERE c.id = l.entity_id AND c.deleted_at IS NOT NULL
				)
			ORDER BY l.entity_id, f.created_at, f.id`,
			r.tableNames.FileLinks(), r.tableNames.Files(), r.tableNames.Commodities())
		rows, err := tx.QueryxContext(ctx, pickQuery,
			models.FileLinkCommodity, models.FileLinkMetaImages, commodityIDs, models.FileTypeImage, models.FileCategoryImages)
		if err != nil {
			return err
		}
		picked := make(map[string]string, len(commodityIDs))
		fileIDs := make([]string, 0, len(commodityIDs))
		for rows.Next() {
			var commodityID, fileID string
			if err := rows.Scan(&commodityID, &fileID); err != nil {
				rows.Close()
				return err
			}
			picked[commodityID] = fileID
			fileIDs = append(fileIDs, fileID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(fileIDs) == 0 {
			return nil
		}

		files, err := r.queryFiles(ctx, tx, fmt.Sprintf(`SELECT * FROM %s WHERE id = ANY($1)`, r.tableNames.Files()), fileIDs)
		if err != nil {
			return err
		}
		byID := make(map[string]*models.FileEntity, len(files))
		for _, f := range files {
			byID[f.ID] = f
		}
		for commodityID, fileID := range picked {
			if f, ok := byID[fileID]; ok {
				out[commodityID] = f
			}
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to pick first photos", err)
	}
	return out, nil
}

// fileWhere assembles the WHERE clause shared by Search, ListPaginated, and
// CountByCategory. It always keeps trashed files out, so it is never empty. The
// outer query selects from the files table without an alias; the linked-entity
// filter refers to it by name.
func fileWhere(tables store.TableNames, query string, fileType *models.FileType, fileCategory *models.FileCategory, tags []string, linkedEntityType, linkedEntityID *string) *sqlb.WhereClause {
	cond := sqlb.NewCond()
	var exprs []string
	add := func(expr string) {
		exprs = append(exprs, expr)
	}

	add(liveRow)

	if fileType != nil {
		add(cond.Equal("type", *fileType))
	}
	if fileCategory != nil {
		add(cond.Equal("category", *fileCategory))
	}

	// Linked-entity filter: both type+id must be supplied together or both
	// nil. Mismatched callers (one nil, one not) get no filter — the
	// interface contract documents this; mirroring the memory path.
	if linkedEntityType != nil && linkedEntityID != nil {
		add(fmt.Sprintf("EXISTS (SELECT 1 FROM %s fl WHERE fl.file_id = %s.id AND fl.entity_type = %s AND fl.entity_id = %s)",
			tables.FileLinks(), tables.Files(), cond.Args.Add(*linkedEntityType), cond.Args.Add(*linkedEntityID)))
	}

	if len(tags) > 0 {
		tagsJSON, _ := json.Marshal(tags)
		// Containment, so every requested tag has to be present.
		add(fmt.Sprintf("tags @> %s", cond.Args.Add(tagsJSON)))
	}

	if query != "" {
		pattern := "%" + query + "%"
		add(cond.Or(
			cond.ILike("title", pattern),
			cond.ILike("description", pattern),
			cond.ILike("path", pattern),
			cond.ILike("original_path", pattern),
		))
	}

	return andWhere(cond, exprs...)
}

// orderFilesNewestFirst is the one order of every file list: newest first,
// then by id. Rows created in the same instant (a bulk upload, a restore)
// share created_at; without the id tiebreaker postgres may return them in a
// different order on each query, and a file can repeat on two pages or be
// skipped when the client walks the list page by page.
func orderFilesNewestFirst(sb *sqlb.SelectBuilder) *sqlb.SelectBuilder {
	return sb.OrderByDesc("created_at").OrderByDesc("id")
}

func (r *FileRegistry) Search(ctx context.Context, query string, fileType *models.FileType, fileCategory *models.FileCategory, tags []string, linkedEntityType, linkedEntityID *string) ([]*models.FileEntity, error) {
	var files []*models.FileEntity

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		sb := orderFilesNewestFirst(newSelect().
			Select("*").
			From(string(r.tableNames.Files())))
		addWhere(sb, fileWhere(r.tableNames, query, fileType, fileCategory, tags, linkedEntityType, linkedEntityID))
		sqlQuery, args := sb.Build()

		var err error
		files, err = r.queryFiles(ctx, tx, sqlQuery, args...)
		if err != nil {
			return errxtrace.Wrap("failed to search files", err)
		}
		return nil
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
		where := fileWhere(r.tableNames, "", fileType, fileCategory, nil, linkedEntityType, linkedEntityID)

		countSB := newSelect().Select("COUNT(*)").From(string(r.tableNames.Files()))
		countQuery, countArgs := addWhere(countSB, where).Build()
		err := tx.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
		if err != nil {
			return errxtrace.Wrap("failed to count files", err)
		}

		dataSB := orderFilesNewestFirst(newSelect().
			Select("*").
			From(string(r.tableNames.Files())))
		addWhere(dataSB, where)
		dataQuery, dataArgs := paginate(dataSB, offset, limit).Build()

		files, err = r.queryFiles(ctx, tx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list paginated files", err)
		}
		return nil
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
func (r *FileRegistry) CountByCategory(ctx context.Context, query string, fileType *models.FileType, tags []string, linkedEntityType, linkedEntityID *string) (map[models.FileCategory]int, map[models.FileCategory]int64, error) {
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
		// COALESCE keeps NULL sums (an entirely empty bucket on this
		// scoped query) from blowing up the int64 scan.
		sb := newSelect().
			Select("category", "COUNT(*)", "COALESCE(SUM(size_bytes), 0)").
			From(string(r.tableNames.Files())).
			GroupBy("category")
		addWhere(sb, fileWhere(r.tableNames, query, fileType, nil, tags, linkedEntityType, linkedEntityID))
		sqlQuery, args := sb.Build()

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

// ListOrphanCandidates returns up to limit file rows whose every link points
// at an entity that no longer exists (#2237). Discovery half of the orphan-file
// GC; see the interface doc in registry.FileRegistry for the full contract.
//
// The candidate predicate is a positive ALLOWLIST on the link types, never a
// negation:
//
//   - no link   → a STANDALONE file. First-class since #2235 (exported in
//     backups, swept as inventory by a full_replace restore). NOT an orphan,
//     never a candidate.
//   - an 'export' link → owned by the backup subsystem's own lifecycle
//     (DeleteExportWithFile / group purge). Never a candidate — and because
//     the exports table is never probed at all, the `deleted_at IS NULL`
//     filter on ExportRegistry.Get can never be mistaken for "gone".
//   - any other type (including a future link type the model validator does
//     not yet know about — the registries do NOT run ValidateWithContext, so
//     the DB is a superset of the validator's enumeration) → kept, fails
//     closed.
//
// The existence probes run under whatever role reg.Do selects. This method is
// SERVICE-MODE ONLY on purpose: a link can point at an entity in another group,
// and a group-scoped probe would read a LIVE entity as missing.
//
// The scan is KEYSET-PAGINATED on (created_at, id): most re-verified candidates
// are KEPT rather than deleted, and several keep-reasons never clear (a tenant
// pinned by a crashed restore, a suspended tenant, a purged owner), so a
// non-resumable oldest-first window would be squatted on by exactly those rows
// forever and no other orphan in the installation would ever be enumerated.
//
// The per-file link probes use idx_file_links_file_entity.
func (r *FileRegistry) ListOrphanCandidates(ctx context.Context, olderThan time.Time, after registry.OrphanCandidateCursor, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}

	query, args := orphanCandidatesQuery(r.tableNames, olderThan, after, limit)

	var files []*models.FileEntity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var err error
		files, err = r.queryFiles(ctx, tx, query, args...)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list orphan file candidates", err)
	}
	return files, nil
}

// orphanCandidatesQuery builds one keyset page of ListOrphanCandidates: files
// that have links, all of them to a commodity, area or location that no longer
// exists, oldest first, resuming after the cursor when it is set.
//
// The second NOT EXISTS reads "no link of this file keeps it": a link keeps
// the file when its type is outside the allowlist, its entity id is empty, it
// is younger than the cutoff, or its entity still exists.
func orphanCandidatesQuery(tables store.TableNames, olderThan time.Time, after registry.OrphanCandidateCursor, limit int) (string, []any) {
	alive := func(table, alias, linkType string) string {
		return fmt.Sprintf("(l.entity_type = '%s' AND EXISTS (SELECT 1 FROM %s %s WHERE %s.id = l.entity_id))", linkType, table, alias, alias)
	}

	sb := newSelect().Select("f.*").From(string(tables.Files()) + " f")
	sb.Where(
		// A trashed file belongs to the Trash purge, not to this GC.
		"f.deleted_at IS NULL",
		sb.LessThan("f.created_at", olderThan),
		sb.LessThan("f.updated_at", olderThan),
	)
	if !after.IsZero() {
		sb.Where(sb.GreaterThan(sqlb.TupleNames("f.created_at", "f.id"), sqlb.Tuple(after.CreatedAt, after.ID)))
	}

	anyLink := newSelect().Select("1").From(string(tables.FileLinks()) + " l").Where("l.file_id = f.id")
	keeper := newSelect().Select("1").From(string(tables.FileLinks()) + " l")
	keeper.Where(
		"l.file_id = f.id",
		keeper.Or(
			"l.entity_type NOT IN ('commodity', 'area', 'location')",
			"l.entity_id = ''",
			keeper.GreaterEqualThan("l.created_at", olderThan),
			alive(string(tables.Commodities()), "c", "commodity"),
			alive(string(tables.Areas()), "a", "area"),
			alive(string(tables.Locations()), "lo", "location"),
		),
	)
	sb.Where(sb.Exists(anyLink), sb.NotExists(keeper))
	return sb.OrderByAsc("f.created_at").OrderByAsc("f.id").Limit(limit).Build()
}

// ExistingIDs returns the subset of ids that still have a file row, in ONE
// `id = ANY($1)` round-trip (#2237). Service-mode only; backs the orphan-file
// GC's thumbnail sweep. Missing ids are simply absent from the result — see the
// interface doc in registry.FileRegistry for the full contract.
func (r *FileRegistry) ExistingIDs(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var existing []string
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT id FROM %s WHERE id = ANY($1)`, r.tableNames.Files())
		rows, qerr := tx.QueryxContext(ctx, query, ids)
		if qerr != nil {
			return errxtrace.Wrap("failed to batch-check file ids", qerr)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if scanErr := rows.Scan(&id); scanErr != nil {
				return errxtrace.Wrap("failed to scan file id", scanErr)
			}
			existing = append(existing, id)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to batch-check file ids", err)
	}
	return existing, nil
}

// ListIDsByOriginalPath returns the IDs of the file rows that reference the
// given blob key, across EVERY tenant and group (#2241). Service-mode only.
//
// Deliberately NOT tenant-scoped. Post-#1793 keys carry a `t/<tenant>/` prefix,
// but legacy flat keys (rows whose blob predates the backfill) do not, so two
// rows in DIFFERENT tenants can reference one legacy key. A blob delete that
// scoped this lookup to the caller's own tenant would happily destroy the other
// tenant's bytes.
//
// An empty path returns no ids rather than matching every row whose blob is
// unset — a caller must never conclude "nobody else references this" from it.
func (r *FileRegistry) ListIDsByOriginalPath(ctx context.Context, originalPath string) ([]string, error) {
	if originalPath == "" {
		return nil, nil
	}
	var ids []string
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT id FROM %s WHERE original_path = $1`, r.tableNames.Files())
		rows, err := tx.QueryxContext(ctx, query, originalPath)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list file ids by original path", err)
	}
	return ids, nil
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
				COALESCE(SUM(CASE WHEN NOT is_trashed AND is_export THEN size_bytes ELSE 0 END), 0) AS exports,
				COALESCE(SUM(CASE WHEN NOT is_trashed AND NOT is_export AND category = 'images' THEN size_bytes ELSE 0 END), 0) AS images,
				COALESCE(SUM(CASE WHEN NOT is_trashed AND NOT is_export AND category = 'documents' THEN size_bytes ELSE 0 END), 0) AS documents,
				COALESCE(SUM(CASE WHEN NOT is_trashed AND NOT is_export AND category = 'other' THEN size_bytes ELSE 0 END), 0) AS other,
				COALESCE(SUM(CASE WHEN is_trashed THEN size_bytes ELSE 0 END), 0) AS trash
			FROM (
				SELECT f.size_bytes, f.category, f.tenant_id, f.group_id,
					f.deleted_at IS NOT NULL AS is_trashed,
					EXISTS (SELECT 1 FROM %s l WHERE l.file_id = f.id AND l.entity_type = 'export') AS is_export
				FROM %s f
			) sized
			WHERE tenant_id = $1 AND group_id = $2`, r.tableNames.FileLinks(), r.tableNames.Files())

		row := tx.QueryRowxContext(ctx, sqlQuery, tenantID, groupID)
		if err := row.Scan(&breakdown.Exports, &breakdown.Images, &breakdown.Documents, &breakdown.Other, &breakdown.Trash); err != nil {
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
// the SQL splits export bundles (files with an export link) out of
// FileCategoryOther so the FE can render a distinct "exports" row in
// the storage card without double-counting, and trashed files out of every
// other bucket into Trash. A file attached to several entities is one stored
// blob and counts once.
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
				COALESCE(SUM(CASE WHEN NOT is_trashed AND is_export THEN size_bytes ELSE 0 END), 0) AS exports,
				COALESCE(SUM(CASE WHEN NOT is_trashed AND NOT is_export AND category = 'images' THEN size_bytes ELSE 0 END), 0) AS images,
				COALESCE(SUM(CASE WHEN NOT is_trashed AND NOT is_export AND category = 'documents' THEN size_bytes ELSE 0 END), 0) AS documents,
				COALESCE(SUM(CASE WHEN NOT is_trashed AND NOT is_export AND category = 'other' THEN size_bytes ELSE 0 END), 0) AS other,
				COALESCE(SUM(CASE WHEN is_trashed THEN size_bytes ELSE 0 END), 0) AS trash
			FROM (
				SELECT f.size_bytes, f.category, f.tenant_id, f.group_id,
					f.deleted_at IS NOT NULL AS is_trashed,
					EXISTS (SELECT 1 FROM %s l WHERE l.file_id = f.id AND l.entity_type = 'export') AS is_export
				FROM %s f
			) sized`, r.tableNames.FileLinks(), r.tableNames.Files())

		row := tx.QueryRowxContext(ctx, sqlQuery)
		if err := row.Scan(&breakdown.Exports, &breakdown.Images, &breakdown.Documents, &breakdown.Other, &breakdown.Trash); err != nil {
			return errxtrace.Wrap("failed to scan storage breakdown", err)
		}
		return nil
	})
	if err != nil {
		return registry.StorageBreakdown{}, errxtrace.Wrap("failed to sum size breakdown", err)
	}

	return breakdown, nil
}

// GetTrashed returns a trashed file with its links.
func (r *FileRegistry) GetTrashed(ctx context.Context, id string) (*models.FileEntity, error) {
	return r.getInState(ctx, id, trashedRow)
}

// ListTrashed returns a page of the files deleted on their own, most recently
// deleted first.
func (r *FileRegistry) ListTrashed(ctx context.Context, offset, limit int) ([]*models.FileEntity, int, error) {
	var files []*models.FileEntity
	var total int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		where := fmt.Sprintf("%s AND deleted_with_commodity_id IS NULL", trashedRow)
		var err error
		total, err = countRows(ctx, tx, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, r.tableNames.Files(), where))
		if err != nil {
			return err
		}
		sb := newSelect().Select("*").From(string(r.tableNames.Files()))
		sb.Where(where).OrderByDesc("deleted_at").OrderByDesc("id")
		query, args := paginate(sb, offset, limit).Build()
		files, err = r.queryFiles(ctx, tx, query, args...)
		return err
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list trashed files", err)
	}
	return files, total, nil
}

// ListTrashedLinkedTo returns the trashed files linked to the entity.
func (r *FileRegistry) ListTrashedLinkedTo(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error) {
	var files []*models.FileEntity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT f.* FROM %s f
			WHERE f.deleted_at IS NOT NULL AND EXISTS (
				SELECT 1 FROM %s l
				WHERE l.file_id = f.id AND l.entity_type = $1 AND l.entity_id = $2
			)
			ORDER BY f.created_at DESC, f.id DESC`, r.tableNames.Files(), r.tableNames.FileLinks())
		var err error
		files, err = r.queryFiles(ctx, tx, query, entityType, entityID)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list trashed files by linked entity", err)
	}
	return files, nil
}

// CountTrashedWith counts the files that went to Trash with each item.
func (r *FileRegistry) CountTrashedWith(ctx context.Context, commodityIDs []string) (map[string]registry.TrashedFilesStats, error) {
	out := make(map[string]registry.TrashedFilesStats, len(commodityIDs))
	if len(commodityIDs) == 0 {
		return out, nil
	}
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT deleted_with_commodity_id, COUNT(*), COALESCE(SUM(size_bytes), 0)
			FROM %s
			WHERE %s AND deleted_with_commodity_id = ANY($1)
			GROUP BY deleted_with_commodity_id`, r.tableNames.Files(), trashedRow)
		rows, err := tx.QueryxContext(ctx, query, commodityIDs)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var stats registry.TrashedFilesStats
			if err := rows.Scan(&id, &stats.Count, &stats.Bytes); err != nil {
				return err
			}
			out[id] = stats
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count files trashed with items", err)
	}
	return out, nil
}

// ListTrashedBefore returns up to limit files deleted on their own before
// cutoff, oldest first.
func (r *FileRegistry) ListTrashedBefore(ctx context.Context, cutoff time.Time, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	var files []*models.FileEntity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT f.* FROM %s f
			WHERE f.deleted_at IS NOT NULL AND f.deleted_at < $1 AND f.deleted_with_commodity_id IS NULL
			ORDER BY f.deleted_at ASC, f.id ASC
			LIMIT $2`, r.tableNames.Files())
		var err error
		files, err = r.queryFiles(ctx, tx, query, cutoff.UTC(), limit)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list expired trashed files", err)
	}
	return files, nil
}

// ListStrandedTrashed returns up to limit trashed files whose item is no
// longer in Trash, oldest first.
func (r *FileRegistry) ListStrandedTrashed(ctx context.Context, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	var files []*models.FileEntity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT f.* FROM %s f
			WHERE f.deleted_at IS NOT NULL AND f.deleted_with_commodity_id IS NOT NULL
				AND NOT EXISTS (
					SELECT 1 FROM %s c
					WHERE c.id = f.deleted_with_commodity_id AND c.deleted_at IS NOT NULL)
			ORDER BY f.deleted_at ASC, f.id ASC
			LIMIT $1`, r.tableNames.Files(), r.tableNames.Commodities())
		var err error
		files, err = r.queryFiles(ctx, tx, query, limit)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list stranded trashed files", err)
	}
	return files, nil
}

// Trash moves a live file to Trash in one conditional UPDATE.
func (r *FileRegistry) Trash(ctx context.Context, id, deletedBy, withCommodityID string, at time.Time) error {
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET deleted_at = $2, deleted_by = $3, deleted_with_commodity_id = NULLIF($4, '')
			 WHERE id = $1 AND %s`, r.tableNames.Files(), liveRow)
		changed, err := execAffected(ctx, tx, query, id, at.UTC(), deletedBy, withCommodityID)
		if err != nil {
			return err
		}
		if !changed {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
		}
		return nil
	})
	if err != nil {
		return errxtrace.Wrap("failed to trash file", err, errx.Attrs("file_id", id))
	}
	return nil
}

// TrashUnlessHeld moves a live file to Trash unless another link holds it
// live; see the interface doc. Like DeleteIfUnlinked it locks the row first
// and checks the links in a statement of its own, so the check sees every
// link committed before the lock was granted.
func (r *FileRegistry) TrashUnlessHeld(ctx context.Context, id, entityType, entityID, deletedBy string, at time.Time) (bool, error) {
	var trashed bool
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		lock := fmt.Sprintf(`SELECT id FROM %s WHERE id = $1 AND %s FOR UPDATE`, r.tableNames.Files(), liveRow)
		var locked string
		if err := tx.QueryRowxContext(ctx, lock, id).Scan(&locked); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil // gone or in Trash already
			}
			return err
		}

		// A link holds the file unless it is the one being dropped or a
		// link to an item in Trash. A link to an item the group cannot see
		// holds it too: it is not known to be in Trash.
		var held bool
		heldQuery := fmt.Sprintf(`
			SELECT EXISTS (
				SELECT 1 FROM %[1]s l
				WHERE l.file_id = $1 AND NOT (l.entity_type = $2 AND l.entity_id = $3)
					AND NOT (l.entity_type = $4 AND EXISTS (
						SELECT 1 FROM %[2]s c WHERE c.id = l.entity_id AND c.deleted_at IS NOT NULL))
			)`, r.tableNames.FileLinks(), r.tableNames.Commodities())
		if err := tx.QueryRowxContext(ctx, heldQuery, id, entityType, entityID, models.FileLinkCommodity).Scan(&held); err != nil {
			return err
		}
		if held {
			return nil
		}

		var withItem sql.NullString
		withQuery := fmt.Sprintf(`
			SELECT l.entity_id FROM %[1]s l
			JOIN %[2]s c ON c.id = l.entity_id AND c.deleted_at IS NOT NULL
			WHERE l.file_id = $1 AND l.entity_type = $2
			ORDER BY l.created_at ASC, l.id ASC
			LIMIT 1`, r.tableNames.FileLinks(), r.tableNames.Commodities())
		if err := tx.QueryRowxContext(ctx, withQuery, id, models.FileLinkCommodity).Scan(&withItem); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		update := fmt.Sprintf(
			`UPDATE %s SET deleted_at = $2, deleted_by = $3, deleted_with_commodity_id = NULLIF($4, '') WHERE id = $1`,
			r.tableNames.Files())
		if _, err := tx.ExecContext(ctx, update, id, at.UTC(), deletedBy, withItem.String); err != nil {
			return err
		}
		trashed = true
		return nil
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to trash file", err, errx.Attrs("file_id", id))
	}
	return trashed, nil
}

// Restore makes a trashed file live again.
func (r *FileRegistry) Restore(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET deleted_at = NULL, deleted_by = NULL, deleted_with_commodity_id = NULL
			 WHERE id = $1 AND %s`, r.tableNames.Files(), trashedRow)
		changed, err := execAffected(ctx, tx, query, id)
		if err != nil {
			return err
		}
		if !changed {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
		}
		return nil
	})
	if err != nil {
		return errxtrace.Wrap("failed to restore file", err, errx.Attrs("file_id", id))
	}
	return nil
}

// SetTrashedWith changes the item a trashed file went to Trash with.
func (r *FileRegistry) SetTrashedWith(ctx context.Context, id, commodityID string) error {
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET deleted_with_commodity_id = NULLIF($2, '') WHERE id = $1 AND %s`,
			r.tableNames.Files(), trashedRow)
		changed, err := execAffected(ctx, tx, query, id, commodityID)
		if err != nil {
			return err
		}
		if !changed {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
		}
		return nil
	})
	if err != nil {
		return errxtrace.Wrap("failed to update trashed file", err, errx.Attrs("file_id", id))
	}
	return nil
}

// PurgeTrashed deletes a trashed file's row; its links go with it through ON
// DELETE CASCADE. A concurrent restore wins and the purge deletes nothing.
func (r *FileRegistry) PurgeTrashed(ctx context.Context, id string) (bool, error) {
	var deleted bool
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1 AND %s`, r.tableNames.Files(), trashedRow)
		var err error
		deleted, err = execAffected(ctx, tx, query, id)
		return err
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to purge file", err, errx.Attrs("file_id", id))
	}
	return deleted, nil
}
