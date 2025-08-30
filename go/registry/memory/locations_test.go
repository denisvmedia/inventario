package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestLocationRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of LocationRegistry
	r := memory.NewLocationRegistry()

	// Create a test location
	location := &models.Location{
		Name:    "Test Location",
		Address: "123 Test Street",
		// Note: ID will be generated server-side for security
	}

	// Create a new location in the registry
	createdLocation, err := r.Create(ctx, *location)
	c.Assert(err, qt.IsNil)
	c.Assert(createdLocation, qt.Not(qt.IsNil))

	// Verify the count of locations in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestLocationRegistry_Areas(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of LocationRegistry
	r := memory.NewLocationRegistry()

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, _ := r.Create(ctx, *location)

	// Add an area to the location
	err := r.AddArea(ctx, createdLocation.GetID(), "area1")
	c.Assert(err, qt.IsNil)
	err = r.AddArea(ctx, createdLocation.GetID(), "area2")
	c.Assert(err, qt.IsNil)

	// Get the areas of the location
	areas, err := r.GetAreas(ctx, createdLocation.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.Contains, "area1")
	c.Assert(areas, qt.Contains, "area2")

	// Delete an area from the location
	err = r.DeleteArea(ctx, createdLocation.GetID(), "area1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted area is not present in the location's areas
	areas, err = r.GetAreas(ctx, createdLocation.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.Not(qt.Contains), "area1")
	c.Assert(areas, qt.Contains, "area2")
}

func TestLocationRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of LocationRegistry
	r := memory.NewLocationRegistry()

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, err := r.Create(ctx, *location)
	c.Assert(err, qt.IsNil)

	// Delete the location from the registry
	r.Delete(ctx, createdLocation.GetID())
	c.Assert(err, qt.IsNil)

	// Verify that the location is deleted
	_, err = r.Get(ctx, createdLocation.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of locations in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestLocationRegistry_Delete_ErrCases(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of LocationRegistry
	r := memory.NewLocationRegistry()

	// Create a test location
	location := models.WithID("location1", &models.Location{})

	// Create a new location in the registry
	createdLocation, _ := r.Create(ctx, *location)

	// Delete a non-existing location from the registry
	err := r.Delete(ctx, "non-existing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Try to delete a location with areas
	err = r.AddArea(ctx, createdLocation.GetID(), "area1")
	c.Assert(err, qt.IsNil)
	err = r.Delete(ctx, createdLocation.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrCannotDelete)

	// Delete the area from the location
	err = r.DeleteArea(ctx, createdLocation.GetID(), "area1")
	c.Assert(err, qt.IsNil)

	// Delete the location from the registry
	err = r.Delete(ctx, createdLocation.GetID())
	c.Assert(err, qt.IsNil)

	// Verify that the location is deleted
	_, err = r.Get(ctx, createdLocation.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of locations in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
