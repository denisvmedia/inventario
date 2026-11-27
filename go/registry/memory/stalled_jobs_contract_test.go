package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestStalledJobContract_Memory(t *testing.T) {
	registrytest.RunStalledJobContract(t, newMemoryStalledJobEnv)
}

func newMemoryStalledJobEnv(t *testing.T) registrytest.StalledJobEnv {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	}
	ctx := appctx.WithGroup(appctx.WithUser(context.Background(), user), &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
		Slug:          "group-1",
		GroupCurrency: "USD",
	})
	set, err := fs.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	return registrytest.StalledJobEnv{
		Ctx:      ctx,
		Set:      set,
		Exports:  fs.ExportRegistryFactory.CreateServiceRegistry(),
		Restores: fs.RestoreOperationRegistryFactory.CreateServiceRegistry(),
	}
}
