package registry_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func TestMemoryLocationRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryLocationRegistry
	r := registry.NewMemoryLocationRegistry()

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, err := r.Create(*location)
	c.Assert(err, qt.IsNil)
	c.Assert(createdLocation, qt.Not(qt.IsNil))

	// Verify the count of locations in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestMemoryLocationRegistry_Areas(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryLocationRegistry
	r := registry.NewMemoryLocationRegistry()

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, _ := r.Create(*location)

	// Add an area to the location
	r.AddArea(createdLocation.GetID(), "area1")
	r.AddArea(createdLocation.GetID(), "area2")

	// Get the areas of the location
	areas := r.GetAreas(createdLocation.GetID())
	c.Assert(areas, qt.Contains, "area1")
	c.Assert(areas, qt.Contains, "area2")

	// Delete an area from the location
	r.DeleteArea(createdLocation.GetID(), "area1")

	// Verify that the deleted area is not present in the location's areas
	areas = r.GetAreas(createdLocation.GetID())
	c.Assert(areas, qt.Not(qt.Contains), "area1")
	c.Assert(areas, qt.Contains, "area2")
}

func TestMemoryLocationRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryLocationRegistry
	r := registry.NewMemoryLocationRegistry()

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, _ := r.Create(*location)

	// Delete the location from the registry
	err := r.Delete(createdLocation.GetID())
	c.Assert(err, qt.IsNil)

	// Verify that the location is deleted
	_, err = r.Get(createdLocation.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of locations in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestMemoryLocationRegistry_Delete_ErrCases(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryLocationRegistry
	r := registry.NewMemoryLocationRegistry()

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, _ := r.Create(*location)

	// Delete a non-existing location from the registry
	err := r.Delete("non-existing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Try to delete a location with areas
	r.AddArea(createdLocation.GetID(), "area1")
	err = r.Delete(createdLocation.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrCannotDelete)

	// Delete the area from the location
	r.DeleteArea(createdLocation.GetID(), "area1")

	// Delete the location from the registry
	err = r.Delete(createdLocation.GetID())
	c.Assert(err, qt.IsNil)

	// Verify that the location is deleted
	_, err = r.Get(createdLocation.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of locations in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
