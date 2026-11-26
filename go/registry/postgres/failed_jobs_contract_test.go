package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/registry/registrytest"
)

// The group scope is checked against two groups of one tenant. The
// thumbnail adapter reads a group scope under the background-worker role,
// whose policies let every row through, so its subtests pass only because
// the query filters by the file's group itself; the other adapters run
// under the application role with the group set and filter by group_id as
// well, so a database role that owns the tables, as in CI, cannot widen the
// scope either.
func TestFailedJobContract_Postgres(t *testing.T) {
	registrytest.RunFailedJobContract(t, newPostgresFailedJobEnv)
}

func newPostgresFailedJobEnv(t *testing.T) registrytest.FailedJobEnv {
	c := qt.New(t)

	groupASet, _ := setupTestRegistrySet(t)
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	factorySet := postgres.NewFactorySet(dbx)
	user := getTestUser(c, groupASet)

	serviceSet := factorySet.CreateServiceRegistrySet()
	groups, err := serviceSet.LocationGroupRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(groups, qt.HasLen, 1)
	groupAID := groups[0].ID

	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	groupB, err := serviceSet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Name:                "Test Group B",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)

	return registrytest.FailedJobEnv{
		Jobs:     factorySet.FailedJobRegistry,
		TenantID: user.TenantID,
		UserID:   user.ID,
		A: registrytest.FailedJobGroup{
			ID: groupAID, Ctx: loanCtxFor(user, groupAID), Set: groupASet,
		},
		B: registrytest.FailedJobGroup{
			ID: groupB.ID, Ctx: loanCtxFor(user, groupB.ID),
			Set: postgres.NewRegistrySetWithUserAndGroupID(dbx, user.ID, user.TenantID, groupB.ID),
		},
		IntakeService: factorySet.IntakeRegistryFactory.CreateServiceRegistry(),
	}
}
