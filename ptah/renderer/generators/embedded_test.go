package generators_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/renderer/generators"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

func TestEmbeddedFields_ProcessEmbeddedFields(t *testing.T) {
	c := qt.New(t)

	// Define embedded types with their fields
	embeddedFields := []meta.EmbeddedField{
		{
			StructName:       "Article",
			Mode:             "inline",
			EmbeddedTypeName: "Timestamps",
		},
		{
			StructName:       "Article",
			Mode:             "inline",
			Prefix:           "audit_",
			EmbeddedTypeName: "AuditInfo",
		},
		{
			StructName:       "Article",
			Mode:             "json",
			Name:             "meta_data",
			Type:             "JSONB",
			EmbeddedTypeName: "Meta",
		},
		{
			StructName:       "Article",
			Mode:             "relation",
			Field:            "author_id",
			Ref:              "users(id)",
			EmbeddedTypeName: "User",
		},
		{
			StructName:       "Article",
			Mode:             "skip",
			EmbeddedTypeName: "SkippedType",
		},
	}

	// Define the source fields from embedded types
	allFields := []meta.SchemaField{
		// Timestamps fields
		{StructName: "Timestamps", Name: "created_at", Type: "TIMESTAMP", Nullable: false},
		{StructName: "Timestamps", Name: "updated_at", Type: "TIMESTAMP", Nullable: false},
		// AuditInfo fields
		{StructName: "AuditInfo", Name: "by", Type: "TEXT", Nullable: true},
		{StructName: "AuditInfo", Name: "reason", Type: "TEXT", Nullable: true},
		// Article's own fields
		{StructName: "Article", Name: "id", Type: "INTEGER", Primary: true},
		{StructName: "Article", Name: "title", Type: "VARCHAR(255)", Nullable: false},
	}

	// Process embedded fields
	generatedFields := meta.ProcessEmbeddedFields(embeddedFields, allFields, "Article")

	// Verify the results
	c.Assert(len(generatedFields), qt.Equals, 6) // 2 from Timestamps + 2 from AuditInfo + 1 JSON + 1 relation

	// Check inline mode (Timestamps)
	timestampFields := filterFieldsByName(generatedFields, []string{"created_at", "updated_at"})
	c.Assert(len(timestampFields), qt.Equals, 2)
	c.Assert(timestampFields[0].StructName, qt.Equals, "Article")
	c.Assert(timestampFields[1].StructName, qt.Equals, "Article")

	// Check inline mode with prefix (AuditInfo)
	auditFields := filterFieldsByName(generatedFields, []string{"audit_by", "audit_reason"})
	c.Assert(len(auditFields), qt.Equals, 2)
	c.Assert(auditFields[0].StructName, qt.Equals, "Article")
	c.Assert(auditFields[1].StructName, qt.Equals, "Article")

	// Check JSON mode
	jsonFields := filterFieldsByName(generatedFields, []string{"meta_data"})
	c.Assert(len(jsonFields), qt.Equals, 1)
	c.Assert(jsonFields[0].Type, qt.Equals, "JSONB")
	c.Assert(jsonFields[0].StructName, qt.Equals, "Article")

	// Check relation mode
	relationFields := filterFieldsByName(generatedFields, []string{"author_id"})
	c.Assert(len(relationFields), qt.Equals, 1)
	c.Assert(relationFields[0].Foreign, qt.Equals, "users(id)")
	c.Assert(relationFields[0].StructName, qt.Equals, "Article")

	// Check that skip mode doesn't generate any fields
	skippedFields := filterFieldsByFieldName(generatedFields, "SkippedType")
	c.Assert(len(skippedFields), qt.Equals, 0)
}

func TestEmbeddedFields_GenerateCreateTableWithEmbedded(t *testing.T) {
	c := qt.New(t)

	table := meta.TableDirective{
		StructName: "Article",
		Name:       "articles",
	}

	fields := []meta.SchemaField{
		{StructName: "Article", Name: "id", Type: "INTEGER", Primary: true},
		{StructName: "Article", Name: "title", Type: "VARCHAR(255)", Nullable: false},
		// Embedded type fields
		{StructName: "Timestamps", Name: "created_at", Type: "TIMESTAMP", Nullable: false},
		{StructName: "Timestamps", Name: "updated_at", Type: "TIMESTAMP", Nullable: false},
	}

	embeddedFields := []meta.EmbeddedField{
		{
			StructName:       "Article",
			Mode:             "inline",
			EmbeddedTypeName: "Timestamps",
		},
		{
			StructName:       "Article",
			Mode:             "json",
			Name:             "meta_data",
			Type:             "JSONB",
			EmbeddedTypeName: "Meta",
		},
	}

	// Test PostgreSQL generation
	result := generators.GenerateCreateTableWithEmbedded(table, fields, nil, nil, embeddedFields, "postgres")

	// Verify the result contains embedded fields
	c.Assert(result, qt.Contains, "CREATE TABLE articles")
	c.Assert(result, qt.Contains, "id INTEGER PRIMARY KEY")
	c.Assert(result, qt.Contains, "title VARCHAR(255) NOT NULL")
	c.Assert(result, qt.Contains, "created_at TIMESTAMP") // From inline embedded
	c.Assert(result, qt.Contains, "updated_at TIMESTAMP") // From inline embedded
	c.Assert(result, qt.Contains, "meta_data JSONB")      // From JSON embedded
}

// Helper functions
func filterFieldsByName(fields []meta.SchemaField, names []string) []meta.SchemaField {
	var result []meta.SchemaField
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, field := range fields {
		if nameSet[field.Name] {
			result = append(result, field)
		}
	}
	return result
}

func filterFieldsByFieldName(fields []meta.SchemaField, fieldName string) []meta.SchemaField {
	var result []meta.SchemaField
	for _, field := range fields {
		if strings.Contains(field.FieldName, fieldName) {
			result = append(result, field)
		}
	}
	return result
}

func TestEmbeddedFields_PlatformSpecificOverrides(t *testing.T) {
	c := qt.New(t)

	// Define embedded field with platform-specific type overrides
	embeddedFields := []meta.EmbeddedField{
		{
			StructName:       "Article",
			Mode:             "json",
			Name:             "meta_data",
			Type:             "JSONB", // Default type
			EmbeddedTypeName: "Meta",
			Overrides: map[string]map[string]string{
				"mysql": {
					"type": "JSON",
				},
				"mariadb": {
					"type": "LONGTEXT",
				},
			},
		},
	}

	// Process embedded fields
	generatedFields := meta.ProcessEmbeddedFields(embeddedFields, nil, "Article")

	// Verify the result
	c.Assert(len(generatedFields), qt.Equals, 1)

	field := generatedFields[0]
	c.Assert(field.Name, qt.Equals, "meta_data")
	c.Assert(field.Type, qt.Equals, "JSONB") // Default type
	c.Assert(field.StructName, qt.Equals, "Article")

	// Verify platform-specific overrides are preserved
	c.Assert(field.Overrides, qt.Not(qt.IsNil))
	c.Assert(field.Overrides["mysql"]["type"], qt.Equals, "JSON")
	c.Assert(field.Overrides["mariadb"]["type"], qt.Equals, "LONGTEXT")

	// Test with MySQL generator to verify override is applied
	table := meta.TableDirective{
		StructName: "Article",
		Name:       "articles",
	}

	mysqlResult := generators.GenerateCreateTableWithEmbedded(table, nil, nil, nil, embeddedFields, "mysql")
	c.Assert(mysqlResult, qt.Contains, "meta_data JSON") // Should use MySQL override

	// Test with MariaDB generator to verify override is applied
	mariadbResult := generators.GenerateCreateTableWithEmbedded(table, nil, nil, nil, embeddedFields, "mariadb")
	c.Assert(mariadbResult, qt.Contains, "meta_data LONGTEXT") // Should use MariaDB override

	// Test with PostgreSQL generator to verify default type is used
	postgresResult := generators.GenerateCreateTableWithEmbedded(table, nil, nil, nil, embeddedFields, "postgres")
	c.Assert(postgresResult, qt.Contains, "meta_data JSONB") // Should use default type
}
