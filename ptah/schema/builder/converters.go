package builder

import (
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
