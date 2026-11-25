package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestSavedViewContract_Postgres(t *testing.T) {
	registrytest.RunSavedViewContract(t, func(t *testing.T) registrytest.SavedViewEnv {
		c := qt.New(t)
		fx := newTagPGFixture(t)
		bob := mustCreateUser(c, context.Background(), fx.factorySet, fx.user.TenantID, "saved-views-bob@test-org.com")
		return registrytest.SavedViewEnv{
			Views:         fx.factorySet.SavedViewRegistryFactory,
			Ctx:           fx.ctxA,
			OtherCtx:      tagCtxFor(bob, fx.groupAID),
			OtherGroupCtx: fx.ctxB,
		}
	})
}

// A member's saved views, shared ones included, go with the member, and so
// do the member's placements of other members' shared views: both user
// columns are NOT NULL foreign keys to users with NO ACTION, so a row left
// behind would block the final DELETE FROM users. Other members' placements
// of the purged member's shared views go with those views.
func TestUserPurger_Postgres_PurgesSavedViews(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := context.Background()
	bob := mustCreateUser(c, ctx, fx.factorySet, fx.user.TenantID, "saved-views-purge-bob@test-org.com")
	bobCtx := tagCtxFor(bob, fx.groupAID)

	views := fx.factorySet.SavedViewRegistryFactory
	pinned := &registry.SavedViewPlacementPatch{ShowInSidebar: new(true)}
	create := func(ctx context.Context, name string, shared bool) *models.SavedView {
		v, err := views.MustCreateUserRegistry(ctx).CreateChecked(ctx, models.SavedView{
			Name:         name,
			Icon:         models.SavedViewIconBookmark,
			Entity:       models.SavedViewEntityCommodities,
			QueryVersion: models.SavedViewQueryVersion,
			Shared:       shared,
		}, pinned, nil)
		c.Assert(err, qt.IsNil)
		return v
	}
	create(bobCtx, "Bob private", false)
	bobShared := create(bobCtx, "Bob shared", true)
	aliceView := create(fx.ctxA, "Alice", true)
	_, err := views.MustCreateUserRegistry(bobCtx).SetPlacement(bobCtx, aliceView.ID, *pinned)
	c.Assert(err, qt.IsNil)
	_, err = views.MustCreateUserRegistry(fx.ctxA).SetPlacement(fx.ctxA, bobShared.ID, *pinned)
	c.Assert(err, qt.IsNil)

	c.Assert(fx.factorySet.UserPurger.PurgeUserDependents(ctx, bob.TenantID, bob.ID), qt.IsNil)

	all, err := views.CreateServiceRegistry().List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)
	c.Assert(all[0].ID, qt.Equals, aliceView.ID)
	placements, err := views.MustCreateUserRegistry(fx.ctxA).Placements(fx.ctxA)
	c.Assert(err, qt.IsNil)
	c.Assert(placements, qt.HasLen, 1)
	c.Assert(placements[aliceView.ID].ShowInSidebar, qt.IsTrue)

	c.Assert(fx.factorySet.UserRegistry.Delete(ctx, bob.ID), qt.IsNil)
	_, err = fx.factorySet.UserRegistry.Get(ctx, bob.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
