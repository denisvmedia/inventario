package memory

import (
	"context"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

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
func (r *GroupNotificationPrefRegistry) Upsert(_ context.Context, pref models.GroupNotificationPref) (*models.GroupNotificationPref, error) {
	// Mirror the postgres twin's required-field guard (and the
	// ListByUserGroup/DeleteByGroup siblings in this file) so a memory
	// test of Upsert doesn't silently accept an empty tuple field that
	// the production postgres path rejects with ErrFieldRequired.
	if pref.TenantID == "" || pref.GroupID == "" || pref.UserID == "" || pref.Category == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "tenant_id|group_id|user_id|category"))
	}
	now := time.Now().UTC()
	// Hold the write lock across the scan AND the insert: releasing it
	// between "not found" and Create opens a window where a concurrent
	// Upsert for the same (tenant, group, user, category) tuple also
	// sees "not found" and both insert duplicate rows. The CreateOnce
	// pattern (see MaintenanceReminderRegistry.CreateOnce) does the
	// find-or-insert under a single Lock; mint IDs ourselves here
	// because base Create would re-acquire the lock we still hold.
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		p := pair.Value
		if p.TenantID == pref.TenantID && p.GroupID == pref.GroupID && p.UserID == pref.UserID && p.Category == pref.Category {
			p.Enabled = pref.Enabled
			p.UpdatedAt = now
			v := *p
			return &v, nil
		}
	}
	// Insert path: stamp the timestamps explicitly — the postgres
	// equivalent populates them via the schema default.
	if pref.CreatedAt.IsZero() {
		pref.CreatedAt = now
	}
	pref.UpdatedAt = now
	row := pref
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
