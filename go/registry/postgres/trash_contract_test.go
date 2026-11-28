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

func TestDraftMarkContract_Postgres(t *testing.T) {
	registrytest.RunDraftMarkContract(t, newPostgresTrashEnv)
}

func TestBulkPatchContract_Postgres(t *testing.T) {
	registrytest.RunBulkPatchContract(t, newPostgresTrashEnv)
}

func TestLocationFilterContract_Postgres(t *testing.T) {
	registrytest.RunLocationFilterContract(t, newPostgresTrashEnv)
}

func TestItemDatesContract_Postgres(t *testing.T) {
	registrytest.RunItemDatesContract(t, newPostgresTrashEnv)
}

// newPostgresTrashEnv returns registries scoped to the test group, with two
// areas of one location to place items in and an area of a second location.
func newPostgresTrashEnv(t *testing.T) registrytest.TrashEnv {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	location := createTestLocation(c, fx.set)
	area := createTestArea(c, fx.set, location.ID)
	other := createTestArea(c, fx.set, location.ID)
	elsewhere := createTestLocation(c, fx.set)
	elsewhereArea := createTestArea(c, fx.set, elsewhere.ID)
	return registrytest.TrashEnv{
		Ctx:          fx.ctx,
		Commodities:  fx.set.CommodityRegistry,
		Files:        fx.set.FileRegistry,
		ServiceFiles: fx.factorySet.FileRegistryFactory.CreateServiceRegistry(),
		AreaID:       area.ID,
		OtherAreaID:  other.ID,

		LocationID:          location.ID,
		ElsewhereLocationID: elsewhere.ID,
		ElsewhereAreaID:     elsewhereArea.ID,
	}
}
