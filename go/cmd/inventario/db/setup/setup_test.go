package setup_test

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	_ "github.com/lib/pq" // PostgreSQL driver

	"github.com/denisvmedia/inventario/cmd/inventario/db/setup"
	"github.com/denisvmedia/inventario/models"
)

func TestDataSetupManager_SetupInitialDataset_DryRun(t *testing.T) {
	c := qt.New(t)

	// Create in-memory database for testing
	db := setupTestDatabase(c)
	defer db.Close()

	var buf bytes.Buffer
	manager := setup.NewDataSetupManager(db, &buf)
	opts := setup.DefaultSetupOptions()
	opts.DryRun = true

	result, err := manager.SetupInitialDataset(context.Background(), opts)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)

	// In dry run mode, no actual changes should be made
	// Verify that no tenant was actually created
	var tenantCount int
	err = db.QueryRow("SELECT COUNT(*) FROM tenants").Scan(&tenantCount)
	c.Assert(err, qt.IsNil)
	c.Assert(tenantCount, qt.Equals, 0)

	// Verify output was written
	output := buf.String()
	c.Assert(output, qt.Contains, "DRY RUN MODE")
}

func TestDataSetupManager_SetupInitialDataset_CreateDefaultTenant(t *testing.T) {
	c := qt.New(t)

	db := setupTestDatabase(c)
	defer db.Close()

	var buf bytes.Buffer
	manager := setup.NewDataSetupManager(db, &buf)
	opts := setup.DefaultSetupOptions()
	opts.DefaultTenantName = "Test Organization"
	opts.DefaultTenantSlug = "test-org"

	result, err := manager.SetupInitialDataset(context.Background(), opts)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.TenantsCreated, qt.Equals, 1)

	// Verify tenant was created
	var tenant models.Tenant
	err = db.QueryRow("SELECT id, name, slug, status FROM tenants WHERE id = $1",
		opts.DefaultTenantID).Scan(&tenant.ID, &tenant.Name, &tenant.Slug, &tenant.Status)
	c.Assert(err, qt.IsNil)
	c.Assert(tenant.Name, qt.Equals, "Test Organization")
	c.Assert(tenant.Slug, qt.Equals, "test-org")
	c.Assert(tenant.Status, qt.Equals, models.TenantStatusActive)
}

func TestDataSetupManager_SetupInitialDataset_CreateAdminUser(t *testing.T) {
	c := qt.New(t)

	db := setupTestDatabase(c)
	defer db.Close()

	var buf bytes.Buffer
	manager := setup.NewDataSetupManager(db, &buf)
	opts := setup.DefaultSetupOptions()
	opts.AdminEmail = "test@example.com"
	opts.AdminName = "Test Admin"

	result, err := manager.SetupInitialDataset(context.Background(), opts)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.UsersCreated, qt.Equals, 1)

	// Verify admin user was created
	var user models.User
	err = db.QueryRow("SELECT id, email, name, role, tenant_id FROM users WHERE email = $1",
		opts.AdminEmail).Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.TenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(user.Email, qt.Equals, "test@example.com")
	c.Assert(user.Name, qt.Equals, "Test Admin")
	c.Assert(user.Role, qt.Equals, models.UserRoleAdmin)
	c.Assert(user.TenantID, qt.Equals, opts.DefaultTenantID)
}

func TestDataSetupManager_SetupInitialDataset_UpdateExistingUser(t *testing.T) {
	c := qt.New(t)

	db := setupTestDatabase(c)
	defer db.Close()

	// Create a temporary tenant first (since tenant_id is NOT NULL)
	tempTenantID := "temp-tenant-" + uuid.New().String()
	_, err := db.Exec(`
		INSERT INTO tenants (id, name, slug, domain, status, settings, created_at, updated_at)
		VALUES ($1, $2, $3, NULL, 'active', '{}', $4, $5)`,
		tempTenantID, "Temp Tenant", "temp-"+uuid.New().String(), time.Now(), time.Now())
	c.Assert(err, qt.IsNil)

	// Create existing user with temporary tenant_id (to simulate user that needs to be moved to default tenant)
	existingUserID := uuid.New().String()
	_, err = db.Exec(`
		INSERT INTO users (id, email, password_hash, name, role, is_active, tenant_id, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $1, $8, $9)`,
		existingUserID, "existing@example.com", "hash", "Existing User", "user", true, tempTenantID, time.Now(), time.Now())
	c.Assert(err, qt.IsNil)

	var buf bytes.Buffer
	manager := setup.NewDataSetupManager(db, &buf)
	opts := setup.DefaultSetupOptions()
	opts.AdminEmail = "existing@example.com" // Use existing user as admin

	result, err := manager.SetupInitialDataset(context.Background(), opts)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.UsersCreated, qt.Equals, 0) // No new user created
	c.Assert(result.UsersUpdated, qt.Equals, 1) // Existing user updated

	// Verify user was updated with tenant_id
	var tenantID string
	err = db.QueryRow("SELECT tenant_id FROM users WHERE id = $1", existingUserID).Scan(&tenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(tenantID, qt.Equals, opts.DefaultTenantID)
}

func TestDataSetupManager_SetupInitialDataset_AssignUserIDsToEntities(t *testing.T) {
	c := qt.New(t)

	db := setupTestDatabase(c)
	defer db.Close()

	// Create a temporary tenant and user first (since tenant_id and user_id are NOT NULL with FK constraints)
	tempTenantID := "temp-tenant-" + uuid.New().String()
	_, err := db.Exec(`
		INSERT INTO tenants (id, name, slug, domain, status, settings, created_at, updated_at)
		VALUES ($1, $2, $3, NULL, 'active', '{}', $4, $5)`,
		tempTenantID, "Temp Tenant", "temp-"+uuid.New().String(), time.Now(), time.Now())
	c.Assert(err, qt.IsNil)

	tempUserID := uuid.New().String()
	_, err = db.Exec(`
		INSERT INTO users (id, email, password_hash, name, role, is_active, tenant_id, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $1, $8, $9)`,
		tempUserID, "temp@example.com", "hash", "Temp User", "user", true, tempTenantID, time.Now(), time.Now())
	c.Assert(err, qt.IsNil)

	// Create test data with temporary tenant/user first
	locationID := uuid.New().String()
	_, err = db.Exec(`
		INSERT INTO locations (id, name, address, tenant_id, user_id)
		VALUES ($1, $2, $3, $4, $5)`,
		locationID, "Test Location", "123 Test St", tempTenantID, tempUserID)
	c.Assert(err, qt.IsNil)

	areaID := uuid.New().String()
	_, err = db.Exec(`
		INSERT INTO areas (id, name, location_id, tenant_id, user_id)
		VALUES ($1, $2, $3, $4, $5)`,
		areaID, "Test Area", locationID, tempTenantID, tempUserID)
	c.Assert(err, qt.IsNil)

	// Now update them to have empty user_id to simulate entities that need user assignment
	// We need to temporarily disable foreign key constraints for this
	_, err = db.Exec(`SET session_replication_role = replica;`)
	c.Assert(err, qt.IsNil)

	_, err = db.Exec(`UPDATE locations SET user_id = '' WHERE id = $1`, locationID)
	c.Assert(err, qt.IsNil)

	_, err = db.Exec(`UPDATE areas SET user_id = '' WHERE id = $1`, areaID)
	c.Assert(err, qt.IsNil)

	// Re-enable foreign key constraints
	_, err = db.Exec(`SET session_replication_role = DEFAULT;`)
	c.Assert(err, qt.IsNil)

	var buf bytes.Buffer
	manager := setup.NewDataSetupManager(db, &buf)
	opts := setup.DefaultSetupOptions()

	result, err := manager.SetupInitialDataset(context.Background(), opts)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.LocationsUpdated, qt.Equals, 1)
	c.Assert(result.AreasUpdated, qt.Equals, 1)

	// Verify entities were assigned user_id
	var locationUserID, areaUserID string
	err = db.QueryRow("SELECT user_id FROM locations WHERE id = $1", locationID).Scan(&locationUserID)
	c.Assert(err, qt.IsNil)
	c.Assert(locationUserID, qt.Not(qt.Equals), "")

	err = db.QueryRow("SELECT user_id FROM areas WHERE id = $1", areaID).Scan(&areaUserID)
	c.Assert(err, qt.IsNil)
	c.Assert(areaUserID, qt.Not(qt.Equals), "")
}

func TestSetupResult_PrintSetupSummary(t *testing.T) {
	c := qt.New(t)

	result := &setup.SetupResult{
		TenantsCreated:     1,
		UsersCreated:       1,
		LocationsUpdated:   5,
		CommoditiesUpdated: 10,
		Errors:             []string{"Test error 1", "Test error 2"},
	}

	var buf bytes.Buffer
	result.PrintSetupSummary(&buf)

	output := buf.String()
	c.Assert(output, qt.Contains, "INITIAL DATASET SETUP SUMMARY")
	c.Assert(output, qt.Contains, "Tenants created: 1")
	c.Assert(output, qt.Contains, "Users created: 1")
	c.Assert(output, qt.Contains, "Locations updated: 5")
	c.Assert(output, qt.Contains, "Commodities updated: 10")
	c.Assert(output, qt.Contains, "Errors encountered: 2")
	c.Assert(output, qt.Contains, "Test error 1")
	c.Assert(output, qt.Contains, "Test error 2")
}

func TestDefaultSetupOptions(t *testing.T) {
	c := qt.New(t)

	opts := setup.DefaultSetupOptions()

	c.Assert(opts.DefaultTenantID, qt.Equals, "default-tenant-id")
	c.Assert(opts.DefaultTenantName, qt.Equals, "Default Organization")
	c.Assert(opts.DefaultTenantSlug, qt.Equals, "default")
	c.Assert(opts.AdminEmail, qt.Equals, "admin@example.com")
	c.Assert(opts.AdminPassword, qt.Equals, "admin123")
	c.Assert(opts.AdminName, qt.Equals, "System Administrator")
	c.Assert(opts.DryRun, qt.Equals, false)
}

// setupTestDatabase creates a test database for testing
// This function will skip the test if PostgreSQL test database is not available
func setupTestDatabase(c *qt.C) *sql.DB {
	// Use environment variable for database DSN
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		c.Skip("POSTGRES_TEST_DSN environment variable not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		c.Skip("PostgreSQL test database not available:", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		c.Skip("PostgreSQL test database not reachable:", err)
	}

	// Clean up any existing test data
	cleanupSQL := `
		DELETE FROM areas WHERE name LIKE 'Test%';
		DELETE FROM locations WHERE name LIKE 'Test%';
		DELETE FROM users WHERE email LIKE '%test%' OR email LIKE '%example.com';
		DELETE FROM tenants WHERE slug LIKE 'test%' OR slug LIKE 'temp%' OR id = 'default-tenant-id' OR id LIKE 'temp-tenant-%';
	`

	_, err = db.Exec(cleanupSQL)
	if err != nil {
		c.Logf("Warning: Could not clean up test data: %v", err)
	}

	return db
}
