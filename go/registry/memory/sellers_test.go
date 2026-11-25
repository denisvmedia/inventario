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

func TestSellerContract_Memory(t *testing.T) {
	registrytest.RunSellerContract(t, func(t *testing.T) registrytest.SellerEnv {
		c := qt.New(t)

		fs := memory.NewFactorySet()
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-1"},
				TenantID: "tenant-1",
			},
		}
		ctxFor := func(groupID string) context.Context {
			ctx := appctx.WithUser(t.Context(), user)
			return appctx.WithGroup(ctx, &models.LocationGroup{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: groupID},
					TenantID: "tenant-1",
				},
				Slug:          groupID,
				GroupCurrency: "USD",
			})
		}
		ctxA, ctxB := ctxFor("group-a"), ctxFor("group-b")
		areaFor := func(ctx context.Context) string {
			loc, err := fs.LocationRegistryFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Location{Name: "Home", Address: "addr"})
			c.Assert(err, qt.IsNil)
			area, err := fs.AreaRegistryFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Area{Name: "Shed", LocationID: loc.GetID()})
			c.Assert(err, qt.IsNil)
			return area.GetID()
		}

		return registrytest.SellerEnv{
			CtxA:         ctxA,
			CtxB:         ctxB,
			SellersA:     fs.SellerRegistryFactory.MustCreateUserRegistry(ctxA),
			SellersB:     fs.SellerRegistryFactory.MustCreateUserRegistry(ctxB),
			CommoditiesA: fs.CommodityRegistryFactory.MustCreateUserRegistry(ctxA),
			CommoditiesB: fs.CommodityRegistryFactory.MustCreateUserRegistry(ctxB),
			AreaA:        areaFor(ctxA),
			AreaB:        areaFor(ctxB),
		}
	})
}
