package types

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// RenderVisitor defines the interface for rendering AST nodes to SQL statements.
//
// This interface extends the ast.Visitor interface with additional methods
// for managing the rendering process and retrieving the generated SQL output.
type RenderVisitor interface {
	ast.Visitor

	// Dialect returns the database dialect this renderer targets
	Dialect() string

	// Reset clears the internal output buffer
	Reset()

	// Output returns the current generated SQL output
	Output() string

	// Render renders an AST node to SQL and returns the result
	Render(node ast.Node) (string, error)

	// GetDialect returns the database dialect (alias for Dialect for compatibility)
	GetDialect() string

	// GetOutput returns the current generated SQL output (alias for Output for compatibility)
	GetOutput() string
}
