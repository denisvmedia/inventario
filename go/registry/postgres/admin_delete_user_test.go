package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
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
