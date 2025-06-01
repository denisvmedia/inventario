package postgres_test

import (
	"database/sql"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/lib/pq"

	"github.com/denisvmedia/inventario/ptah/dbschema/internal/testutils"
	"github.com/denisvmedia/inventario/ptah/dbschema/postgres"
	"github.com/denisvmedia/inventario/ptah/dbschema/types"
)

// skipIfNoPostgreSQL checks if PostgreSQL is available for testing and skips the test if not.
func skipIfNoPostgreSQL(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL tests: POSTGRES_TEST_DSN environment variable not set")
	}

	// Test connection
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("Skipping PostgreSQL tests: failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping PostgreSQL tests: failed to connect to database: %v", err)
	}

	return dsn
}

func TestPostgreSQLReader_ReadSchema_Integration(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Create a test enum
	_, err = db.Exec(`
		DO $$ BEGIN
			CREATE TYPE test_status AS ENUM ('active', 'inactive', 'pending');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;
	`)
	c.Assert(err, qt.IsNil)

	// Create a test table with various column types
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_table (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			status test_status DEFAULT 'active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			data JSONB
		)
	`)
	c.Assert(err, qt.IsNil)

	// Clean up after test
	defer func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS test_table")
		_, _ = db.Exec("DROP TYPE IF EXISTS test_status")
	}()

	reader := postgres.NewPostgreSQLReader(db, "public")
	schema, err := reader.ReadSchema()
	c.Assert(err, qt.IsNil)
	c.Assert(schema, qt.IsNotNil)
	c.Assert(schema.Tables, qt.Not(qt.HasLen), 0)

	// Find our test table
	var testTable *types.DBTable
	for i := range schema.Tables {
		if schema.Tables[i].Name == "test_table" {
			testTable = &schema.Tables[i]
			break
		}
	}
	c.Assert(testTable, qt.IsNotNil)
	c.Assert(testTable.Columns, qt.HasLen, 5)

	// Verify column properties
	idCol := testutils.FindColumn(testTable.Columns, "id")
	c.Assert(idCol, qt.IsNotNil)
	c.Assert(idCol.IsAutoIncrement, qt.IsTrue)
	c.Assert(idCol.IsPrimaryKey, qt.IsTrue)

	nameCol := testutils.FindColumn(testTable.Columns, "name")
	c.Assert(nameCol, qt.IsNotNil)
	c.Assert(nameCol.IsNullable, qt.Equals, "NO")
	c.Assert(nameCol.IsUnique, qt.IsTrue)

	statusCol := testutils.FindColumn(testTable.Columns, "status")
	c.Assert(statusCol, qt.IsNotNil)
	c.Assert(statusCol.UDTName, qt.Equals, "test_status")

	// Verify enums were read
	c.Assert(schema.Enums, qt.Not(qt.HasLen), 0)
	var testEnum *types.DBEnum
	for i := range schema.Enums {
		if schema.Enums[i].Name == "test_status" {
			testEnum = &schema.Enums[i]
			break
		}
	}
	c.Assert(testEnum, qt.IsNotNil)
	c.Assert(testEnum.Values, qt.DeepEquals, []string{"active", "inactive", "pending"})

	// Verify indexes were read
	c.Assert(schema.Indexes, qt.Not(qt.HasLen), 0)

	// Verify constraints were read
	c.Assert(schema.Constraints, qt.Not(qt.HasLen), 0)
}

func TestPostgreSQLWriter_Integration(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	writer := postgres.NewPostgreSQLWriter(db, "public")

	t.Run("transaction lifecycle", func(t *testing.T) {
		// Test successful transaction
		err := writer.BeginTransaction()
		c.Assert(err, qt.IsNil)

		err = writer.ExecuteSQL("SELECT 1")
		c.Assert(err, qt.IsNil)

		err = writer.CommitTransaction()
		c.Assert(err, qt.IsNil)

		// Test rollback transaction
		err = writer.BeginTransaction()
		c.Assert(err, qt.IsNil)

		err = writer.RollbackTransaction()
		c.Assert(err, qt.IsNil)
	})

	t.Run("DropAllTables", func(t *testing.T) {
		// Create a test table first
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS temp_test_table (id SERIAL PRIMARY KEY)")
		c.Assert(err, qt.IsNil)

		err = writer.DropAllTables()
		c.Assert(err, qt.IsNil)

		// Verify table was dropped
		var exists bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = 'temp_test_table'
			)
		`).Scan(&exists)
		c.Assert(err, qt.IsNil)
		c.Assert(exists, qt.IsFalse)
	})
}
