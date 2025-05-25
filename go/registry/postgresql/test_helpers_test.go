package postgresql_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgresql"

)

// skipIfNoPostgreSQL checks if PostgreSQL is available for testing and skips the test if not.
// It checks for the POSTGRES_TEST_DSN environment variable and attempts to connect.
func skipIfNoPostgreSQL(t *testing.T) string {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL tests: POSTGRES_TEST_DSN environment variable not set")
	}

	// Test connection
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("Skipping PostgreSQL tests: failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("Skipping PostgreSQL tests: failed to ping database: %v", err)
	}

	return dsn
}

// applyMigrationsDirectly applies migrations directly using SQL
func applyMigrationsDirectly(pool *pgxpool.Pool) error {
	// Create schema_migrations table
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	// Apply initial schema migration
	_, err = pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS locations (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			address TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS areas (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			location_id TEXT NOT NULL REFERENCES locations(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS commodities (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			short_name TEXT,
			type TEXT NOT NULL,
			area_id TEXT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
			count INTEGER NOT NULL DEFAULT 1,
			original_price DECIMAL(15,2),
			original_price_currency TEXT,
			converted_original_price DECIMAL(15,2),
			current_price DECIMAL(15,2),
			serial_number TEXT,
			extra_serial_numbers JSONB,
			part_numbers JSONB,
			tags JSONB,
			status TEXT NOT NULL,
			purchase_date TEXT,
			registered_date TEXT,
			last_modified_date TEXT,
			urls JSONB,
			comments TEXT,
			draft BOOLEAN NOT NULL DEFAULT FALSE
		);

		CREATE TABLE IF NOT EXISTS images (
			id TEXT PRIMARY KEY,
			commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			original_path TEXT NOT NULL,
			ext TEXT NOT NULL,
			mime_type TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS invoices (
			id TEXT PRIMARY KEY,
			commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			original_path TEXT NOT NULL,
			ext TEXT NOT NULL,
			mime_type TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS manuals (
			id TEXT PRIMARY KEY,
			commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			original_path TEXT NOT NULL,
			ext TEXT NOT NULL,
			mime_type TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS settings (
			id TEXT PRIMARY KEY DEFAULT 'settings',
			data JSONB NOT NULL
		);

		-- Insert default settings if they don't exist
		INSERT INTO settings (id, data)
		VALUES ('settings', '{}')
		ON CONFLICT (id) DO NOTHING;
	`)
	if err != nil {
		return err
	}

	// Record migration as applied
	_, err = pool.Exec(context.Background(), `
		INSERT INTO schema_migrations (version, description, applied_at)
		VALUES (1, 'Initial schema', NOW())
		ON CONFLICT (version) DO NOTHING;
	`)
	if err != nil {
		return err
	}

	return nil
}

// logActiveConnections logs the number of active connections for debugging
func logActiveConnections(pool *pgxpool.Pool) {
	if pool == nil {
		return
	}

	var count int
	err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()").Scan(&count)
	if err == nil {
		fmt.Printf("Active connections: %d\n", count)
	}
}

// setupTestDB creates a clean test database with initialized schema.
// Returns the connection pool and a cleanup function.
func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	// Create connection pool
	pool, err := pgxpool.New(context.Background(), dsn)
	c.Assert(err, qt.IsNil)

	// Clean up any existing test tables
	_, err = pool.Exec(context.Background(), `
		DROP TABLE IF EXISTS images CASCADE;
		DROP TABLE IF EXISTS invoices CASCADE;
		DROP TABLE IF EXISTS manuals CASCADE;
		DROP TABLE IF EXISTS commodities CASCADE;
		DROP TABLE IF EXISTS areas CASCADE;
		DROP TABLE IF EXISTS locations CASCADE;
		DROP TABLE IF EXISTS settings CASCADE;
		DROP TABLE IF EXISTS schema_migrations CASCADE;
	`)
	c.Assert(err, qt.IsNil)

	// Initialize schema by applying migrations directly
	err = applyMigrationsDirectly(pool)
	c.Assert(err, qt.IsNil)

	cleanup := func() {
		if pool != nil {
			pool.Close()
		}
	}

	return pool, cleanup
}

// setupTestRegistrySet creates a complete registry set with clean database.
// Returns the registry set and a cleanup function.
func setupTestRegistrySet(t *testing.T) (*registry.Set, func()) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	// Create connection pool
	pool, err := pgxpool.New(context.Background(), dsn)
	c.Assert(err, qt.IsNil)

	// Clean up any existing test tables
	_, err = pool.Exec(context.Background(), `
		DROP TABLE IF EXISTS images CASCADE;
		DROP TABLE IF EXISTS invoices CASCADE;
		DROP TABLE IF EXISTS manuals CASCADE;
		DROP TABLE IF EXISTS commodities CASCADE;
		DROP TABLE IF EXISTS areas CASCADE;
		DROP TABLE IF EXISTS locations CASCADE;
		DROP TABLE IF EXISTS settings CASCADE;
		DROP TABLE IF EXISTS schema_migrations CASCADE;
	`)
	c.Assert(err, qt.IsNil)

	// Initialize schema by applying migrations directly
	err = applyMigrationsDirectly(pool)
	c.Assert(err, qt.IsNil)

	// Create registry set (this will initialize schema)
	registrySet, err := postgresql.NewRegistrySet(registry.Config(dsn))
	c.Assert(err, qt.IsNil)

	cleanup := func() {
		if pool != nil {
			pool.Close()
		}
	}

	return registrySet, cleanup
}

// createTestLocation creates a test location for use in tests.
func createTestLocation(c *qt.C, locationRegistry registry.LocationRegistry) *models.Location {
	ctx := context.Background()
	location := models.Location{
		Name:    "Test Location",
		Address: "123 Test Street",
	}

	createdLocation, err := locationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)
	c.Assert(createdLocation, qt.IsNotNil)

	return createdLocation
}

// createTestArea creates a test area for use in tests.
func createTestArea(c *qt.C, areaRegistry registry.AreaRegistry, locationID string) *models.Area {
	ctx := context.Background()
	area := models.Area{
		Name:       "Test Area",
		LocationID: locationID,
	}

	createdArea, err := areaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)
	c.Assert(createdArea, qt.IsNotNil)

	return createdArea
}

// setupMainCurrency sets up the main currency for tests
func setupMainCurrency(c *qt.C, settingsRegistry registry.SettingsRegistry) {
	ctx := context.Background()

	// Set main currency to USD
	err := settingsRegistry.Patch(ctx, "system.main_currency", "USD")
	c.Assert(err, qt.IsNil)
}

// createTestCommodity creates a test commodity for use in tests.
func createTestCommodity(c *qt.C, registrySet *registry.Set, areaID string) *models.Commodity {
	ctx := context.Background()

	// Ensure main currency is set
	setupMainCurrency(c, registrySet.SettingsRegistry)

	commodity := models.Commodity{
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 areaID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.NewFromFloat(90.00),
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           models.ToPDate("2023-01-01"),
		RegisteredDate:         models.ToPDate("2023-01-02"),
		LastModifiedDate:       models.ToPDate("2023-01-03"),
		Draft:                  false,
	}

	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)
	c.Assert(createdCommodity, qt.IsNotNil)

	return createdCommodity
}

// createTestImage creates a test image for use in tests.
func createTestImage(c *qt.C, imageRegistry registry.ImageRegistry, commodityID string) *models.Image {
	ctx := context.Background()
	image := models.Image{
		CommodityID: commodityID,
		File: &models.File{
			Path:         "test-image",
			OriginalPath: "test-image.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}

	createdImage, err := imageRegistry.Create(ctx, image)
	c.Assert(err, qt.IsNil)
	c.Assert(createdImage, qt.IsNotNil)

	return createdImage
}

// createTestInvoice creates a test invoice for use in tests.
func createTestInvoice(c *qt.C, invoiceRegistry registry.InvoiceRegistry, commodityID string) *models.Invoice {
	ctx := context.Background()
	invoice := models.Invoice{
		CommodityID: commodityID,
		File: &models.File{
			Path:         "test-invoice",
			OriginalPath: "test-invoice.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}

	createdInvoice, err := invoiceRegistry.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice, qt.IsNotNil)

	return createdInvoice
}

// createTestManual creates a test manual for use in tests.
func createTestManual(c *qt.C, manualRegistry registry.ManualRegistry, commodityID string) *models.Manual {
	ctx := context.Background()
	manual := models.Manual{
		CommodityID: commodityID,
		File: &models.File{
			Path:         "test-manual",
			OriginalPath: "test-manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}

	createdManual, err := manualRegistry.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual, qt.IsNotNil)

	return createdManual
}

// setupTestHierarchy creates a complete test hierarchy: location -> area -> commodity.
// Returns the created entities.
func setupTestHierarchy(c *qt.C, registrySet *registry.Set) (*models.Location, *models.Area, *models.Commodity) {
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)

	return location, area, commodity
}
