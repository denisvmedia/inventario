package mysql

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/sqlutil"
	"github.com/denisvmedia/inventario/ptah/dbschema/types"
)

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
		validate    func(c *qt.C, table types.DBTable)
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
			validate: func(c *qt.C, table types.DBTable) {
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
			name: "table with unique constraint",
			ddl: "CREATE TABLE `products` (\n" +
				"  `id` int NOT NULL AUTO_INCREMENT,\n" +
				"  `sku` varchar(100) NOT NULL,\n" +
				"  PRIMARY KEY (`id`),\n" +
				"  UNIQUE KEY `uk_sku` (`sku`)\n" +
				") ENGINE=InnoDB",
			expectError: false,
			validate: func(c *qt.C, table types.DBTable) {
				c.Assert(table.Name, qt.Equals, "products")
				c.Assert(len(table.Columns), qt.Equals, 2)

				// Check sku column should be marked as unique
				skuCol := table.Columns[1]
				c.Assert(skuCol.Name, qt.Equals, "sku")
				c.Assert(skuCol.IsUnique, qt.IsTrue)
			},
		},
		{
			name: "real MySQL SHOW CREATE TABLE output",
			ddl: "CREATE TABLE `test_table` (\n" +
				"  `id` int NOT NULL AUTO_INCREMENT,\n" +
				"  `name` varchar(255) NOT NULL,\n" +
				"  `status` enum('active','inactive') DEFAULT 'active',\n" +
				"  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,\n" +
				"  PRIMARY KEY (`id`),\n" +
				"  UNIQUE KEY `unique_name` (`name`)\n" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
			expectError: false,
			validate: func(c *qt.C, table types.DBTable) {
				c.Assert(table.Name, qt.Equals, "test_table")
				c.Assert(len(table.Columns), qt.Equals, 4)

				// Check id column
				idCol := table.Columns[0]
				c.Assert(idCol.Name, qt.Equals, "id")
				c.Assert(idCol.IsAutoIncrement, qt.IsTrue)
				c.Assert(idCol.IsPrimaryKey, qt.IsTrue)

				// Check name column
				nameCol := table.Columns[1]
				c.Assert(nameCol.Name, qt.Equals, "name")
				c.Assert(nameCol.IsUnique, qt.IsTrue)

				// Check status column (enum)
				statusCol := table.Columns[2]
				c.Assert(statusCol.Name, qt.Equals, "status")
				c.Assert(statusCol.DataType, qt.Equals, "enum('active','inactive')")

				// Check created_at column
				createdCol := table.Columns[3]
				c.Assert(createdCol.Name, qt.Equals, "created_at")
				c.Assert(createdCol.DataType, qt.Equals, "timestamp")
				c.Assert(createdCol.IsNullable, qt.Equals, "YES")
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
			reader := &Reader{}

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
				result := sqlutil.SplitSQLStatements(test.input)
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

func TestMySQLWriter_SchemaWriterInterface(t *testing.T) {
	c := qt.New(t)
	writer := NewMySQLWriter(nil, "test")
	var _ types.SchemaWriter = writer
	c.Assert(writer, qt.IsNotNil)
}
