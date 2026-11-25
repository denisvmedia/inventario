package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/registry/registrytest"
)

func TestCommodityDraftContract_Postgres(t *testing.T) {
	registrytest.RunCommodityDraftContract(t, newPostgresDraftEnv)
}

// newPostgresDraftEnv returns registries scoped to the test group, with one
// area to place items in.
func newPostgresDraftEnv(t *testing.T) registrytest.DraftEnv {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	location := createTestLocation(c, fx.set)
	area := createTestArea(c, fx.set, location.ID)
	drafts, err := fx.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(fx.ctx)
	c.Assert(err, qt.IsNil)
	return registrytest.DraftEnv{
		Ctx:         fx.ctx,
		Drafts:      drafts,
		Commodities: fx.set.CommodityRegistry,
		Files:       fx.set.FileRegistry,
		Sellers:     fx.set.SellerRegistry,
		AreaID:      area.ID,
	}
}
