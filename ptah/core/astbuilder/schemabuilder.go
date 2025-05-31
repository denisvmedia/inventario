package astbuilder

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// SchemaBuilder provides a fluent API for building complete database schemas
type SchemaBuilder struct {
	statements []ast.Node
}

// NewSchema creates a new schema builder
func NewSchema() *SchemaBuilder {
	return &SchemaBuilder{
		statements: make([]ast.Node, 0),
	}
}

// Comment adds a comment to the schema
func (sb *SchemaBuilder) Comment(text string) *SchemaBuilder {
	sb.statements = append(sb.statements, ast.NewComment(text))
	return sb
}

// Enum adds an enum definition (PostgreSQL)
func (sb *SchemaBuilder) Enum(name string, values ...string) *SchemaBuilder {
	sb.statements = append(sb.statements, ast.NewEnum(name, values...))
	return sb
}

// Table adds a table definition and returns a table builder that can return to schema
func (sb *SchemaBuilder) Table(name string) *SchemaTableBuilder {
	tb := NewTable(name)
	// We'll add the table to statements when End() is called
	return &SchemaTableBuilder{
		TableBuilder: tb,
		schema:       sb,
	}
}

// Index adds an index definition and returns an index builder that can return to schema
func (sb *SchemaBuilder) Index(name, table string, columns ...string) *SchemaIndexBuilder {
	ib := NewIndex(name, table, columns...)
	// We'll add the index to statements when End() is called
	return &SchemaIndexBuilder{
		IndexBuilder: ib,
		schema:       sb,
	}
}

// Build returns the completed schema as a statement list
func (sb *SchemaBuilder) Build() *ast.StatementList {
	return &ast.StatementList{
		Statements: sb.statements,
	}
}
