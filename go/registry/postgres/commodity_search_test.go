package postgres_test

import (
	"testing"

	"go.5x5.cz/inventario/registry/registrytest"
)

func TestCommoditySearchContract_Postgres(t *testing.T) {
	registrytest.RunCommoditySearchContract(t, func(t *testing.T) registrytest.CommoditySearchEnv {
		fx := newTagPGFixture(t)
		return registrytest.CommoditySearchEnv{
			Ctx:         fx.ctxA,
			Commodities: fx.groupASet.CommodityRegistry,
			Tags:        fx.groupASet.TagRegistry,
			AreaID:      fx.areaAID,
		}
	})
}
