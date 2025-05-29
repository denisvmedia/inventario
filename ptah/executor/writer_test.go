package executor

import (
	"database/sql"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/builder"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

func TestNewPostgreSQLWriter(t *testing.T) {
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
				writer := NewPostgreSQLWriter(nil, test.schema)
				c.Assert(writer, qt.IsNotNil)
				c.Assert(writer.schema, qt.Equals, test.expectedSchema)
				c.Assert(writer.db, qt.IsNil) // We passed nil for testing
				c.Assert(writer.tx, qt.IsNil) // No transaction initially
			})
		}
	})
}

func TestNewMySQLWriter(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test_schema")
		c.Assert(writer, qt.IsNotNil)
		c.Assert(writer.schema, qt.Equals, "test_schema")
		c.Assert(writer.db, qt.IsNil) // We passed nil for testing
		c.Assert(writer.tx, qt.IsNil) // No transaction initially
	})
}

func TestSchemaWriterInterface(t *testing.T) {
	t.Run("PostgreSQLWriter implements SchemaWriter", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")
		var _ SchemaWriter = writer
		c.Assert(writer, qt.IsNotNil)
	})

	t.Run("MySQLWriter implements SchemaWriter", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		var _ SchemaWriter = writer
		c.Assert(writer, qt.IsNotNil)
	})
}

func TestPostgreSQLWriter_TransactionMethods_NoConnection(t *testing.T) {
	c := qt.New(t)
	writer := NewPostgreSQLWriter(nil, "public")

	t.Run("ExecuteSQL with no transaction", func(t *testing.T) {
		err := writer.ExecuteSQL("SELECT 1")
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, "no active transaction")
	})

	t.Run("CommitTransaction with no transaction", func(t *testing.T) {
		err := writer.CommitTransaction()
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, "no active transaction")
	})

	t.Run("RollbackTransaction with no transaction", func(t *testing.T) {
		err := writer.RollbackTransaction()
		c.Assert(err, qt.IsNil) // Should not error when no transaction
	})
}

func TestMySQLWriter_TransactionMethods_NoConnection(t *testing.T) {
	c := qt.New(t)
	writer := NewMySQLWriter(nil, "test")

	t.Run("ExecuteSQL with no transaction", func(t *testing.T) {
		err := writer.ExecuteSQL("SELECT 1")
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, "no active transaction")
	})

	t.Run("CommitTransaction with no transaction", func(t *testing.T) {
		err := writer.CommitTransaction()
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, "no active transaction")
	})

	t.Run("RollbackTransaction with no transaction", func(t *testing.T) {
		err := writer.RollbackTransaction()
		c.Assert(err, qt.IsNil) // Should not error when no transaction
	})
}

func TestMySQLWriter_UtilityMethods(t *testing.T) {
	c := qt.New(t)
	writer := NewMySQLWriter(nil, "test")

	t.Run("splitSQLStatements", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected []string
		}{
			{
				name:     "single statement",
				input:    "CREATE TABLE test (id INT)",
				expected: []string{"CREATE TABLE test (id INT)"},
			},
			{
				name:     "multiple statements",
				input:    "CREATE TABLE test (id INT); CREATE INDEX idx_test ON test (id);",
				expected: []string{"CREATE TABLE test (id INT)", "CREATE INDEX idx_test ON test (id)"},
			},
			{
				name:     "with comments",
				input:    "CREATE TABLE test (id INT); -- This is a comment; CREATE INDEX idx_test ON test (id);",
				expected: []string{"CREATE TABLE test (id INT)", "CREATE INDEX idx_test ON test (id)"},
			},
			{
				name:     "empty statements",
				input:    "CREATE TABLE test (id INT);;; CREATE INDEX idx_test ON test (id);",
				expected: []string{"CREATE TABLE test (id INT)", "CREATE INDEX idx_test ON test (id)"},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				result := writer.splitSQLStatements(test.input)
				c.Assert(result, qt.DeepEquals, test.expected)
			})
		}
	})

	t.Run("isCreateTableStatement", func(t *testing.T) {
		tests := []struct {
			name     string
			sql      string
			expected bool
		}{
			{
				name:     "CREATE TABLE statement",
				sql:      "CREATE TABLE test (id INT)",
				expected: true,
			},
			{
				name:     "CREATE TABLE with whitespace",
				sql:      "  CREATE TABLE test (id INT)",
				expected: true,
			},
			{
				name:     "CREATE INDEX statement",
				sql:      "CREATE INDEX idx_test ON test (id)",
				expected: false,
			},
			{
				name:     "SELECT statement",
				sql:      "SELECT * FROM test",
				expected: false,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				result := writer.isCreateTableStatement(test.sql)
				c.Assert(result, qt.Equals, test.expected)
			})
		}
	})

	t.Run("isCreateIndexStatement", func(t *testing.T) {
		tests := []struct {
			name     string
			sql      string
			expected bool
		}{
			{
				name:     "CREATE INDEX statement",
				sql:      "CREATE INDEX idx_test ON test (id)",
				expected: true,
			},
			{
				name:     "CREATE UNIQUE INDEX statement",
				sql:      "CREATE UNIQUE INDEX idx_test ON test (id)",
				expected: true,
			},
			{
				name:     "CREATE TABLE statement",
				sql:      "CREATE TABLE test (id INT)",
				expected: false,
			},
			{
				name:     "SELECT statement",
				sql:      "SELECT * FROM test",
				expected: false,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				result := writer.isCreateIndexStatement(test.sql)
				c.Assert(result, qt.Equals, test.expected)
			})
		}
	})

	t.Run("extractTableNameFromCreateTable", func(t *testing.T) {
		tests := []struct {
			name     string
			sql      string
			expected string
		}{
			{
				name:     "simple CREATE TABLE",
				sql:      "CREATE TABLE test (id INT)",
				expected: "test",
			},
			{
				name:     "CREATE TABLE with parenthesis",
				sql:      "CREATE TABLE users(",
				expected: "users",
			},
			{
				name:     "invalid statement",
				sql:      "SELECT * FROM test",
				expected: "",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				result := writer.extractTableNameFromCreateTable(test.sql)
				c.Assert(result, qt.Equals, test.expected)
			})
		}
	})

	t.Run("extractTableNameFromCreateIndex", func(t *testing.T) {
		tests := []struct {
			name     string
			sql      string
			expected string
		}{
			{
				name:     "simple CREATE INDEX",
				sql:      "CREATE INDEX idx_test ON test (id)",
				expected: "test",
			},
			{
				name:     "CREATE INDEX with parenthesis",
				sql:      "CREATE UNIQUE INDEX idx_users ON users(",
				expected: "users",
			},
			{
				name:     "invalid statement",
				sql:      "CREATE TABLE test (id INT)",
				expected: "",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				result := writer.extractTableNameFromCreateIndex(test.sql)
				c.Assert(result, qt.Equals, test.expected)
			})
		}
	})
}

func TestPostgreSQLWriter_Integration(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	writer := NewPostgreSQLWriter(db, "public")

	t.Run("CheckSchemaExists with empty result", func(t *testing.T) {
		result := createTestParseResult()
		existing, err := writer.CheckSchemaExists(result)
		c.Assert(err, qt.IsNil)
		c.Assert(existing, qt.HasLen, 0)
	})

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

func TestMySQLWriter_Integration(t *testing.T) {
	dsn := skipIfNoMySQL(t)
	c := qt.New(t)

	db, err := sql.Open("mysql", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	writer := NewMySQLWriter(db, "")

	t.Run("CheckSchemaExists with empty result", func(t *testing.T) {
		result := createTestParseResult()
		existing, err := writer.CheckSchemaExists(result)
		c.Assert(err, qt.IsNil)
		c.Assert(existing, qt.HasLen, 0)
	})

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

	t.Run("tableExists", func(t *testing.T) {
		// Create a test table first
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS temp_test_table (id INT AUTO_INCREMENT PRIMARY KEY)")
		c.Assert(err, qt.IsNil)

		exists := writer.tableExists("temp_test_table")
		c.Assert(exists, qt.IsTrue)

		exists = writer.tableExists("non_existent_table")
		c.Assert(exists, qt.IsFalse)

		// Clean up
		_, _ = db.Exec("DROP TABLE IF EXISTS temp_test_table")
	})

	t.Run("DropAllTables", func(t *testing.T) {
		// Create a test table first
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS temp_test_table (id INT AUTO_INCREMENT PRIMARY KEY)")
		c.Assert(err, qt.IsNil)

		err = writer.DropAllTables()
		c.Assert(err, qt.IsNil)

		// Verify table was dropped
		exists := writer.tableExists("temp_test_table")
		c.Assert(exists, qt.IsFalse)
	})
}

// createTestParseResult creates a minimal PackageParseResult for testing
func createTestParseResult() *builder.PackageParseResult {
	return &builder.PackageParseResult{
		Tables: []meta.TableDirective{
			{Name: "test_table", StructName: "TestTable"},
		},
		Fields: []meta.SchemaField{
			{Name: "id", Type: "int", StructName: "TestTable"},
			{Name: "name", Type: "string", StructName: "TestTable"},
		},
		Indexes: []meta.SchemaIndex{},
		Enums: []meta.GlobalEnum{
			{Name: "test_status", Values: []string{"active", "inactive"}},
		},
		EmbeddedFields: []meta.EmbeddedField{},
	}
}
