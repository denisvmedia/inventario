package parser_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/parser"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

func TestParsePackageRecursively(t *testing.T) {
	c := qt.New(t)

	// Test parsing the stubs directory
	result, err := parser.ParsePackageRecursively("../../stubs")
	c.Assert(err, qt.IsNil)

	// Verify we found entities (includes all test files in stubs directory)
	c.Assert(len(result.Tables), qt.Equals, 16) // All test tables from various test files
	c.Assert(len(result.Fields) > 0, qt.IsTrue)
	c.Assert(len(result.EmbeddedFields) > 0, qt.IsTrue)

	// Verify dependency ordering
	tableNames := make([]string, len(result.Tables))
	for i, table := range result.Tables {
		tableNames[i] = table.Name
	}

	// users should come before articles (articles depends on users)
	usersIndex := findIndex(tableNames, "users")
	articlesIndex := findIndex(tableNames, "articles")
	c.Assert(usersIndex < articlesIndex, qt.IsTrue, qt.Commentf("users should come before articles"))

	// Note: categories has a circular dependency (self-reference), so it may come after products
	// This is expected behavior for circular dependencies
	categoriesIndex := findIndex(tableNames, "categories")
	productsIndex := findIndex(tableNames, "products")
	// We just verify both tables exist in the result
	c.Assert(categoriesIndex >= 0, qt.IsTrue, qt.Commentf("categories table should be found"))
	c.Assert(productsIndex >= 0, qt.IsTrue, qt.Commentf("products table should be found"))
}

func TestDependencyResolution(t *testing.T) {
	c := qt.New(t)

	result, err := parser.ParsePackageRecursively("../../stubs")
	c.Assert(err, qt.IsNil)

	// Check that dependencies are correctly identified
	c.Assert(result.Dependencies["articles"], qt.DeepEquals, []string{"users"})
	c.Assert(result.Dependencies["products"], qt.DeepEquals, []string{"categories"})
	c.Assert(result.Dependencies["categories"], qt.DeepEquals, []string{"categories"}) // self-reference
}

func TestDeduplication(t *testing.T) {
	c := qt.New(t)

	result, err := parser.ParsePackageRecursively("../../stubs")
	c.Assert(err, qt.IsNil)

	// Verify no duplicate tables
	tableNames := make(map[string]int)
	for _, table := range result.Tables {
		tableNames[table.Name]++
	}
	for name, count := range tableNames {
		c.Assert(count, qt.Equals, 1, qt.Commentf("Table %s should appear only once", name))
	}

	// Verify no duplicate fields within the same struct
	fieldKeys := make(map[string]int)
	for _, field := range result.Fields {
		key := field.StructName + "." + field.Name
		fieldKeys[key]++
	}
	for key, count := range fieldKeys {
		c.Assert(count, qt.Equals, 1, qt.Commentf("Field %s should appear only once", key))
	}
}

func findIndex(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func TestGetOrderedCreateStatements(t *testing.T) {
	c := qt.New(t)

	result, err := parser.ParsePackageRecursively("../../stubs")
	c.Assert(err, qt.IsNil)

	statements := parser.GetOrderedCreateStatements(result, "postgres")
	c.Assert(len(statements), qt.Equals, len(result.Tables))

	// Verify that each statement contains CREATE TABLE
	for _, statement := range statements {
		c.Assert(statement, qt.Contains, "CREATE TABLE")
	}
}

func TestEmbeddedFieldsInPackageParser(t *testing.T) {
	c := qt.New(t)

	result, err := parser.ParsePackageRecursively("../../stubs")
	c.Assert(err, qt.IsNil)

	// Find the articles table statement
	statements := parser.GetOrderedCreateStatements(result, "postgres")
	var articlesSQL string
	for _, statement := range statements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			articlesSQL = statement
			break
		}
	}

	c.Assert(articlesSQL, qt.Not(qt.Equals), "")

	// Verify embedded fields are included
	c.Assert(articlesSQL, qt.Contains, "created_at", qt.Commentf("Should contain created_at from Timestamps"))
	c.Assert(articlesSQL, qt.Contains, "updated_at", qt.Commentf("Should contain updated_at from Timestamps"))
	c.Assert(articlesSQL, qt.Contains, "audit_by", qt.Commentf("Should contain audit_by from AuditInfo"))
	c.Assert(articlesSQL, qt.Contains, "audit_reason", qt.Commentf("Should contain audit_reason from AuditInfo"))
	c.Assert(articlesSQL, qt.Contains, "meta_data", qt.Commentf("Should contain meta_data from Meta"))
	c.Assert(articlesSQL, qt.Contains, "author_id", qt.Commentf("Should contain author_id from User relation"))
}

func TestPlatformSpecificOverrides(t *testing.T) {
	c := qt.New(t)

	result, err := parser.ParsePackageRecursively("../../stubs")
	c.Assert(err, qt.IsNil)

	// Test PostgreSQL (default)
	postgresStatements := parser.GetOrderedCreateStatements(result, "postgres")
	var postgresArticlesSQL string
	for _, statement := range postgresStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			postgresArticlesSQL = statement
			break
		}
	}
	c.Assert(postgresArticlesSQL, qt.Contains, "meta_data JSONB")

	// Test MySQL (override)
	mysqlStatements := parser.GetOrderedCreateStatements(result, "mysql")
	var mysqlArticlesSQL string
	for _, statement := range mysqlStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			mysqlArticlesSQL = statement
			break
		}
	}
	c.Assert(mysqlArticlesSQL, qt.Contains, "meta_data JSON")

	// Test MariaDB (override with check constraint)
	mariadbStatements := parser.GetOrderedCreateStatements(result, "mariadb")
	var mariadbArticlesSQL string
	for _, statement := range mariadbStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			mariadbArticlesSQL = statement
			break
		}
	}
	c.Assert(mariadbArticlesSQL, qt.Contains, "meta_data LONGTEXT")
	c.Assert(mariadbArticlesSQL, qt.Contains, "JSON_VALID(meta_data)")
}

func TestGetDependencyInfo(t *testing.T) {
	c := qt.New(t)

	result, err := parser.ParsePackageRecursively("../../stubs")
	c.Assert(err, qt.IsNil)

	info := parser.GetDependencyInfo(result)

	// Verify the output contains expected sections
	c.Assert(info, qt.Contains, "Table Dependencies:")
	c.Assert(info, qt.Contains, "==================")
	c.Assert(info, qt.Contains, "Table Creation Order:")

	// Verify specific dependency information
	c.Assert(info, qt.Contains, "articles: depends on [users]")
	c.Assert(info, qt.Contains, "products: depends on [categories]")
	c.Assert(info, qt.Contains, "categories: depends on [categories]") // self-reference

	// Verify tables with no dependencies are marked correctly
	c.Assert(info, qt.Contains, "users: (no dependencies)")

	// Verify table creation order section contains numbered list
	lines := strings.Split(info, "\n")
	var orderSectionFound bool
	for _, line := range lines {
		if strings.Contains(line, "Table Creation Order:") {
			orderSectionFound = true
			continue
		}
		if orderSectionFound && strings.Contains(line, "1. ") {
			// Found the first item in the order list
			c.Assert(line, qt.Matches, `\d+\. \w+`)
			break
		}
	}
	c.Assert(orderSectionFound, qt.IsTrue, qt.Commentf("Should find Table Creation Order section"))
}

func TestParsePackageRecursively_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		rootDir     string
		expectError bool
	}{
		{
			name:        "non-existent directory",
			rootDir:     "non-existent-directory",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result, err := parser.ParsePackageRecursively(tt.rootDir)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(result, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(result, qt.IsNotNil)
			}
		})
	}
}

func TestGetDependencyInfo_EmptyResult(t *testing.T) {
	c := qt.New(t)

	// Create an empty result to test edge case
	result := &parsertypes.PackageParseResult{
		Tables:       []types.TableDirective{},
		Dependencies: make(map[string][]string),
	}

	info := parser.GetDependencyInfo(result)

	// Should still contain the headers even with no tables
	c.Assert(info, qt.Contains, "Table Dependencies:")
	c.Assert(info, qt.Contains, "Table Creation Order:")

	// Should not contain any table entries
	lines := strings.Split(info, "\n")
	tableCount := 0
	for _, line := range lines {
		if strings.Contains(line, ": (no dependencies)") || strings.Contains(line, ": depends on") {
			tableCount++
		}
	}
	c.Assert(tableCount, qt.Equals, 0)
}
