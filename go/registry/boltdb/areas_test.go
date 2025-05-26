package boltdb_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

func setupTestAreaRegistry(t *testing.T) (*boltdb.AreaRegistry, *boltdb.LocationRegistry, func()) {
	c := qt.New(t)

	// Create a temporary directory for the test database
	tempDir := c.TempDir()

	// Create a new database in the temporary directory
	db, err := dbx.NewDB(tempDir, "test.db").Open()
	c.Assert(err, qt.IsNil)

	// Create a location registry
	locationRegistry := boltdb.NewLocationRegistry(db)

	// Create an area registry
	areaRegistry := boltdb.NewAreaRegistry(db, locationRegistry)

	// Return the registries and a cleanup function
	cleanup := func() {
		err = db.Close()
		c.Assert(err, qt.IsNil)
	}

	return areaRegistry, locationRegistry, cleanup
}

func TestAreaRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of AreaRegistry
	areaRegistry, locationRegistry, cleanup := setupTestAreaRegistry(t)
	defer cleanup()

	// Test area
	var area models.Area

	// Create a new location in the location registry
	var location models.Location
	location.Name = "Test Location"
	createdLocation, err := locationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)
	area.LocationID = createdLocation.GetID()
	area.Name = "area1"

	// Create a new area in the registry
	createdArea, err := areaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)
	c.Assert(createdArea, qt.Not(qt.IsNil))
	c.Assert(createdArea.LocationID, qt.Equals, area.LocationID)

	// Verify the count of areas in the registry
	count, err := areaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestAreaRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of AreaRegistry
	areaRegistry, _, cleanup := setupTestAreaRegistry(t)
	defer cleanup()

	// Create a test area without a location ID
	var area models.Area

	// Attempt to create the area - validation failure
	_, err := areaRegistry.Create(ctx, area)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Contains, "Name")

	// Attempt to create the area in the registry and expect not found error
	area.Name = "area1"
	area.LocationID = "location1"
	_, err = areaRegistry.Create(ctx, area)
	c.Assert(err, qt.Not(qt.IsNil))
}

func TestAreaRegistry_Commodities(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of AreaRegistry
	areaRegistry, locationRegistry, cleanup := setupTestAreaRegistry(t)
	defer cleanup()

	// Test area
	var area models.Area

	// Create a new location in the location registry
	var location models.Location
	location.Name = "Test Location"
	createdLocation, err := locationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)
	area.LocationID = createdLocation.GetID()
	area.Name = "area1"

	// Create a new area in the registry
	createdArea, err := areaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	// Add a commodity to the area
	err = areaRegistry.AddCommodity(ctx, createdArea.ID, "commodity1")
	c.Assert(err, qt.IsNil)
	err = areaRegistry.AddCommodity(ctx, createdArea.ID, "commodity2")
	c.Assert(err, qt.IsNil)

	// Get the commodities of the area
	commodities, err := areaRegistry.GetCommodities(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.Contains, "commodity1")
	c.Assert(commodities, qt.Contains, "commodity2")

	// Delete a commodity from the area
	err = areaRegistry.DeleteCommodity(ctx, createdArea.ID, "commodity1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted commodity is not present in the area's commodities
	commodities, err = areaRegistry.GetCommodities(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.Not(qt.Contains), "commodity1")
	c.Assert(commodities, qt.Contains, "commodity2")
}

func TestAreaRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of AreaRegistry
	areaRegistry, locationRegistry, cleanup := setupTestAreaRegistry(t)
	defer cleanup()

	// Test area
	var area models.Area

	// Create a new location in the location registry
	var location models.Location
	location.Name = "Test Location"
	createdLocation, err := locationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)
	area.LocationID = createdLocation.GetID()
	area.Name = "area1"

	// Create a new area in the registry
	createdArea, err := areaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	// Verify that the area is there
	_, err = areaRegistry.Get(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)

	// Delete a non-existing area from the registry
	err = areaRegistry.Delete(ctx, "non-existing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Delete the area from the registry
	err = areaRegistry.Delete(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the area is deleted
	_, err = areaRegistry.Get(ctx, createdArea.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of areas in the registry
	count, err := areaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
