package memory

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.GroupInviteAuditRegistry = (*GroupInviteAuditRegistry)(nil)

type baseGroupInviteAuditRegistry = Registry[models.GroupInviteAudit, *models.GroupInviteAudit]

type GroupInviteAuditRegistry struct {
	*baseGroupInviteAuditRegistry
}

func NewGroupInviteAuditRegistry() *GroupInviteAuditRegistry {
	return &GroupInviteAuditRegistry{
		baseGroupInviteAuditRegistry: NewRegistry[models.GroupInviteAudit, *models.GroupInviteAudit](),
	}
}

// Create is idempotent per (tenant_id, original_invite_id), mirroring the
// Postgres counterpart and the idx_group_invites_audit_tenant_invite unique
// constraint: if a snapshot already exists for the same source invite, the
// existing row is returned unchanged so GroupPurgeService retries after a
// partial failure produce no duplicate audit records.
func (r *GroupInviteAuditRegistry) Create(_ context.Context, audit models.GroupInviteAudit) (*models.GroupInviteAudit, error) {
	// Hold the write lock across the dup-check AND the insert. Dropping
	// the read-lock between the two (as the old RLock-then-base.Create
	// did) lets two concurrent retries for the same source invite both
	// pass the check and both insert duplicate snapshots. Mint IDs here
	// because base Create would re-acquire the lock we still hold —
	// mirrors MaintenanceReminderRegistry.CreateOnce.
	r.lock.Lock()
	defer r.lock.Unlock()

	if audit.TenantID != "" && audit.OriginalInviteID != "" {
		for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
			existing := pair.Value
			if existing.TenantID == audit.TenantID && existing.OriginalInviteID == audit.OriginalInviteID {
				v := *existing
				return &v, nil
			}
		}
	}

	row := audit
	if row.ID == "" {
		row.ID = uuid.New().String()
	}
	if row.UUID == "" {
		row.UUID = uuid.New().String()
	}
	r.items.Set(row.ID, &row)
	v := row
	return &v, nil
}

func (r *GroupInviteAuditRegistry) ListByOriginalGroup(_ context.Context, originalGroupID string) ([]*models.GroupInviteAudit, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var audits []*models.GroupInviteAudit
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		audit := pair.Value
		if audit.OriginalGroupID == originalGroupID {
			v := *audit
			audits = append(audits, &v)
		}
	}
	return audits, nil
}

func (r *GroupInviteAuditRegistry) ListByTenant(_ context.Context, tenantID string) ([]*models.GroupInviteAudit, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var audits []*models.GroupInviteAudit
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		audit := pair.Value
		if audit.TenantID == tenantID {
			v := *audit
			audits = append(audits, &v)
		}
	}

	// Most recent first — the postgres counterpart matches this ordering.
	sort.SliceStable(audits, func(i, j int) bool {
		return audits[i].ArchivedAt.After(audits[j].ArchivedAt)
	})
	return audits, nil
}
