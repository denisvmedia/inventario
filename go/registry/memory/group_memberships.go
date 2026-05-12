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
	// userRegistry, when set, allows ListByGroupWithUsers to resolve the
	// joined user rows. The memory backend has no SQL JOIN to use, so it
	// needs a reference to look users up. Tests that construct the
	// registry directly (without going through NewFactorySet) and don't
	// exercise the join can leave this nil — ListByGroupWithUsers will
	// return memberships with a nil User field, mirroring "row missing
	// from join" semantics rather than panicking.
	userRegistry registry.UserRegistry
}

func NewGroupMembershipRegistry() *GroupMembershipRegistry {
	return &GroupMembershipRegistry{
		baseGroupMembershipRegistry: NewRegistry[models.GroupMembership, *models.GroupMembership](),
	}
}

// SetUserRegistry wires the user registry used by ListByGroupWithUsers.
// Called by NewFactorySet once both registries exist; safe to skip in
// targeted tests that don't touch the join path.
func (r *GroupMembershipRegistry) SetUserRegistry(u registry.UserRegistry) {
	r.userRegistry = u
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
		if m.GroupID == groupID && m.IsAdmin() {
			count++
		}
	}

	return count, nil
}

func (r *GroupMembershipRegistry) CountOwnersByGroup(_ context.Context, groupID string) (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	count := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID == groupID && m.Role == models.GroupRoleOwner {
			count++
		}
	}

	return count, nil
}

func (r *GroupMembershipRegistry) ListByGroupWithUsers(ctx context.Context, groupID string) ([]*models.MembershipWithUser, error) {
	r.lock.RLock()
	var memberships []models.GroupMembership
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID == groupID {
			memberships = append(memberships, *m)
		}
	}
	r.lock.RUnlock()

	out := make([]*models.MembershipWithUser, 0, len(memberships))
	for i := range memberships {
		// Take the address of the slice element rather than a loop-
		// local copy. This guarantees a stable pointer per iteration
		// regardless of compiler / Go-version loopvar semantics, and
		// also keeps the heap allocation predictable for the reader.
		var user *models.User
		if r.userRegistry != nil {
			u, err := r.userRegistry.Get(ctx, memberships[i].MemberUserID)
			if err == nil && u != nil {
				user = u
			}
		}
		out = append(out, &models.MembershipWithUser{
			Membership: &memberships[i],
			User:       user,
		})
	}
	return out, nil
}
