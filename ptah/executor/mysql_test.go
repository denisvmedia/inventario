package executor

import (
	"database/sql"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/go-sql-driver/mysql"
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
	var testTable *Table
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

// Helper function to find a column by name
func findColumn(columns []Column, name string) *Column {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}
