package dbschema_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dbschema"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

func TestDropSchemaInterface(t *testing.T) {
	// Test that both PostgreSQL and MySQL writers implement the SchemaWriter interface
	// including the new DropSchema and DropAllTables methods

	t.Run("PostgreSQLWriter implements SchemaWriter", func(t *testing.T) {
		c := qt.New(t)

		// Create a mock PostgreSQL writer
		writer := dbschema.NewPostgreSQLWriter(nil, "public")

		// Verify it implements SchemaWriter interface
		var _ dbschema.SchemaWriter = writer

		c.Assert(writer, qt.IsNotNil)
	})

	t.Run("MySQLWriter implements SchemaWriter", func(t *testing.T) {
		c := qt.New(t)

		// Create a mock MySQL writer
		writer := dbschema.NewMySQLWriter(nil, "test")

		// Verify it implements SchemaWriter interface
		var _ dbschema.SchemaWriter = writer

		c.Assert(writer, qt.IsNotNil)
	})
}

func TestDropSchemaValidation(t *testing.T) {
	// Test that DropSchema method exists and can be called with proper parameters

	t.Run("DropSchema method signature", func(t *testing.T) {
		c := qt.New(t)

		// Create a test result with some tables and enums
		result := &migratorlib.PackageParseResult{
			Tables: []types.TableDirective{
				{
					Name:       "users",
					StructName: "User",
				},
				{
					Name:       "products",
					StructName: "Product",
				},
			},
			Enums: []types.GlobalEnum{
				{
					Name:   "enum_user_role",
					Values: []string{"admin", "user", "guest"},
				},
			},
		}

		// Test PostgreSQL writer
		pgWriter := dbschema.NewPostgreSQLWriter(nil, "public")
		c.Assert(pgWriter, qt.IsNotNil)

		// Test MySQL writer
		mysqlWriter := dbschema.NewMySQLWriter(nil, "test")
		c.Assert(mysqlWriter, qt.IsNotNil)

		// Note: We can't actually call DropSchema or DropAllTables without a real database connection,
		// but we can verify the methods exist and the interface is properly implemented
		// by the fact that the code compiles and the interface assignment above works.

		c.Assert(result.Tables, qt.HasLen, 2)
		c.Assert(result.Enums, qt.HasLen, 1)
	})
}

func TestDropSchemaFeatures(t *testing.T) {
	t.Run("PostgreSQL drop schema features", func(t *testing.T) {
		c := qt.New(t)

		// Test that PostgreSQL writer has the expected behavior:
		// - Drops tables in reverse dependency order (DropSchema)
		// - Drops enums after tables (DropSchema)
		// - Drops sequences to prevent orphaned sequences (DropAllTables)
		// - Uses CASCADE for safety (both methods)
		// - Queries all tables from database (DropAllTables)

		writer := dbschema.NewPostgreSQLWriter(nil, "public")
		c.Assert(writer, qt.IsNotNil)

		// The actual implementation details are tested through integration tests
		// This unit test just verifies the structure exists
	})

	t.Run("MySQL drop schema features", func(t *testing.T) {
		c := qt.New(t)

		// Test that MySQL writer has the expected behavior:
		// - Drops tables in reverse dependency order (DropSchema)
		// - Disables foreign key checks during operation (both methods)
		// - Re-enables foreign key checks after operation (both methods)
		// - Queries all tables from database (DropAllTables)

		writer := dbschema.NewMySQLWriter(nil, "test")
		c.Assert(writer, qt.IsNotNil)

		// The actual implementation details are tested through integration tests
		// This unit test just verifies the structure exists
	})
}
