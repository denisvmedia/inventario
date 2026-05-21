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

// DeleteWithMemberInvariants atomically counts the group's
// memberships, enforces the two ≥1-owner / ≥1-member invariants, and
// deletes the row — all under the embedded Registry's write lock
// (#1652). The lock is the in-memory equivalent of the postgres
// per-group advisory lock: two goroutines calling this for the same
// group serialize, so a race where both pass the count check and
// both delete is impossible.
func (r *GroupMembershipRegistry) DeleteWithMemberInvariants(_ context.Context, membershipID string) error {
	if membershipID == "" {
		return registry.ErrFieldRequired
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	target, ok := r.items.Get(membershipID)
	if !ok {
		return registry.ErrNotFound
	}

	groupID := target.GroupID
	memberCount := 0
	ownerCount := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID != groupID {
			continue
		}
		memberCount++
		if m.Role == models.GroupRoleOwner {
			ownerCount++
		}
	}

	// Owner-first ordering so a sole-owner self-leave surfaces the
	// more specific ErrLastOwner ("transfer ownership first"); the
	// member-count fallback only fires when the owner check passes
	// vacuously (role data drift on a single-member group).
	if target.Role == models.GroupRoleOwner && ownerCount <= 1 {
		return registry.ErrLastOwner
	}
	if memberCount <= 1 {
		return registry.ErrLastMember
	}

	r.items.Delete(membershipID)
	return nil
}

// UpdateRoleWithMemberInvariants shares the same write lock as
// DeleteWithMemberInvariants so a concurrent leave + owner-demotion
// pair can no longer both observe ownerCount=2 and both commit —
// they serialize under the registry's mutex (#1652). Postgres uses
// the same pg_advisory_xact_lock key for the same effect.
func (r *GroupMembershipRegistry) UpdateRoleWithMemberInvariants(_ context.Context, membershipID string, newRole models.GroupRole) (*models.GroupMembership, error) {
	if membershipID == "" {
		return nil, registry.ErrFieldRequired
	}
	if err := newRole.Validate(); err != nil {
		return nil, registry.ErrFieldRequired
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	target, ok := r.items.Get(membershipID)
	if !ok {
		return nil, registry.ErrNotFound
	}

	if target.Role == models.GroupRoleOwner && newRole != models.GroupRoleOwner {
		ownerCount := 0
		for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
			m := pair.Value
			if m.GroupID == target.GroupID && m.Role == models.GroupRoleOwner {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			return nil, registry.ErrLastOwner
		}
	}

	target.Role = newRole
	out := *target
	return &out, nil
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

// CountByGroup mirrors the postgres SELECT COUNT(*) — the in-memory
// store walks the shared OrderedMap. Used to populate members_count
// on the LocationGroup resource (#1650).
func (r *GroupMembershipRegistry) CountByGroup(_ context.Context, groupID string) (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	count := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID == groupID {
			count++
		}
	}
	return count, nil
}

// CountByGroups walks the membership map once and tallies per-group.
// Pre-seeds the result with zeros for every requested ID so callers
// can treat a missing key as "no rows scanned" rather than ambiguous.
func (r *GroupMembershipRegistry) CountByGroups(_ context.Context, groupIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(groupIDs))
	if len(groupIDs) == 0 {
		return out, nil
	}
	wanted := make(map[string]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		out[id] = 0
		wanted[id] = struct{}{}
	}

	r.lock.RLock()
	defer r.lock.RUnlock()

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if _, ok := wanted[m.GroupID]; ok {
			out[m.GroupID]++
		}
	}
	return out, nil
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

// ListByGroupWithUsersAdmin is the cross-tenant twin of
// ListByGroupWithUsers (#1756 admin membership editor). The memory
// backend has no row-level security, so the per-group join is already
// cross-tenant — this simply delegates. The dedicated method exists so
// the registry interface stays symmetric with the postgres backend,
// which DOES need a distinct RLS-bypass code path.
func (r *GroupMembershipRegistry) ListByGroupWithUsersAdmin(ctx context.Context, groupID string) ([]*models.MembershipWithUser, error) {
	return r.ListByGroupWithUsers(ctx, groupID)
}
