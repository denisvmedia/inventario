package commonsql_test

import (
	"context"
	"net/url"
	"os"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/registry/ptah"
)

var (
	// Global migration setup - track if migrations have been run
	migrationsRun  = make(map[string]bool)
	migrationMutex sync.Mutex
	// Shared connection pool for tests
	sharedPools = make(map[string]*pgxpool.Pool)
	poolMutex   sync.Mutex
)

// ensureMigrationsRun runs database migrations once per DSN
func ensureMigrationsRun(dsn string) error {
	migrationMutex.Lock()
	defer migrationMutex.Unlock()

	// Check if migrations have already been run for this DSN
	if migrationsRun[dsn] {
		return nil
	}

	// Run migrations using Ptah migrator
	migrator, err := ptah.NewPtahMigrator(nil, dsn, "../../models")
	if err != nil {
		return err
	}

	// Use a context with timeout for migrations
	ctx := context.Background()
	err = migrator.MigrateUp(ctx, false)
	if err != nil {
		return err
	}

	// Mark migrations as run for this DSN
	migrationsRun[dsn] = true
	return nil
}

// cleanupTestData removes all test data by dropping and recreating the schema
func cleanupTestData(dsn string) error {
	// Use the migration drop and recreate functionality
	migrator, err := ptah.NewPtahMigrator(nil, dsn, "../../models")
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Drop all tables (this cleans all data)
	err = migrator.DropDatabase(ctx, false, true) // dryRun=false, confirm=true
	if err != nil {
		return err
	}

	// Recreate the schema
	err = migrator.MigrateUp(ctx, false)
	if err != nil {
		return err
	}

	return nil
}

// getOrCreatePool gets or creates a shared connection pool for the given DSN
func getOrCreatePool(dsn string) (*pgxpool.Pool, error) {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	if pool, exists := sharedPools[dsn]; exists {
		return pool, nil
	}

	// Parse the DSN and add connection pool limits for testing
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	// Add connection pool limits to prevent exhaustion
	q := u.Query()
	q.Set("pool_max_conns", "2")
	q.Set("pool_min_conns", "1")
	u.RawQuery = q.Encode()

	pool, err := pgxpool.New(context.Background(), u.String())
	if err != nil {
		return nil, err
	}

	sharedPools[dsn] = pool
	return pool, nil
}

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
	dsn = u.String()

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

	// Ensure shared connection pool exists and migrations are run
	_, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)

	err = ensureMigrationsRun(dsn)
	c.Assert(err, qt.IsNil)

	// Don't clean at the beginning - let tests run on existing data
	// Cleanup will happen at the end via cleanupFunc

	// Create registry set using the postgres package
	registrySetFunc, _ := postgres.NewRegistrySet()
	registrySet, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil)

	cleanupFunc := func() {
		// Clean test data using migration drop/recreate
		_ = cleanupTestData(dsn) // Ignore errors during cleanup
		// Don't close the shared pool - it will be reused by other tests
		// Registry-specific resources will be cleaned up automatically when they go out of scope
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
