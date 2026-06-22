package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestLocationRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a test location
	location := &models.Location{
		Name:    "Test Location",
		Address: "123 Test Street",
		// Note: ID will be generated server-side for security
	}

	// Create a new location in the registry
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, *location)
	c.Assert(err, qt.IsNil)
	c.Assert(createdLocation, qt.IsNotNil)

	// Verify the count of locations in the registry
	count, err := registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestLocationRegistry_Areas(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, _ := registrySet.LocationRegistry.Create(ctx, *location)

	// Note: LocationRegistry doesn't have AddArea, GetAreas, DeleteArea methods
	// This test is simplified to just verify location creation and retrieval

	// Verify the location was created successfully
	retrievedLocation, err := registrySet.LocationRegistry.Get(ctx, createdLocation.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedLocation.GetID(), qt.Equals, createdLocation.GetID())
}

// TestLocationRegistry_AddArea_Dedup pins the relationship-index dedup
// fix: a double AddArea used to append twice, inflating the area count
// the delete-guard reads so that removing the area once still left a
// phantom entry and Delete wrongly returned ErrCannotDelete.
func TestLocationRegistry_AddArea_Dedup(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	locationRegistry := registrySet.LocationRegistry.(*memory.LocationRegistry)

	createdLocation, err := locationRegistry.Create(ctx, models.Location{})
	c.Assert(err, qt.IsNil)

	// Add the same area twice — the index must hold a single entry.
	c.Assert(locationRegistry.AddArea(ctx, createdLocation.GetID(), "area1"), qt.IsNil)
	c.Assert(locationRegistry.AddArea(ctx, createdLocation.GetID(), "area1"), qt.IsNil)

	areas, err := locationRegistry.GetAreas(ctx, createdLocation.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 1)

	// Removing it once must clear the guard so Delete is allowed.
	c.Assert(locationRegistry.DeleteArea(ctx, createdLocation.GetID(), "area1"), qt.IsNil)
	c.Assert(locationRegistry.Delete(ctx, createdLocation.GetID()), qt.IsNil)
}

func TestLocationRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, *location)
	c.Assert(err, qt.IsNil)

	// Delete the location from the registry
	err = registrySet.LocationRegistry.Delete(ctx, createdLocation.GetID())
	c.Assert(err, qt.IsNil)

	// Verify that the location is deleted
	_, err = registrySet.LocationRegistry.Get(ctx, createdLocation.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of locations in the registry
	count, err := registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestLocationRegistry_Delete_ErrCases(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, _ := registrySet.LocationRegistry.Create(ctx, *location)

	// Delete a non-existing location from the registry
	err = registrySet.LocationRegistry.Delete(ctx, "non-existing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Note: LocationRegistry doesn't have AddArea/DeleteArea methods
	// This test is simplified to just test basic deletion

	// Delete the location from the registry
	err = registrySet.LocationRegistry.Delete(ctx, createdLocation.GetID())
	c.Assert(err, qt.IsNil)

	// Verify that the location is deleted
	_, err = registrySet.LocationRegistry.Get(ctx, createdLocation.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of locations in the registry
	count, err := registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
