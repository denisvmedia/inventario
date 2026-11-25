package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/registry/registrytest"
)

func TestIntakeContract_Postgres(t *testing.T) {
	registrytest.RunIntakeContract(t, newPostgresIntakeEnv)
}

// newPostgresIntakeEnv returns the registries of the test group.
func newPostgresIntakeEnv(t *testing.T) registrytest.IntakeEnv {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	intake, err := fx.factorySet.IntakeRegistryFactory.CreateUserRegistry(fx.ctx)
	c.Assert(err, qt.IsNil)
	return registrytest.IntakeEnv{
		Ctx:     fx.ctx,
		Intake:  intake,
		Service: fx.factorySet.IntakeRegistryFactory.CreateServiceRegistry(),
		Files:   fx.set.FileRegistry,
		Drafts:  fx.set.CommodityDraftRegistry,
	}
}
