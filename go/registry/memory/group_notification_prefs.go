package memory

import (
	"context"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

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

// ListByUserGroup mirrors the postgres impl's required-field validation
// so the two backends raise the same `ErrFieldRequired` on empty input
// — tests written against memory don't silently pass on a missing slug
// that the production postgres path would reject.
func (r *GroupNotificationPrefRegistry) ListByUserGroup(_ context.Context, tenantID, groupID, userID string) ([]*models.GroupNotificationPref, error) {
	if tenantID == "" || groupID == "" || userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "tenant_id|group_id|user_id"))
	}
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
// is created. CreatedAt/UpdatedAt are populated in UTC here so the
// in-memory state matches what postgres persists (schema column has
// `default_expr="CURRENT_TIMESTAMP"`, and the postgres Upsert sets
// `time.Now().UTC()`) — keeping in-memory tests deterministic and
// directly comparable to integration runs.
func (r *GroupNotificationPrefRegistry) Upsert(ctx context.Context, pref models.GroupNotificationPref) (*models.GroupNotificationPref, error) {
	now := time.Now().UTC()
	r.lock.Lock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		p := pair.Value
		if p.TenantID == pref.TenantID && p.GroupID == pref.GroupID && p.UserID == pref.UserID && p.Category == pref.Category {
			p.Enabled = pref.Enabled
			p.UpdatedAt = now
			v := *p
			r.lock.Unlock()
			return &v, nil
		}
	}
	r.lock.Unlock()
	// Insert path: stamp the timestamps explicitly before delegating
	// to base Create — the base path doesn't populate them, and the
	// postgres equivalent does so via the schema default.
	if pref.CreatedAt.IsZero() {
		pref.CreatedAt = now
	}
	pref.UpdatedAt = now
	return r.baseGroupNotificationPrefRegistry.Create(ctx, pref)
}

// DeleteByGroup removes every per-user notification override for the
// given (tenant, group) from the in-memory store. Mirrors the postgres
// parameterized DELETE used by the group-deletion cleanup path.
// Idempotent: zero matches returns (0, nil).
func (r *GroupNotificationPrefRegistry) DeleteByGroup(_ context.Context, tenantID, groupID string) (int, error) {
	if tenantID == "" || groupID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "tenant_id|group_id"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	var toDelete []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		p := pair.Value
		if p.TenantID == tenantID && p.GroupID == groupID {
			toDelete = append(toDelete, p.GetID())
		}
	}
	for _, id := range toDelete {
		r.items.Delete(id)
	}
	return len(toDelete), nil
}
