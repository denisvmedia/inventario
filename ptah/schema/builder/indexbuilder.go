package builder

import (
	"github.com/denisvmedia/inventario/ptah/schema/ast"
)

// IndexBuilder provides a fluent API for building CREATE INDEX statements
type IndexBuilder struct {
	index *ast.IndexNode
}

// NewIndex creates a new index builder
func NewIndex(name, table string, columns ...string) *IndexBuilder {
	return &IndexBuilder{
		index: ast.NewIndex(name, table, columns...),
	}
}

// Unique marks the index as unique
func (ib *IndexBuilder) Unique() *IndexBuilder {
	ib.index.SetUnique()
	return ib
}

// Type sets the index type (e.g., BTREE, HASH)
func (ib *IndexBuilder) Type(indexType string) *IndexBuilder {
	ib.index.Type = indexType
	return ib
}

// Comment sets the index comment
func (ib *IndexBuilder) Comment(comment string) *IndexBuilder {
	ib.index.Comment = comment
	return ib
}

// Build returns the completed CREATE INDEX AST node
func (ib *IndexBuilder) Build() *ast.IndexNode {
	return ib.index
}

// SchemaIndexBuilder wraps IndexBuilder and allows returning to schema
type SchemaIndexBuilder struct {
	*IndexBuilder
	schema *SchemaBuilder
}

// Unique marks the index as unique
func (sib *SchemaIndexBuilder) Unique() *SchemaIndexBuilder {
	sib.IndexBuilder.Unique()
	return sib
}

// Type sets the index type (e.g., BTREE, HASH)
func (sib *SchemaIndexBuilder) Type(indexType string) *SchemaIndexBuilder {
	sib.IndexBuilder.Type(indexType)
	return sib
}

// Comment sets the index comment
func (sib *SchemaIndexBuilder) Comment(comment string) *SchemaIndexBuilder {
	sib.IndexBuilder.Comment(comment)
	return sib
}

// End completes the index definition and returns to the schema builder
func (sib *SchemaIndexBuilder) End() *SchemaBuilder {
	sib.schema.statements = append(sib.schema.statements, sib.IndexBuilder.Build())
	return sib.schema
}
