package memory_test

import (
	"context"
	"testing"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func memberCtx(userID, groupID string) context.Context {
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "tenant-1",
		},
	})
	return appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: groupID},
			TenantID: "tenant-1",
		},
		Slug: groupID,
	})
}

func TestSavedViewContract_Memory(t *testing.T) {
	registrytest.RunSavedViewContract(t, func(*testing.T) registrytest.SavedViewEnv {
		return registrytest.SavedViewEnv{
			Views:         memory.NewSavedViewRegistryFactory(),
			Ctx:           memberCtx("alice", "group-1"),
			OtherCtx:      memberCtx("bob", "group-1"),
			OtherGroupCtx: memberCtx("alice", "group-2"),
		}
	})
}
