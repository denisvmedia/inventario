package astbuilder

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// TableBuilder provides a fluent API for building CREATE TABLE statements
type TableBuilder struct {
	table *ast.CreateTableNode
}

// NewTable creates a new table builder
func NewTable(name string) *TableBuilder {
	return &TableBuilder{
		table: ast.NewCreateTable(name),
	}
}

// Comment sets the table comment
func (tb *TableBuilder) Comment(comment string) *TableBuilder {
	tb.table.Comment = comment
	return tb
}

// Engine sets the table engine (MySQL/MariaDB specific)
func (tb *TableBuilder) Engine(engine string) *TableBuilder {
	tb.table.SetOption("ENGINE", engine)
	return tb
}

// Option sets a custom table option
func (tb *TableBuilder) Option(key, value string) *TableBuilder {
	tb.table.SetOption(key, value)
	return tb
}

// Column adds a column using a fluent column builder
func (tb *TableBuilder) Column(name, dataType string) *ColumnBuilder {
	column := ast.NewColumn(name, dataType)
	tb.table.AddColumn(column)
	return &ColumnBuilder{
		column: column,
		table:  tb,
	}
}

// PrimaryKey adds a composite primary key constraint
func (tb *TableBuilder) PrimaryKey(columns ...string) *TableBuilder {
	constraint := ast.NewPrimaryKeyConstraint(columns...)
	tb.table.AddConstraint(constraint)
	return tb
}

// Unique adds a unique constraint
func (tb *TableBuilder) Unique(name string, columns ...string) *TableBuilder {
	constraint := ast.NewUniqueConstraint(name, columns...)
	tb.table.AddConstraint(constraint)
	return tb
}

// ForeignKey adds a foreign key constraint
func (tb *TableBuilder) ForeignKey(name string, columns []string, refTable, refColumn string) *ForeignKeyBuilder {
	ref := &ast.ForeignKeyRef{
		Table:  refTable,
		Column: refColumn,
		Name:   name,
	}
	constraint := ast.NewForeignKeyConstraint(name, columns, ref)
	tb.table.AddConstraint(constraint)

	return &ForeignKeyBuilder{
		ref:   ref,
		table: tb,
	}
}

// Build returns the completed CREATE TABLE AST node
func (tb *TableBuilder) Build() *ast.CreateTableNode {
	return tb.table
}

// SchemaTableBuilder wraps TableBuilder and allows returning to schema
type SchemaTableBuilder struct {
	*TableBuilder
	schema *SchemaBuilder
}

// Column adds a column using a fluent column builder that can return to schema table
func (stb *SchemaTableBuilder) Column(name, dataType string) *SchemaColumnBuilder {
	column := ast.NewColumn(name, dataType)
	stb.TableBuilder.table.AddColumn(column)
	return &SchemaColumnBuilder{
		column:      column,
		schemaTable: stb,
	}
}

// End completes the table definition and returns to the schema builder
func (stb *SchemaTableBuilder) End() *SchemaBuilder {
	stb.schema.statements = append(stb.schema.statements, stb.TableBuilder.Build())
	return stb.schema
}
