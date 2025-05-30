package differ_test

import (
	"strings"
	"testing"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/renderer"
	"github.com/denisvmedia/inventario/ptah/schema/differ"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/types"

	qt "github.com/frankban/quicktest"
)

// TestWorkflowExample demonstrates the complete workflow
func TestWorkflowExample(t *testing.T) {
	// This test demonstrates the workflow without requiring a real database
	// It tests the individual components that make up the complete workflow

	t.Run("URL formatting works", func(t *testing.T) {
		c := qt.New(t)

		// Test password masking
		url := "postgres://user:secret123@localhost:5432/mydb"
		formatted := executor.FormatDatabaseURL(url)
		c.Assert(formatted, qt.Equals, "postgres://user:***@localhost:5432/mydb")
	})

	t.Run("Schema diff detects no changes", func(t *testing.T) {
		c := qt.New(t)

		// Create empty diff
		diff := &differ.SchemaDiff{}

		// Should report no changes
		c.Assert(diff.HasChanges(), qt.Equals, false)

		// Format should show no changes
		output := renderer.FormatSchemaDiff(diff)
		c.Assert(output, qt.Contains, "NO SCHEMA CHANGES DETECTED")
	})

	t.Run("Schema diff detects changes", func(t *testing.T) {
		c := qt.New(t)

		// Create diff with changes
		diff := &differ.SchemaDiff{
			TablesAdded:   []string{"new_table"},
			TablesRemoved: []string{"old_table"},
			EnumsAdded:    []string{"new_enum"},
		}

		// Should report changes
		c.Assert(diff.HasChanges(), qt.Equals, true)

		// Format should show changes
		output := renderer.FormatSchemaDiff(diff)
		c.Assert(output, qt.Contains, "SCHEMA DIFFERENCES DETECTED")
		c.Assert(output, qt.Contains, "new_table")
		c.Assert(output, qt.Contains, "old_table")
		c.Assert(output, qt.Contains, "new_enum")
	})

	t.Run("Migration SQL generation", func(t *testing.T) {
		c := qt.New(t)

		// Create diff with enum changes
		diff := &differ.SchemaDiff{
			EnumsAdded: []string{"test_enum"},
		}

		// Mock generated schema (simplified)
		// In real usage, this would come from parsing Go entities
		mockResult := &parsertypes.PackageParseResult{
			Enums: []types.GlobalEnum{
				{
					Name:   "test_enum",
					Values: []string{"value1", "value2"},
				},
			},
		}

		statements := diff.GenerateMigrationSQL(mockResult, "postgres")

		// Should generate some statements
		c.Assert(len(statements), qt.Not(qt.Equals), 0)

		// Should contain CREATE TYPE statement
		found := false
		for _, stmt := range statements {
			if strings.Contains(stmt, "CREATE TYPE test_enum") {
				found = true
				break
			}
		}
		c.Assert(found, qt.Equals, true)
	})
}

// TestDatabaseConnectionErrors tests error handling
func TestDatabaseConnectionErrors(t *testing.T) {

	tests := []struct {
		name     string
		dbURL    string
		expected string
	}{
		{
			name:     "Invalid URL",
			dbURL:    "not-a-url",
			expected: "invalid database URL: missing scheme",
		},
		{
			name:     "Unsupported dialect",
			dbURL:    "sqlite://test.db",
			expected: "unsupported database dialect: sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			conn, err := executor.ConnectToDatabase(tt.dbURL)
			c.Assert(err, qt.ErrorMatches, ".*"+tt.expected+".*")
			c.Assert(conn, qt.IsNil)
		})
	}
}

// Example of how the workflow would be used in practice
func Example() {
	// This example shows the typical workflow steps
	// (This is documentation, not a runnable test)

	// Step 1: Generate schema from Go entities
	// go run ./cmd/package-migrator generate ./models postgres

	// Step 2: Write initial schema to database
	// go run ./cmd/package-migrator write-db ./models postgres://user:pass@localhost/db

	// Step 3: Read current database schema
	// go run ./cmd/package-migrator read-db postgres://user:pass@localhost/db

	// Step 4: After updating Go entities, compare schemas
	// go run ./cmd/package-migrator compare ./models postgres://user:pass@localhost/db

	// Step 5: Generate migration SQL
	// go run ./cmd/package-migrator migrate ./models postgres://user:pass@localhost/db

	// Step 6: Apply migration (manually for now)
	// Execute the generated SQL in your database

	// Step 7: Verify the changes
	// go run ./cmd/package-migrator compare ./models postgres://user:pass@localhost/db
	// Should show "NO SCHEMA CHANGES DETECTED"
}
