package builder

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// ColumnBuilder provides a fluent API for building column definitions
type ColumnBuilder struct {
	column *ast.ColumnNode
	table  *TableBuilder
}

// Primary marks the column as primary key
func (cb *ColumnBuilder) Primary() *ColumnBuilder {
	cb.column.SetPrimary()
	return cb
}

// NotNull marks the column as NOT NULL
func (cb *ColumnBuilder) NotNull() *ColumnBuilder {
	cb.column.SetNotNull()
	return cb
}

// Nullable marks the column as nullable (default)
func (cb *ColumnBuilder) Nullable() *ColumnBuilder {
	cb.column.Nullable = true
	return cb
}

// Unique marks the column as UNIQUE
func (cb *ColumnBuilder) Unique() *ColumnBuilder {
	cb.column.SetUnique()
	return cb
}

// AutoIncrement marks the column as auto-incrementing
func (cb *ColumnBuilder) AutoIncrement() *ColumnBuilder {
	cb.column.SetAutoIncrement()
	return cb
}

// Default sets a literal default value
func (cb *ColumnBuilder) Default(value string) *ColumnBuilder {
	cb.column.SetDefault(value)
	return cb
}

// DefaultExpression sets a function as default value
func (cb *ColumnBuilder) DefaultExpression(fn string) *ColumnBuilder {
	cb.column.SetDefaultExpression(fn)
	return cb
}

// Check sets a check constraint
func (cb *ColumnBuilder) Check(expression string) *ColumnBuilder {
	cb.column.SetCheck(expression)
	return cb
}

// Comment sets a column comment
func (cb *ColumnBuilder) Comment(comment string) *ColumnBuilder {
	cb.column.SetComment(comment)
	return cb
}

// ForeignKey sets a foreign key reference
func (cb *ColumnBuilder) ForeignKey(table, column, name string) *ForeignKeyBuilder {
	cb.column.SetForeignKey(table, column, name)

	ref := cb.column.ForeignKey
	return &ForeignKeyBuilder{
		ref:   ref,
		table: cb.table,
	}
}

// End returns to the table builder
func (cb *ColumnBuilder) End() *TableBuilder {
	return cb.table
}

// SchemaColumnBuilder wraps column building and allows returning to schema table
type SchemaColumnBuilder struct {
	column      *ast.ColumnNode
	schemaTable *SchemaTableBuilder
}

// Primary marks the column as primary key
func (scb *SchemaColumnBuilder) Primary() *SchemaColumnBuilder {
	scb.column.SetPrimary()
	return scb
}

// NotNull marks the column as NOT NULL
func (scb *SchemaColumnBuilder) NotNull() *SchemaColumnBuilder {
	scb.column.SetNotNull()
	return scb
}

// Nullable marks the column as nullable (default)
func (scb *SchemaColumnBuilder) Nullable() *SchemaColumnBuilder {
	scb.column.Nullable = true
	return scb
}

// Unique marks the column as UNIQUE
func (scb *SchemaColumnBuilder) Unique() *SchemaColumnBuilder {
	scb.column.SetUnique()
	return scb
}

// AutoIncrement marks the column as auto-incrementing
func (scb *SchemaColumnBuilder) AutoIncrement() *SchemaColumnBuilder {
	scb.column.SetAutoIncrement()
	return scb
}

// Default sets a literal default value
func (scb *SchemaColumnBuilder) Default(value string) *SchemaColumnBuilder {
	scb.column.SetDefault(value)
	return scb
}

// DefaultExpression sets a function as default value
func (scb *SchemaColumnBuilder) DefaultExpression(fn string) *SchemaColumnBuilder {
	scb.column.SetDefaultExpression(fn)
	return scb
}

// Check sets a check constraint
func (scb *SchemaColumnBuilder) Check(expression string) *SchemaColumnBuilder {
	scb.column.SetCheck(expression)
	return scb
}

// Comment sets a column comment
func (scb *SchemaColumnBuilder) Comment(comment string) *SchemaColumnBuilder {
	scb.column.SetComment(comment)
	return scb
}

// ForeignKey sets a foreign key reference
func (scb *SchemaColumnBuilder) ForeignKey(table, column, name string) *SchemaForeignKeyBuilder {
	scb.column.SetForeignKey(table, column, name)

	ref := scb.column.ForeignKey
	return &SchemaForeignKeyBuilder{
		ref:         ref,
		schemaTable: scb.schemaTable,
	}
}

// End returns to the schema table builder
func (scb *SchemaColumnBuilder) End() *SchemaTableBuilder {
	return scb.schemaTable
}
