package commonsql_test

import (
	"net/url"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	pgmigrations "github.com/denisvmedia/inventario/registry/postgres/migrations"
)

// skipIfNoPostgreSQL checks if PostgreSQL is available for testing and skips the test if not.
func skipIfNoPostgreSQL(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv("POSTGRES_TEST_DSN")
	// if dsn == "" {
	//	dsn = "postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable&pool_max_conns=1&pool_min_conns=1"
	// }
	if dsn == "" {
		t.Skip("Skipping PostgreSQL tests: POSTGRES_TEST_DSN environment variable not set")
	}

	u, err := url.Parse(dsn)
	if err != nil {
		t.Skipf("Skipping PostgreSQL tests: failed to parse DSN: %v", err)
	}
	dsn = postgres.ParsePostgreSQLURL(u)

	// Test connection
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Skipf("Skipping PostgreSQL tests: failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(t.Context()); err != nil {
		t.Skipf("Skipping PostgreSQL tests: failed to ping database: %v", err)
	}

	return dsn
}

// setupTestRegistrySet creates a complete registry set with clean database.
func setupTestRegistrySet(t *testing.T) (*registry.Set, func()) {
	t.Helper()

	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	// Create connection pool
	pool, err := pgxpool.New(t.Context(), dsn)
	c.Assert(err, qt.IsNil)

	// Clean up any existing test tables
	_, err = pool.Exec(t.Context(), `
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

	// Run migrations
	err = pgmigrations.RunMigrations(t.Context(), pool)
	c.Assert(err, qt.IsNil)

	// Create registry set using the postgres package
	registrySetFunc, cleanup := postgres.NewRegistrySet()
	registrySet, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil)

	cleanupFunc := func() {
		if err := cleanup(); err != nil {
			t.Logf("Cleanup error: %v", err)
		}
		if pool != nil {
			pool.Close()
		}
	}

	return registrySet, cleanupFunc
}

// createTestLocation creates a test location for use in tests.
func createTestLocation(c *qt.C, locationRegistry registry.LocationRegistry) *models.Location {
	c.Helper()

	ctx := c.Context()
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
	c.Helper()

	ctx := c.Context()
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
	c.Helper()

	ctx := c.Context()

	// Set main currency to USD
	err := settingsRegistry.Patch(ctx, "system.main_currency", "USD")
	c.Assert(err, qt.IsNil)
}

// createTestCommodity creates a test commodity for use in tests.
func createTestCommodity(c *qt.C, registrySet *registry.Set, areaID string) *models.Commodity {
	c.Helper()

	ctx := c.Context()

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
