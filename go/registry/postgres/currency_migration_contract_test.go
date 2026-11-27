package postgres_test

import (
	"testing"

	"go.5x5.cz/inventario/registry/registrytest"
)

func TestCurrencyMigrationCleanupContract_Postgres(t *testing.T) {
	registrytest.RunCurrencyMigrationCleanupContract(t, func(t *testing.T) registrytest.CurrencyMigrationEnv {
		fx := newFileLinkPGFixture(t)
		return registrytest.CurrencyMigrationEnv{
			Ctx:        fx.ctx,
			GroupID:    fx.groupID,
			Migrations: fx.factorySet.CurrencyMigrationRegistryFactory.MustCreateUserRegistry(fx.ctx),
			Service:    fx.factorySet.CurrencyMigrationRegistryFactory.CreateServiceRegistry(),
			Groups:     fx.set.LocationGroupRegistry,
		}
	})
}
