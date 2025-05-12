package boltdb_test

import (
	"os"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

func setupTestLocationRegistry(t *testing.T) (*boltdb.LocationRegistry, func()) {
	c := qt.New(t)

	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "boltdb-test-*")
	c.Assert(err, qt.IsNil)

	// Create a new database in the temporary directory
	db, err := dbx.NewDB(tempDir, "test.db").Open()
	c.Assert(err, qt.IsNil)

	// Create a location registry
	locationRegistry := boltdb.NewLocationRegistry(db)

	// Return the registry and a cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return locationRegistry, cleanup
}

func TestLocationRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of LocationRegistry
	r, cleanup := setupTestLocationRegistry(t)
	defer cleanup()

	// Create a test location
	location := models.Location{
		Name: "Test Location",
	}

	// Create a new location in the registry
	createdLocation, err := r.Create(location)
	c.Assert(err, qt.IsNil)
	c.Assert(createdLocation, qt.Not(qt.IsNil))
	c.Assert(createdLocation.Name, qt.Equals, location.Name)

	// Verify the count of locations in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestLocationRegistry_Areas(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of LocationRegistry
	r, cleanup := setupTestLocationRegistry(t)
	defer cleanup()

	// Create a test location
	location := models.Location{
		Name: "Test Location",
	}

	// Create a new location in the registry
	createdLocation, err := r.Create(location)
	c.Assert(err, qt.IsNil)

	// Add an area to the location
	err = r.AddArea(createdLocation.GetID(), "area1")
	c.Assert(err, qt.IsNil)
	err = r.AddArea(createdLocation.GetID(), "area2")
	c.Assert(err, qt.IsNil)

	// Get the areas of the location
	areas, err := r.GetAreas(createdLocation.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.Contains, "area1")
	c.Assert(areas, qt.Contains, "area2")

	// Delete an area from the location
	err = r.DeleteArea(createdLocation.GetID(), "area1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted area is not present in the location's areas
	areas, err = r.GetAreas(createdLocation.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.Not(qt.Contains), "area1")
	c.Assert(areas, qt.Contains, "area2")
}

func TestLocationRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of LocationRegistry
	r, cleanup := setupTestLocationRegistry(t)
	defer cleanup()

	// Create a test location
	location := models.Location{
		Name: "Test Location",
	}

	// Create a new location in the registry
	createdLocation, err := r.Create(location)
	c.Assert(err, qt.IsNil)

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

func TestLocationRegistry_Delete_ErrCases(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of LocationRegistry
	r, cleanup := setupTestLocationRegistry(t)
	defer cleanup()

	// Create a test location
	location := models.Location{
		Name: "Test Location",
	}

	// Create a new location in the registry
	createdLocation, err := r.Create(location)
	c.Assert(err, qt.IsNil)

	// Delete a non-existing location from the registry
	err = r.Delete("non-existing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Try to delete a location with areas
	err = r.AddArea(createdLocation.GetID(), "area1")
	c.Assert(err, qt.IsNil)
	err = r.Delete(createdLocation.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrCannotDelete)

	// Delete the area from the location
	err = r.DeleteArea(createdLocation.GetID(), "area1")
	c.Assert(err, qt.IsNil)

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