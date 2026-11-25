package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/registry/registrytest"
)

func TestTrashContract_Postgres(t *testing.T) {
	registrytest.RunTrashContract(t, func(t *testing.T) registrytest.TrashEnv {
		c := qt.New(t)
		fx := newFileLinkPGFixture(t)
		location := createTestLocation(c, fx.set)
		area := createTestArea(c, fx.set, location.ID)
		return registrytest.TrashEnv{
			Ctx:          fx.ctx,
			Commodities:  fx.set.CommodityRegistry,
			Files:        fx.set.FileRegistry,
			ServiceFiles: fx.factorySet.FileRegistryFactory.CreateServiceRegistry(),
			AreaID:       area.ID,
		}
	})
}
