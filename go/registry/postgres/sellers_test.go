package postgres_test

import (
	"testing"

	"go.5x5.cz/inventario/registry/registrytest"
)

func TestSellerContract_Postgres(t *testing.T) {
	registrytest.RunSellerContract(t, func(t *testing.T) registrytest.SellerEnv {
		fx := newTagPGFixture(t)
		return registrytest.SellerEnv{
			CtxA:         fx.ctxA,
			CtxB:         fx.ctxB,
			SellersA:     fx.groupASet.SellerRegistry,
			SellersB:     fx.groupBSet.SellerRegistry,
			CommoditiesA: fx.groupASet.CommodityRegistry,
			CommoditiesB: fx.groupBSet.CommodityRegistry,
			AreaA:        fx.areaAID,
			AreaB:        fx.areaBID,
		}
	})
}
