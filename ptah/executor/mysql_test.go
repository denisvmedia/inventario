package executor

import (
	"database/sql"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/go-sql-driver/mysql"

	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
)

// skipIfNoMySQL checks if MySQL is available for testing and skips the test if not.
func skipIfNoMySQL(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping MySQL tests: MYSQL_TEST_DSN environment variable not set")
	}

	// Test connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("Skipping MySQL tests: failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping MySQL tests: failed to connect to database: %v", err)
	}

	return dsn
}

func TestNewMySQLReader(t *testing.T) {
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
				name:           "with empty schema defaults to information_schema",
				schema:         "",
				expectedSchema: "information_schema",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				reader := NewMySQLReader(nil, test.schema)
				c.Assert(reader, qt.IsNotNil)
				c.Assert(reader.schema, qt.Equals, test.expectedSchema)
				c.Assert(reader.db, qt.IsNil) // We passed nil for testing
			})
		}
	})
}

func TestMySQLReader_parseEnumValues(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tests := []struct {
			name         string
			columnType   string
			expectedVals []string
		}{
			{
				name:         "simple enum",
				columnType:   "enum('active','inactive')",
				expectedVals: []string{"active", "inactive"},
			},
			{
				name:         "enum with spaces",
				columnType:   "enum('value 1', 'value 2', 'value 3')",
				expectedVals: []string{"value 1", "value 2", "value 3"},
			},
			{
				name:         "enum with double quotes",
				columnType:   `enum("active","inactive")`,
				expectedVals: []string{"active", "inactive"},
			},
			{
				name:         "single value enum",
				columnType:   "enum('single')",
				expectedVals: []string{"single"},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				result := parseEnumValues(test.columnType)
				c.Assert(result, qt.DeepEquals, test.expectedVals)
			})
		}
	})

	t.Run("unhappy path", func(t *testing.T) {
		tests := []struct {
			name       string
			columnType string
		}{
			{
				name:       "not an enum",
				columnType: "varchar(255)",
			},
			{
				name:       "empty enum",
				columnType: "enum()",
			},
			{
				name:       "invalid format",
				columnType: "not_enum_format",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				result := parseEnumValues(test.columnType)
				c.Assert(result, qt.IsNil)
			})
		}
	})
}

// TestMySQLReader_parseTableFromDDL tests the new DDL parsing functionality
func TestMySQLReader_parseTableFromDDL(t *testing.T) {

	tests := []struct {
		name        string
		ddl         string
		expectError bool
		validate    func(c *qt.C, table parsertypes.Table)
	}{
		{
			name: "simple table with primary key",
			ddl: "CREATE TABLE `users` (\n" +
				"  `id` int NOT NULL AUTO_INCREMENT,\n" +
				"  `name` varchar(255) NOT NULL,\n" +
				"  `email` varchar(255) DEFAULT NULL,\n" +
				"  PRIMARY KEY (`id`)\n" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
			expectError: false,
			validate: func(c *qt.C, table parsertypes.Table) {
				c.Assert(table.Name, qt.Equals, "users")
				c.Assert(table.Type, qt.Equals, "BASE TABLE")
				c.Assert(len(table.Columns), qt.Equals, 3)

				// Check id column
				idCol := table.Columns[0]
				c.Assert(idCol.Name, qt.Equals, "id")
				c.Assert(idCol.DataType, qt.Equals, "int")
				c.Assert(idCol.IsNullable, qt.Equals, "NO")
				c.Assert(idCol.IsAutoIncrement, qt.IsTrue)
				c.Assert(idCol.IsPrimaryKey, qt.IsTrue)

				// Check name column
				nameCol := table.Columns[1]
				c.Assert(nameCol.Name, qt.Equals, "name")
				c.Assert(nameCol.DataType, qt.Equals, "varchar(255)")
				c.Assert(nameCol.IsNullable, qt.Equals, "NO")
				c.Assert(nameCol.IsAutoIncrement, qt.IsFalse)
				c.Assert(nameCol.IsPrimaryKey, qt.IsFalse)

				// Check email column
				emailCol := table.Columns[2]
				c.Assert(emailCol.Name, qt.Equals, "email")
				c.Assert(emailCol.DataType, qt.Equals, "varchar(255)")
				c.Assert(emailCol.IsNullable, qt.Equals, "YES")
				c.Assert(emailCol.IsAutoIncrement, qt.IsFalse)
				c.Assert(emailCol.IsPrimaryKey, qt.IsFalse)
			},
		},
		{
			name:        "invalid DDL",
			ddl:         "INVALID SQL STATEMENT",
			expectError: true,
			validate:    nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			reader := &MySQLReader{}

			table, err := reader.parseTableFromDDL(test.ddl)

			if test.expectError {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
				if test.validate != nil {
					test.validate(c, table)
				}
			}
		})
	}
}
func TestMySQLReader_ReadSchema_Integration(t *testing.T) {
	dsn := skipIfNoMySQL(t)
	c := qt.New(t)

	db, err := sql.Open("mysql", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Create a test table with various column types
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_table (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			status ENUM('active', 'inactive') DEFAULT 'active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY unique_name (name)
		)
	`)
	c.Assert(err, qt.IsNil)

	// Clean up after test
	defer func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS test_table")
	}()

	reader := NewMySQLReader(db, "")
	schema, err := reader.ReadSchema()
	c.Assert(err, qt.IsNil)
	c.Assert(schema, qt.IsNotNil)
	c.Assert(schema.Tables, qt.Not(qt.HasLen), 0)

	// Find our test table
	var testTable *parsertypes.Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == "test_table" {
			testTable = &schema.Tables[i]
			break
		}
	}
	c.Assert(testTable, qt.IsNotNil)
	c.Assert(testTable.Columns, qt.HasLen, 4)

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
	c.Assert(statusCol.DataType, qt.Equals, "enum")
}

func TestMySQLReader_ReadSchema_NoConnection(t *testing.T) {
	c := qt.New(t)

	// Test that reader can be created with nil database
	reader := NewMySQLReader(nil, "test")
	c.Assert(reader, qt.IsNotNil)
	c.Assert(reader.schema, qt.Equals, "test")
	c.Assert(reader.db, qt.IsNil)

	// Note: We don't test ReadSchema() with nil db as it would panic
	// This is expected behavior - the reader requires a valid database connection
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
				input:    "CREATE TABLE test (id INT); /* This is a comment; */ CREATE INDEX idx_test ON test (id);",
				expected: []string{"CREATE TABLE test (id INT)", "/* This is a comment; */ CREATE INDEX idx_test ON test (id)"},
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

func TestMySQLWriter_SchemaWriterInterface(t *testing.T) {
	c := qt.New(t)
	writer := NewMySQLWriter(nil, "test")
	var _ SchemaWriter = writer
	c.Assert(writer, qt.IsNotNil)
}

// Helper function to find a column by name
func findColumn(columns []parsertypes.Column, name string) *parsertypes.Column {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}
