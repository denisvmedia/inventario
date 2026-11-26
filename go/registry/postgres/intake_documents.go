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

// IntakeDocumentRegistryFactory creates IntakeDocumentRegistry instances with
// proper context (#2809).
type IntakeDocumentRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// IntakeDocumentRegistry is the postgres-backed group-scoped registry of the
// documents other systems delivered.
type IntakeDocumentRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.IntakeDocumentRegistry        = (*IntakeDocumentRegistry)(nil)
	_ registry.IntakeDocumentRegistryFactory = (*IntakeDocumentRegistryFactory)(nil)
)

func NewIntakeDocumentRegistry(dbx *sqlx.DB) *IntakeDocumentRegistryFactory {
	return NewIntakeDocumentRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewIntakeDocumentRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *IntakeDocumentRegistryFactory {
	return &IntakeDocumentRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *IntakeDocumentRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.IntakeDocumentRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *IntakeDocumentRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.IntakeDocumentRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &IntakeDocumentRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
	}, nil
}

func (f *IntakeDocumentRegistryFactory) CreateServiceRegistry() registry.IntakeDocumentRegistry {
	return &IntakeDocumentRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *IntakeDocumentRegistry) newSQLRegistry() *store.RLSGroupRepository[models.IntakeDocument, *models.IntakeDocument] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.IntakeDocument](r.dbx, r.tableNames.IntakeDocuments())
	}
	return store.NewGroupAwareSQLRegistry[models.IntakeDocument](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.IntakeDocuments())
}

func (r *IntakeDocumentRegistry) requireGroup() error {
	if r.service {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	if r.tenantID == "" {
		return store.ErrTenantIDRequired
	}
	if r.groupID == "" {
		return store.ErrGroupIDRequired
	}
	if r.createdByUserID == "" {
		return store.ErrCreatedByUserIDRequired
	}
	return nil
}

func intakeDocumentNotFound(id string) error {
	return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "intake_document", "entity_id", id))
}

func (r *IntakeDocumentRegistry) Get(ctx context.Context, id string) (*models.IntakeDocument, error) {
	var doc models.IntakeDocument
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &doc); err != nil {
		return nil, errxtrace.Wrap("failed to get intake document", err)
	}
	return &doc, nil
}

func (r *IntakeDocumentRegistry) List(ctx context.Context) ([]*models.IntakeDocument, error) {
	var docs []*models.IntakeDocument
	for doc, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list intake documents", err)
		}
		docs = append(docs, new(doc))
	}
	return docs, nil
}

func (r *IntakeDocumentRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count intake documents", err)
	}
	return cnt, nil
}

// Create stores the document; a group that already has its identity gets
// ErrAlreadyExists. CreateOrGet is the form the service uses.
func (r *IntakeDocumentRegistry) Create(ctx context.Context, doc models.IntakeDocument) (*models.IntakeDocument, error) {
	stored, created, err := r.CreateOrGet(ctx, doc)
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("entity_type", "intake_document", "entity_id", stored.ID))
	}
	return stored, nil
}

// Update writes every field of the document as given. The service writes
// through RecordProgress; Update exists for the Registry contract.
func (r *IntakeDocumentRegistry) Update(ctx context.Context, doc models.IntakeDocument) (*models.IntakeDocument, error) {
	doc.UpdatedAt = time.Now().UTC()
	if err := r.newSQLRegistry().Update(ctx, doc, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update intake document", err)
	}
	return &doc, nil
}

func (r *IntakeDocumentRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

// CreateOrGet inserts with ON CONFLICT DO NOTHING on the identity index, so
// two concurrent deliveries of one document store one row and both callers
// get it.
func (r *IntakeDocumentRegistry) CreateOrGet(ctx context.Context, doc models.IntakeDocument) (*models.IntakeDocument, bool, error) {
	if err := r.requireGroup(); err != nil {
		return nil, false, err
	}
	if err := doc.ValidateFields(ctx); err != nil {
		return nil, false, errxtrace.Wrap("invalid intake document", err)
	}
	now := time.Now().UTC()
	doc.ID = generateID()
	doc.UUID = generateID()
	doc.TenantID = r.tenantID
	doc.GroupID = r.groupID
	doc.CreatedByUserID = r.createdByUserID
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = doc.CreatedAt
	}

	var (
		stored  models.IntakeDocument
		created bool
	)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		insert := fmt.Sprintf(`
			INSERT INTO %s (id, uuid, tenant_id, group_id, created_by_user_id,
				external_system, external_instance, external_document_id, source_name, source_url,
				payload_hash, file_id, session_id, draft_id, commodity_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			ON CONFLICT (group_id, external_system, external_instance, external_document_id) DO NOTHING
			RETURNING *`, r.tableNames.IntakeDocuments())
		err := tx.QueryRowxContext(ctx, insert,
			doc.ID, doc.UUID, doc.TenantID, doc.GroupID, doc.CreatedByUserID,
			doc.ExternalSystem, doc.ExternalInstance, doc.ExternalDocumentID, doc.SourceName, doc.SourceURL,
			doc.PayloadHash, doc.FileID, doc.SessionID, doc.DraftID, doc.CommodityID, doc.CreatedAt, doc.UpdatedAt,
		).StructScan(&stored)
		if err == nil {
			created = true
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return errxtrace.Wrap("failed to insert intake document", err)
		}
		return r.scanByExternalID(ctx, tx, &stored, doc.ExternalSystem, doc.ExternalInstance, doc.ExternalDocumentID)
	})
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to create intake document", err)
	}
	return &stored, created, nil
}

// scanByExternalID reads the group's document with the identity. The group
// condition is explicit, not left to RLS alone: the identity is unique per
// group only.
func (r *IntakeDocumentRegistry) scanByExternalID(ctx context.Context, tx *sqlx.Tx, out *models.IntakeDocument, system, instance, documentID string) error {
	query := fmt.Sprintf(`
		SELECT * FROM %s
		WHERE group_id = $1 AND external_system = $2 AND external_instance = $3 AND external_document_id = $4`,
		r.tableNames.IntakeDocuments())
	err := tx.QueryRowxContext(ctx, query, r.groupID, system, instance, documentID).StructScan(out)
	if errors.Is(err, sql.ErrNoRows) {
		return intakeDocumentNotFound(documentID)
	}
	return err
}

func (r *IntakeDocumentRegistry) GetByExternalID(ctx context.Context, system, instance, documentID string) (*models.IntakeDocument, error) {
	if err := r.requireGroup(); err != nil {
		return nil, err
	}
	var doc models.IntakeDocument
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		return r.scanByExternalID(ctx, tx, &doc, system, instance, documentID)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to get intake document by external id", err)
	}
	return &doc, nil
}

// RecordProgress fills the empty fields with COALESCE in one statement, so
// the first delivery to record a field wins.
func (r *IntakeDocumentRegistry) RecordProgress(ctx context.Context, id string, progress registry.IntakeDocumentProgress) (*models.IntakeDocument, error) {
	if err := r.requireGroup(); err != nil {
		return nil, err
	}
	var doc models.IntakeDocument
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// The draft is locked as a promotion locks it: a promotion either
		// committed, and the draft names its item, or waits for this
		// transaction and then finds the draft recorded.
		var promotedItem *string
		if progress.DraftID != nil {
			lock := fmt.Sprintf(`SELECT promoted_commodity_id FROM %s WHERE id = $1 AND group_id = $2 FOR SHARE`,
				r.tableNames.CommodityDrafts())
			err := tx.QueryRowxContext(ctx, lock, *progress.DraftID, r.groupID).Scan(&promotedItem)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Wrap("failed to lock the recorded draft", err)
			}
		}
		query := fmt.Sprintf(`
			UPDATE %s SET
				file_id = COALESCE(file_id, $1),
				session_id = COALESCE(session_id, $2),
				draft_id = COALESCE(draft_id, $3),
				commodity_id = COALESCE(commodity_id, $4, CASE WHEN draft_id IS NULL THEN $5::text END),
				updated_at = $6
			WHERE id = $7 AND group_id = $8
			RETURNING *`, r.tableNames.IntakeDocuments())
		err := tx.QueryRowxContext(ctx, query,
			progress.FileID, progress.SessionID, progress.DraftID, progress.CommodityID, promotedItem, time.Now().UTC(), id, r.groupID,
		).StructScan(&doc)
		if errors.Is(err, sql.ErrNoRows) {
			return intakeDocumentNotFound(id)
		}
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to record intake document progress", err)
	}
	return &doc, nil
}

func (r *IntakeDocumentRegistry) ListByDrafts(ctx context.Context, draftIDs ...string) ([]*models.IntakeDocument, error) {
	return r.listBy(ctx, "draft_id", draftIDs)
}

func (r *IntakeDocumentRegistry) ListByCommodities(ctx context.Context, commodityIDs ...string) ([]*models.IntakeDocument, error) {
	return r.listBy(ctx, "commodity_id", commodityIDs)
}

// listBy reads the group's documents whose column holds one of ids. column
// is one of the two constants above, never input.
func (r *IntakeDocumentRegistry) listBy(ctx context.Context, column string, ids []string) ([]*models.IntakeDocument, error) {
	if err := r.requireGroup(); err != nil {
		return nil, err
	}
	docs := []*models.IntakeDocument{}
	if len(ids) == 0 {
		return docs, nil
	}
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE group_id = $1 AND %s = ANY($2) ORDER BY created_at, id`,
			r.tableNames.IntakeDocuments(), column)
		return selectInto(ctx, tx, &docs, query, r.groupID, ids)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list intake documents", err)
	}
	return docs, nil
}
