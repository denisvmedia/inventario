package executor

import (
	"database/sql"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/lib/pq"
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

func TestNewPostgreSQLReader(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tests := []struct {
			name           string
			schema         string
			expectedSchema string
		}{
			{
				name:           "with custom schema",
				schema:         "test_schema",
				expectedSchema: "test_schema",
			},
			{
				name:           "with empty schema defaults to public",
				schema:         "",
				expectedSchema: "public",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				reader := NewPostgreSQLReader(nil, test.schema)
				c.Assert(reader, qt.IsNotNil)
				c.Assert(reader.schema, qt.Equals, test.expectedSchema)
				c.Assert(reader.db, qt.IsNil) // We passed nil for testing
			})
		}
	})
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

	reader := NewPostgreSQLReader(db, "public")
	schema, err := reader.ReadSchema()
	c.Assert(err, qt.IsNil)
	c.Assert(schema, qt.IsNotNil)
	c.Assert(schema.Tables, qt.Not(qt.HasLen), 0)

	// Find our test table
	var testTable *Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == "test_table" {
			testTable = &schema.Tables[i]
			break
		}
	}
	c.Assert(testTable, qt.IsNotNil)
	c.Assert(testTable.Columns, qt.HasLen, 5)

	// Verify column properties
	idCol := findColumn(testTable.Columns, "id")
	c.Assert(idCol, qt.IsNotNil)
	c.Assert(idCol.IsAutoIncrement, qt.IsTrue)
	c.Assert(idCol.IsPrimaryKey, qt.IsTrue)

	nameCol := findColumn(testTable.Columns, "name")
	c.Assert(nameCol, qt.IsNotNil)
	c.Assert(nameCol.IsNullable, qt.Equals, "NO")
	c.Assert(nameCol.IsUnique, qt.IsTrue)

	statusCol := findColumn(testTable.Columns, "status")
	c.Assert(statusCol, qt.IsNotNil)
	c.Assert(statusCol.UDTName, qt.Equals, "test_status")

	// Verify enums were read
	c.Assert(schema.Enums, qt.Not(qt.HasLen), 0)
	var testEnum *Enum
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

func TestPostgreSQLReader_ReadSchema_NoConnection(t *testing.T) {
	c := qt.New(t)

	// Test that reader can be created with nil database
	reader := NewPostgreSQLReader(nil, "public")
	c.Assert(reader, qt.IsNotNil)
	c.Assert(reader.schema, qt.Equals, "public")
	c.Assert(reader.db, qt.IsNil)

	// Note: We don't test ReadSchema() with nil db as it would panic
	// This is expected behavior - the reader requires a valid database connection
}

func TestPostgreSQLReader_enhanceTablesWithConstraints(t *testing.T) {
	c := qt.New(t)

	reader := NewPostgreSQLReader(nil, "public")

	// Create test data
	tables := []Table{
		{
			Name: "test_table",
			Columns: []Column{
				{Name: "id", IsPrimaryKey: false, IsUnique: false},
				{Name: "email", IsPrimaryKey: false, IsUnique: false},
				{Name: "name", IsPrimaryKey: false, IsUnique: false},
			},
		},
	}

	constraints := []Constraint{
		{TableName: "test_table", ColumnName: "id", Type: "PRIMARY KEY"},
		{TableName: "test_table", ColumnName: "email", Type: "UNIQUE"},
	}

	// Test the enhancement
	reader.enhanceTablesWithConstraints(tables, constraints)

	// Verify results
	idCol := findColumn(tables[0].Columns, "id")
	c.Assert(idCol.IsPrimaryKey, qt.IsTrue)
	c.Assert(idCol.IsUnique, qt.IsFalse)

	emailCol := findColumn(tables[0].Columns, "email")
	c.Assert(emailCol.IsPrimaryKey, qt.IsFalse)
	c.Assert(emailCol.IsUnique, qt.IsTrue)

	nameCol := findColumn(tables[0].Columns, "name")
	c.Assert(nameCol.IsPrimaryKey, qt.IsFalse)
	c.Assert(nameCol.IsUnique, qt.IsFalse)
}
