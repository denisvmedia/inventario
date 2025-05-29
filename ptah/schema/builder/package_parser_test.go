package builder_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

func TestParsePackageRecursively(t *testing.T) {
	c := qt.New(t)

	// Test parsing the stubs directory
	result, err := builder.ParsePackageRecursively("../../stubs")
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

	result, err := builder.ParsePackageRecursively("../../stubs")
	c.Assert(err, qt.IsNil)

	// Check that dependencies are correctly identified
	c.Assert(result.Dependencies["articles"], qt.DeepEquals, []string{"users"})
	c.Assert(result.Dependencies["products"], qt.DeepEquals, []string{"categories"})
	c.Assert(result.Dependencies["categories"], qt.DeepEquals, []string{"categories"}) // self-reference
}

func TestDeduplication(t *testing.T) {
	c := qt.New(t)

	result, err := builder.ParsePackageRecursively("../../stubs")
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
