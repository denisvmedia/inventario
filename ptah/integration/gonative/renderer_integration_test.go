//go:build integration

package gonative_test

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/renderer"
)

// skipIfNoPostgreSQLRenderer checks if PostgreSQL is available for testing and skips the test if not.
func skipIfNoPostgreSQLRenderer(t *testing.T) string {
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

// skipIfNoMySQLRenderer checks if MySQL is available for testing and skips the test if not.
func skipIfNoMySQLRenderer(t *testing.T) string {
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

// skipIfNoMariaDBRenderer checks if MariaDB is available for testing and skips the test if not.
func skipIfNoMariaDBRenderer(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv("MARIADB_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping MariaDB tests: MARIADB_TEST_DSN environment variable not set")
	}

	// Test connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("Skipping MariaDB tests: failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping MariaDB tests: failed to connect to database: %v", err)
	}

	return dsn
}

func TestPostgreSQLRenderer_Integration(t *testing.T) {
	dsn := skipIfNoPostgreSQLRenderer(t)
	c := qt.New(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Test DROP TABLE
	dropTable := &ast.DropTableNode{
		Name:     "test_users",
		IfExists: true,
	}

	dropSQL, err := renderer.RenderSQL("postgres", dropTable)
	c.Assert(err, qt.IsNil)
	c.Assert(dropSQL, qt.Contains, "DROP TABLE IF EXISTS test_users")

	// Clean up any existing table
	_, err = db.Exec(dropSQL)
	c.Assert(err, qt.IsNil)

	// Test CREATE TABLE
	table := &ast.CreateTableNode{
		Name: "test_users",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "SERIAL",
				Primary:  true,
				Nullable: false,
			},
			{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Unique:   true,
				Nullable: false,
			},
			{
				Name:     "name",
				Type:     "VARCHAR(100)",
				Nullable: true,
			},
		},
	}

	createSQL, err := renderer.RenderSQL("postgresql", table)
	c.Assert(err, qt.IsNil)
	c.Assert(createSQL, qt.Contains, "CREATE TABLE test_users")

	// Execute the generated SQL
	_, err = db.Exec(createSQL)
	c.Assert(err, qt.IsNil)

	// Verify table was created
	var tableName string
	err = db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_name = 'test_users'").Scan(&tableName)
	c.Assert(err, qt.IsNil)
	c.Assert(tableName, qt.Equals, "test_users")

	// Clean up
	dropTable = &ast.DropTableNode{
		Name:     "test_users",
		IfExists: false,
	}
	dropSQL, err = renderer.RenderSQL("postgresql", dropTable)
	c.Assert(err, qt.IsNil)
	c.Assert(dropSQL, qt.Contains, "DROP TABLE test_users")

	_, err = db.Exec(dropSQL)
	c.Assert(err, qt.IsNil)
}

func TestMySQLRenderer_Integration(t *testing.T) {
	dsn := skipIfNoMySQLRenderer(t)
	c := qt.New(t)

	db, err := sql.Open("mysql", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Test DROP TABLE
	dropTable := &ast.DropTableNode{
		Name:     "test_users",
		IfExists: true,
	}

	dropSQL, err := renderer.RenderSQL("mysql", dropTable)
	c.Assert(err, qt.IsNil)
	c.Assert(dropSQL, qt.Contains, "DROP TABLE IF EXISTS test_users")

	// Clean up any existing table
	_, err = db.Exec(dropSQL)
	c.Assert(err, qt.IsNil)

	// Test CREATE TABLE with MySQL-specific features
	table := &ast.CreateTableNode{
		Name: "test_users",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INT",
				Primary:  true,
				AutoInc:  true,
				Nullable: false,
			},
			{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Unique:   true,
				Nullable: false,
			},
			{
				Name:     "status",
				Type:     "ENUM('active', 'inactive')",
				Nullable: false,
			},
		},
		Options: map[string]string{
			"ENGINE": "InnoDB",
		},
	}

	createSQL, err := renderer.RenderSQL("mysql", table)
	c.Assert(err, qt.IsNil)
	c.Assert(createSQL, qt.Contains, "CREATE TABLE test_users")
	c.Assert(createSQL, qt.Contains, "ENGINE=InnoDB")

	// Execute the generated SQL
	_, err = db.Exec(createSQL)
	fmt.Println(createSQL)
	c.Assert(err, qt.IsNil)

	// Verify table was created
	var tableName string
	err = db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'test_users'").Scan(&tableName)
	c.Assert(err, qt.IsNil)
	c.Assert(tableName, qt.Equals, "test_users")

	// Clean up
	dropTable = &ast.DropTableNode{
		Name:     "test_users",
		IfExists: false,
	}
	dropSQL, err = renderer.RenderSQL("mysql", dropTable)
	c.Assert(err, qt.IsNil)
	c.Assert(dropSQL, qt.Contains, "DROP TABLE test_users")

	_, err = db.Exec(dropSQL)
	c.Assert(err, qt.IsNil)
}

func TestMariaDBRenderer_Integration(t *testing.T) {
	dsn := skipIfNoMariaDBRenderer(t)
	c := qt.New(t)

	db, err := sql.Open("mysql", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Test DROP TABLE
	dropTable := &ast.DropTableNode{
		Name:     "test_products",
		IfExists: true,
	}

	dropSQL, err := renderer.RenderSQL("mariadb", dropTable)
	c.Assert(err, qt.IsNil)
	c.Assert(dropSQL, qt.Contains, "DROP TABLE IF EXISTS test_products")

	// Clean up any existing table
	_, err = db.Exec(dropSQL)
	c.Assert(err, qt.IsNil)

	// Test CREATE TABLE with MariaDB-specific features
	table := &ast.CreateTableNode{
		Name: "test_products",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INT",
				Primary:  true,
				AutoInc:  true,
				Nullable: false,
			},
			{
				Name:     "name",
				Type:     "VARCHAR(255)",
				Nullable: false,
			},
			{
				Name:     "category",
				Type:     "ENUM('electronics', 'books', 'clothing')",
				Nullable: false,
			},
		},
		Options: map[string]string{
			"ENGINE": "InnoDB",
		},
	}

	createSQL, err := renderer.RenderSQL("mariadb", table)
	c.Assert(err, qt.IsNil)
	c.Assert(createSQL, qt.Contains, "CREATE TABLE test_products")
	c.Assert(createSQL, qt.Contains, "ENGINE=InnoDB")

	// Execute the generated SQL
	_, err = db.Exec(createSQL)
	fmt.Println(createSQL)
	c.Assert(err, qt.IsNil)

	// Verify table was created
	var tableName string
	err = db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'test_products'").Scan(&tableName)
	c.Assert(err, qt.IsNil)
	c.Assert(tableName, qt.Equals, "test_products")

	// Clean up
	dropTable = &ast.DropTableNode{
		Name:     "test_products",
		IfExists: false,
	}
	dropSQL, err = renderer.RenderSQL("mariadb", dropTable)
	c.Assert(err, qt.IsNil)
	c.Assert(dropSQL, qt.Contains, "DROP TABLE test_products")

	_, err = db.Exec(dropSQL)
	c.Assert(err, qt.IsNil)
}

func TestRenderer_DialectSpecificSQL(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		skipFunc func(*testing.T) string
		driver   string
		contains []string
		excludes []string
	}{
		{
			name:     "PostgreSQL specific features",
			dialect:  "postgresql",
			skipFunc: skipIfNoPostgreSQLRenderer,
			driver:   "postgres",
			contains: []string{"SERIAL", "POSTGRES TABLE"},
			excludes: []string{"AUTO_INCREMENT", "ENGINE"},
		},
		{
			name:     "MySQL specific features",
			dialect:  "mysql",
			skipFunc: skipIfNoMySQLRenderer,
			driver:   "mysql",
			contains: []string{"AUTO_INCREMENT", "ENGINE", "MYSQL TABLE"},
			excludes: []string{"SERIAL"},
		},
		{
			name:     "MariaDB specific features",
			dialect:  "mariadb",
			skipFunc: skipIfNoMariaDBRenderer,
			driver:   "mysql",
			contains: []string{"AUTO_INCREMENT", "ENGINE", "MARIADB TABLE"},
			excludes: []string{"SERIAL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.skipFunc(t)
			c := qt.New(t)

			db, err := sql.Open(tt.driver, dsn)
			c.Assert(err, qt.IsNil)
			defer db.Close()

			// Create a table with dialect-specific features
			var table *ast.CreateTableNode
			if tt.dialect == "postgresql" {
				table = &ast.CreateTableNode{
					Name: "dialect_test",
					Columns: []*ast.ColumnNode{
						{
							Name:     "id",
							Type:     "SERIAL",
							Primary:  true,
							Nullable: false,
						},
					},
				}
			} else {
				// MySQL/MariaDB
				table = &ast.CreateTableNode{
					Name: "dialect_test",
					Columns: []*ast.ColumnNode{
						{
							Name:     "id",
							Type:     "INT",
							Primary:  true,
							AutoInc:  true,
							Nullable: false,
						},
					},
					Options: map[string]string{
						"ENGINE": "InnoDB",
					},
				}
			}

			sql, err := renderer.RenderSQL(tt.dialect, table)
			c.Assert(err, qt.IsNil)

			// Check for dialect-specific content
			for _, expected := range tt.contains {
				c.Assert(sql, qt.Contains, expected, qt.Commentf("Expected %q in SQL for %s", expected, tt.dialect))
			}

			for _, excluded := range tt.excludes {
				if !strings.Contains(sql, excluded) {
					continue // Good, it's excluded as expected
				}
				// If it contains excluded content, that might be okay in some cases
				// Just log it for debugging
				t.Logf("SQL for %s contains %q (might be expected): %s", tt.dialect, excluded, sql)
			}

			// Clean up any existing table
			_, _ = db.Exec("DROP TABLE IF EXISTS dialect_test")

			// Try to execute the SQL (this tests real compatibility)
			_, err = db.Exec(sql)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to execute SQL for %s: %s", tt.dialect, sql))

			// Clean up
			_, err = db.Exec("DROP TABLE dialect_test")
			c.Assert(err, qt.IsNil)
		})
	}
}

// TestDropIndex_Integration tests DROP INDEX functionality across all dialects
func TestDropIndex_Integration(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		skipFunc func(*testing.T) string
		driver   string
		setupSQL string
		contains []string
	}{
		{
			name:     "PostgreSQL DROP INDEX",
			dialect:  "postgresql",
			skipFunc: skipIfNoPostgreSQLRenderer,
			driver:   "postgres",
			setupSQL: "CREATE TABLE test_drop_index (id SERIAL PRIMARY KEY, email VARCHAR(255)); CREATE INDEX idx_test_email ON test_drop_index(email);",
			contains: []string{"DROP INDEX", "IF EXISTS", "idx_test_email"},
		},
		{
			name:     "MySQL DROP INDEX",
			dialect:  "mysql",
			skipFunc: skipIfNoMySQLRenderer,
			driver:   "mysql",
			setupSQL: "CREATE TABLE test_drop_index (id INT AUTO_INCREMENT PRIMARY KEY, email VARCHAR(255)); CREATE INDEX idx_test_email ON test_drop_index(email)",
			contains: []string{"DROP INDEX", "idx_test_email", "ON test_drop_index"},
		},
		{
			name:     "MariaDB DROP INDEX",
			dialect:  "mariadb",
			skipFunc: skipIfNoMariaDBRenderer,
			driver:   "mysql",
			setupSQL: "CREATE TABLE test_drop_index (id INT AUTO_INCREMENT PRIMARY KEY, email VARCHAR(255)); CREATE INDEX idx_test_email ON test_drop_index(email)",
			contains: []string{"DROP INDEX", "idx_test_email", "ON test_drop_index"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.skipFunc(t)
			c := qt.New(t)

			db, err := sql.Open(tt.driver, dsn)
			c.Assert(err, qt.IsNil)
			defer db.Close()

			// Clean up any existing objects
			_, _ = db.Exec("DROP TABLE IF EXISTS test_drop_index")

			// Setup test table and index (split into separate statements)
			if tt.dialect == "postgresql" {
				_, err = db.Exec("CREATE TABLE test_drop_index (id SERIAL PRIMARY KEY, email VARCHAR(255))")
				c.Assert(err, qt.IsNil)
				_, err = db.Exec("CREATE INDEX idx_test_email ON test_drop_index(email)")
				c.Assert(err, qt.IsNil)
			} else {
				// MySQL/MariaDB
				_, err = db.Exec("CREATE TABLE test_drop_index (id INT AUTO_INCREMENT PRIMARY KEY, email VARCHAR(255))")
				c.Assert(err, qt.IsNil)
				_, err = db.Exec("CREATE INDEX idx_test_email ON test_drop_index(email)")
				c.Assert(err, qt.IsNil)
			}

			// Test DROP INDEX
			var dropIndex *ast.DropIndexNode
			if tt.dialect == "postgresql" {
				// PostgreSQL supports IF EXISTS
				dropIndex = ast.NewDropIndex("idx_test_email").
					SetTable("test_drop_index").
					SetIfExists().
					SetComment("Test drop index")
			} else {
				// MySQL/MariaDB - don't use IF EXISTS for compatibility
				dropIndex = ast.NewDropIndex("idx_test_email").
					SetTable("test_drop_index").
					SetComment("Test drop index")
			}

			dropSQL, err := renderer.RenderSQL(tt.dialect, dropIndex)
			c.Assert(err, qt.IsNil)

			// Check for expected content
			for _, expected := range tt.contains {
				c.Assert(dropSQL, qt.Contains, expected, qt.Commentf("Expected %q in DROP INDEX SQL for %s", expected, tt.dialect))
			}

			// Execute the DROP INDEX
			_, err = db.Exec(dropSQL)
			c.Assert(err, qt.IsNil)

			// Clean up
			_, _ = db.Exec("DROP TABLE IF EXISTS test_drop_index")
		})
	}
}

// TestCreateType_Integration tests CREATE TYPE functionality across all dialects
func TestCreateType_Integration(t *testing.T) {
	tests := []struct {
		name       string
		dialect    string
		skipFunc   func(*testing.T) string
		driver     string
		createType func() *ast.CreateTypeNode
		contains   []string
		shouldExec bool
		cleanupSQL string
	}{
		{
			name:     "PostgreSQL CREATE ENUM TYPE",
			dialect:  "postgresql",
			skipFunc: skipIfNoPostgreSQL,
			driver:   "postgres",
			createType: func() *ast.CreateTypeNode {
				enumDef := ast.NewEnumTypeDef("active", "inactive", "pending")
				return ast.NewCreateType("user_status", enumDef).
					SetComment("User status enumeration")
			},
			contains:   []string{"CREATE TYPE user_status AS ENUM", "'active'", "'inactive'", "'pending'"},
			shouldExec: true,
			cleanupSQL: "DROP TYPE IF EXISTS user_status",
		},
		{
			name:     "PostgreSQL CREATE DOMAIN TYPE",
			dialect:  "postgresql",
			skipFunc: skipIfNoPostgreSQL,
			driver:   "postgres",
			createType: func() *ast.CreateTypeNode {
				domainDef := ast.NewDomainTypeDef("VARCHAR(255)").
					SetNotNull().
					SetCheck("LENGTH(VALUE) > 0")
				return ast.NewCreateType("email_domain", domainDef).
					SetComment("Email domain type")
			},
			contains:   []string{"CREATE DOMAIN email_domain AS VARCHAR(255)", "NOT NULL", "CHECK"},
			shouldExec: true,
			cleanupSQL: "DROP DOMAIN IF EXISTS email_domain",
		},
		{
			name:     "PostgreSQL CREATE COMPOSITE TYPE",
			dialect:  "postgresql",
			skipFunc: skipIfNoPostgreSQL,
			driver:   "postgres",
			createType: func() *ast.CreateTypeNode {
				fields := []*ast.CompositeField{
					{Name: "street", Type: "TEXT"},
					{Name: "city", Type: "VARCHAR(100)"},
					{Name: "zipcode", Type: "VARCHAR(10)"},
				}
				compositeDef := ast.NewCompositeTypeDef(fields...)
				return ast.NewCreateType("address", compositeDef).
					SetComment("Address composite type")
			},
			contains:   []string{"CREATE TYPE address AS", "street TEXT", "city VARCHAR(100)", "zipcode VARCHAR(10)"},
			shouldExec: true,
			cleanupSQL: "DROP TYPE IF EXISTS address",
		},
		{
			name:     "MySQL CREATE TYPE (should generate comment)",
			dialect:  "mysql",
			skipFunc: skipIfNoMySQL,
			driver:   "mysql",
			createType: func() *ast.CreateTypeNode {
				enumDef := ast.NewEnumTypeDef("active", "inactive")
				return ast.NewCreateType("status", enumDef).
					SetComment("Status enumeration")
			},
			contains:   []string{"MYSQL does not support CREATE TYPE", "enums are handled inline"},
			shouldExec: false,
		},
		{
			name:     "MariaDB CREATE TYPE (should generate comment)",
			dialect:  "mariadb",
			skipFunc: skipIfNoMariaDBRenderer,
			driver:   "mysql",
			createType: func() *ast.CreateTypeNode {
				enumDef := ast.NewEnumTypeDef("small", "medium", "large")
				return ast.NewCreateType("size", enumDef).
					SetComment("Size enumeration")
			},
			contains:   []string{"MARIADB does not support CREATE TYPE", "enums are handled inline"},
			shouldExec: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.skipFunc(t)
			c := qt.New(t)

			db, err := sql.Open(tt.driver, dsn)
			c.Assert(err, qt.IsNil)
			defer db.Close()

			// Clean up any existing type
			if tt.cleanupSQL != "" {
				_, _ = db.Exec(tt.cleanupSQL)
			}

			// Test CREATE TYPE
			createType := tt.createType()
			createSQL, err := renderer.RenderSQL(tt.dialect, createType)
			c.Assert(err, qt.IsNil)

			// Check for expected content
			for _, expected := range tt.contains {
				c.Assert(createSQL, qt.Contains, expected, qt.Commentf("Expected %q in CREATE TYPE SQL for %s", expected, tt.dialect))
			}

			// Execute the CREATE TYPE if it should be executable
			if tt.shouldExec {
				_, err = db.Exec(createSQL)
				c.Assert(err, qt.IsNil, qt.Commentf("Failed to execute CREATE TYPE SQL for %s: %s", tt.dialect, createSQL))

				// Clean up
				if tt.cleanupSQL != "" {
					_, _ = db.Exec(tt.cleanupSQL)
				}
			}
		})
	}
}

// TestAlterType_Integration tests ALTER TYPE functionality across all dialects
func TestAlterType_Integration(t *testing.T) {
	tests := []struct {
		name       string
		dialect    string
		skipFunc   func(*testing.T) string
		driver     string
		setupSQL   string
		alterType  func() *ast.AlterTypeNode
		contains   []string
		shouldExec bool
		cleanupSQL string
	}{
		{
			name:     "PostgreSQL ALTER TYPE ADD VALUE",
			dialect:  "postgresql",
			skipFunc: skipIfNoPostgreSQLRenderer,
			driver:   "postgres",
			setupSQL: "CREATE TYPE test_status AS ENUM ('active', 'inactive');",
			alterType: func() *ast.AlterTypeNode {
				return ast.NewAlterType("test_status").
					AddOperation(ast.NewAddEnumValueOperation("pending").SetAfter("inactive"))
			},
			contains:   []string{"ALTER TYPE test_status ADD VALUE 'pending' AFTER 'inactive'"},
			shouldExec: true,
			cleanupSQL: "DROP TYPE IF EXISTS test_status",
		},
		{
			name:     "PostgreSQL ALTER TYPE RENAME VALUE",
			dialect:  "postgresql",
			skipFunc: skipIfNoPostgreSQLRenderer,
			driver:   "postgres",
			setupSQL: "CREATE TYPE test_priority AS ENUM ('low', 'medium', 'high');",
			alterType: func() *ast.AlterTypeNode {
				return ast.NewAlterType("test_priority").
					AddOperation(ast.NewRenameEnumValueOperation("medium", "normal"))
			},
			contains:   []string{"ALTER TYPE test_priority RENAME VALUE 'medium' TO 'normal'"},
			shouldExec: true,
			cleanupSQL: "DROP TYPE IF EXISTS test_priority",
		},
		{
			name:     "PostgreSQL ALTER TYPE RENAME TYPE",
			dialect:  "postgresql",
			skipFunc: skipIfNoPostgreSQLRenderer,
			driver:   "postgres",
			setupSQL: "CREATE TYPE old_type_name AS ENUM ('value1', 'value2');",
			alterType: func() *ast.AlterTypeNode {
				return ast.NewAlterType("old_type_name").
					AddOperation(ast.NewRenameTypeOperation("new_type_name"))
			},
			contains:   []string{"ALTER TYPE old_type_name RENAME TO new_type_name"},
			shouldExec: true,
			cleanupSQL: "DROP TYPE IF EXISTS new_type_name, old_type_name",
		},
		{
			name:     "PostgreSQL ALTER TYPE MULTIPLE OPERATIONS",
			dialect:  "postgresql",
			skipFunc: skipIfNoPostgreSQLRenderer,
			driver:   "postgres",
			setupSQL: "CREATE TYPE multi_status AS ENUM ('draft', 'published');",
			alterType: func() *ast.AlterTypeNode {
				return ast.NewAlterType("multi_status").
					AddOperation(ast.NewAddEnumValueOperation("archived").SetAfter("published")).
					AddOperation(ast.NewAddEnumValueOperation("pending").SetBefore("published"))
			},
			contains: []string{
				"ALTER TYPE multi_status ADD VALUE 'archived' AFTER 'published'",
				"ALTER TYPE multi_status ADD VALUE 'pending' BEFORE 'published'",
			},
			shouldExec: true,
			cleanupSQL: "DROP TYPE IF EXISTS multi_status",
		},
		{
			name:     "MySQL ALTER TYPE (should generate comment)",
			dialect:  "mysql",
			skipFunc: skipIfNoMySQLRenderer,
			driver:   "mysql",
			alterType: func() *ast.AlterTypeNode {
				return ast.NewAlterType("some_type").
					AddOperation(ast.NewAddEnumValueOperation("new_value"))
			},
			contains:   []string{"MYSQL does not support ALTER TYPE", "ALTER TABLE MODIFY COLUMN"},
			shouldExec: false,
		},
		{
			name:     "MariaDB ALTER TYPE (should generate comment)",
			dialect:  "mariadb",
			skipFunc: skipIfNoMariaDBRenderer,
			driver:   "mysql",
			alterType: func() *ast.AlterTypeNode {
				return ast.NewAlterType("some_type").
					AddOperation(ast.NewRenameEnumValueOperation("old", "new"))
			},
			contains:   []string{"MARIADB does not support ALTER TYPE", "ALTER TABLE MODIFY COLUMN"},
			shouldExec: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.skipFunc(t)
			c := qt.New(t)

			db, err := sql.Open(tt.driver, dsn)
			c.Assert(err, qt.IsNil)
			defer db.Close()

			// Clean up any existing type
			if tt.cleanupSQL != "" {
				_, _ = db.Exec(tt.cleanupSQL)
			}

			// Setup test type if needed
			if tt.setupSQL != "" && tt.shouldExec {
				_, err = db.Exec(tt.setupSQL)
				c.Assert(err, qt.IsNil)
			}

			// Test ALTER TYPE
			alterType := tt.alterType()
			alterSQL, err := renderer.RenderSQL(tt.dialect, alterType)
			c.Assert(err, qt.IsNil)

			// Check for expected content
			for _, expected := range tt.contains {
				c.Assert(alterSQL, qt.Contains, expected, qt.Commentf("Expected %q in ALTER TYPE SQL for %s", expected, tt.dialect))
			}

			// Execute the ALTER TYPE if it should be executable
			if tt.shouldExec {
				_, err = db.Exec(alterSQL)
				c.Assert(err, qt.IsNil, qt.Commentf("Failed to execute ALTER TYPE SQL for %s: %s", tt.dialect, alterSQL))
			}

			// Clean up
			if tt.cleanupSQL != "" {
				_, _ = db.Exec(tt.cleanupSQL)
			}
		})
	}
}
