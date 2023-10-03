package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestMemoryAreaRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of LocationRegistry
	locationRegistry := memory.NewLocationRegistry()
	r := memory.NewAreaRegistry(locationRegistry)

	// Test area
	var area models.Area

	// Create a new location in the location registry
	var location models.Location
	createdLocation, err := locationRegistry.Create(location)
	c.Assert(err, qt.IsNil)
	area.LocationID = createdLocation.GetID()
	area.Name = "area1"

	// Create a new area in the registry
	createdArea, err := r.Create(area)
	c.Assert(err, qt.IsNil)
	c.Assert(createdArea, qt.Not(qt.IsNil))
	c.Assert(createdArea.LocationID, qt.Equals, area.LocationID)

	// Verify the count of areas in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestAreaRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of LocationRegistry
	locationRegistry := memory.NewLocationRegistry()
	r := memory.NewAreaRegistry(locationRegistry)

	// Create a test area without a location ID
	var area models.Area

	// Attempt to create the area - validation failure
	_, err := r.Create(area)
	valErrs := validation.Errors{}
	c.Assert(err, qt.ErrorAs, &valErrs)
	c.Assert(valErrs, qt.HasLen, 2)
	c.Assert(valErrs["location_id"], qt.Not(qt.IsNil))
	c.Assert(valErrs["location_id"].Error(), qt.Equals, "cannot be blank")
	c.Assert(valErrs["name"], qt.Not(qt.IsNil))
	c.Assert(valErrs["name"].Error(), qt.Equals, "cannot be blank")

	// Attempt to create the area in the registry and expect not found error
	area.Name = "area1"
	area.LocationID = "location1"
	_, err = r.Create(area)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(err, qt.ErrorMatches, "location not found.*")
}

func TestAreaRegistry_Commodities(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of AreaRegistry
	locationRegistry := memory.NewLocationRegistry()
	r := memory.NewAreaRegistry(locationRegistry)

	// Test area
	var area models.Area

	// Create a new location in the location registry
	var location models.Location
	createdLocation, err := locationRegistry.Create(location)
	c.Assert(err, qt.IsNil)
	area.LocationID = createdLocation.GetID()
	area.Name = "area1"

	// Create a new area in the registry
	createdArea, err := r.Create(area)
	c.Assert(err, qt.IsNil)

	// Add a commodity to the area
	r.AddCommodity(createdArea.ID, "commodity1")
	r.AddCommodity(createdArea.ID, "commodity2")

	// Get the commodities of the area
	commodities := r.GetCommodities(createdArea.ID)
	c.Assert(commodities, qt.Contains, "commodity1")
	c.Assert(commodities, qt.Contains, "commodity2")

	// Delete a commodity from the area
	r.DeleteCommodity(createdArea.ID, "commodity1")

	// Verify that the deleted commodity is not present in the area's commodities
	commodities = r.GetCommodities(createdArea.ID)
	c.Assert(commodities, qt.Not(qt.Contains), "commodity1")
	c.Assert(commodities, qt.Contains, "commodity2")
}

func TestAreaRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of AreaRegistry
	locationRegistry := memory.NewLocationRegistry()
	r := memory.NewAreaRegistry(locationRegistry)

	// Test area
	var area models.Area

	// Create a new location in the location registry
	var location models.Location
	createdLocation, err := locationRegistry.Create(location)
	c.Assert(err, qt.IsNil)
	area.LocationID = createdLocation.GetID()
	area.Name = "area1"

	// Create a new area in the registry
	createdArea, err := r.Create(area)
	c.Assert(err, qt.IsNil)

	// Verify that the area is there
	_, err = r.Get(createdArea.ID)
	c.Assert(err, qt.IsNil)

	// Delete a non-existing area from the registry
	err = r.Delete("non-existing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Delete the area from the registry
	err = r.Delete(createdArea.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the area is deleted
	_, err = r.Get(createdArea.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of areas in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
