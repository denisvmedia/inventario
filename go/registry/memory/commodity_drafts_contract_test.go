package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestCommodityDraftContract_Memory(t *testing.T) {
	registrytest.RunCommodityDraftContract(t, newMemoryDraftEnv)
}

// newMemoryDraftEnv returns registries scoped to one empty group, with one
// area to place items in.
func newMemoryDraftEnv(t *testing.T) registrytest.DraftEnv {
	c := qt.New(t)

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

	fs := memory.NewFactorySet()
	set, err := fs.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	loc, err := set.LocationRegistry.Create(ctx, models.Location{Name: "Home", Address: "addr"})
	c.Assert(err, qt.IsNil)
	area, err := set.AreaRegistry.Create(ctx, models.Area{Name: "Shed", LocationID: loc.GetID()})
	c.Assert(err, qt.IsNil)

	return registrytest.DraftEnv{
		Ctx:         ctx,
		Drafts:      set.CommodityDraftRegistry,
		Commodities: set.CommodityRegistry,
		Files:       set.FileRegistry,
		Sellers:     set.SellerRegistry,
		Fields:      set.CustomFieldRegistry,
		AreaID:      area.GetID(),
	}
}
