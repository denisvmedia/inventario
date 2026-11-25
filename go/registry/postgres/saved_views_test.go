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

// A member's saved views, shared ones included, go with the member: the
// column is a NOT NULL foreign key to users with NO ACTION, so a view left
// behind would block the final DELETE FROM users.
func TestUserPurger_Postgres_PurgesSavedViews(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := context.Background()
	bob := mustCreateUser(c, ctx, fx.factorySet, fx.user.TenantID, "saved-views-purge-bob@test-org.com")
	bobCtx := tagCtxFor(bob, fx.groupAID)

	views := fx.factorySet.SavedViewRegistryFactory
	create := func(ctx context.Context, name string, shared bool) *models.SavedView {
		v, err := views.MustCreateUserRegistry(ctx).Create(ctx, models.SavedView{
			Name:         name,
			Icon:         models.SavedViewIconBookmark,
			Entity:       models.SavedViewEntityCommodities,
			QueryVersion: models.SavedViewQueryVersion,
			Shared:       shared,
		})
		c.Assert(err, qt.IsNil)
		return v
	}
	create(bobCtx, "Bob private", false)
	create(bobCtx, "Bob shared", true)
	aliceView := create(fx.ctxA, "Alice", false)

	c.Assert(fx.factorySet.UserPurger.PurgeUserDependents(ctx, bob.TenantID, bob.ID), qt.IsNil)

	all, err := views.CreateServiceRegistry().List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)
	c.Assert(all[0].ID, qt.Equals, aliceView.ID)

	c.Assert(fx.factorySet.UserRegistry.Delete(ctx, bob.ID), qt.IsNil)
	_, err = fx.factorySet.UserRegistry.Get(ctx, bob.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
