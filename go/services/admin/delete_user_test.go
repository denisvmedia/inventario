package admin_test

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/admin"
)

// Deleting a user from the CLI deletes their profile photo too (#2894): a
// photo left in storage after the users row is gone is a personal image
// nobody can see or remove.
func TestService_DeleteUser_DeletesTheProfilePhoto(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	photo := env.storePhoto(c)
	c.Assert(env.blobExists(c, photo), qt.IsTrue)

	c.Assert(env.svc.DeleteUser(env.ctx, env.user.Email), qt.IsNil)

	_, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(env.blobExists(c, photo), qt.IsFalse)
}

// A user who still owns an item is refused before anything changes: they
// stay a member of their group, keep their sessions, and the item stays
// theirs.
func TestService_DeleteUser_RefusesAUserWhoOwnsAnItem(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	group := env.otherOwnersGroup(c)
	membership, err := env.fs.GroupMembershipRegistry.Create(env.ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.from},
		GroupID:             group.ID,
		MemberUserID:        env.user.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)
	item := env.seedCommodity(c, group)

	err = env.svc.DeleteUser(env.ctx, env.user.Email)
	c.Assert(err, qt.ErrorIs, admin.ErrUserOwnsContent)

	_, err = env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	_, err = env.fs.GroupMembershipRegistry.Get(env.ctx, membership.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("a refused delete must leave the user in their group"))
	_, err = env.fs.RefreshTokenRegistry.GetByTokenHash(env.ctx, "session-hash")
	c.Assert(err, qt.IsNil, qt.Commentf("a refused delete must leave the user's sessions"))
	stored, err := env.fs.CommodityRegistryFactory.CreateServiceRegistry().Get(env.ctx, item.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.CreatedByUserID, qt.Equals, env.user.ID)
}

// A user who is the only owner of a group that has other members is refused
// before anything changes, as self-service account deletion refuses them:
// the group would keep its members and have no owner. The group's creator
// left it after handing it over.
func TestService_DeleteUser_RefusesTheOnlyOwnerOfASharedGroup(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	group := env.otherOwnersGroup(c)
	ownership := env.addMember(c, group, env.user.ID, models.GroupRoleOwner)
	env.addMember(c, group, env.otherUser(c, "member@example.com").ID, models.GroupRoleUser)

	err := env.svc.DeleteUser(env.ctx, env.user.Email)
	c.Assert(err, qt.ErrorIs, admin.ErrUserSoleOwnerOfSharedGroup)
	c.Assert(err, qt.ErrorIs, services.ErrAccountSoleOwnerOfSharedGroup)
	c.Assert(err.Error(), qt.Contains, fmt.Sprintf("the group %q (%s)", group.Name, group.ID))

	_, err = env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	_, err = env.fs.GroupMembershipRegistry.Get(env.ctx, ownership.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("a refused delete must leave the group its owner"))
	_, err = env.fs.RefreshTokenRegistry.GetByTokenHash(env.ctx, "session-hash")
	c.Assert(err, qt.IsNil, qt.Commentf("a refused delete must leave the user's sessions"))
}

// One of two owners of a shared group is deleted: the group keeps an owner.
func TestService_DeleteUser_DeletesACoOwner(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	group := env.otherOwnersGroup(c)
	env.addMember(c, group, env.user.ID, models.GroupRoleOwner)
	env.addMember(c, group, env.otherUser(c, "co-owner@example.com").ID, models.GroupRoleOwner)

	c.Assert(env.svc.DeleteUser(env.ctx, env.user.Email), qt.IsNil)

	_, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func (e *moveEnv) addMember(c *qt.C, group *models.LocationGroup, userID string, role models.GroupRole) *models.GroupMembership {
	c.Helper()
	m, err := e.fs.GroupMembershipRegistry.Create(e.ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: group.TenantID},
		GroupID:             group.ID,
		MemberUserID:        userID,
		Role:                role,
	})
	c.Assert(err, qt.IsNil)
	return m
}

func (e *moveEnv) otherUser(c *qt.C, email string) *models.User {
	c.Helper()
	u, err := e.fs.UserRegistry.Create(e.ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: e.from},
		Email:               email,
		Name:                email,
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	return u
}
