package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestCommoditySearchContract_Memory(t *testing.T) {
	registrytest.RunCommoditySearchContract(t, func(t *testing.T) registrytest.CommoditySearchEnv {
		c := qt.New(t)

		locFactory := memory.NewLocationRegistryFactory()
		areaFactory := memory.NewAreaRegistryFactory(locFactory)
		commodityFactory := memory.NewCommodityRegistryFactory(areaFactory)
		fileFactory := memory.NewFileRegistryFactory()
		tagFactory := memory.NewTagRegistryFactory(commodityFactory, fileFactory)

		ctx := appctx.WithUser(t.Context(), &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-1"},
				TenantID: "tenant-1",
			},
		})
		ctx = appctx.WithGroup(ctx, &models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "group-1"},
				TenantID: "tenant-1",
			},
			Slug:          "group-1",
			GroupCurrency: "USD",
		})

		loc, err := locFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Location{Name: "Home", Address: "addr"})
		c.Assert(err, qt.IsNil)
		area, err := areaFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Area{Name: "Shed", LocationID: loc.GetID()})
		c.Assert(err, qt.IsNil)

		return registrytest.CommoditySearchEnv{
			Ctx:         ctx,
			Commodities: commodityFactory.MustCreateUserRegistry(ctx),
			Tags:        tagFactory.MustCreateUserRegistry(ctx),
			AreaID:      area.GetID(),
		}
	})
}
