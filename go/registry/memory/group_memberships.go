package memory

import (
	"context"

	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.GroupMembershipRegistry = (*GroupMembershipRegistry)(nil)

type baseGroupMembershipRegistry = Registry[models.GroupMembership, *models.GroupMembership]

type GroupMembershipRegistry struct {
	*baseGroupMembershipRegistry
}

func NewGroupMembershipRegistry() *GroupMembershipRegistry {
	return &GroupMembershipRegistry{
		baseGroupMembershipRegistry: NewRegistry[models.GroupMembership, *models.GroupMembership](),
	}
}

func (r *GroupMembershipRegistry) GetByGroupAndUser(_ context.Context, groupID, userID string) (*models.GroupMembership, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID == groupID && m.MemberUserID == userID {
			v := *m
			return &v, nil
		}
	}

	return nil, registry.ErrNotFound
}

func (r *GroupMembershipRegistry) ListByGroup(_ context.Context, groupID string) ([]*models.GroupMembership, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var memberships []*models.GroupMembership
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID == groupID {
			v := *m
			memberships = append(memberships, &v)
		}
	}

	return memberships, nil
}

func (r *GroupMembershipRegistry) ListByUser(_ context.Context, tenantID, userID string) ([]*models.GroupMembership, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var memberships []*models.GroupMembership
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.MemberUserID == userID && m.TenantID == tenantID {
			v := *m
			memberships = append(memberships, &v)
		}
	}

	return memberships, nil
}

// CountByUser mirrors the postgres SELECT COUNT(*) — the in-memory
// store walks the shared OrderedMap, but callers see the same
// interface (a single int) so cap-check code is uniform across
// backends.
func (r *GroupMembershipRegistry) CountByUser(_ context.Context, tenantID, userID string) (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	count := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.MemberUserID == userID && m.TenantID == tenantID {
			count++
		}
	}
	return count, nil
}

// CreateUnderCap mirrors the postgres atomic cap-check + insert. The
// memory backend takes the shared write lock for the duration of the
// count + create, so two concurrent callers serialize against each
// other (equivalent to the postgres advisory lock + tx). Returns
// (nil, true, nil) when the cap is already met.
func (r *GroupMembershipRegistry) CreateUnderCap(ctx context.Context, membership models.GroupMembership, maxMemberships int) (*models.GroupMembership, bool, error) {
	if maxMemberships <= 0 {
		return nil, false, registry.ErrFieldRequired
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	count := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.MemberUserID == membership.MemberUserID && m.TenantID == membership.TenantID {
			count++
		}
		if m.GroupID == membership.GroupID && m.MemberUserID == membership.MemberUserID {
			return nil, false, registry.ErrAlreadyExists
		}
	}
	if count >= maxMemberships {
		return nil, true, nil
	}

	// Reuse the embedded Registry's Create through a helper that
	// doesn't take the lock again (we already hold it). Falling back
	// to a manual insert keeps this path symmetric with how Create
	// stamps IDs / timestamps via the shared store.
	created, err := r.createLocked(ctx, membership)
	if err != nil {
		return nil, false, err
	}
	return created, false, nil
}

// createLocked is Create() without the lock acquisition — the caller
// must already hold r.lock.Lock(). Only CreateUnderCap uses this; the
// embedded Registry's regular Create handles the standard path.
func (r *GroupMembershipRegistry) createLocked(_ context.Context, m models.GroupMembership) (*models.GroupMembership, error) {
	if m.ID == "" {
		m.SetID(uuid.New().String())
	}
	stored := m
	r.items.Set(stored.ID, &stored)
	out := stored
	return &out, nil
}

func (r *GroupMembershipRegistry) CountAdminsByGroup(_ context.Context, groupID string) (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	count := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID == groupID && m.Role == models.GroupRoleAdmin {
			count++
		}
	}

	return count, nil
}
