package postgres_test

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/internal/pgtest"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/schema/bootstrap"
	"go.5x5.cz/inventario/schema/migrations/migrator"
)

var (
	// Shared connection pool for tests
	sharedPools = make(map[string]*pgxpool.Pool)
	poolMutex   sync.Mutex

	// schemaBuild guards building the schema once per DSN. Every test needs an
	// empty database, which is not the same as a newly created one -- see
	// migrateUp.
	schemaBuild      = make(map[string]*sync.Once)
	schemaBuildErr   = make(map[string]error)
	schemaBuildMutex sync.Mutex
)

// migrateUp gives the caller an empty database.
//
// It builds the schema once per DSN and truncates between tests, rather than
// dropping and recreating the schema every time. Every test in this package
// needs empty tables; none needs a newly created schema, and rebuilding it per
// test made the suite's cost the migrator's rather than the assertions' (#2413).
func migrateUp(t *testing.T, ctx context.Context, migr *migrator.Migrator, dsn string) error {
	t.Helper()

	if err := buildSchemaOnce(ctx, migr, dsn); err != nil {
		return err
	}

	return truncateAllTables(ctx, dsn)
}

// buildSchemaOnce drops and rebuilds the schema the first time it is called for
// a DSN, and does nothing afterwards. The drop is what makes the first call
// authoritative: the database may carry a schema from an earlier run of the
// suite, possibly from a different commit.
func buildSchemaOnce(ctx context.Context, migr *migrator.Migrator, dsn string) error {
	schemaBuildMutex.Lock()
	once, ok := schemaBuild[dsn]
	if !ok {
		once = &sync.Once{}
		schemaBuild[dsn] = once
	}
	schemaBuildMutex.Unlock()

	once.Do(func() {
		err := buildSchema(ctx, migr, dsn)
		schemaBuildMutex.Lock()
		schemaBuildErr[dsn] = err
		schemaBuildMutex.Unlock()
	})

	schemaBuildMutex.Lock()
	defer schemaBuildMutex.Unlock()

	return schemaBuildErr[dsn]
}

func buildSchema(ctx context.Context, migr *migrator.Migrator, dsn string) error {
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

// truncateAllTables empties every table the schema owns, leaving the schema and
// the migration history in place.
//
// One TRUNCATE naming every table at once, because the schema has foreign-key
// cycles that a table-at-a-time loop cannot order. CASCADE covers anything not
// named, RESTART IDENTITY resets the sequences so a test never sees an id from
// its predecessor.
func truncateAllTables(ctx context.Context, dsn string) error {
	pool, err := getOrCreatePool(dsn)
	if err != nil {
		return err
	}

	rows, err := pool.Query(ctx, `
		SELECT quote_ident(tablename)
		FROM pg_tables
		WHERE schemaname = 'public'
		  AND tablename <> 'schema_migrations'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(tables) == 0 {
		return nil
	}

	_, err = pool.Exec(ctx,
		"TRUNCATE TABLE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE")

	return err
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

// skipIfNoPostgreSQL returns the DSN of the PostgreSQL this suite runs
// against. pgtest hands back POSTGRES_TEST_DSN when it is set and otherwise
// starts an embedded server the first time a test asks (#1953), so the only
// case that still skips is short mode.
//
// The connection is probed here rather than at first use: a DSN that points
// at nothing produces a clearer failure from one place than from whichever
// query happened to run first.
func skipIfNoPostgreSQL(t *testing.T) string {
	t.Helper()

	dsn := pgtest.DSN(t)

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("failed to parse DSN: %v", err)
	}
	dsn = u.String()

	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("failed to connect to the test database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(t.Context()); err != nil {
		t.Fatalf("failed to ping the test database: %v", err)
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

	// Create a default group for the test user. Stamp GroupCurrency=USD
	// explicitly so commodity validation — which reads group_currency
	// off the context — passes regardless of whether the DB default
	// fires for this INSERT path.
	groupSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	testGroup, err := serviceRegistrySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Name:                "Test Group",
		Slug:                groupSlug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           userID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)
	groupID := testGroup.ID

	// Now create a user+group-aware registry set with the actual generated IDs
	sqlDB := stdlib.OpenDBFromPool(pool)
	sqlxDB := sqlx.NewDb(sqlDB, "pgx")
	userAwareRegistrySet := postgres.NewRegistrySetWithUserAndGroupID(sqlxDB, userID, tenantID, groupID)

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
		IsActive: true,
	}

	err = testUser1.SetPassword("TestPassword123")
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
	c.Assert(users, qt.Not(qt.HasLen), 0, qt.Commentf("No users found - ensure setupTestTenantAndUser was called"))

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
	c.Assert(users, qt.Not(qt.HasLen), 0, qt.Commentf("No users found - ensure setupTestTenantAndUser was called"))

	// Use the first seeded user (should be the admin user created by seeddata)
	seededUser := users[0]

	location := models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        seededUser.TenantID,
			CreatedByUserID: seededUser.ID, // Use the actual generated user ID
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
	c.Assert(users, qt.Not(qt.HasLen), 0, qt.Commentf("No users found - ensure setupTestTenantAndUser was called"))

	// Use the first seeded user (should be the admin user created by seeddata)
	seededUser := users[0]

	area := models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        seededUser.TenantID,
			CreatedByUserID: seededUser.ID, // Use the actual generated user ID
		},
		Name:       "Test Area",
		LocationID: locationID,
	}

	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)
	c.Assert(createdArea, qt.IsNotNil)

	return createdArea
}

// createTestCommodity creates a test commodity for use in tests.
func createTestCommodity(c *qt.C, registrySet *registry.Set, areaID string) *models.Commodity {
	c.Helper()

	ctx := c.Context()

	// Get the first seeded user to use for creating the commodity
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.Not(qt.HasLen), 0, qt.Commentf("No users found - ensure setupTestTenantAndUser was called"))

	// Use the first seeded user (should be the admin user created by seeddata)
	seededUser := users[0]

	commodity := models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        seededUser.TenantID,
			CreatedByUserID: seededUser.ID, // Use the actual generated user ID
		},
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 new(areaID),
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
