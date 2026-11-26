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

func TestFailedJobContract_Memory(t *testing.T) {
	registrytest.RunFailedJobContract(t, newMemoryFailedJobEnv)
}

// newMemoryFailedJobEnv returns two empty groups of one tenant with the same
// member.
func newMemoryFailedJobEnv(t *testing.T) registrytest.FailedJobEnv {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	}
	group := func(id string) registrytest.FailedJobGroup {
		ctx := appctx.WithGroup(appctx.WithUser(context.Background(), user), &models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: id},
				TenantID: "tenant-1",
			},
			Slug:          id,
			GroupCurrency: "USD",
		})
		set, err := fs.CreateUserRegistrySet(ctx)
		c.Assert(err, qt.IsNil)
		return registrytest.FailedJobGroup{ID: id, Ctx: ctx, Set: set}
	}
	return registrytest.FailedJobEnv{
		Jobs:          fs.FailedJobRegistry,
		TenantID:      "tenant-1",
		UserID:        user.ID,
		A:             group("group-a"),
		B:             group("group-b"),
		IntakeService: fs.IntakeRegistryFactory.CreateServiceRegistry(),
	}
}
