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
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			UserID:   memberUserID,
		},
		GroupID:      groupID,
		MemberUserID: memberUserID,
		Role:         role,
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
