package postgres_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// membershipFor builds a valid GroupMembership owned by the given user in
// tenantID with the provided role. Not persisted — call Create yourself.
func membershipFor(tenantID, groupID, memberUserID string, role models.GroupRole) models.GroupMembership {
	return models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             groupID,
		MemberUserID:        memberUserID,
		Role:                role,
	}
}

func TestGroupMembershipRegistry_Create_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Membership Target"))
	c.Assert(err, qt.IsNil)

	m, err := registrySet.GroupMembershipRegistry.Create(ctx, membershipFor(user.TenantID, group.ID, user.ID, models.GroupRoleAdmin))
	c.Assert(err, qt.IsNil)
	c.Assert(m.ID, qt.Not(qt.Equals), "")
	c.Assert(m.Role, qt.Equals, models.GroupRoleAdmin)
}

func TestGroupMembershipRegistry_Create_MissingFields(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "MF"))
	c.Assert(err, qt.IsNil)

	base := membershipFor(user.TenantID, group.ID, user.ID, models.GroupRoleAdmin)
	cases := []struct {
		name string
		mut  func(*models.GroupMembership)
	}{
		{"group_id empty", func(m *models.GroupMembership) { m.GroupID = "" }},
		{"member_user_id empty", func(m *models.GroupMembership) { m.MemberUserID = "" }},
		{"tenant empty", func(m *models.GroupMembership) { m.TenantID = "" }},
		{"role invalid", func(m *models.GroupMembership) { m.Role = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			m := base
			tc.mut(&m)
			_, err := registrySet.GroupMembershipRegistry.Create(ctx, m)
			c.Assert(err, qt.IsNotNil)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

func TestGroupMembershipRegistry_Create_Duplicate(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Dup"))
	c.Assert(err, qt.IsNil)

	first := membershipFor(user.TenantID, group.ID, user.ID, models.GroupRoleAdmin)
	_, err = registrySet.GroupMembershipRegistry.Create(ctx, first)
	c.Assert(err, qt.IsNil)

	// Same (group_id, member_user_id) is rejected (unique index).
	_, err = registrySet.GroupMembershipRegistry.Create(ctx, first)
	c.Assert(err, qt.IsNotNil)
	c.Assert(errors.Is(err, registry.ErrAlreadyExists), qt.IsTrue)
}

func TestGroupMembershipRegistry_GetByGroupAndUser(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Get"))
	c.Assert(err, qt.IsNil)

	created, err := registrySet.GroupMembershipRegistry.Create(ctx, membershipFor(user.TenantID, group.ID, user.ID, models.GroupRoleAdmin))
	c.Assert(err, qt.IsNil)

	fetched, err := registrySet.GroupMembershipRegistry.GetByGroupAndUser(ctx, group.ID, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)

	_, err = registrySet.GroupMembershipRegistry.GetByGroupAndUser(ctx, group.ID, "no-such-user")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

// #1652: postgres-side coverage for the new invariant-checked delete
// path. The shared service tests exercise the memory backend; this
// pins the hand-written SQL + pg_advisory_xact_lock + sentinel
// mapping on the production registry so regressions in the
// postgres-specific code can't slip past memory-only tests.
func TestGroupMembershipRegistry_DeleteWithMemberInvariants(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "DelInvariants"))
	c.Assert(err, qt.IsNil)

	// Sole owner — both invariants would fire; owner-first ordering
	// must surface ErrLastOwner (the more actionable copy).
	sole, err := registrySet.GroupMembershipRegistry.Create(ctx, membershipFor(user.TenantID, group.ID, user.ID, models.GroupRoleOwner))
	c.Assert(err, qt.IsNil)
	err = registrySet.GroupMembershipRegistry.DeleteWithMemberInvariants(ctx, sole.ID)
	c.Assert(errors.Is(err, registry.ErrLastOwner), qt.IsTrue, qt.Commentf("expected ErrLastOwner, got %v", err))

	// Add a non-owner second member. The sole-owner block above
	// still holds; removing the user-role row now drops memberCount
	// from 2 → 1 but the owner stays. Happy path.
	secondUser, err := registrySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Email:               "del-inv-second@test-org.com",
		Name:                "Second",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	m2, err := registrySet.GroupMembershipRegistry.Create(ctx, membershipFor(user.TenantID, group.ID, secondUser.ID, models.GroupRoleUser))
	c.Assert(err, qt.IsNil)
	err = registrySet.GroupMembershipRegistry.DeleteWithMemberInvariants(ctx, m2.ID)
	c.Assert(err, qt.IsNil)
	// Verify the row is actually gone.
	_, err = registrySet.GroupMembershipRegistry.Get(ctx, m2.ID)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)

	// ErrNotFound on an unknown id (the initial group_id lookup
	// fails fast — no group lock needed beyond that).
	err = registrySet.GroupMembershipRegistry.DeleteWithMemberInvariants(ctx, "00000000-0000-0000-0000-000000000000")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)

	// ErrLastMember: construct a one-member group whose sole row is
	// NOT an owner (simulates the role-drift case the defense-in-depth
	// invariant exists to catch). The owner check passes vacuously,
	// the member-count check fires.
	driftGroup, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "DriftDel"))
	c.Assert(err, qt.IsNil)
	driftUser, err := registrySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Email:               "drift-del@test-org.com",
		Name:                "Drift",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	soleUser, err := registrySet.GroupMembershipRegistry.Create(ctx, membershipFor(user.TenantID, driftGroup.ID, driftUser.ID, models.GroupRoleUser))
	c.Assert(err, qt.IsNil)
	err = registrySet.GroupMembershipRegistry.DeleteWithMemberInvariants(ctx, soleUser.ID)
	c.Assert(errors.Is(err, registry.ErrLastMember), qt.IsTrue, qt.Commentf("expected ErrLastMember, got %v", err))
}

// #1652: postgres-side coverage for the new invariant-checked role
// update. Catches regressions in the hand-written `UPDATE ...
// RETURNING *` scan, the advisory-lock key (must match
// DeleteWithMemberInvariants), and the sentinel propagation.
func TestGroupMembershipRegistry_UpdateRoleWithMemberInvariants(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "UpdInvariants"))
	c.Assert(err, qt.IsNil)

	// Sole owner cannot be demoted — zero owners afterwards.
	sole, err := registrySet.GroupMembershipRegistry.Create(ctx, membershipFor(user.TenantID, group.ID, user.ID, models.GroupRoleOwner))
	c.Assert(err, qt.IsNil)
	_, err = registrySet.GroupMembershipRegistry.UpdateRoleWithMemberInvariants(ctx, sole.ID, models.GroupRoleAdmin)
	c.Assert(errors.Is(err, registry.ErrLastOwner), qt.IsTrue, qt.Commentf("expected ErrLastOwner, got %v", err))

	// Add a second owner; now the original can be demoted. The
	// returned row must reflect the new role (verifies the RETURNING
	// scan, not just the column write).
	secondUser, err := registrySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Email:               "upd-inv-second@test-org.com",
		Name:                "Second",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	_, err = registrySet.GroupMembershipRegistry.Create(ctx, membershipFor(user.TenantID, group.ID, secondUser.ID, models.GroupRoleOwner))
	c.Assert(err, qt.IsNil)
	updated, err := registrySet.GroupMembershipRegistry.UpdateRoleWithMemberInvariants(ctx, sole.ID, models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)
	c.Assert(updated, qt.IsNotNil)
	c.Assert(updated.Role, qt.Equals, models.GroupRoleAdmin)
	c.Assert(updated.ID, qt.Equals, sole.ID)
	// Persisted, not just echoed.
	fresh, err := registrySet.GroupMembershipRegistry.Get(ctx, sole.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fresh.Role, qt.Equals, models.GroupRoleAdmin)

	// Demoting back to owner (no-op invariant-wise) is allowed —
	// the owner-count branch only fires when transitioning OUT of
	// owner, so this exercises the "skip the count" path.
	_, err = registrySet.GroupMembershipRegistry.UpdateRoleWithMemberInvariants(ctx, sole.ID, models.GroupRoleOwner)
	c.Assert(err, qt.IsNil)

	// ErrNotFound on an unknown id.
	_, err = registrySet.GroupMembershipRegistry.UpdateRoleWithMemberInvariants(ctx, "00000000-0000-0000-0000-000000000000", models.GroupRoleAdmin)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestGroupMembershipRegistry_ListByGroup_ByUser_CountAdmins(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Multi"))
	c.Assert(err, qt.IsNil)

	// Admin member.
	_, err = registrySet.GroupMembershipRegistry.Create(ctx, membershipFor(user.TenantID, group.ID, user.ID, models.GroupRoleAdmin))
	c.Assert(err, qt.IsNil)

	// A second user for a "user"-role membership.
	secondUser, err := registrySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Email:               "second@test-org.com",
		Name:                "Second",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	_, err = registrySet.GroupMembershipRegistry.Create(ctx, membershipFor(user.TenantID, group.ID, secondUser.ID, models.GroupRoleUser))
	c.Assert(err, qt.IsNil)

	byGroup, err := registrySet.GroupMembershipRegistry.ListByGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(byGroup, qt.HasLen, 2)

	byUser, err := registrySet.GroupMembershipRegistry.ListByUser(ctx, user.TenantID, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(byUser) >= 1, qt.IsTrue)

	admins, err := registrySet.GroupMembershipRegistry.CountAdminsByGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(admins, qt.Equals, 1)
}
