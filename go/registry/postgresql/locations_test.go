package postgresql_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgresql"
)

func setupTestLocationRegistry(t *testing.T) (*postgresql.LocationRegistry, func()) {
	// Skip if no PostgreSQL DSN is provided
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL integration tests: POSTGRES_TEST_DSN not set")
	}

	c := qt.New(t)

	// Create a test database
	pool, err := pgxpool.New(context.Background(), dsn)
	c.Assert(err, qt.IsNil)

	// Clean up any existing test tables
	_, err = pool.Exec(context.Background(), `
		DROP TABLE IF EXISTS locations CASCADE;
		DROP TABLE IF EXISTS areas CASCADE;
		DROP TABLE IF EXISTS commodities CASCADE;
		DROP TABLE IF EXISTS images CASCADE;
		DROP TABLE IF EXISTS invoices CASCADE;
		DROP TABLE IF EXISTS manuals CASCADE;
		DROP TABLE IF EXISTS settings CASCADE;
	`)
	c.Assert(err, qt.IsNil)

	// Initialize the schema
	err = postgresql.InitSchemaForTesting(pool)
	c.Assert(err, qt.IsNil)

	// Create a location registry
	locationRegistry := postgresql.NewLocationRegistry(pool)
	c.Assert(locationRegistry, qt.Not(qt.IsNil))

	// Return the registry and a cleanup function
	cleanup := func() {
		pool.Close()
	}

	return locationRegistry, cleanup
}

func TestLocationRegistry_HappyPath(t *testing.T) {
	t.Run("create, get, update, and delete location", func(t *testing.T) {
		c := qt.New(t)

		// Set up the test registry
		locationRegistry, cleanup := setupTestLocationRegistry(t)
		defer cleanup()

		// Create a location
		location := models.Location{
			Name:    "Test Location",
			Address: "123 Test Street",
		}

		// Create the location
		createdLocation, err := locationRegistry.Create(location)
		c.Assert(err, qt.IsNil)
		c.Assert(createdLocation.ID, qt.Not(qt.Equals), "")
		c.Assert(createdLocation.Name, qt.Equals, location.Name)
		c.Assert(createdLocation.Address, qt.Equals, location.Address)

		// Get the location
		retrievedLocation, err := locationRegistry.Get(createdLocation.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(retrievedLocation.ID, qt.Equals, createdLocation.ID)
		c.Assert(retrievedLocation.Name, qt.Equals, location.Name)
		c.Assert(retrievedLocation.Address, qt.Equals, location.Address)

		// Update the location
		updatedLocation := *retrievedLocation
		updatedLocation.Name = "Updated Location"
		updatedLocation.Address = "456 Updated Street"

		result, err := locationRegistry.Update(updatedLocation)
		c.Assert(err, qt.IsNil)
		c.Assert(result.ID, qt.Equals, updatedLocation.ID)
		c.Assert(result.Name, qt.Equals, updatedLocation.Name)
		c.Assert(result.Address, qt.Equals, updatedLocation.Address)

		// Get the updated location
		retrievedUpdatedLocation, err := locationRegistry.Get(updatedLocation.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(retrievedUpdatedLocation.ID, qt.Equals, updatedLocation.ID)
		c.Assert(retrievedUpdatedLocation.Name, qt.Equals, updatedLocation.Name)
		c.Assert(retrievedUpdatedLocation.Address, qt.Equals, updatedLocation.Address)

		// List locations
		locations, err := locationRegistry.List()
		c.Assert(err, qt.IsNil)
		c.Assert(locations, qt.HasLen, 1)
		c.Assert(locations[0].ID, qt.Equals, updatedLocation.ID)
		c.Assert(locations[0].Name, qt.Equals, updatedLocation.Name)
		c.Assert(locations[0].Address, qt.Equals, updatedLocation.Address)

		// Count locations
		count, err := locationRegistry.Count()
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 1)

		// Delete the location
		err = locationRegistry.Delete(updatedLocation.ID)
		c.Assert(err, qt.IsNil)

		// Verify the location is deleted
		_, err = locationRegistry.Get(updatedLocation.ID)
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(err.Error(), qt.Contains, "not found")

		// Verify the count is 0
		count, err = locationRegistry.Count()
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 0)
	})
}

func TestLocationRegistry_UnhappyPath(t *testing.T) {
	t.Run("get non-existent location", func(t *testing.T) {
		c := qt.New(t)

		// Set up the test registry
		locationRegistry, cleanup := setupTestLocationRegistry(t)
		defer cleanup()

		// Try to get a non-existent location
		_, err := locationRegistry.Get("non-existent-id")
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(err.Error(), qt.Contains, "not found")
	})

	t.Run("update non-existent location", func(t *testing.T) {
		c := qt.New(t)

		// Set up the test registry
		locationRegistry, cleanup := setupTestLocationRegistry(t)
		defer cleanup()

		// Try to update a non-existent location
		location := models.Location{
			EntityID: models.EntityID{ID: "non-existent-id"},
			Name:     "Test Location",
			Address:  "123 Test Street",
		}

		_, err := locationRegistry.Update(location)
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(err.Error(), qt.Contains, "not found")
	})

	t.Run("delete non-existent location", func(t *testing.T) {
		c := qt.New(t)

		// Set up the test registry
		locationRegistry, cleanup := setupTestLocationRegistry(t)
		defer cleanup()

		// Try to delete a non-existent location
		err := locationRegistry.Delete("non-existent-id")
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(err.Error(), qt.Contains, "not found")
	})

	t.Run("create location with invalid data", func(t *testing.T) {
		c := qt.New(t)

		// Set up the test registry
		locationRegistry, cleanup := setupTestLocationRegistry(t)
		defer cleanup()

		// Try to create a location with invalid data
		location := models.Location{
			Name:    "", // Empty name is invalid
			Address: "123 Test Street",
		}

		_, err := locationRegistry.Create(location)
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(err.Error(), qt.Contains, "validation failed")
	})
}
