package postgres

import (
	"context"
	"errors"
	"sort"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.GroupInviteAuditRegistry = (*GroupInviteAuditRegistry)(nil)

// GroupInviteAuditRegistry persists audit snapshots of used invites after
// the parent LocationGroup is hard-deleted by the purge worker. Like
// RefreshTokenRegistry and GroupInviteRegistry, it runs in service mode so
// inserts during the purge transaction are not blocked by tenant RLS
// (background-worker role has a bypass policy).
type GroupInviteAuditRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewGroupInviteAuditRegistry(dbx *sqlx.DB) *GroupInviteAuditRegistry {
	return &GroupInviteAuditRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

func (r *GroupInviteAuditRegistry) newSQLRegistry() *store.RLSRepository[models.GroupInviteAudit, *models.GroupInviteAudit] {
	return store.NewServiceSQLRegistry[models.GroupInviteAudit, *models.GroupInviteAudit](r.dbx, r.tableNames.GroupInvitesAudit())
}

func (r *GroupInviteAuditRegistry) Get(ctx context.Context, id string) (*models.GroupInviteAudit, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var audit models.GroupInviteAudit
	err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &audit)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "GroupInviteAudit",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get group invite audit", err)
	}

	return &audit, nil
}

func (r *GroupInviteAuditRegistry) List(ctx context.Context) ([]*models.GroupInviteAudit, error) {
	var audits []*models.GroupInviteAudit
	for audit, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list group invite audits", err)
		}
		audits = append(audits, &audit)
	}
	return audits, nil
}

func (r *GroupInviteAuditRegistry) Count(ctx context.Context) (int, error) {
	count, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count group invite audits", err)
	}
	return count, nil
}

func (r *GroupInviteAuditRegistry) Create(ctx context.Context, audit models.GroupInviteAudit) (*models.GroupInviteAudit, error) {
	if audit.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if audit.OriginalInviteID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "OriginalInviteID"))
	}
	if audit.OriginalGroupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "OriginalGroupID"))
	}
	if audit.UsedBy == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UsedBy"))
	}
	if audit.UsedAt.IsZero() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UsedAt"))
	}

	created, err := r.newSQLRegistry().Create(ctx, audit, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create group invite audit", err)
	}
	return &created, nil
}

// Update is provided for Registry-interface completeness. Audit rows are
// intentionally immutable in normal operation — no caller in the codebase
// invokes this method outside of tests/maintenance.
func (r *GroupInviteAuditRegistry) Update(ctx context.Context, audit models.GroupInviteAudit) (*models.GroupInviteAudit, error) {
	if audit.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	err := r.newSQLRegistry().Update(ctx, audit, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update group invite audit", err)
	}
	return &audit, nil
}

func (r *GroupInviteAuditRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	err := r.newSQLRegistry().Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete group invite audit", err)
	}
	return nil
}

func (r *GroupInviteAuditRegistry) ListByOriginalGroup(ctx context.Context, originalGroupID string) ([]*models.GroupInviteAudit, error) {
	if originalGroupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "OriginalGroupID"))
	}

	var audits []*models.GroupInviteAudit
	for audit, err := range r.newSQLRegistry().ScanByField(ctx, store.Pair("original_group_id", originalGroupID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list group invite audits by original group", err)
		}
		audits = append(audits, &audit)
	}
	return audits, nil
}

func (r *GroupInviteAuditRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.GroupInviteAudit, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	var audits []*models.GroupInviteAudit
	for audit, err := range r.newSQLRegistry().ScanByField(ctx, store.Pair("tenant_id", tenantID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list group invite audits by tenant", err)
		}
		audits = append(audits, &audit)
	}

	// Match memory behavior: most recent first.
	sort.SliceStable(audits, func(i, j int) bool {
		return audits[i].ArchivedAt.After(audits[j].ArchivedAt)
	})
	return audits, nil
}
