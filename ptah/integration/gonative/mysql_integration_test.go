//go:build integration

package mysql_test

import (
	"database/sql"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/go-sql-driver/mysql"

	"github.com/denisvmedia/inventario/ptah/dbschema/mysql"
	"github.com/denisvmedia/inventario/ptah/dbschema/types"
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

// Helper function to find a column by name
func findColumn(columns []types.DBColumn, name string) *types.DBColumn {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}

func tableExists(db *sql.DB, tableName string, dryRun bool) bool {
	if dryRun {
		// In dry run mode, assume table doesn't exist to show all operations
		return false
	}

	var exists bool
	checkSQL := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = DATABASE() AND table_name = ?
		)`

	err := db.QueryRow(checkSQL, tableName).Scan(&exists)
	return err == nil && exists
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

	reader := mysql.NewMySQLReader(db, "")
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
	c.Assert(statusCol.DataType, qt.Equals, "enum('active','inactive')")
}

func TestMySQLWriter_Integration(t *testing.T) {
	dsn := skipIfNoMySQL(t)
	c := qt.New(t)
	const noDryRun = false

	db, err := sql.Open("mysql", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	writer := mysql.NewMySQLWriter(db, "")

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
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS temp_test_table (id INT AUTO_INCREMENT PRIMARY KEY)")
		c.Assert(err, qt.IsNil)

		err = writer.DropAllTables()
		c.Assert(err, qt.IsNil)

		// Verify table was dropped
		exists := tableExists(db, "temp_test_table", noDryRun)
		c.Assert(exists, qt.IsFalse)
	})
}
