package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestCustomFieldContract_Memory(t *testing.T) {
	registrytest.RunCustomFieldContract(t, func(t *testing.T) registrytest.CustomFieldEnv {
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

		return registrytest.CustomFieldEnv{
			CtxA:         ctxA,
			CtxB:         ctxB,
			FieldsA:      fs.CustomFieldRegistryFactory.MustCreateUserRegistry(ctxA),
			FieldsB:      fs.CustomFieldRegistryFactory.MustCreateUserRegistry(ctxB),
			TypesA:       fs.ItemTypeRegistryFactory.MustCreateUserRegistry(ctxA),
			CommoditiesA: fs.CommodityRegistryFactory.MustCreateUserRegistry(ctxA),
			CommoditiesB: fs.CommodityRegistryFactory.MustCreateUserRegistry(ctxB),
			AreaA:        areaFor(ctxA),
			AreaB:        areaFor(ctxB),
		}
	})
}
