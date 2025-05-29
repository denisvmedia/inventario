package executor

import (
	"github.com/denisvmedia/inventario/ptah/renderer/generators"
	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

// GetOrderedCreateStatements generates CREATE TABLE statements in dependency order from parsed schema results.
//
// This function takes the complete schema information from a PackageParseResult and converts it
// into a series of executable SQL CREATE TABLE statements. The statements are ordered according
// to table dependencies to ensure that foreign key constraints can be satisfied during execution.
//
// The function leverages the dependency analysis performed during schema parsing to maintain
// proper table creation order. Tables with no dependencies are created first, followed by
// tables that reference them, ensuring a valid execution sequence.
//
// Statement generation process:
//  1. Iterates through tables in their dependency-sorted order
//  2. For each table, generates a complete CREATE TABLE statement including:
//     - All field definitions with proper data types and constraints
//     - Index definitions for performance optimization
//     - Enum type references where applicable
//     - Embedded field expansions with their relationships
//  3. Uses dialect-specific generators for optimal SQL syntax
//
// The generated statements are ready for immediate execution and include all necessary
// schema elements to create a fully functional database structure.
//
// Parameters:
//   - r: PackageParseResult containing the complete parsed schema with dependency ordering
//   - dialect: Target database dialect identifier (e.g., "postgresql", "mysql", "mariadb")
//
// Returns a slice of SQL CREATE TABLE statements ordered by dependencies, ready for execution.
//
// Example usage:
//
//	// Parse schema from source files
//	result, err := builder.ParsePackageRecursively("./internal/entities")
//	if err != nil {
//		return fmt.Errorf("schema parsing failed: %w", err)
//	}
//
//	// Generate PostgreSQL statements
//	statements := GetOrderedCreateStatements(result, "postgresql")
//
//	// Execute in transaction for atomicity
//	tx, err := db.Begin()
//	if err != nil {
//		return fmt.Errorf("transaction start failed: %w", err)
//	}
//	defer tx.Rollback()
//
//	for i, stmt := range statements {
//		if _, err := tx.Exec(stmt); err != nil {
//			return fmt.Errorf("failed to execute statement %d: %w\nSQL: %s", i+1, err, stmt)
//		}
//	}
//
//	return tx.Commit()
//
// Note: The input PackageParseResult should be fully processed (deduplicated, dependency-analyzed,
// and topologically sorted) before calling this function. Use builder.ParsePackageRecursively()
// to ensure proper preprocessing.
func GetOrderedCreateStatements(r *builder.PackageParseResult, dialect string) []string {
	statements := []string{}

	for _, table := range r.Tables {
		sql := generators.GenerateCreateTableWithEmbedded(table, r.Fields, r.Indexes, r.Enums, r.EmbeddedFields, dialect)
		statements = append(statements, sql)
	}

	return statements
}
