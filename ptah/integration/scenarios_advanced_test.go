package integration

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/dbschema"
)

// TestMigrationGeneratorValidation tests the migration generator validation scenario
func TestMigrationGeneratorValidation(t *testing.T) {
	c := qt.New(t)

	// Skip if no database URL is provided
	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("No test database URL provided")
	}

	// Connect to database
	conn, err := dbschema.ConnectToDatabase(dbURL)
	c.Assert(err, qt.IsNil)
	defer conn.Close()

	// Clean database before test
	err = conn.Writer().DropAllTables()
	c.Assert(err, qt.IsNil)

	// Run the migration generator validation test
	ctx := context.Background()
	recorder := &StepRecorder{}
	err = testMigrationGeneratorValidation(ctx, conn, testFixtures, recorder)
	c.Assert(err, qt.IsNil)
}

// TestValidateSchemaConsistency tests the schema consistency validation helper
func TestValidateSchemaConsistency(t *testing.T) {
	c := qt.New(t)

	// Skip if no database URL is provided
	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("No test database URL provided")
	}

	// Connect to database
	conn, err := dbschema.ConnectToDatabase(dbURL)
	c.Assert(err, qt.IsNil)
	defer conn.Close()

	// Clean database before test
	err = conn.Writer().DropAllTables()
	c.Assert(err, qt.IsNil)

	// Create versioned entity manager
	vem, err := NewVersionedEntityManager(testFixtures)
	c.Assert(err, qt.IsNil)
	defer vem.Cleanup()

	// Apply initial migration
	ctx := context.Background()
	err = vem.MigrateToVersion(ctx, conn, "000-initial", "Create initial tables")
	c.Assert(err, qt.IsNil)

	// Validate schema consistency - should pass
	err = validateSchemaConsistency(ctx, conn, vem, "000-initial")
	c.Assert(err, qt.IsNil)
}

// TestValidateEmptySchema tests the empty schema validation helper
func TestValidateEmptySchema(t *testing.T) {
	c := qt.New(t)

	// Skip if no database URL is provided
	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("No test database URL provided")
	}

	// Connect to database
	conn, err := dbschema.ConnectToDatabase(dbURL)
	c.Assert(err, qt.IsNil)
	defer conn.Close()

	// Clean database before test
	err = conn.Writer().DropAllTables()
	c.Assert(err, qt.IsNil)

	// Validate empty schema - should pass
	ctx := context.Background()
	err = validateEmptySchema(ctx, conn)
	c.Assert(err, qt.IsNil)

	// Create a table
	vem, err := NewVersionedEntityManager(testFixtures)
	c.Assert(err, qt.IsNil)
	defer vem.Cleanup()

	err = vem.MigrateToVersion(ctx, conn, "000-initial", "Create initial tables")
	c.Assert(err, qt.IsNil)

	// Validate empty schema - should fail now
	err = validateEmptySchema(ctx, conn)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "expected empty schema but found")
}


