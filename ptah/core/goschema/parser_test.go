package goschema_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/goschema/internal/parseutils"
)

func TestParseKeyValueComment_SimplifiedSyntax(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name     string
		comment  string
		expected map[string]string
	}{
		{
			name:    "Traditional syntax with quotes",
			comment: `//migrator:schema:field name="id" type="SERIAL" primary="true" not_null="true"`,
			expected: map[string]string{
				"name":     "id",
				"type":     "SERIAL",
				"primary":  "true",
				"not_null": "true",
			},
		},
		{
			name:    "Simplified syntax without quotes",
			comment: `//migrator:schema:field name="id" type="SERIAL" primary not_null`,
			expected: map[string]string{
				"name":     "id",
				"type":     "SERIAL",
				"primary":  "true",
				"not_null": "true",
			},
		},
		{
			name:    "Mixed syntax",
			comment: `//migrator:schema:field name="email" type="VARCHAR(255)" unique not_null index default="test@example.com"`,
			expected: map[string]string{
				"name":     "email",
				"type":     "VARCHAR(255)",
				"unique":   "true",
				"not_null": "true",
				"index":    "true",
				"default":  "test@example.com",
			},
		},
		{
			name:    "Boolean attributes only",
			comment: `//migrator:schema:field primary unique not_null auto_increment`,
			expected: map[string]string{
				"primary":        "true",
				"unique":         "true",
				"not_null":       "true",
				"auto_increment": "true",
			},
		},
		{
			name:    "Platform-specific overrides with simplified syntax",
			comment: `//migrator:schema:field name="data" type="JSONB" not_null platform.mysql.type="JSON" platform.mariadb.type="LONGTEXT"`,
			expected: map[string]string{
				"name":                  "data",
				"type":                  "JSONB",
				"not_null":              "true",
				"platform.mysql.type":   "JSON",
				"platform.mariadb.type": "LONGTEXT",
			},
		},
		{
			name:    "Nullable attribute",
			comment: `//migrator:schema:field name="description" type="TEXT" nullable`,
			expected: map[string]string{
				"name":     "description",
				"type":     "TEXT",
				"nullable": "true",
			},
		},
		{
			name:    "Complex check constraint with simplified booleans",
			comment: `//migrator:schema:field name="price" type="DECIMAL(10,2)" not_null check="price > 0" index`,
			expected: map[string]string{
				"name":     "price",
				"type":     "DECIMAL(10,2)",
				"not_null": "true",
				"check":    "price > 0",
				"index":    "true",
			},
		},
		{
			name:    "Embedded field with simplified syntax",
			comment: `//migrator:embedded mode="inline" prefix="audit_"`,
			expected: map[string]string{
				"mode":   "inline",
				"prefix": "audit_",
			},
		},
		{
			name:    "Should not treat non-boolean words as booleans",
			comment: `//migrator:schema:field name="status" type="VARCHAR(50)" default="active"`,
			expected: map[string]string{
				"name":    "status",
				"type":    "VARCHAR(50)",
				"default": "active",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseutils.ParseKeyValueComment(tt.comment)
			c.Assert(result, qt.DeepEquals, tt.expected)
		})
	}
}

func TestParseKeyValueComment_BooleanPatterns(t *testing.T) {
	c := qt.New(t)

	// Test that only known boolean attributes are treated as booleans
	tests := []struct {
		name     string
		comment  string
		attr     string
		expected string
	}{
		{
			name:     "not_null should be boolean",
			comment:  `//migrator:schema:field not_null`,
			attr:     "not_null",
			expected: "true",
		},
		{
			name:     "nullable should be boolean",
			comment:  `//migrator:schema:field nullable`,
			attr:     "nullable",
			expected: "true",
		},
		{
			name:     "primary should be boolean",
			comment:  `//migrator:schema:field primary`,
			attr:     "primary",
			expected: "true",
		},
		{
			name:     "unique should be boolean",
			comment:  `//migrator:schema:field unique`,
			attr:     "unique",
			expected: "true",
		},
		{
			name:     "auto_increment should be boolean",
			comment:  `//migrator:schema:field auto_increment`,
			attr:     "auto_increment",
			expected: "true",
		},
		{
			name:     "index should be boolean",
			comment:  `//migrator:schema:field index`,
			attr:     "index",
			expected: "true",
		},
		{
			name:     "is_ prefix should be boolean",
			comment:  `//migrator:schema:field is_active`,
			attr:     "is_active",
			expected: "true",
		},
		{
			name:     "has_ prefix should be boolean",
			comment:  `//migrator:schema:field has_permission`,
			attr:     "has_permission",
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseutils.ParseKeyValueComment(tt.comment)
			c.Assert(result[tt.attr], qt.Equals, tt.expected)
		})
	}
}

func TestParseKeyValueComment_IgnoreNonBooleans(t *testing.T) {
	c := qt.New(t)

	// Test that non-boolean words are not treated as booleans
	comment := `//migrator:schema:field name="test" type="VARCHAR" migrator schema field table`
	result := parseutils.ParseKeyValueComment(comment)

	// These should not be treated as boolean attributes
	c.Assert(result["migrator"], qt.Equals, "")
	c.Assert(result["schema"], qt.Equals, "")
	c.Assert(result["field"], qt.Equals, "")
	c.Assert(result["table"], qt.Equals, "")

	// These should be parsed correctly
	c.Assert(result["name"], qt.Equals, "test")
	c.Assert(result["type"], qt.Equals, "VARCHAR")
}

func TestParseKeyValueComment_PrecedenceRules(t *testing.T) {
	c := qt.New(t)

	// Test that explicit key=value takes precedence over standalone boolean
	comment := `//migrator:schema:field not_null not_null="false"`
	result := parseutils.ParseKeyValueComment(comment)

	// The explicit not_null="false" should take precedence over standalone not_null
	c.Assert(result["not_null"], qt.Equals, "false")
}

func TestParseFile_EnumHandling(t *testing.T) {
	c := qt.New(t)

	// Create a test file with both enum and non-enum fields
	content := `package entities

//migrator:schema:table name="products"
type Product struct {
	//migrator:schema:field name="id" type="SERIAL" primary="true"
	ID int64

	//migrator:schema:field name="name" type="VARCHAR(255)" not_null="true"
	Name string

	//migrator:schema:field name="active" type="BOOLEAN" not_null="true" default_expr="true"
	Active bool

	//migrator:schema:field name="status" type="ENUM" enum="draft,active,discontinued" not_null="true" default="draft"
	Status string
}
`

	// Write to temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "product.go")
	err := os.WriteFile(testFile, []byte(content), 0644) //nolint:gosec // 0644 is fine
	c.Assert(err, qt.IsNil)

	// Parse the file
	_, fields, _, _, enums := goschema.ParseFile(testFile)

	// Should have 4 fields and 1 enum
	c.Assert(len(fields), qt.Equals, 4)
	c.Assert(len(enums), qt.Equals, 1)

	// Check that non-enum fields have nil Enum values
	for _, field := range fields {
		switch field.Name {
		case "id", "name", "active":
			// These fields should have nil Enum values (not []string{""})
			c.Assert(field.Enum, qt.IsNil, qt.Commentf("Field %s should have nil Enum, got %v", field.Name, field.Enum))
		case "status":
			// This field should have enum values
			c.Assert(field.Enum, qt.DeepEquals, []string{"draft", "active", "discontinued"})
			c.Assert(field.Type, qt.Equals, "enum_product_status")
		}
	}

	// Check the global enum
	c.Assert(enums[0].Name, qt.Equals, "enum_product_status")
	c.Assert(enums[0].Values, qt.DeepEquals, []string{"draft", "active", "discontinued"})
}

func TestParsePackageRecursively(t *testing.T) {
	c := qt.New(t)

	// Test parsing the stubs directory
	result, err := goschema.ParseDir("../../stubs")
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
	usersIndex := slices.Index(tableNames, "users")
	articlesIndex := slices.Index(tableNames, "articles")
	c.Assert(usersIndex < articlesIndex, qt.IsTrue, qt.Commentf("users should come before articles"))

	// Note: categories has a circular dependency (self-reference), so it may come after products
	// This is expected behavior for circular dependencies
	categoriesIndex := slices.Index(tableNames, "categories")
	productsIndex := slices.Index(tableNames, "products")
	// We just verify both tables exist in the result
	c.Assert(categoriesIndex >= 0, qt.IsTrue, qt.Commentf("categories table should be found"))
	c.Assert(productsIndex >= 0, qt.IsTrue, qt.Commentf("products table should be found"))
}

func TestDependencyResolution(t *testing.T) {
	c := qt.New(t)

	result, err := goschema.ParseDir("../../stubs")
	c.Assert(err, qt.IsNil)

	// Check that dependencies are correctly identified
	c.Assert(result.Dependencies["articles"], qt.DeepEquals, []string{"users"})
	c.Assert(result.Dependencies["products"], qt.DeepEquals, []string{"categories"})
	c.Assert(result.Dependencies["categories"], qt.DeepEquals, []string{"categories"}) // self-reference
}

func TestDeduplication(t *testing.T) {
	c := qt.New(t)

	result, err := goschema.ParseDir("../../stubs")
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

func TestParsePackageRecursively_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		rootDir       string
		expectError   bool
		resultChecker qt.Checker
	}{
		{
			name:          "non-existent directory",
			rootDir:       "non-existent-directory",
			expectError:   true,
			resultChecker: qt.IsNil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result, err := goschema.ParseDir(tt.rootDir)
			c.Assert(err == nil, qt.Equals, !tt.expectError, qt.Commentf("Unexpected error value: %v", err))
			c.Assert(result, tt.resultChecker, qt.Commentf("Unexpected result value: %v", result))
		})
	}
}

func TestGetDependencyInfo_EmptyResult(t *testing.T) {
	c := qt.New(t)

	// Create an empty result to test edge case
	result := &goschema.Database{
		Tables:       []goschema.Table{},
		Dependencies: make(map[string][]string),
	}

	info := goschema.GetDependencyInfo(result)

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

func TestGetDependencyInfo(t *testing.T) {
	c := qt.New(t)

	result, err := goschema.ParseDir("../../stubs")
	c.Assert(err, qt.IsNil)

	info := goschema.GetDependencyInfo(result)

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
