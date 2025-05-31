package transform

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// FromSchemaField converts a SchemaField to a ColumnNode
func FromSchemaField(field types.SchemaField, enums []types.GlobalEnum) *ast.ColumnNode {
	column := ast.NewColumn(field.Name, field.Type)

	// Validate enum type if field references an enum
	if isEnumType(field.Type) {
		validateEnumField(field, enums)
	}

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
func FromTableDirective(table types.TableDirective, fields []types.SchemaField, enums []types.GlobalEnum) *ast.CreateTableNode {
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
func FromSchemaIndex(index types.SchemaIndex) *ast.IndexNode {
	indexNode := ast.NewIndex(index.Name, index.StructName, index.Fields...)

	if index.Unique {
		indexNode.SetUnique()
	}

	if index.Comment != "" {
		indexNode.Comment = index.Comment
	}

	return indexNode
}

// FromGlobalEnum converts a GlobalEnum to an EnumNode
func FromGlobalEnum(enum types.GlobalEnum) *ast.EnumNode {
	return ast.NewEnum(enum.Name, enum.Values...)
}

// isEnumType checks if a field type represents an enum type
// Enum types typically start with "enum_" prefix
func isEnumType(fieldType string) bool {
	return strings.HasPrefix(fieldType, "enum_")
}

// validateEnumField validates that an enum field references a valid global enum
func validateEnumField(field types.SchemaField, enums []types.GlobalEnum) {
	// Find the corresponding global enum
	var globalEnum *types.GlobalEnum
	for _, enum := range enums {
		if enum.Name == field.Type {
			globalEnum = &enum
			break
		}
	}

	// If no global enum found, this might be an issue but we don't panic
	// as the field might be using a custom enum type
	if globalEnum == nil {
		return
	}

	// If field has enum values, validate they match the global enum
	if len(field.Enum) > 0 {
		// Check that all field enum values exist in the global enum
		globalEnumMap := make(map[string]bool)
		for _, value := range globalEnum.Values {
			globalEnumMap[value] = true
		}

		for _, fieldValue := range field.Enum {
			if fieldValue != "" && !globalEnumMap[fieldValue] {
				// Log warning or handle validation error
				// For now, we'll just continue without panicking
				fmt.Printf("Warning: enum field %s has value '%s' not found in global enum %s\n",
					field.Name, fieldValue, globalEnum.Name)
			}
		}
	}
}
