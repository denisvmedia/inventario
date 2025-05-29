package builder

import (
	"github.com/denisvmedia/inventario/ptah/schema/ast"
)

// ForeignKeyBuilder provides a fluent API for building foreign key constraints
type ForeignKeyBuilder struct {
	ref   *ast.ForeignKeyRef
	table *TableBuilder
}

// OnDelete sets the ON DELETE action
func (fkb *ForeignKeyBuilder) OnDelete(action string) *ForeignKeyBuilder {
	fkb.ref.OnDelete = action
	return fkb
}

// OnUpdate sets the ON UPDATE action
func (fkb *ForeignKeyBuilder) OnUpdate(action string) *ForeignKeyBuilder {
	fkb.ref.OnUpdate = action
	return fkb
}

// End returns to the table builder
func (fkb *ForeignKeyBuilder) End() *TableBuilder {
	return fkb.table
}

// SchemaForeignKeyBuilder provides a fluent API for building foreign key constraints in schema context
type SchemaForeignKeyBuilder struct {
	ref         *ast.ForeignKeyRef
	schemaTable *SchemaTableBuilder
}

// OnDelete sets the ON DELETE action
func (sfkb *SchemaForeignKeyBuilder) OnDelete(action string) *SchemaForeignKeyBuilder {
	sfkb.ref.OnDelete = action
	return sfkb
}

// OnUpdate sets the ON UPDATE action
func (sfkb *SchemaForeignKeyBuilder) OnUpdate(action string) *SchemaForeignKeyBuilder {
	sfkb.ref.OnUpdate = action
	return sfkb
}

// End returns to the schema table builder
func (sfkb *SchemaForeignKeyBuilder) End() *SchemaTableBuilder {
	return sfkb.schemaTable
}
