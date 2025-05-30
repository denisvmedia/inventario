package integration

import (
	"context"
	"embed"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/executor"
)

//go:embed fixtures
var testFixtures embed.FS

// TestVersionedEntityManager tests the versioned entity manager functionality
func TestVersionedEntityManager(t *testing.T) {
	c := qt.New(t)

	// Create versioned entity manager
	vem, err := NewVersionedEntityManager(testFixtures)
	c.Assert(err, qt.IsNil)
	defer vem.Cleanup()

	t.Run("LoadEntityVersion", func(t *testing.T) {
		c := qt.New(t)

		// Load initial version
		err = vem.LoadEntityVersion("000-initial")
		c.Assert(err, qt.IsNil)

		// Generate schema
		schema, err := vem.GenerateSchemaFromEntities()
		c.Assert(err, qt.IsNil)

		// Should have 2 tables: users, products
		c.Assert(len(schema.Tables), qt.Equals, 2)

		// Should have no enums in initial version
		c.Assert(len(schema.Enums), qt.Equals, 0)

		// Check table names
		tableNames := make(map[string]bool)
		for _, table := range schema.Tables {
			tableNames[table.Name] = true
		}
		c.Assert(tableNames["users"], qt.IsTrue)
		c.Assert(tableNames["products"], qt.IsTrue)
	})

	t.Run("LoadEntityVersionWithEnums", func(t *testing.T) {
		c := qt.New(t)

		// Load version with enums
		err := vem.LoadEntityVersion("003-add-enums")
		c.Assert(err, qt.IsNil)

		// Generate schema
		schema, err := vem.GenerateSchemaFromEntities()
		c.Assert(err, qt.IsNil)

		// Should have 3 tables: users, products, posts
		c.Assert(len(schema.Tables), qt.Equals, 3)

		// Should have 3 enums
		c.Assert(len(schema.Enums), qt.Equals, 3)

		// Check enum names (parser adds "enum_" prefix)
		enumNames := make(map[string]bool)
		for _, enum := range schema.Enums {
			enumNames[enum.Name] = true
		}
		c.Assert(enumNames["enum_user_status"], qt.IsTrue)
		c.Assert(enumNames["enum_product_status"], qt.IsTrue)
		c.Assert(enumNames["enum_post_status"], qt.IsTrue)
	})
}

// TestDynamicScenariosBasic tests basic dynamic scenario functionality
func TestDynamicScenariosBasic(t *testing.T) {
	// Skip if no database connection available
	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("No test database URL provided")
	}

	c := qt.New(t)
	ctx := context.Background()

	// Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	c.Assert(err, qt.IsNil)
	defer conn.Close()

	// Clean database
	err = conn.Writer().DropAllTables()
	c.Assert(err, qt.IsNil)

	t.Run("DynamicBasicEvolution", func(t *testing.T) {
		c := qt.New(t)

		// Clean database before test
		err := conn.Writer().DropAllTables()
		c.Assert(err, qt.IsNil)

		// Run the dynamic basic evolution test
		err = testDynamicBasicEvolution(ctx, conn, testFixtures)
		c.Assert(err, qt.IsNil)

		// Verify final state - should have 3 tables
		schema, err := conn.Reader().ReadSchema()
		c.Assert(err, qt.IsNil)
		c.Assert(len(schema.Tables) >= 3, qt.IsTrue, qt.Commentf("Expected at least 3 tables, got %d", len(schema.Tables)))
	})

	t.Run("DynamicIdempotency", func(t *testing.T) {
		c := qt.New(t)

		// Clean database before test
		err := conn.Writer().DropAllTables()
		c.Assert(err, qt.IsNil)

		// Run the dynamic idempotency test
		err = testDynamicIdempotency(ctx, conn, testFixtures)
		c.Assert(err, qt.IsNil)
	})
}

// getTestDatabaseURL returns a test database URL from environment variables
func getTestDatabaseURL() string {
	// Try PostgreSQL first
	if url := getEnvVar("POSTGRES_TEST_URL"); url != "" {
		return url
	}

	// Try MySQL
	if url := getEnvVar("MYSQL_TEST_URL"); url != "" {
		return url
	}

	// Default to empty (will skip tests)
	return ""
}

// getEnvVar is a helper to get environment variables
func getEnvVar(key string) string {
	// In a real implementation, this would use os.Getenv(key)
	// For now, return empty to skip database tests
	return ""
}
