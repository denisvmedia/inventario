package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestItemTypeContract_Postgres(t *testing.T) {
	registrytest.RunItemTypeContract(t, func(t *testing.T) registrytest.ItemTypeEnv {
		fx := newTagPGFixture(t)
		return registrytest.ItemTypeEnv{
			CtxA:         fx.ctxA,
			CtxB:         fx.ctxB,
			TypesA:       fx.groupASet.ItemTypeRegistry,
			TypesB:       fx.groupBSet.ItemTypeRegistry,
			ServiceTypes: fx.factorySet.ItemTypeRegistryFactory.CreateServiceRegistry(),
			CommoditiesA: fx.groupASet.CommodityRegistry,
			DraftsA:      fx.groupASet.CommodityDraftRegistry,
			CommoditiesB: fx.groupBSet.CommodityRegistry,
			AreaA:        fx.areaAID,
			AreaB:        fx.areaBID,
			GroupA:       fx.groupAID,
			GroupB:       fx.groupBID,
		}
	})
}

// The group and tenant purges drop item types, which Delete alone could not
// do for a group's last one, and the ownership check sees the seeded rows
// as the group creator's.
func TestItemTypes_PurgeAndOwnership_Postgres(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	_, err := fx.groupASet.ItemTypeRegistry.EnsureBuiltins(fx.ctxA)
	c.Assert(err, qt.IsNil)
	_, err = fx.groupBSet.ItemTypeRegistry.EnsureBuiltins(fx.ctxB)
	c.Assert(err, qt.IsNil)

	list, err := fx.groupASet.ItemTypeRegistry.List(fx.ctxA)
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, len(models.BuiltinItemTypes))
	for _, it := range list {
		c.Check(it.CreatedByUserID, qt.Equals, fx.user.ID)
		c.Check(it.TenantID, qt.Equals, fx.user.TenantID)
	}

	c.Assert(fx.factorySet.GroupPurger.PurgeGroupDependents(t.Context(), fx.user.TenantID, fx.groupAID), qt.IsNil)
	countA, err := fx.groupASet.ItemTypeRegistry.Count(fx.ctxA)
	c.Assert(err, qt.IsNil)
	c.Check(countA, qt.Equals, 0)
	countB, err := fx.groupBSet.ItemTypeRegistry.Count(fx.ctxB)
	c.Assert(err, qt.IsNil)
	c.Check(countB, qt.Equals, len(models.BuiltinItemTypes))

	owns, err := fx.factorySet.UserContentOwnershipChecker.HasRetainedOwnedContent(t.Context(), fx.user.TenantID, fx.user.ID, []string{fx.groupAID, fx.groupBID})
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsFalse, qt.Commentf("item types in purged groups are not retained content"))
}

// A member's item types and custom fields in a shared group do not block the
// member's account deletion: the pre-check leaves them out, and the user
// purge hands them, template entries included, to the group's owner, so the
// users row can go.
func TestItemTypes_MemberTypesGoToTheOwner_Postgres(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := appctx.WithUser(context.Background(), fx.user)
	tenantID := fx.user.TenantID

	memberModel := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "item-types-member@test-org.com",
		Name:                "Member",
		IsActive:            true,
	}
	c.Assert(memberModel.SetPassword("Password123"), qt.IsNil)
	member, err := fx.factorySet.UserRegistry.Create(ctx, memberModel)
	c.Assert(err, qt.IsNil)

	memberships := fx.factorySet.GroupMembershipRegistry
	if _, err := memberships.GetByGroupAndUser(ctx, fx.groupAID, fx.user.ID); err != nil {
		_, err = memberships.Create(ctx, models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
			GroupID:             fx.groupAID,
			MemberUserID:        fx.user.ID,
			Role:                models.GroupRoleOwner,
		})
		c.Assert(err, qt.IsNil)
	}
	_, err = memberships.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             fx.groupAID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	memberSet := postgres.NewRegistrySetWithUserAndGroupID(fx.dbx, member.ID, tenantID, fx.groupAID)
	memberCtx := tagCtxFor(member, fx.groupAID)
	vehicle, err := memberSet.ItemTypeRegistry.Create(memberCtx, models.ItemType{Name: "Vehicle"})
	c.Assert(err, qt.IsNil)
	c.Assert(vehicle.CreatedByUserID, qt.Equals, member.ID)
	plate, err := memberSet.CustomFieldRegistry.Create(memberCtx, models.CustomFieldDefinition{
		Name: "Plate", Type: models.CustomFieldTypeText,
		Templates: []models.CustomFieldTemplate{{ItemTypeID: vehicle.ID}},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(plate.CreatedByUserID, qt.Equals, member.ID)

	owns, err := fx.factorySet.UserContentOwnershipChecker.HasRetainedOwnedContent(t.Context(), tenantID, member.ID, nil)
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsFalse, qt.Commentf("item types and custom fields are group configuration"))

	c.Assert(fx.factorySet.UserPurger.PurgeUserDependents(t.Context(), tenantID, member.ID), qt.IsNil)
	got, err := fx.groupASet.ItemTypeRegistry.Get(fx.ctxA, vehicle.ID)
	c.Assert(err, qt.IsNil)
	c.Check(got.CreatedByUserID, qt.Equals, fx.user.ID)
	gotField, err := fx.groupASet.CustomFieldRegistry.Get(fx.ctxA, plate.ID)
	c.Assert(err, qt.IsNil)
	c.Check(gotField.CreatedByUserID, qt.Equals, fx.user.ID)
	c.Check(gotField.Templates, qt.HasLen, 1)

	c.Check(fx.factorySet.UserRegistry.Delete(ctx, member.ID), qt.IsNil)
}

// Without an owner, the types go to an admin before an earlier member; among
// members of one rank the one who joined first takes them.
func TestItemTypes_HandoverPicksAdminThenJoinOrder_Postgres(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := appctx.WithUser(context.Background(), fx.user)
	tenantID := fx.user.TenantID
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	newUser := func(name string) *models.User {
		m := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
			Email:               "handover-" + name + "@test-org.com",
			Name:                name,
			IsActive:            true,
		}
		c.Assert(m.SetPassword("Password123"), qt.IsNil)
		u, err := fx.factorySet.UserRegistry.Create(ctx, m)
		c.Assert(err, qt.IsNil)
		return u
	}
	services := fx.factorySet.CreateServiceRegistrySet()
	newGroup := func(name string) string {
		g, err := services.LocationGroupRegistry.Create(ctx, models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
			Name:                name,
			Slug:                must.Must(models.GenerateGroupSlug()),
			Status:              models.LocationGroupStatusActive,
			CreatedBy:           fx.user.ID,
			GroupCurrency:       models.Currency("USD"),
		})
		c.Assert(err, qt.IsNil)
		return g.ID
	}
	join := func(groupID string, u *models.User, role models.GroupRole, joined time.Time) {
		_, err := fx.factorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
			GroupID:             groupID,
			MemberUserID:        u.ID,
			Role:                role,
			JoinedAt:            joined,
		})
		c.Assert(err, qt.IsNil)
	}

	leaver, early, late, admin := newUser("leaver"), newUser("early"), newUser("late"), newUser("admin")

	withAdmin := newGroup("With an admin")
	join(withAdmin, leaver, models.GroupRoleUser, t0.Add(-time.Hour))
	join(withAdmin, early, models.GroupRoleUser, t0)
	join(withAdmin, admin, models.GroupRoleAdmin, t0.Add(time.Hour))

	membersOnly := newGroup("Members only")
	join(membersOnly, leaver, models.GroupRoleUser, t0.Add(-time.Hour))
	join(membersOnly, late, models.GroupRoleUser, t0.Add(time.Hour))
	join(membersOnly, early, models.GroupRoleViewer, t0)

	created := map[string]*models.ItemType{}
	for _, groupID := range []string{withAdmin, membersOnly} {
		set := postgres.NewRegistrySetWithUserAndGroupID(fx.dbx, leaver.ID, tenantID, groupID)
		it, err := set.ItemTypeRegistry.Create(tagCtxFor(leaver, groupID), models.ItemType{Name: "Vehicle"})
		c.Assert(err, qt.IsNil)
		created[groupID] = it
	}

	c.Assert(fx.factorySet.UserPurger.PurgeUserDependents(t.Context(), tenantID, leaver.ID), qt.IsNil)

	for groupID, heir := range map[string]string{withAdmin: admin.ID, membersOnly: early.ID} {
		set := postgres.NewRegistrySetWithUserAndGroupID(fx.dbx, fx.user.ID, tenantID, groupID)
		got, err := set.ItemTypeRegistry.Get(tagCtxFor(fx.user, groupID), created[groupID].ID)
		c.Assert(err, qt.IsNil)
		c.Check(got.CreatedByUserID, qt.Equals, heir, qt.Commentf("group %s", groupID))
	}
	c.Check(fx.factorySet.UserRegistry.Delete(ctx, leaver.ID), qt.IsNil)
}
