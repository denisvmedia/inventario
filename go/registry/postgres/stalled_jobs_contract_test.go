package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestStalledJobContract_Postgres(t *testing.T) {
	registrytest.RunStalledJobContract(t, newPostgresStalledJobEnv)
}

func newPostgresStalledJobEnv(t *testing.T) registrytest.StalledJobEnv {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	factorySet := postgres.NewFactorySet(sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx"))
	user := getTestUser(c, set)

	groups, err := factorySet.CreateServiceRegistrySet().LocationGroupRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(groups, qt.HasLen, 1)

	return registrytest.StalledJobEnv{
		Ctx:      loanCtxFor(user, groups[0].ID),
		Set:      set,
		Exports:  factorySet.ExportRegistryFactory.CreateServiceRegistry(),
		Restores: factorySet.RestoreOperationRegistryFactory.CreateServiceRegistry(),
	}
}
