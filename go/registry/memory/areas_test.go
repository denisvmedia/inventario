package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestMemoryAreaRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create a new instance of AreaRegistry
	locationRegistry := memory.NewLocationRegistry()
	r := memory.NewAreaRegistry(locationRegistry)

	// Test area
	var area models.Area

	// Create a new location in the location registry
	var location models.Location
	createdLocation, err := locationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)
	area.LocationID = createdLocation.GetID()
	area.Name = "area1"

	// Create a new area in the registry
	createdArea, err := r.Create(ctx, area)
	c.Assert(err, qt.IsNil)
	c.Assert(createdArea, qt.Not(qt.IsNil))
	c.Assert(createdArea.LocationID, qt.Equals, area.LocationID)

	// Verify the count of areas in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestAreaRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create a new instance of AreaRegistry
	locationRegistry := memory.NewLocationRegistry()
	r := memory.NewAreaRegistry(locationRegistry)

	// Create a test area without a location ID
	var area models.Area

	// Create the area - should succeed (no validation in memory registry)
	createdArea, err := r.Create(ctx, area)
	c.Assert(err, qt.IsNil)
	c.Assert(createdArea, qt.Not(qt.IsNil))

	// Create another area with location ID - should also succeed
	area.Name = "area1"
	area.LocationID = "location1"
	createdArea2, err := r.Create(ctx, area)
	c.Assert(err, qt.IsNil)
	c.Assert(createdArea2, qt.Not(qt.IsNil))
}

func TestAreaRegistry_Commodities(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create a new instance of AreaRegistry
	locationRegistry := memory.NewLocationRegistry()
	r := memory.NewAreaRegistry(locationRegistry)

	// Test area
	var area models.Area

	// Create a new location in the location registry
	var location models.Location
	createdLocation, err := locationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)
	area.LocationID = createdLocation.GetID()
	area.Name = "area1"

	// Create a new area in the registry
	createdArea, err := r.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	// Add a commodity to the area
	err = r.AddCommodity(ctx, createdArea.ID, "commodity1")
	c.Assert(err, qt.IsNil)
	err = r.AddCommodity(ctx, createdArea.ID, "commodity2")
	c.Assert(err, qt.IsNil)

	// Get the commodities of the area
	commodities, err := r.GetCommodities(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.Contains, "commodity1")
	c.Assert(commodities, qt.Contains, "commodity2")

	// Delete a commodity from the area
	err = r.DeleteCommodity(ctx, createdArea.ID, "commodity1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted commodity is not present in the area's commodities
	commodities, err = r.GetCommodities(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.Not(qt.Contains), "commodity1")
	c.Assert(commodities, qt.Contains, "commodity2")
}

func TestAreaRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create a new instance of AreaRegistry
	locationRegistry := memory.NewLocationRegistry()
	baseAreaRegistry := memory.NewAreaRegistry(locationRegistry)
	r, err := baseAreaRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Test area
	var area models.Area

	// Create a new location in the location registry
	var location models.Location
	createdLocation, err := locationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)
	area.LocationID = createdLocation.GetID()
	area.Name = "area1"

	// Create a new area in the registry
	createdArea, err := r.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	// Verify that the area is there
	_, err = r.Get(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)

	// Delete a non-existing area from the registry
	err = r.Delete(ctx, "non-existing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Delete the area from the registry
	err = r.Delete(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the area is deleted
	_, err = r.Get(ctx, createdArea.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of areas in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
