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
