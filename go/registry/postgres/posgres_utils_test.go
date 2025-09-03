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
	"github.com/denisvmedia/inventario/schema/bootstrap"
	"github.com/denisvmedia/inventario/schema/migrations/migrator"
)

var (
	// Shared connection pool for tests
	sharedPools = make(map[string]*pgxpool.Pool)
	poolMutex   sync.Mutex
)

// migrateUp removes all test data by dropping and recreating the schema
func migrateUp(t *testing.T, ctx context.Context, migr *migrator.Migrator, dsn string) error {
	t.Helper()

	// Drop all tables (this cleans all data)
	err := migr.DropTables(ctx, false, true) // dryRun=false, confirm=true
	if err != nil {
		return err
	}

	// extract user from dsn
	u, err := url.Parse(dsn)
	if err != nil {
		return err
	}

	boots := bootstrap.New()

	err = boots.Apply(ctx, bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:                    u.User.Username(),
			UsernameForMigrations:       u.User.Username(),
			UsernameForBackgroundWorker: u.User.Username(),
		},
		DryRun: false,
	})
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
func createRegistrySetFromPool(pool *pgxpool.Pool) *registry.FactorySet {
	// Create sqlx DB wrapper from the shared pgxpool
	sqlDB := stdlib.OpenDBFromPool(pool)
	sqlxDB := sqlx.NewDb(sqlDB, "pgx")

	// Create PostgreSQL factory set
	factorySet := postgres.NewFactorySet(sqlxDB)

	return factorySet
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

	// Create factory set using the shared pool
	factorySet := createRegistrySetFromPool(pool)

	// Create a service registry set (without user context) to create tenant and user
	serviceRegistrySet := factorySet.CreateServiceRegistrySet()

	// Create test tenant and user that the tests expect
	tenantID, userID := setupTestTenantAndUser(c, serviceRegistrySet)

	// Now create a user-aware registry set with the actual generated IDs
	sqlDB := stdlib.OpenDBFromPool(pool)
	sqlxDB := sqlx.NewDb(sqlDB, "pgx")
	userAwareRegistrySet := postgres.NewRegistrySetWithUserID(sqlxDB, userID, tenantID)

	return userAwareRegistrySet, func() {}
}

// setupTestTenantAndUser creates the test tenant and user that the tests expect
// Returns the created tenant ID and user ID for use in creating user-aware registry sets
func setupTestTenantAndUser(c *qt.C, registrySet *registry.Set) (tenantID, userID string) {
	c.Helper()

	ctx := context.Background()

	// Create test tenant (let the system generate the ID for security)
	testTenant := models.Tenant{
		// ID will be generated server-side for security
		Name:   "Test Organization",
		Slug:   "test-org",
		Status: models.TenantStatusActive,
	}

	// Check if tenant already exists by slug
	tenants, err := registrySet.TenantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)

	var existingTenant *models.Tenant
	for _, tenant := range tenants {
		if tenant.Slug == testTenant.Slug {
			existingTenant = tenant
			tenantID = tenant.ID
			break
		}
	}

	if existingTenant == nil {
		// Tenant doesn't exist, create it
		createdTenant, err := registrySet.TenantRegistry.Create(ctx, testTenant)
		c.Assert(err, qt.IsNil)
		tenantID = createdTenant.ID
	}

	// Create test user (let the system generate the ID for security)
	testUser1 := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			// ID will be generated server-side for security
			TenantID: tenantID, // Use the generated tenant ID
		},
		Email:    "admin@test-org.com",
		Name:     "Test Administrator",
		Role:     models.UserRoleAdmin,
		IsActive: true,
	}

	err = testUser1.SetPassword("testpassword123")
	c.Assert(err, qt.IsNil)

	// Check if user already exists by email
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)

	var existingUser *models.User
	for _, user := range users {
		if user.Email == testUser1.Email {
			existingUser = user
			break
		}
	}

	if existingUser == nil {
		// User doesn't exist, create it
		createdUser, err := registrySet.UserRegistry.Create(ctx, testUser1)
		c.Assert(err, qt.IsNil)
		return tenantID, createdUser.ID
	}

	// User exists, return its ID
	return tenantID, existingUser.ID
}

// getTestUser gets the test user created by setupTestTenantAndUser
// This is a helper function for tests that need to set user context
func getTestUser(c *qt.C, registrySet *registry.Set) *models.User {
	c.Helper()

	ctx := context.Background()
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(users), qt.Not(qt.Equals), 0, qt.Commentf("No users found - ensure setupTestTenantAndUser was called"))

	// Use the first seeded user (should be the admin user created by setupTestTenantAndUser)
	return users[0]
}

// createTestLocation creates a test location for use in tests.
// This function requires that setupTestTenantAndUser has been called to seed test data.
func createTestLocation(c *qt.C, registrySet *registry.Set) *models.Location {
	c.Helper()

	ctx := c.Context()

	// Get the first seeded user to use for creating the location
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(users), qt.Not(qt.Equals), 0, qt.Commentf("No users found - ensure setupTestTenantAndUser was called"))

	// Use the first seeded user (should be the admin user created by seeddata)
	seededUser := users[0]

	location := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: seededUser.TenantID,
			UserID:   seededUser.ID, // Use the actual generated user ID
		},
		Name:    "Test Location",
		Address: "123 Test Street",
	}

	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)
	c.Assert(createdLocation, qt.IsNotNil)

	return createdLocation
}

// createTestArea creates a test area for use in tests.
func createTestArea(c *qt.C, registrySet *registry.Set, locationID string) *models.Area {
	c.Helper()

	ctx := c.Context()

	// Get the first seeded user to use for creating the area
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(users), qt.Not(qt.Equals), 0, qt.Commentf("No users found - ensure setupTestTenantAndUser was called"))

	// Use the first seeded user (should be the admin user created by seeddata)
	seededUser := users[0]

	area := models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: seededUser.TenantID,
			UserID:   seededUser.ID, // Use the actual generated user ID
		},
		Name:       "Test Area",
		LocationID: locationID,
	}

	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
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

	// Get the first seeded user to use for creating the commodity
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(users), qt.Not(qt.Equals), 0, qt.Commentf("No users found - ensure setupTestTenantAndUser was called"))

	// Use the first seeded user (should be the admin user created by seeddata)
	seededUser := users[0]

	commodity := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: seededUser.TenantID,
			UserID:   seededUser.ID, // Use the actual generated user ID
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
