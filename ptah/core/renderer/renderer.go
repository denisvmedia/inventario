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
	"github.com/denisvmedia/inventario/ptah/core/convert/fromschema"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/renderer/dialects/mariadb"
	"github.com/denisvmedia/inventario/ptah/core/renderer/dialects/mysql"
	"github.com/denisvmedia/inventario/ptah/core/renderer/dialects/postgres"
	"github.com/denisvmedia/inventario/ptah/core/renderer/types"
)

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
func NewRenderer(dialect string) types.RenderVisitor {
	normalizedDialect := strings.ToLower(strings.TrimSpace(dialect))

	switch normalizedDialect {
	case "postgresql", "postgres":
		return postgres.New()
	case "mysql":
		return mysql.New()
	case "mariadb":
		return mariadb.New()
	default:
		panic(fmt.Sprintf("unsupported database dialect: %s", dialect))
	}
}

// RenderSQL is a convenience function that creates a renderer and renders an AST node in one call.
//
// This function is useful for one-off SQL generation where you don't need to reuse the renderer.
// For multiple operations, it's more efficient to create a renderer once and reuse it.
func RenderSQL(dialect string, nodes ...ast.Node) (string, error) {
	r := NewRenderer(dialect)
	return VisitorRenderSQL(r, nodes...)
}

func VisitorRenderSQL(r types.RenderVisitor, nodes ...ast.Node) (string, error) {
	r.Reset()
	for _, node := range nodes {
		if err := node.Accept(r); err != nil {
			return "", err
		}
	}
	return r.Output(), nil
}

func GetOrderedCreateStatements(r *goschema.Database, dialect string) []string {
	var statements []string

	astNodes := fromschema.FromDatabase(*r, dialect)
	for _, node := range astNodes.Statements {
		sql, err := RenderSQL(dialect, node)
		if err != nil {
			panic(err)
		}
		statements = append(statements, sql)
	}

	return statements
}
