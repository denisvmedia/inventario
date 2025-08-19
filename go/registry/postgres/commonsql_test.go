package postgres_test

import (
	"context"
	"net/url"
	"os"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/schema/migrations/migrator"
)

var (
	// Shared connection pool for tests
	sharedPools = make(map[string]*pgxpool.Pool)
	poolMutex   sync.Mutex
)

// migrateUp removes all test data by dropping and recreating the schema
func migrateUp(t *testing.T, ctx context.Context, migr *migrator.Migrator, dsn string) error {
	// Drop all tables (this cleans all data)
	err := migr.DropTables(ctx, false, true) // dryRun=false, confirm=true
	if err != nil {
		return err
	}

	// Recreate the schema
	err = migr.MigrateUp(ctx, migrator.Args{
		DryRun: false,
	})
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

	// Create pool config with connection limits for testing
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	// Set connection pool limits to prevent exhaustion - increased for better test performance
	config.MaxConns = 10 // Increased from 2 to 10
	config.MinConns = 2  // Increased from 1 to 2

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	sharedPools[dsn] = pool
	return pool, nil
}

// createRegistrySetFromPool creates a registry set using an existing shared pool
func createRegistrySetFromPool(pool *pgxpool.Pool) *registry.Set {
	// Create sqlx DB wrapper from the shared pgxpool
	sqlDB := stdlib.OpenDBFromPool(pool)
	sqlxDB := sqlx.NewDb(sqlDB, "pgx")

	// Create enhanced PostgreSQL registry with the shared pool
	enhancedRegistry := postgres.NewEnhancedPostgreSQLRegistry(pool, sqlxDB)

	return enhancedRegistry.Set
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
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)

	// Use the migration drop and recreate functionality
	migr := migrator.NewWithFallback(dsn, "../../models")

	ctx := context.Background()
	err = migrateUp(t, ctx, migr, dsn)
	c.Assert(err, qt.IsNil)

	// Create registry set using the shared pool instead of creating a new one
	registrySet := createRegistrySetFromPool(pool)

	// Create test tenant and user that the tests expect
	setupTestTenantAndUser(c, registrySet)

	return registrySet, func() {}
}

// setupTestTenantAndUser creates the test tenant and user that the tests expect
func setupTestTenantAndUser(c *qt.C, registrySet *registry.Set) {
	c.Helper()

	ctx := c.Context()

	// Create test tenant if it doesn't exist
	testTenant := models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant-id"},
		Name:     "Test Tenant",
		Slug:     "test-tenant",
		Status:   models.TenantStatusActive,
	}

	// Try to get existing tenant first
	existingTenant, err := registrySet.TenantRegistry.Get(ctx, "test-tenant-id")
	if err != nil {
		// Tenant doesn't exist, create it
		_, err = registrySet.TenantRegistry.Create(ctx, testTenant)
		c.Assert(err, qt.IsNil)
	} else {
		// Tenant exists, use it
		_ = existingTenant
	}

	// Create test user if it doesn't exist
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
			UserID:   "test-user-id", // Self-reference
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}

	// Set a password
	err = testUser.SetPassword("testpassword123")
	c.Assert(err, qt.IsNil)

	// Try to get existing user first
	existingUser, err := registrySet.UserRegistry.Get(ctx, "test-user-id")
	if err != nil {
		// User doesn't exist, create it
		_, err = registrySet.UserRegistry.Create(ctx, testUser)
		c.Assert(err, qt.IsNil)
	} else {
		// User exists, use it
		_ = existingUser
	}
}

// createTestLocation creates a test location for use in tests.
func createTestLocation(c *qt.C, locationRegistry registry.LocationRegistry) *models.Location {
	c.Helper()

	ctx := c.Context()
	location := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			UserID:   "test-user-id",
		},
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
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			UserID:   "test-user-id",
		},
		Name:       "Test Area",
		LocationID: locationID,
	}

	createdArea, err := areaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)
	c.Assert(createdArea, qt.IsNotNil)

	return createdArea
}

// createTestTenant creates a test tenant for use in tests.
func createTestTenant(c *qt.C, tenantRegistry registry.TenantRegistry) *models.Tenant { //nolint:unused // for future use
	c.Helper()

	ctx := c.Context()
	tenant := models.Tenant{
		Name:   "Test Tenant",
		Slug:   "test-tenant",
		Status: models.TenantStatusActive,
	}

	createdTenant, err := tenantRegistry.Create(ctx, tenant)
	c.Assert(err, qt.IsNil)
	c.Assert(createdTenant, qt.IsNotNil)

	return createdTenant
}

// createTestUser creates a test user for use in tests.
func createTestUser(c *qt.C, userRegistry registry.UserRegistry, tenantID string) *models.User { //nolint:unused // for future use
	c.Helper()

	ctx := c.Context()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			UserID:   "", // Will be set to the user's own ID after creation
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}

	// Set a password
	err := user.SetPassword("testpassword123")
	c.Assert(err, qt.IsNil)

	createdUser, err := userRegistry.Create(ctx, user)
	c.Assert(err, qt.IsNil)
	c.Assert(createdUser, qt.IsNotNil)

	// Update the user to set user_id to its own ID (self-reference)
	createdUser.UserID = createdUser.ID
	updatedUser, err := userRegistry.Update(ctx, *createdUser)
	c.Assert(err, qt.IsNil)

	return updatedUser
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
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			UserID:   "test-user-id",
		},
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
