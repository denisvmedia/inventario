package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/admin"
)

// An operator's delete of a user who still owns an item is refused before
// anything changes: the user stays a member of the group and keeps the item
// and the invite they created.
func TestAdminDeleteUser_Postgres_RefusesAUserWhoOwnsAnItem(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)
	ctx := context.Background()
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	db := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := f.factorySet
	owner := f.user

	member, err := fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		Email:               "item-owning-member@test-org.com",
		Name:                "Item Owning Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	membership, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleAdmin,
	})
	c.Assert(err, qt.IsNil)

	memberCtx := loanCtxFor(member, f.groupAID)
	memberSet := postgres.NewRegistrySetWithUserAndGroupID(db, member.ID, member.TenantID, f.groupAID)
	ownersItem, err := f.groupASet.CommodityRegistry.Get(f.ctxA, f.commAID)
	c.Assert(err, qt.IsNil)
	itemID := seedTagCommodity(c, memberSet, memberCtx, *ownersItem.AreaID, "Member's drill")
	invite, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             f.groupAID,
		Token:               "member-invite-token",
		CreatedBy:           member.ID,
		ExpiresAt:           time.Now().Add(24 * time.Hour),
	})
	c.Assert(err, qt.IsNil)

	err = admin.NewServiceWithFactorySet(fs).DeleteUser(ctx, member.Email)
	c.Assert(err, qt.ErrorIs, admin.ErrUserOwnsContent)

	_, err = fs.UserRegistry.Get(ctx, member.ID)
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupMembershipRegistry.Get(ctx, membership.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("a refused delete must leave the user in the group"))
	_, err = fs.GroupInviteRegistry.Get(ctx, invite.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("a refused delete must leave the user's invites"))
	c.Check(creatorOf(c, db, "commodities", "id = $1", itemID), qt.Equals, member.ID)
}

// A user who owns nothing is deleted, membership and invites with them.
func TestAdminDeleteUser_Postgres_DeletesAMemberWhoOwnsNothing(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)
	ctx := context.Background()
	fs := f.factorySet

	member, err := fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.user.TenantID},
		Email:               "empty-member@test-org.com",
		Name:                "Empty Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	membership, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.user.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	c.Assert(admin.NewServiceWithFactorySet(fs).DeleteUser(ctx, member.Email), qt.IsNil)

	_, err = fs.UserRegistry.Get(ctx, member.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fs.GroupMembershipRegistry.Get(ctx, membership.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// An operator's delete of the only owner of a group that has other members
// is refused before anything changes: the group's creator handed it over and
// left, and the delete would leave it with a member and no owner.
func TestAdminDeleteUser_Postgres_RefusesTheOnlyOwnerOfASharedGroup(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)
	ctx := context.Background()
	fs := f.factorySet

	heir := createPGTenantUser(c, fs, f.user.TenantID, "handed-over-owner@test-org.com")
	member := createPGTenantUser(c, fs, f.user.TenantID, "remaining-member@test-org.com")
	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	group, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.user.TenantID},
		Name:                "Handed over",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           f.user.ID,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	ownership := addPGMember(c, fs, group, heir.ID, models.GroupRoleOwner)
	addPGMember(c, fs, group, member.ID, models.GroupRoleUser)

	err = admin.NewServiceWithFactorySet(fs).DeleteUser(ctx, heir.Email)
	c.Assert(err, qt.ErrorIs, admin.ErrUserSoleOwnerOfSharedGroup)
	c.Assert(err, qt.ErrorIs, services.ErrAccountSoleOwnerOfSharedGroup)
	c.Assert(err.Error(), qt.Contains, fmt.Sprintf("the group %q (%s)", group.Name, group.ID))

	_, err = fs.UserRegistry.Get(ctx, heir.ID)
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupMembershipRegistry.Get(ctx, ownership.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("a refused delete must leave the group its owner"))
}

func createPGTenantUser(c *qt.C, fs *registry.FactorySet, tenantID, email string) *models.User {
	c.Helper()
	u, err := fs.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               email,
		Name:                email,
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	return u
}
