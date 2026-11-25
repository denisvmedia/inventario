package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestItemTypeContract_Memory(t *testing.T) {
	registrytest.RunItemTypeContract(t, func(t *testing.T) registrytest.ItemTypeEnv {
		c := qt.New(t)

		fs := memory.NewFactorySet()
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-1"},
				TenantID: "tenant-1",
			},
		}
		newGroup := func(slug string) *models.LocationGroup {
			g, err := fs.LocationGroupRegistry.Create(t.Context(), models.LocationGroup{
				TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
				Name:                slug,
				Slug:                slug,
				Status:              models.LocationGroupStatusActive,
				CreatedBy:           user.ID,
				GroupCurrency:       "USD",
			})
			c.Assert(err, qt.IsNil)
			return g
		}
		groupA, groupB := newGroup("group-a"), newGroup("group-b")
		ctxFor := func(g *models.LocationGroup) context.Context {
			return appctx.WithGroup(appctx.WithUser(t.Context(), user), g)
		}
		ctxA, ctxB := ctxFor(groupA), ctxFor(groupB)
		areaFor := func(ctx context.Context) string {
			loc, err := fs.LocationRegistryFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Location{Name: "Home", Address: "addr"})
			c.Assert(err, qt.IsNil)
			area, err := fs.AreaRegistryFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Area{Name: "Shed", LocationID: loc.GetID()})
			c.Assert(err, qt.IsNil)
			return area.GetID()
		}

		return registrytest.ItemTypeEnv{
			CtxA:         ctxA,
			CtxB:         ctxB,
			TypesA:       fs.ItemTypeRegistryFactory.MustCreateUserRegistry(ctxA),
			TypesB:       fs.ItemTypeRegistryFactory.MustCreateUserRegistry(ctxB),
			ServiceTypes: fs.ItemTypeRegistryFactory.CreateServiceRegistry(),
			CommoditiesA: fs.CommodityRegistryFactory.MustCreateUserRegistry(ctxA),
			CommoditiesB: fs.CommodityRegistryFactory.MustCreateUserRegistry(ctxB),
			AreaA:        areaFor(ctxA),
			AreaB:        areaFor(ctxB),
			GroupA:       groupA.ID,
			GroupB:       groupB.ID,
		}
	})
}

// The group purge drops a group's item types, which Delete alone could not
// do for the last one, and leaves the other groups' types alone.
func TestGroupPurger_DropsItemTypes_Memory(t *testing.T) {
	c := qt.New(t)

	fs := memory.NewFactorySet()
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	}
	group := func(id string) context.Context {
		return appctx.WithGroup(appctx.WithUser(t.Context(), user), &models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: id}, TenantID: "tenant-1"},
		})
	}
	ctxA, ctxB := group("group-a"), group("group-b")
	for _, ctx := range []context.Context{ctxA, ctxB} {
		_, err := fs.ItemTypeRegistryFactory.MustCreateUserRegistry(ctx).EnsureBuiltins(ctx)
		c.Assert(err, qt.IsNil)
	}

	c.Assert(fs.GroupPurger.PurgeGroupDependents(t.Context(), "tenant-1", "group-a"), qt.IsNil)

	countA, err := fs.ItemTypeRegistryFactory.MustCreateUserRegistry(ctxA).Count(ctxA)
	c.Assert(err, qt.IsNil)
	c.Check(countA, qt.Equals, 0)
	countB, err := fs.ItemTypeRegistryFactory.MustCreateUserRegistry(ctxB).Count(ctxB)
	c.Assert(err, qt.IsNil)
	c.Check(countB, qt.Equals, len(models.BuiltinItemTypes))

	c.Assert(fs.TenantPurger.PurgeTenantDependents(t.Context(), "tenant-1"), qt.IsNil)
	all, err := fs.ItemTypeRegistryFactory.CreateServiceRegistry().List(t.Context())
	c.Assert(err, qt.IsNil)
	c.Check(all, qt.HasLen, 0)
}

// A seeded built-in type is authored by the group's creator, so another
// member who triggered the seeding keeps no content that blocks deleting
// their account.
func TestItemTypes_SeededTypesBelongToGroupCreator_Memory(t *testing.T) {
	c := qt.New(t)

	fs := memory.NewFactorySet()
	g, err := fs.LocationGroupRegistry.Create(t.Context(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Name:                "Home",
		Slug:                "home",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "owner",
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	member := &models.User{TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "member"}, TenantID: "tenant-1"}}
	ctx := appctx.WithGroup(appctx.WithUser(t.Context(), member), g)

	list, err := fs.ItemTypeRegistryFactory.MustCreateUserRegistry(ctx).List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, len(models.BuiltinItemTypes))
	for _, it := range list {
		c.Check(it.CreatedByUserID, qt.Equals, "owner")
		c.Check(it.TenantID, qt.Equals, "tenant-1")
	}

	owns, err := fs.UserContentOwnershipChecker.HasRetainedOwnedContent(t.Context(), "tenant-1", "member", nil)
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsFalse)
	owns, err = fs.UserContentOwnershipChecker.HasRetainedOwnedContent(t.Context(), "tenant-1", "owner", nil)
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsTrue)
}

var _ registry.ItemTypeRegistry = (*memory.ItemTypeRegistry)(nil)

// A member's item types in a shared group do not block the member's account
// deletion: they stay with the group and go to its owner.
func TestItemTypes_MemberTypesGoToTheOwner_Memory(t *testing.T) {
	c := qt.New(t)

	fs := memory.NewFactorySet()
	g, err := fs.LocationGroupRegistry.Create(t.Context(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Name:                "Home",
		Slug:                "home",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "owner",
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	for _, m := range []struct {
		user string
		role models.GroupRole
	}{{"member", models.GroupRoleUser}, {"owner", models.GroupRoleOwner}} {
		_, err := fs.GroupMembershipRegistry.Create(t.Context(), models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
			GroupID:             g.ID,
			MemberUserID:        m.user,
			Role:                m.role,
		})
		c.Assert(err, qt.IsNil)
	}
	member := &models.User{TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "member"}, TenantID: "tenant-1"}}
	ctx := appctx.WithGroup(appctx.WithUser(t.Context(), member), g)
	types := fs.ItemTypeRegistryFactory.MustCreateUserRegistry(ctx)
	vehicle, err := types.Create(ctx, models.ItemType{Name: "Vehicle"})
	c.Assert(err, qt.IsNil)
	c.Assert(vehicle.CreatedByUserID, qt.Equals, "member")

	owns, err := fs.UserContentOwnershipChecker.HasRetainedOwnedContent(t.Context(), "tenant-1", "member", nil)
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsFalse, qt.Commentf("item types are group configuration"))

	c.Assert(fs.UserPurger.PurgeUserDependents(t.Context(), "tenant-1", "member"), qt.IsNil)
	got, err := types.Get(ctx, vehicle.ID)
	c.Assert(err, qt.IsNil)
	c.Check(got.CreatedByUserID, qt.Equals, "owner")
}
