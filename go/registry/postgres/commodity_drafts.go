package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// CommodityDraftRegistryFactory creates CommodityDraftRegistry instances
// with proper context (#2812).
type CommodityDraftRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// CommodityDraftRegistry is the postgres-backed group-scoped registry of
// unfinished items.
type CommodityDraftRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.CommodityDraftRegistry        = (*CommodityDraftRegistry)(nil)
	_ registry.CommodityDraftRegistryFactory = (*CommodityDraftRegistryFactory)(nil)
)

func NewCommodityDraftRegistry(dbx *sqlx.DB) *CommodityDraftRegistryFactory {
	return NewCommodityDraftRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewCommodityDraftRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *CommodityDraftRegistryFactory {
	return &CommodityDraftRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *CommodityDraftRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityDraftRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityDraftRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityDraftRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &CommodityDraftRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
	}, nil
}

func (f *CommodityDraftRegistryFactory) CreateServiceRegistry() registry.CommodityDraftRegistry {
	return &CommodityDraftRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *CommodityDraftRegistry) newSQLRegistry() *store.RLSGroupRepository[models.CommodityDraft, *models.CommodityDraft] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.CommodityDraft](r.dbx, r.tableNames.CommodityDrafts())
	}
	return store.NewGroupAwareSQLRegistry[models.CommodityDraft](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CommodityDrafts())
}

func (r *CommodityDraftRegistry) Get(ctx context.Context, id string) (*models.CommodityDraft, error) {
	var draft models.CommodityDraft
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &draft); err != nil {
		return nil, errxtrace.Wrap("failed to get commodity draft", err)
	}
	return &draft, nil
}

func (r *CommodityDraftRegistry) GetByClientKey(ctx context.Context, clientKey string) (*models.CommodityDraft, error) {
	var draft models.CommodityDraft
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("client_key", clientKey), &draft); err != nil {
		return nil, errxtrace.Wrap("failed to get commodity draft by client key", err)
	}
	return &draft, nil
}

func (r *CommodityDraftRegistry) List(ctx context.Context) ([]*models.CommodityDraft, error) {
	var drafts []*models.CommodityDraft
	for draft, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list commodity drafts", err)
		}
		drafts = append(drafts, new(draft))
	}
	return drafts, nil
}

func (r *CommodityDraftRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count commodity drafts", err)
	}
	return cnt, nil
}

// Create stores a new open draft. A client key the group already has fails
// on the unique index; CreateOrGet is the idempotent form.
func (r *CommodityDraftRegistry) Create(ctx context.Context, draft models.CommodityDraft) (*models.CommodityDraft, error) {
	resetNewDraft(ctx, &draft, time.Now())
	created, err := r.newSQLRegistry().Create(ctx, draft, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity draft", err)
	}
	return &created, nil
}

// resetNewDraft sets the server-managed fields of a draft about to be
// created: open, revision 1, created now, or the archived revision and
// timestamps of a draft a backup restore recreates.
func resetNewDraft(ctx context.Context, draft *models.CommodityDraft, now time.Time) {
	draft.Status = models.CommodityDraftOpen
	draft.Revision = 1
	draft.PromotedCommodityID = nil
	draft.PromotedAt = nil
	draft.PromotionKey = nil
	draft.CreatedAt = now
	draft.UpdatedAt = now
	if restored, ok := registry.RestoredDraftFromContext(ctx, now); ok {
		draft.Revision = restored.Revision
		draft.CreatedAt = restored.CreatedAt
		draft.UpdatedAt = restored.UpdatedAt
	}
	if draft.Values == nil {
		draft.Values = models.CommodityDraftValues{}
	}
}

// CreateOrGet stores a new draft unless the group has one with the same
// client key. The insert uses ON CONFLICT DO NOTHING on the (group_id,
// client_key) index, so two concurrent creates with one key store one row
// and both callers get it.
func (r *CommodityDraftRegistry) CreateOrGet(ctx context.Context, draft models.CommodityDraft) (*models.CommodityDraft, bool, error) {
	if r.service {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	if r.tenantID == "" {
		return nil, false, store.ErrTenantIDRequired
	}
	if r.groupID == "" {
		return nil, false, store.ErrGroupIDRequired
	}
	if r.createdByUserID == "" {
		return nil, false, store.ErrCreatedByUserIDRequired
	}

	resetNewDraft(ctx, &draft, time.Now())
	draft.ID = generateID()
	draft.UUID = generateID()
	draft.TenantID = r.tenantID
	draft.GroupID = r.groupID
	draft.CreatedByUserID = r.createdByUserID

	var (
		stored  models.CommodityDraft
		created bool
	)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		insert := fmt.Sprintf(`
			INSERT INTO %s (id, uuid, tenant_id, group_id, created_by_user_id, client_key, form_values, revision, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (group_id, client_key) DO NOTHING
			RETURNING *`, r.tableNames.CommodityDrafts())
		err := tx.QueryRowxContext(ctx, insert,
			draft.ID, draft.UUID, draft.TenantID, draft.GroupID, draft.CreatedByUserID,
			draft.ClientKey, draft.Values, draft.Revision, string(draft.Status), draft.CreatedAt, draft.UpdatedAt,
		).StructScan(&stored)
		if err == nil {
			created = true
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return errxtrace.Wrap("failed to insert commodity draft", err)
		}
		existing := fmt.Sprintf(`SELECT * FROM %s WHERE group_id = $1 AND client_key = $2`, r.tableNames.CommodityDrafts())
		err = tx.QueryRowxContext(ctx, existing, r.groupID, draft.ClientKey).StructScan(&stored)
		if errors.Is(err, sql.ErrNoRows) {
			// The row the insert conflicted with is gone: a discard deleted
			// it between the two statements. Only a discard deletes a
			// draft, so the key belongs to a discarded draft.
			return errxtrace.Classify(registry.ErrDraftDiscarded, errx.Attrs("client_key", draft.ClientKey))
		}
		if err != nil {
			return errxtrace.Wrap("failed to read existing commodity draft", err)
		}
		return nil
	})
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to create commodity draft", err)
	}
	return &stored, created, nil
}

// Update writes every field of the draft as given. The service writes
// through SaveValues and Promote; Update exists for the Registry contract.
func (r *CommodityDraftRegistry) Update(ctx context.Context, draft models.CommodityDraft) (*models.CommodityDraft, error) {
	draft.UpdatedAt = time.Now()
	if draft.Values == nil {
		draft.Values = models.CommodityDraftValues{}
	}
	if err := r.newSQLRegistry().Update(ctx, draft, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update commodity draft", err)
	}
	return &draft, nil
}

func (r *CommodityDraftRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

func (r *CommodityDraftRegistry) ListOpen(ctx context.Context, offset, limit int) ([]*models.CommodityDraft, int, error) {
	var (
		drafts []*models.CommodityDraft
		total  int
	)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		count := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE status = $1`, r.tableNames.CommodityDrafts())
		if err := tx.GetContext(ctx, &total, count, string(models.CommodityDraftOpen)); err != nil {
			return errxtrace.Wrap("failed to count open commodity drafts", err)
		}
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE status = $1 ORDER BY updated_at DESC, id DESC OFFSET $2 LIMIT $3`,
			r.tableNames.CommodityDrafts())
		rows, err := tx.QueryxContext(ctx, query, string(models.CommodityDraftOpen), max(offset, 0), max(limit, 0))
		if err != nil {
			return errxtrace.Wrap("failed to query open commodity drafts", err)
		}
		defer rows.Close()
		for rows.Next() {
			var draft models.CommodityDraft
			if err := rows.StructScan(&draft); err != nil {
				return errxtrace.Wrap("failed to scan commodity draft", err)
			}
			drafts = append(drafts, new(draft))
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list open commodity drafts", err)
	}
	return drafts, total, nil
}

// OpenIDs returns the ids of the open drafts, without their values; see the
// interface doc.
func (r *CommodityDraftRegistry) OpenIDs(ctx context.Context) ([]string, error) {
	ids := make([]string, 0)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT id FROM %s WHERE status = $1 ORDER BY updated_at DESC, id DESC`,
			r.tableNames.CommodityDrafts())
		return tx.SelectContext(ctx, &ids, query, string(models.CommodityDraftOpen))
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list open commodity draft ids", err)
	}
	return ids, nil
}

// ListDiscarding returns the drafts whose discard stopped before the given
// time; see the interface doc.
func (r *CommodityDraftRegistry) ListDiscarding(ctx context.Context, before time.Time, limit int) ([]*models.CommodityDraft, error) {
	var drafts []*models.CommodityDraft
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE status = $1 AND updated_at < $2 ORDER BY updated_at, id LIMIT $3`,
			r.tableNames.CommodityDrafts())
		rows, err := tx.QueryxContext(ctx, query, string(models.CommodityDraftDiscarding), before, max(limit, 0))
		if err != nil {
			return errxtrace.Wrap("failed to query commodity drafts being discarded", err)
		}
		defer rows.Close()
		for rows.Next() {
			var draft models.CommodityDraft
			if err := rows.StructScan(&draft); err != nil {
				return errxtrace.Wrap("failed to scan commodity draft", err)
			}
			drafts = append(drafts, new(draft))
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodity drafts being discarded", err)
	}
	return drafts, nil
}

// lockDraft reads the draft and holds its row lock until tx ends, so a save
// and a promotion of the same draft run one after the other.
func (r *CommodityDraftRegistry) lockDraft(ctx context.Context, tx *sqlx.Tx, id string) (*models.CommodityDraft, error) {
	var draft models.CommodityDraft
	query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.CommodityDrafts())
	if err := tx.QueryRowxContext(ctx, query, id).StructScan(&draft); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity_draft", "entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to read commodity draft", err)
	}
	return &draft, nil
}

// checkOpen reports why draft cannot change, or nil when it is open.
func checkOpen(draft *models.CommodityDraft) error {
	switch draft.Status {
	case models.CommodityDraftOpen:
		return nil
	case models.CommodityDraftDiscarding:
		return errxtrace.Classify(registry.ErrDraftDiscarded, errx.Attrs("entity_id", draft.ID))
	default:
		return errxtrace.Classify(registry.ErrDraftPromoted, errx.Attrs("entity_id", draft.ID))
	}
}

// checkOpenRevision reports why a write that expects revision cannot change
// draft, or nil when it can.
func checkOpenRevision(draft *models.CommodityDraft, revision int) error {
	if err := checkOpen(draft); err != nil {
		return err
	}
	if draft.Revision != revision {
		return errxtrace.Classify(registry.ErrDraftRevisionConflict,
			errx.Attrs("entity_id", draft.ID, "revision", draft.Revision, "expected_revision", revision))
	}
	return nil
}

func (r *CommodityDraftRegistry) SaveValues(ctx context.Context, id string, revision int, values models.CommodityDraftValues) (*models.CommodityDraft, error) {
	if values == nil {
		values = models.CommodityDraftValues{}
	}
	var saved models.CommodityDraft
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		draft, err := r.lockDraft(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := checkOpenRevision(draft, revision); err != nil {
			return err
		}
		update := fmt.Sprintf(`
			UPDATE %s SET form_values = $1, revision = revision + 1, updated_at = $2
			WHERE id = $3
			RETURNING *`, r.tableNames.CommodityDrafts())
		return tx.QueryRowxContext(ctx, update, values, time.Now(), id).StructScan(&saved)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to save commodity draft", err)
	}
	return &saved, nil
}

// Promote creates the item, moves the draft's file links to it and marks the
// draft promoted in one transaction. Every statement runs in that
// transaction; no other registry is called while it is open.
func (r *CommodityDraftRegistry) Promote(ctx context.Context, id string, revision int, promotionKey string, commodity models.Commodity) (registry.DraftPromotion, error) {
	if r.service {
		return registry.DraftPromotion{}, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	if promotionKey == "" {
		return registry.DraftPromotion{}, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "PromotionKey"))
	}
	requestedAssetNumber := prepareNewCommodity(ctx, &commodity)

	var (
		result models.CommodityDraft
		item   *models.Commodity
	)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		draft, err := r.lockDraft(ctx, tx, id)
		if err != nil {
			return err
		}
		if draft.Status == models.CommodityDraftPromoted && draft.PromotionKey != nil && *draft.PromotionKey == promotionKey {
			result = *draft
			return nil
		}
		if err := checkOpenRevision(draft, revision); err != nil {
			return err
		}

		commodity.ID = generateID()
		commodity.UUID = generateID()
		commodity.TenantID = r.tenantID
		commodity.GroupID = r.groupID
		commodity.CreatedByUserID = r.createdByUserID
		if err := prepareNewCommodityInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, &commodity, requestedAssetNumber); err != nil {
			return err
		}
		if err := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities()).Insert(ctx, commodity); err != nil {
			return errxtrace.Wrap("failed to insert commodity", err)
		}
		// The custom field values are checked and written in this
		// transaction too, so a refused value promotes nothing.
		if len(commodity.CustomFields) > 0 {
			if err := applyCustomFieldValuesInTx(ctx, tx, r.tableNames, &commodity, commodity.CustomFields); err != nil {
				return err
			}
		}

		// The new item's id is fresh, so no file can already link to it and
		// the unique (file_id, entity_type, entity_id) index cannot trip.
		relink := fmt.Sprintf(`
			UPDATE %s SET entity_type = $1, entity_id = $2
			WHERE entity_type = $3 AND entity_id = $4`, r.tableNames.FileLinks())
		if _, err := tx.ExecContext(ctx, relink, models.FileLinkCommodity, commodity.ID, models.FileLinkDraft, draft.ID); err != nil {
			return errxtrace.Wrap("failed to move the draft's file links", err)
		}

		now := time.Now()
		// The document another system delivered for the draft (#2809)
		// points at the item from now on.
		documents := fmt.Sprintf(`
			UPDATE %s SET commodity_id = $1, updated_at = $2
			WHERE group_id = $3 AND draft_id = $4`, r.tableNames.IntakeDocuments())
		if _, err := tx.ExecContext(ctx, documents, commodity.ID, now.UTC(), r.groupID, draft.ID); err != nil {
			return errxtrace.Wrap("failed to record the item on the delivered document", err)
		}
		markPromoted := fmt.Sprintf(`
			UPDATE %s SET status = $1, promoted_commodity_id = $2, promoted_at = $3, promotion_key = $4,
				form_values = '{}'::jsonb, revision = revision + 1, updated_at = $3
			WHERE id = $5
			RETURNING *`, r.tableNames.CommodityDrafts())
		if err := tx.QueryRowxContext(ctx, markPromoted, string(models.CommodityDraftPromoted), commodity.ID, now, promotionKey, draft.ID).StructScan(&result); err != nil {
			return errxtrace.Wrap("failed to mark the draft promoted", err)
		}

		var stored models.Commodity
		if err := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities()).ScanOneByField(ctx, store.Pair("id", commodity.ID), &stored); err != nil {
			return errxtrace.Wrap("failed to read the new commodity", err)
		}
		if err := loadCustomFieldValuesInTx(ctx, tx, r.tableNames, &stored); err != nil {
			return err
		}
		item = &stored
		return nil
	})
	if err != nil {
		return registry.DraftPromotion{}, errxtrace.Wrap("failed to promote commodity draft", err)
	}
	return registry.DraftPromotion{Draft: &result, Commodity: item}, nil
}

// AttachFile links a file to an open draft. The draft row is read FOR SHARE
// and stays locked until the link is written, so a promotion or a discard,
// which lock it FOR UPDATE, runs before or after the attachment, never in
// between. The link is written with the file registry's statements on this
// transaction; no other registry opens a transaction of its own.
func (r *CommodityDraftRegistry) AttachFile(ctx context.Context, draftID string, link models.FileLink) (*models.FileLink, bool, error) {
	if r.service {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	if link.FileID == "" {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "FileID"))
	}
	link.EntityType = models.FileLinkDraft
	link.EntityID = draftID
	files := &FileRegistry{tableNames: r.tableNames, createdByUserID: r.createdByUserID}
	var (
		stored  *models.FileLink
		created bool
	)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var draft models.CommodityDraft
		query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR SHARE`, r.tableNames.CommodityDrafts())
		if err := tx.QueryRowxContext(ctx, query, draftID).StructScan(&draft); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity_draft", "entity_id", draftID))
			}
			return errxtrace.Wrap("failed to read commodity draft", err)
		}
		if err := checkOpen(&draft); err != nil {
			return err
		}
		file, err := files.lockFileForShare(ctx, tx, link.FileID)
		if err != nil {
			return err
		}
		if file.IsTrashed() {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", link.FileID))
		}
		stored, created, err = files.insertLink(ctx, tx, file, link)
		return err
	})
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to attach file to commodity draft", err)
	}
	return stored, created, nil
}

// MarkDiscarding begins the discard of an open draft; see the interface doc.
func (r *CommodityDraftRegistry) MarkDiscarding(ctx context.Context, id string) (*models.CommodityDraft, error) {
	var marked models.CommodityDraft
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		draft, err := r.lockDraft(ctx, tx, id)
		if err != nil {
			return err
		}
		switch draft.Status {
		case models.CommodityDraftDiscarding:
			marked = *draft
			return nil
		case models.CommodityDraftOpen:
		default:
			return errxtrace.Classify(registry.ErrDraftPromoted, errx.Attrs("entity_id", id))
		}
		update := fmt.Sprintf(`UPDATE %s SET status = $1, updated_at = $2 WHERE id = $3 RETURNING *`, r.tableNames.CommodityDrafts())
		return tx.QueryRowxContext(ctx, update, string(models.CommodityDraftDiscarding), time.Now(), id).StructScan(&marked)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to begin the discard of commodity draft", err)
	}
	return &marked, nil
}

// DeleteDiscarding deletes a draft whose discard has begun; see the
// interface doc.
func (r *CommodityDraftRegistry) DeleteDiscarding(ctx context.Context, id string) error {
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1 AND status = $2`, r.tableNames.CommodityDrafts())
		deleted, err := execAffected(ctx, tx, query, id, string(models.CommodityDraftDiscarding))
		if err != nil {
			return err
		}
		if !deleted {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity_draft", "entity_id", id))
		}
		return nil
	})
	if err != nil {
		return errxtrace.Wrap("failed to delete commodity draft", err)
	}
	return nil
}
