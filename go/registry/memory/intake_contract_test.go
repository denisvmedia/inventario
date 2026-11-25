package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestIntakeContract_Memory(t *testing.T) {
	registrytest.RunIntakeContract(t, newMemoryIntakeEnv)
}

// newMemoryIntakeEnv returns the registries of one empty group.
func newMemoryIntakeEnv(t *testing.T) registrytest.IntakeEnv {
	c := qt.New(t)

	ctx := appctx.WithUser(t.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
		Slug:          "group-1",
		GroupCurrency: "USD",
	})

	fs := memory.NewFactorySet()
	set, err := fs.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	return registrytest.IntakeEnv{
		Ctx:     ctx,
		Intake:  set.IntakeRegistry,
		Service: fs.IntakeRegistryFactory.CreateServiceRegistry(),
		Files:   set.FileRegistry,
	}
}
