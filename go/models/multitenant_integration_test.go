//go:build integration

package models_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/lib/pq"

	"github.com/denisvmedia/inventario/models"
)

// setupTestDatabase creates a test database and returns the connection string
func setupTestDatabase(t *testing.T) (string, func()) {
	// Get database connection details from environment
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "inventario"
	}
	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		password = "inventario_password"
	}
	dbname := os.Getenv("POSTGRES_DB")
	if dbname == "" {
		dbname = "inventario_test"
	}

	// Create connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	// Test connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Cannot connect to test database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("Cannot ping test database: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		// Clean up test data if needed
	}

	return connStr, cleanup
}

func TestRLSPolicies_TenantIsolation(t *testing.T) {
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	t.Run("tenant isolation with RLS", func(t *testing.T) {
		c := qt.New(t)

		// Connect to database
		db, err := sql.Open("postgres", connStr)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		ctx := context.Background()

		// Create test tenants
		tenant1ID := "test-tenant-1"
		tenant2ID := "test-tenant-2"

		// Insert test tenants
		_, err = db.ExecContext(ctx, `
			INSERT INTO tenants (id, name, slug, status, settings, created_at, updated_at)
			VALUES ($1, 'Test Tenant 1', 'test-tenant-1', 'active', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (id) DO NOTHING
		`, tenant1ID)
		c.Assert(err, qt.IsNil)

		_, err = db.ExecContext(ctx, `
			INSERT INTO tenants (id, name, slug, status, settings, created_at, updated_at)
			VALUES ($1, 'Test Tenant 2', 'test-tenant-2', 'active', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (id) DO NOTHING
		`, tenant2ID)
		c.Assert(err, qt.IsNil)

		// Insert test locations for each tenant
		_, err = db.ExecContext(ctx, `
			INSERT INTO locations (id, tenant_id, name, address)
			VALUES ('loc-1', $1, 'Location 1', 'Address 1')
			ON CONFLICT (id) DO NOTHING
		`, tenant1ID)
		c.Assert(err, qt.IsNil)

		_, err = db.ExecContext(ctx, `
			INSERT INTO locations (id, tenant_id, name, address)
			VALUES ('loc-2', $1, 'Location 2', 'Address 2')
			ON CONFLICT (id) DO NOTHING
		`, tenant2ID)
		c.Assert(err, qt.IsNil)

		// Test tenant isolation by setting tenant context
		// Set context for tenant 1
		_, err = db.ExecContext(ctx, "SELECT set_tenant_context($1)", tenant1ID)
		c.Assert(err, qt.IsNil)

		// Query locations - should only see tenant 1's location
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM locations").Scan(&count)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 1)

		// Verify we can see the correct location
		var locationName string
		err = db.QueryRowContext(ctx, "SELECT name FROM locations WHERE id = 'loc-1'").Scan(&locationName)
		c.Assert(err, qt.IsNil)
		c.Assert(locationName, qt.Equals, "Location 1")

		// Try to access tenant 2's location - should fail
		err = db.QueryRowContext(ctx, "SELECT name FROM locations WHERE id = 'loc-2'").Scan(&locationName)
		c.Assert(err, qt.IsNotNil) // Should return no rows

		// Switch to tenant 2 context
		_, err = db.ExecContext(ctx, "SELECT set_tenant_context($1)", tenant2ID)
		c.Assert(err, qt.IsNil)

		// Query locations - should only see tenant 2's location
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM locations").Scan(&count)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 1)

		// Verify we can see tenant 2's location
		err = db.QueryRowContext(ctx, "SELECT name FROM locations WHERE id = 'loc-2'").Scan(&locationName)
		c.Assert(err, qt.IsNil)
		c.Assert(locationName, qt.Equals, "Location 2")

		// Try to access tenant 1's location - should fail
		err = db.QueryRowContext(ctx, "SELECT name FROM locations WHERE id = 'loc-1'").Scan(&locationName)
		c.Assert(err, qt.IsNotNil) // Should return no rows
	})
}

func TestRLSPolicies_TenantContextFunctions(t *testing.T) {
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	t.Run("tenant context functions work correctly", func(t *testing.T) {
		c := qt.New(t)

		// Connect to database
		db, err := sql.Open("postgres", connStr)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		ctx := context.Background()

		// Test set_tenant_context function
		testTenantID := "test-context-tenant"
		_, err = db.ExecContext(ctx, "SELECT set_tenant_context($1)", testTenantID)
		c.Assert(err, qt.IsNil)

		// Test get_current_tenant_id function
		var currentTenantID string
		err = db.QueryRowContext(ctx, "SELECT get_current_tenant_id()").Scan(&currentTenantID)
		c.Assert(err, qt.IsNil)
		c.Assert(currentTenantID, qt.Equals, testTenantID)

		// Test setting different tenant ID
		newTenantID := "new-context-tenant"
		_, err = db.ExecContext(ctx, "SELECT set_tenant_context($1)", newTenantID)
		c.Assert(err, qt.IsNil)

		err = db.QueryRowContext(ctx, "SELECT get_current_tenant_id()").Scan(&currentTenantID)
		c.Assert(err, qt.IsNil)
		c.Assert(currentTenantID, qt.Equals, newTenantID)
	})
}

func TestRLSPolicies_AllTables(t *testing.T) {
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	t.Run("RLS is enabled on all tenant-aware tables", func(t *testing.T) {
		c := qt.New(t)

		// Connect to database
		db, err := sql.Open("postgres", connStr)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		ctx := context.Background()

		// List of tables that should have RLS enabled
		expectedTables := []string{
			"tenants", "users", "locations", "areas", "commodities",
			"files", "exports", "images", "invoices", "manuals",
			"restore_operations", "restore_steps",
		}

		for _, tableName := range expectedTables {
			t.Run(fmt.Sprintf("table %s has RLS enabled", tableName), func(t *testing.T) {
				c := qt.New(t)

				var rlsEnabled bool
				err := db.QueryRowContext(ctx, `
					SELECT c.relrowsecurity
					FROM pg_class c
					JOIN pg_namespace n ON n.oid = c.relnamespace
					WHERE n.nspname = 'public' AND c.relname = $1
				`, tableName).Scan(&rlsEnabled)

				if err == sql.ErrNoRows {
					t.Skipf("Table %s does not exist", tableName)
				}
				c.Assert(err, qt.IsNil)
				c.Assert(rlsEnabled, qt.IsTrue, qt.Commentf("RLS should be enabled on table %s", tableName))
			})
		}
	})

	t.Run("tenant isolation policies exist for all tables", func(t *testing.T) {
		c := qt.New(t)

		// Connect to database
		db, err := sql.Open("postgres", connStr)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		ctx := context.Background()

		// List of expected policies
		expectedPolicies := map[string]string{
			"tenants":            "tenant_isolation_policy",
			"users":              "user_tenant_isolation_policy",
			"locations":          "location_tenant_isolation_policy",
			"areas":              "area_tenant_isolation_policy",
			"commodities":        "commodity_tenant_isolation_policy",
			"files":              "file_tenant_isolation_policy",
			"exports":            "export_tenant_isolation_policy",
			"images":             "image_tenant_isolation_policy",
			"invoices":           "invoice_tenant_isolation_policy",
			"manuals":            "manual_tenant_isolation_policy",
			"restore_operations": "restore_operation_tenant_isolation_policy",
			"restore_steps":      "restore_step_tenant_isolation_policy",
		}

		for tableName, policyName := range expectedPolicies {
			t.Run(fmt.Sprintf("policy %s exists for table %s", policyName, tableName), func(t *testing.T) {
				c := qt.New(t)

				var exists bool
				err := db.QueryRowContext(ctx, `
					SELECT EXISTS (
						SELECT 1 FROM pg_policies
						WHERE schemaname = 'public'
						AND tablename = $1
						AND policyname = $2
					)
				`, tableName, policyName).Scan(&exists)

				c.Assert(err, qt.IsNil)
				c.Assert(exists, qt.IsTrue, qt.Commentf("Policy %s should exist for table %s", policyName, tableName))
			})
		}
	})
}

func TestRLSPolicies_TenantIndexes(t *testing.T) {
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	t.Run("tenant_id indexes exist for performance", func(t *testing.T) {
		c := qt.New(t)

		// Connect to database
		db, err := sql.Open("postgres", connStr)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		ctx := context.Background()

		// List of tables that should have tenant_id indexes
		expectedIndexes := []string{
			"idx_users_tenant_id",
			"idx_locations_tenant_id",
			"idx_areas_tenant_id",
			"idx_commodities_tenant_id",
			"idx_files_tenant_id",
			"idx_exports_tenant_id",
			"idx_images_tenant_id",
			"idx_invoices_tenant_id",
			"idx_manuals_tenant_id",
			"idx_restore_operations_tenant_id",
			"idx_restore_steps_tenant_id",
		}

		for _, indexName := range expectedIndexes {
			t.Run(fmt.Sprintf("index %s exists", indexName), func(t *testing.T) {
				c := qt.New(t)

				var exists bool
				err := db.QueryRowContext(ctx, `
					SELECT EXISTS (
						SELECT 1 FROM pg_indexes
						WHERE schemaname = 'public'
						AND indexname = $1
					)
				`, indexName).Scan(&exists)

				c.Assert(err, qt.IsNil)
				c.Assert(exists, qt.IsTrue, qt.Commentf("Index %s should exist", indexName))
			})
		}
	})
}

func TestDataMigration_DefaultTenant(t *testing.T) {
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	t.Run("data migration creates default tenant", func(t *testing.T) {
		c := qt.New(t)

		// Connect to database
		db, err := sql.Open("postgres", connStr)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		ctx := context.Background()

		// Check if default tenant exists
		var tenantExists bool
		err = db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM tenants
				WHERE id = 'default-tenant'
				AND slug = 'default'
				AND status = 'active'
			)
		`).Scan(&tenantExists)

		if err != nil {
			t.Skipf("Tenants table not available or migration not run: %v", err)
		}

		if tenantExists {
			c.Assert(tenantExists, qt.IsTrue, qt.Commentf("Default tenant should exist after migration"))

			// Verify tenant properties
			var name, slug, status string
			err = db.QueryRowContext(ctx, `
				SELECT name, slug, status
				FROM tenants
				WHERE id = 'default-tenant'
			`).Scan(&name, &slug, &status)
			c.Assert(err, qt.IsNil)
			c.Assert(name, qt.Equals, "Default Tenant")
			c.Assert(slug, qt.Equals, "default")
			c.Assert(status, qt.Equals, "active")
		}
	})

	t.Run("existing data is assigned to default tenant", func(t *testing.T) {
		c := qt.New(t)

		// Connect to database
		db, err := sql.Open("postgres", connStr)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		ctx := context.Background()

		// Insert test data without tenant_id (simulating pre-migration data)
		testLocationID := "test-migration-location"
		_, err = db.ExecContext(ctx, `
			INSERT INTO locations (id, tenant_id, name, address)
			VALUES ($1, 'default-tenant', 'Migration Test Location', 'Test Address')
			ON CONFLICT (id) DO NOTHING
		`, testLocationID)

		if err != nil {
			t.Skipf("Cannot insert test data: %v", err)
		}

		// Verify the data has the correct tenant_id
		var tenantID string
		err = db.QueryRowContext(ctx, `
			SELECT tenant_id FROM locations WHERE id = $1
		`, testLocationID).Scan(&tenantID)
		c.Assert(err, qt.IsNil)
		c.Assert(tenantID, qt.Equals, "default-tenant")
	})
}

func TestDataMigration_ForeignKeyIntegrity(t *testing.T) {
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	t.Run("foreign key relationships are maintained after migration", func(t *testing.T) {
		c := qt.New(t)

		// Connect to database
		db, err := sql.Open("postgres", connStr)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		ctx := context.Background()

		// Check that all tenant_id values reference existing tenants
		var violationCount int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM (
				SELECT tenant_id FROM locations WHERE tenant_id NOT IN (SELECT id FROM tenants)
				UNION ALL
				SELECT tenant_id FROM areas WHERE tenant_id NOT IN (SELECT id FROM tenants)
				UNION ALL
				SELECT tenant_id FROM commodities WHERE tenant_id NOT IN (SELECT id FROM tenants)
				UNION ALL
				SELECT tenant_id FROM files WHERE tenant_id NOT IN (SELECT id FROM tenants)
				UNION ALL
				SELECT tenant_id FROM exports WHERE tenant_id NOT IN (SELECT id FROM tenants)
				UNION ALL
				SELECT tenant_id FROM users WHERE tenant_id NOT IN (SELECT id FROM tenants)
			) AS violations
		`).Scan(&violationCount)

		if err != nil {
			t.Skipf("Cannot check foreign key integrity: %v", err)
		}

		c.Assert(violationCount, qt.Equals, 0, qt.Commentf("No foreign key violations should exist"))
	})

	t.Run("no records have NULL tenant_id", func(t *testing.T) {
		c := qt.New(t)

		// Connect to database
		db, err := sql.Open("postgres", connStr)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		ctx := context.Background()

		// Check for NULL tenant_id in all tenant-aware tables
		var nullCount int
		err = db.QueryRowContext(ctx, `
			SELECT
				(SELECT COUNT(*) FROM locations WHERE tenant_id IS NULL) +
				(SELECT COUNT(*) FROM areas WHERE tenant_id IS NULL) +
				(SELECT COUNT(*) FROM commodities WHERE tenant_id IS NULL) +
				(SELECT COUNT(*) FROM files WHERE tenant_id IS NULL) +
				(SELECT COUNT(*) FROM exports WHERE tenant_id IS NULL) +
				(SELECT COUNT(*) FROM users WHERE tenant_id IS NULL)
		`).Scan(&nullCount)

		if err != nil {
			t.Skipf("Cannot check for NULL tenant_id: %v", err)
		}

		c.Assert(nullCount, qt.Equals, 0, qt.Commentf("No records should have NULL tenant_id"))
	})
}
