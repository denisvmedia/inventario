package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func TestEntityService_DeleteLocationRecursive(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create registry set with proper dependencies
	registrySet := memory.NewRegistrySet()

	// Make registries user-aware
	userAwareAreaRegistry, err := registrySet.AreaRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)
	registrySet.AreaRegistry = userAwareAreaRegistry

	userAwareCommodityRegistry, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)
	registrySet.CommodityRegistry = userAwareCommodityRegistry

	// Create entity service
	entityService := services.NewEntityService(registrySet, "file://./test_uploads?create_dir=true")

	// Create test data hierarchy: Location -> Area -> Commodity
	location := models.Location{Name: "Test Location"}
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{Name: "Test Area", LocationID: createdLocation.ID}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		Name:   "Test Commodity",
		AreaID: createdArea.ID,
	}
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Verify the hierarchy exists
	areas, err := registrySet.LocationRegistry.GetAreas(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 1)
	c.Assert(areas[0], qt.Equals, createdArea.ID)

	commodities, err := registrySet.AreaRegistry.GetCommodities(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)
	c.Assert(commodities[0], qt.Equals, createdCommodity.ID)

	// Test that regular delete fails due to constraints
	err = registrySet.LocationRegistry.Delete(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "location has areas")

	// Test recursive delete succeeds
	err = entityService.DeleteLocationRecursive(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNil)

	// Verify everything is deleted
	_, err = registrySet.LocationRegistry.Get(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNotNil)

	_, err = registrySet.AreaRegistry.Get(ctx, createdArea.ID)
	c.Assert(err, qt.IsNotNil)

	_, err = registrySet.CommodityRegistry.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNotNil)
}

func TestEntityService_DeleteAreaRecursive(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create registry set with proper dependencies
	registrySet := memory.NewRegistrySet()

	// Make registries user-aware
	userAwareAreaRegistry, err := registrySet.AreaRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)
	registrySet.AreaRegistry = userAwareAreaRegistry

	userAwareCommodityRegistry, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)
	registrySet.CommodityRegistry = userAwareCommodityRegistry

	// Create entity service
	entityService := services.NewEntityService(registrySet, "file://./test_uploads?create_dir=true")

	// Create test data hierarchy: Location -> Area -> Commodity
	location := models.Location{Name: "Test Location"}
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{Name: "Test Area", LocationID: createdLocation.ID}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		Name:   "Test Commodity",
		AreaID: createdArea.ID,
	}
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Test that regular delete fails due to constraints
	err = registrySet.AreaRegistry.Delete(ctx, createdArea.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "area has commodities")

	// Test recursive delete succeeds
	err = entityService.DeleteAreaRecursive(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)

	// Verify area and commodity are deleted, but location remains
	_, err = registrySet.LocationRegistry.Get(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNil) // Location should still exist

	_, err = registrySet.AreaRegistry.Get(ctx, createdArea.ID)
	c.Assert(err, qt.IsNotNil) // Area should be deleted

	_, err = registrySet.CommodityRegistry.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNotNil) // Commodity should be deleted

	// Verify location no longer has areas
	areas, err := registrySet.LocationRegistry.GetAreas(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 0)
}
