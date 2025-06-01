// Package renderer provides dialect-aware SQL rendering capabilities for the Ptah migration system.
//
// This package serves as the main entry point for converting AST nodes to SQL statements
// across different database dialects. It implements a factory pattern to create appropriate
// dialect renderers and provides a unified interface for SQL generation.
//
// The package supports multiple database platforms including PostgreSQL, MySQL, MariaDB,
// and provides a generic fallback for unknown dialects. Each dialect renderer implements
// the ast.Visitor interface to ensure consistent behavior across different database systems.
//
// Example usage:
//
//	renderer, err := renderer.NewRenderer("postgresql")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	sql, err := renderer.Render(astNode)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Println(sql)
//
// The renderer automatically handles dialect-specific SQL generation, including:
//   - Data type mappings
//   - Constraint syntax differences
//   - Enum handling (PostgreSQL vs MySQL inline enums)
//   - Index creation syntax
//   - Table options and engine specifications
package renderer

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/mariadb"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/mysql"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/postgresql"
)

// Renderer defines the interface for rendering AST nodes to SQL statements.
//
// This interface extends the ast.Visitor interface with additional methods
// for managing the rendering process and retrieving the generated SQL output.
type Renderer interface {
	ast.Visitor

	// Render converts an AST node to SQL and returns the generated statement
	Render(node ast.Node) (string, error)

	// GetDialect returns the database dialect this renderer targets
	GetDialect() string

	// Reset clears the internal output buffer
	Reset()

	// GetOutput returns the current generated SQL output
	GetOutput() string
}

// SupportedDialects returns a list of all supported database dialects.
func SupportedDialects() []string {
	return []string{"postgresql", "postgres", "mysql", "mariadb"}
}

// NewRenderer creates a new renderer for the specified database dialect.
//
// The dialect parameter should be one of the supported dialects returned by
// SupportedDialects(). The function performs case-insensitive matching and
// handles common dialect aliases (e.g., "postgres" for "postgresql").
//
// Returns an error if the dialect is not supported.
func NewRenderer(dialect string) (Renderer, error) {
	normalizedDialect := strings.ToLower(strings.TrimSpace(dialect))

	switch normalizedDialect {
	case "postgresql", "postgres":
		return postgresql.NewPostgreSQLRenderer(), nil
	case "mysql":
		return mysql.NewMySQLRenderer(), nil
	case "mariadb":
		return mariadb.NewMariaDBRenderer(), nil
	default:
		return nil, fmt.Errorf("unsupported database dialect: %s (supported: %s)",
			dialect, strings.Join(SupportedDialects(), ", "))
	}
}

// MustNewRenderer creates a new renderer for the specified database dialect.
//
// This function is similar to NewRenderer but panics if the dialect is not supported.
// It should only be used when the dialect is known to be valid at compile time.
func MustNewRenderer(dialect string) Renderer {
	renderer, err := NewRenderer(dialect)
	if err != nil {
		panic(fmt.Sprintf("failed to create renderer: %v", err))
	}
	return renderer
}

// RenderSQL is a convenience function that creates a renderer and renders an AST node in one call.
//
// This function is useful for one-off SQL generation where you don't need to reuse the renderer.
// For multiple operations, it's more efficient to create a renderer once and reuse it.
func RenderSQL(dialect string, node ast.Node) (string, error) {
	renderer, err := NewRenderer(dialect)
	if err != nil {
		return "", err
	}

	return renderer.Render(node)
}

// ValidateDialect checks if the given dialect is supported.
//
// Returns true if the dialect is supported, false otherwise.
// This function performs the same normalization as NewRenderer.
func ValidateDialect(dialect string) bool {
	_, err := NewRenderer(dialect)
	return err == nil
}
