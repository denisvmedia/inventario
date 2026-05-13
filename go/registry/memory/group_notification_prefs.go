package memory

import (
	"context"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.GroupNotificationPrefRegistry = (*GroupNotificationPrefRegistry)(nil)

type baseGroupNotificationPrefRegistry = Registry[models.GroupNotificationPref, *models.GroupNotificationPref]

type GroupNotificationPrefRegistry struct {
	*baseGroupNotificationPrefRegistry
}

func NewGroupNotificationPrefRegistry() *GroupNotificationPrefRegistry {
	return &GroupNotificationPrefRegistry{
		baseGroupNotificationPrefRegistry: NewRegistry[models.GroupNotificationPref, *models.GroupNotificationPref](),
	}
}

func (r *GroupNotificationPrefRegistry) ListByUserGroup(_ context.Context, tenantID, groupID, userID string) ([]*models.GroupNotificationPref, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var out []*models.GroupNotificationPref
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		p := pair.Value
		if p.TenantID == tenantID && p.GroupID == groupID && p.UserID == userID {
			v := *p
			out = append(out, &v)
		}
	}
	return out, nil
}

// Upsert mirrors the postgres ON CONFLICT (tenant_id, group_id,
// user_id, category) DO UPDATE behaviour: a matching row gets its
// `enabled` + `updated_at` rewritten in place; otherwise a fresh row
// is created via the base Create path.
func (r *GroupNotificationPrefRegistry) Upsert(ctx context.Context, pref models.GroupNotificationPref) (*models.GroupNotificationPref, error) {
	r.lock.Lock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		p := pair.Value
		if p.TenantID == pref.TenantID && p.GroupID == pref.GroupID && p.UserID == pref.UserID && p.Category == pref.Category {
			p.Enabled = pref.Enabled
			p.UpdatedAt = time.Now()
			v := *p
			r.lock.Unlock()
			return &v, nil
		}
	}
	r.lock.Unlock()
	return r.baseGroupNotificationPrefRegistry.Create(ctx, pref)
}
