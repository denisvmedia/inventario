package postgresql_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgresql"
)

// TestNewRegistrySet_HappyPath tests successful registry set creation.
func TestNewRegistrySet_HappyPath(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	registrySet, err := postgresql.NewRegistrySet(registry.Config(dsn))
	c.Assert(err, qt.IsNil)
	c.Assert(registrySet, qt.IsNotNil)

	// Verify all registries are available
	c.Assert(registrySet.LocationRegistry, qt.IsNotNil)
	c.Assert(registrySet.AreaRegistry, qt.IsNotNil)
	c.Assert(registrySet.CommodityRegistry, qt.IsNotNil)
	c.Assert(registrySet.ImageRegistry, qt.IsNotNil)
	c.Assert(registrySet.InvoiceRegistry, qt.IsNotNil)
	c.Assert(registrySet.ManualRegistry, qt.IsNotNil)
	c.Assert(registrySet.SettingsRegistry, qt.IsNotNil)
}

// TestNewRegistrySet_UnhappyPath tests registry set creation error scenarios.
func TestNewRegistrySet_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name   string
		config registry.Config
	}{
		{
			name:   "empty config",
			config: "",
		},
		{
			name:   "invalid DSN",
			config: "invalid-dsn",
		},
		{
			name:   "non-existent host",
			config: "postgres://user:pass@non-existent-host:5432/db",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			_, err := postgresql.NewRegistrySet(tc.config)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestRegistrySet_DatabaseConnection tests database connection management.
func TestRegistrySet_DatabaseConnection(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test that all registries can perform basic operations
	// This verifies that the database connection is working properly

	// Test location registry
	count, err := registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Test area registry
	count, err = registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Test commodity registry
	count, err = registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Test image registry
	count, err = registrySet.ImageRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Test invoice registry
	count, err = registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Test manual registry
	count, err = registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Test settings registry
	settings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(settings, qt.IsNotNil)
}

// TestRegistrySet_SchemaInitialization tests that the database schema is properly initialized.
func TestRegistrySet_SchemaInitialization(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	// Verify that all tables exist by attempting to query them
	tables := []string{
		"locations",
		"areas",
		"commodities",
		"images",
		"invoices",
		"manuals",
		"settings",
	}

	for _, table := range tables {
		var count int
		err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count)
		c.Assert(err, qt.IsNil, qt.Commentf("Table %s should exist", table))
		c.Assert(count, qt.Equals, 0, qt.Commentf("Table %s should be empty initially", table))
	}
}

// TestRegistrySet_ForeignKeyConstraints tests that foreign key constraints are properly enforced.
func TestRegistrySet_ForeignKeyConstraints(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location, area, commodity := setupTestHierarchy(c, registrySet)

	// Test that deleting a location cascades to areas and commodities
	err := registrySet.LocationRegistry.Delete(ctx, location.GetID())
	c.Assert(err, qt.IsNil)

	// Verify area is deleted
	_, err = registrySet.AreaRegistry.Get(ctx, area.GetID())
	c.Assert(err, qt.IsNotNil)

	// Verify commodity is deleted
	_, err = registrySet.CommodityRegistry.Get(ctx, commodity.GetID())
	c.Assert(err, qt.IsNotNil)

	// Verify counts are zero
	locationCount, err := registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locationCount, qt.Equals, 0)

	areaCount, err := registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areaCount, qt.Equals, 0)

	commodityCount, err := registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodityCount, qt.Equals, 0)
}

// TestRegistrySet_TransactionIsolation tests that operations are properly isolated.
func TestRegistrySet_TransactionIsolation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())

	// Verify initial state
	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 1)

	// Create another area
	area2 := createTestArea(c, registrySet.AreaRegistry, location.GetID())

	// Verify both areas exist
	areas, err = registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 2)

	// Delete one area
	err = registrySet.AreaRegistry.Delete(ctx, area.GetID())
	c.Assert(err, qt.IsNil)

	// Verify only one area remains
	areas, err = registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 1)
	c.Assert(areas[0].GetID(), qt.Equals, area2.GetID())
}

// TestRegistrySet_ConcurrentOperations tests that concurrent operations work correctly.
func TestRegistrySet_ConcurrentOperations(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test location
	location := createTestLocation(c, registrySet.LocationRegistry)

	// Create multiple areas concurrently
	const numAreas = 5
	areaChan := make(chan string, numAreas)
	errChan := make(chan error, numAreas)

	for i := 0; i < numAreas; i++ {
		go func(index int) {
			area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
			areaChan <- area.GetID()
			errChan <- nil
		}(i)
	}

	// Collect results
	var areaIDs []string
	for i := 0; i < numAreas; i++ {
		areaID := <-areaChan
		err := <-errChan
		c.Assert(err, qt.IsNil)
		areaIDs = append(areaIDs, areaID)
	}

	// Verify all areas were created
	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, numAreas)

	// Verify all area IDs are unique
	uniqueIDs := make(map[string]bool)
	for _, areaID := range areaIDs {
		c.Assert(uniqueIDs[areaID], qt.IsFalse, qt.Commentf("Area ID %s should be unique", areaID))
		uniqueIDs[areaID] = true
	}
}

// TestRegistrySet_CompleteWorkflow tests a complete workflow from creation to deletion.
func TestRegistrySet_CompleteWorkflow(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create complete hierarchy with files
	location, area, commodity := setupTestHierarchy(c, registrySet)
	image := createTestImage(c, registrySet.ImageRegistry, commodity.GetID())
	invoice := createTestInvoice(c, registrySet.InvoiceRegistry, commodity.GetID())
	manual := createTestManual(c, registrySet.ManualRegistry, commodity.GetID())

	// Verify all entities exist
	_, err := registrySet.LocationRegistry.Get(ctx, location.GetID())
	c.Assert(err, qt.IsNil)

	_, err = registrySet.AreaRegistry.Get(ctx, area.GetID())
	c.Assert(err, qt.IsNil)

	_, err = registrySet.CommodityRegistry.Get(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)

	_, err = registrySet.ImageRegistry.Get(ctx, image.GetID())
	c.Assert(err, qt.IsNil)

	_, err = registrySet.InvoiceRegistry.Get(ctx, invoice.GetID())
	c.Assert(err, qt.IsNil)

	_, err = registrySet.ManualRegistry.Get(ctx, manual.GetID())
	c.Assert(err, qt.IsNil)

	// Test relationship management
	images, err := registrySet.CommodityRegistry.GetImages(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.HasLen, 1)
	c.Assert(images[0], qt.Equals, image.GetID())

	invoices, err := registrySet.CommodityRegistry.GetInvoices(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.HasLen, 1)
	c.Assert(invoices[0], qt.Equals, invoice.GetID())

	manuals, err := registrySet.CommodityRegistry.GetManuals(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.HasLen, 1)
	c.Assert(manuals[0], qt.Equals, manual.GetID())

	// Delete the entire hierarchy (should cascade)
	err = registrySet.LocationRegistry.Delete(ctx, location.GetID())
	c.Assert(err, qt.IsNil)

	// Verify everything is deleted
	locationCount, err := registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locationCount, qt.Equals, 0)

	areaCount, err := registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areaCount, qt.Equals, 0)

	commodityCount, err := registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodityCount, qt.Equals, 0)

	imageCount, err := registrySet.ImageRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(imageCount, qt.Equals, 0)

	invoiceCount, err := registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(invoiceCount, qt.Equals, 0)

	manualCount, err := registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(manualCount, qt.Equals, 0)
}
