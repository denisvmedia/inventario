package transform

import (
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// ProcessEmbeddedFields processes embedded fields and generates corresponding schema fields
func ProcessEmbeddedFields(embeddedFields []types.EmbeddedField, allFields []types.SchemaField, structName string) []types.SchemaField {
	var generatedFields []types.SchemaField

	for _, embedded := range embeddedFields {
		if embedded.StructName != structName {
			continue
		}

		switch embedded.Mode {
		case "inline":
			// Find fields from the embedded type and add them with optional prefix
			for _, field := range allFields {
				if field.StructName == embedded.EmbeddedTypeName {
					newField := field
					newField.StructName = structName

					// Apply prefix if specified
					if embedded.Prefix != "" {
						newField.Name = embedded.Prefix + field.Name
					}

					generatedFields = append(generatedFields, newField)
				}
			}

		case "json":
			// Create a single JSON/JSONB column
			columnName := embedded.Name
			if columnName == "" {
				columnName = strings.ToLower(embedded.EmbeddedTypeName) + "_data"
			}

			columnType := embedded.Type
			if columnType == "" {
				columnType = "JSONB" // Default to JSONB
			}

			generatedFields = append(generatedFields, types.SchemaField{
				StructName: structName,
				FieldName:  embedded.EmbeddedTypeName,
				Name:       columnName,
				Type:       columnType,
				Nullable:   embedded.Nullable,
				Comment:    embedded.Comment,
				Overrides:  embedded.Overrides, // Pass through platform-specific overrides
			})

		case "relation":
			// Create a foreign key field
			if embedded.Field == "" || embedded.Ref == "" {
				continue // Skip if required fields are missing
			}

			// Parse the reference to get the type
			refType := "INTEGER" // Default type
			if strings.Contains(embedded.Ref, "VARCHAR") || strings.Contains(embedded.Ref, "TEXT") {
				refType = "VARCHAR(36)" // Assume UUID if not integer
			}

			generatedFields = append(generatedFields, types.SchemaField{
				StructName:     structName,
				FieldName:      embedded.EmbeddedTypeName,
				Name:           embedded.Field,
				Type:           refType,
				Nullable:       embedded.Nullable,
				Foreign:        embedded.Ref,
				ForeignKeyName: "fk_" + strings.ToLower(structName) + "_" + strings.ToLower(embedded.Field),
				Comment:        embedded.Comment,
			})

		case "skip":
			// Do nothing - skip this embedded field
			continue

		default:
			// Default to inline mode if no mode specified
			for _, field := range allFields {
				if field.StructName == embedded.EmbeddedTypeName {
					newField := field
					newField.StructName = structName
					generatedFields = append(generatedFields, newField)
				}
			}
		}
	}

	return generatedFields
}
