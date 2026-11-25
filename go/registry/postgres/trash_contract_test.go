package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/registry/registrytest"
)

func TestTrashContract_Postgres(t *testing.T) {
	registrytest.RunTrashContract(t, newPostgresTrashEnv)
}

func TestCoverPhotoContract_Postgres(t *testing.T) {
	registrytest.RunCoverPhotoContract(t, newPostgresTrashEnv)
}

func TestCountEachContract_Postgres(t *testing.T) {
	registrytest.RunCountEachContract(t, newPostgresTrashEnv)
}

func TestMissingDataContract_Postgres(t *testing.T) {
	registrytest.RunMissingDataContract(t, newPostgresTrashEnv)
}

// newPostgresTrashEnv returns registries scoped to the test group, with one
// area to place items in.
func newPostgresTrashEnv(t *testing.T) registrytest.TrashEnv {
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
}
