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
	c.Assert(createdLocation, qt.Not(qt.IsNil))

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
