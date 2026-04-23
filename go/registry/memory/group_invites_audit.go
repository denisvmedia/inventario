package memory

import (
	"context"
	"sort"

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
func (r *GroupInviteAuditRegistry) Create(ctx context.Context, audit models.GroupInviteAudit) (*models.GroupInviteAudit, error) {
	if audit.TenantID != "" && audit.OriginalInviteID != "" {
		r.lock.RLock()
		for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
			existing := pair.Value
			if existing.TenantID == audit.TenantID && existing.OriginalInviteID == audit.OriginalInviteID {
				v := *existing
				r.lock.RUnlock()
				return &v, nil
			}
		}
		r.lock.RUnlock()
	}
	return r.baseGroupInviteAuditRegistry.Create(ctx, audit)
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
