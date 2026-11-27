package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestCurrencyMigrationCleanupContract_Memory(t *testing.T) {
	registrytest.RunCurrencyMigrationCleanupContract(t, func(t *testing.T) registrytest.CurrencyMigrationEnv {
		c := qt.New(t)

		fs := memory.NewFactorySet()
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-1"},
				TenantID: "tenant-1",
			},
		}
		group, err := fs.LocationGroupRegistry.Create(t.Context(), models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
			Name:                "Home",
			Slug:                "home",
			Status:              models.LocationGroupStatusActive,
			CreatedBy:           user.ID,
			GroupCurrency:       "USD",
		})
		c.Assert(err, qt.IsNil)
		ctx := appctx.WithGroup(appctx.WithUser(t.Context(), user), group)

		return registrytest.CurrencyMigrationEnv{
			Ctx:        ctx,
			GroupID:    group.ID,
			Migrations: fs.CurrencyMigrationRegistryFactory.MustCreateUserRegistry(ctx),
			Service:    fs.CurrencyMigrationRegistryFactory.CreateServiceRegistry(),
			Groups:     fs.LocationGroupRegistry,
		}
	})
}
