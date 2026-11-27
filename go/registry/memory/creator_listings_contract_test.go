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

func TestCreatorListingsContract_Memory(t *testing.T) {
	registrytest.RunCreatorListingsContract(t, func(t *testing.T) registrytest.CreatorListingsEnv {
		c := qt.New(t)
		group := &models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "group-1"}, TenantID: "tenant-1"},
			Slug:                "group-1",
			GroupCurrency:       "USD",
		}
		member := func(id string) context.Context {
			return appctx.WithGroup(appctx.WithUser(t.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: id}, TenantID: "tenant-1"},
			}), group)
		}
		ctxA, ctxB := member("user-a"), member("user-b")
		fs := memory.NewFactorySet()
		set, err := fs.CreateUserRegistrySet(ctxA)
		c.Assert(err, qt.IsNil)
		loc, err := set.LocationRegistry.Create(ctxA, models.Location{Name: "Home", Address: "addr"})
		c.Assert(err, qt.IsNil)
		area, err := set.AreaRegistry.Create(ctxA, models.Area{Name: "Shed", LocationID: loc.GetID()})
		c.Assert(err, qt.IsNil)
		return registrytest.CreatorListingsEnv{
			Factories: fs, CtxA: ctxA, CtxB: ctxB, UserA: "user-a", UserB: "user-b",
			LocationID: loc.GetID(), LocationName: loc.Name, AreaID: area.GetID(), AreaName: area.Name,
		}
	})
}
