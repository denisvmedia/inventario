package builder

import (
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

// FromSchemaField converts a SchemaField to a ColumnNode
func FromSchemaField(field meta.SchemaField, enums []meta.GlobalEnum) *ast.ColumnNode {
	column := ast.NewColumn(field.Name, field.Type)

	if !field.Nullable {
		column.SetNotNull()
	}

	if field.Primary {
		column.SetPrimary()
	}

	if field.Unique {
		column.SetUnique()
	}

	if field.AutoInc {
		column.SetAutoIncrement()
	}

	if field.Default != "" {
		column.SetDefault(field.Default)
	}

	if field.DefaultFn != "" {
		column.SetDefaultFunction(field.DefaultFn)
	}

	if field.Check != "" {
		column.SetCheck(field.Check)
	}

	if field.Comment != "" {
		column.SetComment(field.Comment)
	}

	if field.Foreign != "" {
		column.SetForeignKey(field.Foreign, "", field.ForeignKeyName)
	}

	return column
}

// FromTableDirective converts a TableDirective to a CreateTableNode
func FromTableDirective(table meta.TableDirective, fields []meta.SchemaField, enums []meta.GlobalEnum) *ast.CreateTableNode {
	createTable := ast.NewCreateTable(table.Name)

	if table.Comment != "" {
		createTable.Comment = table.Comment
	}

	if table.Engine != "" {
		createTable.SetOption("ENGINE", table.Engine)
	}

	// Add columns
	for _, field := range fields {
		if field.StructName == table.StructName {
			column := FromSchemaField(field, enums)
			createTable.AddColumn(column)
		}
	}

	// Add composite primary key if specified
	if len(table.PrimaryKey) > 1 {
		constraint := ast.NewPrimaryKeyConstraint(table.PrimaryKey...)
		createTable.AddConstraint(constraint)
	}

	return createTable
}

// FromSchemaIndex converts a SchemaIndex to an IndexNode
func FromSchemaIndex(index meta.SchemaIndex) *ast.IndexNode {
	indexNode := ast.NewIndex(index.Name, index.StructName, index.Fields...)

	if index.Unique {
		indexNode.SetUnique()
	}

	if index.Comment != "" {
		indexNode.Comment = index.Comment
	}

	return indexNode
}

// ProcessEmbeddedFields processes embedded fields and generates corresponding schema fields
func ProcessEmbeddedFields(embeddedFields []meta.EmbeddedField, allFields []meta.SchemaField, structName string) []meta.SchemaField {
	var generatedFields []meta.SchemaField

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

			generatedFields = append(generatedFields, meta.SchemaField{
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

			generatedFields = append(generatedFields, meta.SchemaField{
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
