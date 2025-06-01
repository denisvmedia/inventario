// Package parser provides functionality for parsing Go packages to extract database schema information.
//
// This package implements a recursive parser that walks through Go source files to discover
// entity definitions, table directives, field mappings, indexes, enums, and embedded fields.
// It builds a complete database schema representation that can be used for migration generation.
//
// The parser handles:
//   - Recursive directory traversal to find all Go files
//   - Extraction of database entities from struct definitions
//   - Dependency analysis for foreign key relationships
//   - Topological sorting to ensure proper table creation order
//   - Deduplication of entities found in multiple files
//   - Generation of ordered CREATE TABLE statements
//
// Key features:
//   - Skips test files and vendor directories automatically
//   - Resolves circular dependencies with warnings
//   - Supports embedded fields with relation modes
//   - Provides debugging information for dependency analysis
//
// Example usage:
//
//	result, err := builder.ParseDir("./entities")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	statements := result.GetOrderedCreateStatements("postgresql")
//	for _, stmt := range statements {
//		fmt.Println(stmt)
//	}
package parser

import (
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/renderer/generators"
)

func GetOrderedCreateStatements(r *goschema.Database, dialect string) []string {
	statements := []string{}

	for _, table := range r.Tables {
		sql := generators.GenerateCreateTableWithEmbedded(table, r.Fields, r.Indexes, r.Enums, r.EmbeddedFields, dialect)
		statements = append(statements, sql)
	}

	return statements
}
