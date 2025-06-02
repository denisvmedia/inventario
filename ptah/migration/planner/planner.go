// Package planner provides the core migration planning functionality for the Ptah schema management system.
//
// This package serves as the central orchestrator for converting schema differences into executable
// SQL migration statements. It acts as a bridge between schema comparison results and database-specific
// SQL generation, providing a unified interface for migration planning across multiple database dialects.
//
// # Architecture Overview
//
// The planner package follows a factory pattern with dialect-specific implementations:
//
//	┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
//	│   SchemaDiff    │───▶│     Planner      │───▶│   AST Nodes     │
//	│   (Changes)     │    │   (Factory)      │    │  (SQL Logic)    │
//	└─────────────────┘    └──────────────────┘    └─────────────────┘
//	                                │
//	                                ▼
//	                       ┌──────────────────┐
//	                       │ Dialect-Specific │
//	                       │   Generators     │
//	                       │ (postgres/mysql) │
//	                       └──────────────────┘
//
// # Core Interface
//
// The Planner interface defines the contract for all dialect-specific migration generators:
//
//	type Planner interface {
//		GenerateMigrationAST(diff *types.SchemaDiff, generated *goschema.Database) []ast.Node
//	}
//
// Each implementation handles dialect-specific features, constraints, and SQL generation patterns.
//
// # Supported Database Dialects
//
// Currently supported database platforms:
//   - PostgreSQL: Full support with ENUM types, SERIAL columns, and advanced constraints
//   - MySQL: Complete support with AUTO_INCREMENT, ENGINE specifications, and charset handling
//   - MariaDB: Planned support (currently panics with "not implemented")
//
// # Usage Patterns
//
// The package provides multiple levels of abstraction for different use cases:
//
//	// High-level: Get SQL statements directly
//	statements := planner.GenerateSchemaDiffSQLStatements(diff, generated, "postgres")
//
//	// Mid-level: Get complete SQL string
//	sql := planner.GenerateSchemaDiffSQL(diff, generated, "postgres")
//
//	// Low-level: Get AST nodes for custom processing
//	nodes := planner.GenerateSchemaDiffAST(diff, generated, "postgres")
//
// # Error Handling
//
// The package uses panic-based error handling for unrecoverable errors such as:
//   - Unsupported database dialects
//   - SQL rendering failures
//   - Unimplemented dialect features
//
// This design choice reflects the fact that these are typically configuration or
// implementation errors that should be caught during development rather than runtime.
package planner

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/platform"
	"github.com/denisvmedia/inventario/ptah/core/renderer"
	"github.com/denisvmedia/inventario/ptah/core/sqlutil"
	"github.com/denisvmedia/inventario/ptah/migration/planner/dialects/mysql"
	"github.com/denisvmedia/inventario/ptah/migration/planner/dialects/postgres"
	"github.com/denisvmedia/inventario/ptah/migration/schemadiff/types"
)

// Planner defines the interface for database-specific migration planning.
//
// Implementations of this interface are responsible for converting schema differences
// into Abstract Syntax Tree (AST) nodes that represent the SQL operations needed to
// migrate from the current database schema to the target schema.
//
// The interface is designed to be dialect-agnostic at the contract level while
// allowing implementations to handle database-specific features, constraints,
// and optimization strategies.
//
// # Implementation Requirements
//
// Implementations must:
//   - Generate AST nodes in dependency-aware order (e.g., create tables before foreign keys)
//   - Handle dialect-specific data types and constraints appropriately
//   - Provide safe migration paths that minimize data loss risks
//   - Support rollback scenarios where applicable
//
// # Parameters
//
//   - diff: Contains the differences between target and current schemas
//   - generated: The target schema derived from Go struct annotations
//
// # Return Value
//
// Returns a slice of AST nodes representing the SQL operations needed for migration.
// The nodes are ordered to respect database dependencies and constraints.
//
// # Example Implementation Pattern
//
//	func (p *PostgresPlanner) GenerateMigrationAST(diff *types.SchemaDiff, generated *goschema.Database) []ast.Node {
//		var nodes []ast.Node
//
//		// 1. Create enum types first (PostgreSQL-specific)
//		nodes = append(nodes, p.generateEnumCreations(diff, generated)...)
//
//		// 2. Create tables in dependency order
//		nodes = append(nodes, p.generateTableCreations(diff, generated)...)
//
//		// 3. Add indexes and constraints
//		nodes = append(nodes, p.generateIndexCreations(diff, generated)...)
//
//		return nodes
//	}
type Planner interface {
	// GenerateMigrationAST converts schema differences into database-specific AST nodes.
	//
	// This method is the core of the migration planning process. It takes the differences
	// identified by schema comparison and the target schema definition, then generates
	// a sequence of AST nodes that represent the SQL operations needed to transform
	// the current database schema to match the target schema.
	//
	// The generated AST nodes are ordered to respect database dependencies:
	//   1. Type definitions (enums, custom types)
	//   2. Table creations in dependency order
	//   3. Column additions and modifications
	//   4. Index and constraint additions
	//   5. Data migrations (if applicable)
	//   6. Cleanup operations (drops, if safe)
	//
	// Parameters:
	//   - diff: Schema differences identified by the schemadiff package
	//   - generated: Target schema parsed from Go struct annotations
	//
	// Returns:
	//   - []ast.Node: Ordered sequence of AST nodes for migration execution
	//
	// The returned nodes can be rendered to SQL using the renderer package or
	// processed further for validation, optimization, or custom transformations.
	GenerateMigrationAST(diff *types.SchemaDiff, generated *goschema.Database) []ast.Node
}

// GetPlanner returns a dialect-specific migration planner for the given database dialect.
//
// This factory function creates and returns the appropriate planner implementation
// based on the specified database dialect. Each planner handles dialect-specific
// features, SQL syntax variations, and optimization strategies.
//
// # Supported Dialects
//
// The function supports the following database dialects:
//   - "postgres": Returns a PostgreSQL-specific planner with support for ENUM types,
//     SERIAL columns, and PostgreSQL-specific constraints
//   - "mysql": Returns a MySQL-specific planner with support for AUTO_INCREMENT,
//     ENGINE specifications, and MySQL-specific features
//   - "mariadb": Currently not implemented (panics with "not implemented")
//
// # Parameters
//
//   - dialect: Database dialect identifier (use constants from platform package)
//
// # Return Value
//
// Returns a Planner implementation specific to the requested dialect.
//
// # Panics
//
// This function panics in the following cases:
//   - Unknown or unsupported dialect is specified
//   - MariaDB dialect is requested (not yet implemented)
//   - Empty or invalid dialect string is provided
//
// # Usage Example
//
//	import "github.com/denisvmedia/inventario/ptah/core/platform"
//
//	// Get PostgreSQL planner
//	pgPlanner := planner.GetPlanner(platform.Postgres)
//
//	// Get MySQL planner
//	mysqlPlanner := planner.GetPlanner(platform.MySQL)
//
//	// Generate migration AST
//	nodes := pgPlanner.GenerateMigrationAST(diff, generated)
//
// # Design Rationale
//
// The factory pattern is used here to:
//   - Provide a clean, consistent interface for planner creation
//   - Allow easy extension for new database dialects
//   - Centralize dialect validation and error handling
//   - Enable dependency injection and testing scenarios
func GetPlanner(dialect string) Planner {
	switch dialect {
	case platform.Postgres:
		return postgres.New()
	case platform.MySQL:
		return mysql.New()
	case platform.MariaDB:
		panic("not implemented")
	default:
		// For unknown dialects, use a generic generator that doesn't apply dialect-specific transformations
		panic("not implemented")
	}
}

// GenerateSchemaDiffAST generates AST nodes for schema differences using the specified dialect.
//
// This is a convenience function that combines planner creation and AST generation
// into a single call. It internally uses GetPlanner to obtain the appropriate
// dialect-specific planner and then calls GenerateMigrationAST on it.
//
// # Parameters
//
//   - diff: Schema differences identified by the schemadiff package
//   - generated: Target schema parsed from Go struct annotations
//   - dialect: Database dialect identifier (use constants from platform package)
//
// # Return Value
//
// Returns a slice of AST nodes representing the SQL operations needed for migration.
// The nodes are ordered to respect database dependencies and constraints.
//
// # Panics
//
// This function panics if:
//   - An unsupported dialect is specified
//   - The dialect-specific planner is not implemented
//
// # Usage Example
//
//	import "github.com/denisvmedia/inventario/ptah/core/platform"
//
//	// Generate AST nodes for PostgreSQL
//	nodes := planner.GenerateSchemaDiffAST(diff, generated, platform.Postgres)
//
//	// Process nodes for custom validation or transformation
//	for _, node := range nodes {
//		// Custom processing logic
//	}
//
// # See Also
//
//   - GenerateSchemaDiffSQL: For complete SQL string generation
//   - GenerateSchemaDiffSQLStatements: For individual SQL statements
//   - GetPlanner: For direct planner access
func GenerateSchemaDiffAST(diff *types.SchemaDiff, generated *goschema.Database, dialect string) []ast.Node {
	planner := GetPlanner(dialect)
	return planner.GenerateMigrationAST(diff, generated)
}

// GenerateSchemaDiffSQLStatements generates individual SQL statements for schema differences.
//
// This high-level convenience function provides the most commonly used output format:
// a slice of individual SQL statements that can be executed sequentially to perform
// the migration. It combines AST generation, SQL rendering, and statement splitting
// into a single operation.
//
// # Parameters
//
//   - diff: Schema differences identified by the schemadiff package
//   - generated: Target schema parsed from Go struct annotations
//   - dialect: Database dialect identifier (use constants from platform package)
//
// # Return Value
//
// Returns a slice of individual SQL statements, each ending with a semicolon.
// The statements are ordered to respect database dependencies and can be executed
// sequentially to perform the migration.
//
// # Statement Processing
//
// The function performs the following processing steps:
//  1. Generate AST nodes using GenerateSchemaDiffAST
//  2. Render AST nodes to complete SQL using the renderer package
//  3. Split the SQL into individual statements using sqlutil.SplitSQLStatements
//  4. Return the statements as a string slice
//
// # Panics
//
// This function panics if:
//   - An unsupported dialect is specified
//   - SQL rendering fails due to invalid AST nodes
//   - Statement splitting encounters malformed SQL
//
// # Usage Example
//
//	import "github.com/denisvmedia/inventario/ptah/core/platform"
//
//	// Generate SQL statements for MySQL
//	statements := planner.GenerateSchemaDiffSQLStatements(diff, generated, platform.MySQL)
//
//	// Execute statements sequentially
//	for _, stmt := range statements {
//		if err := db.Exec(stmt); err != nil {
//			log.Fatalf("Failed to execute statement: %v", err)
//		}
//	}
//
// # See Also
//
//   - GenerateSchemaDiffSQL: For complete SQL string without splitting
//   - GenerateSchemaDiffAST: For AST nodes without rendering
func GenerateSchemaDiffSQLStatements(diff *types.SchemaDiff, generated *goschema.Database, dialect string) []string {
	output := GenerateSchemaDiffSQL(diff, generated, dialect)
	statements := sqlutil.SplitSQLStatements(output)
	return statements
}

// GenerateSchemaDiffSQL generates complete SQL for schema differences as a single string.
//
// This function provides a mid-level interface that generates a complete SQL script
// containing all the statements needed to perform the migration. The output is a
// single string with multiple SQL statements separated by semicolons and newlines.
//
// # Parameters
//
//   - diff: Schema differences identified by the schemadiff package
//   - generated: Target schema parsed from Go struct annotations
//   - dialect: Database dialect identifier (use constants from platform package)
//
// # Return Value
//
// Returns a complete SQL script as a single string. The script contains all
// statements needed for the migration, properly formatted and ordered.
//
// # SQL Generation Process
//
// The function performs the following steps:
//  1. Generate AST nodes using GenerateSchemaDiffAST
//  2. Render all AST nodes to SQL using the dialect-specific renderer
//  3. Return the complete SQL as a single string
//
// # Output Format
//
// The generated SQL includes:
//   - Proper statement termination with semicolons
//   - Appropriate line breaks and formatting
//   - Comments for complex operations (dialect-dependent)
//   - Dependency-ordered statements
//
// # Panics
//
// This function panics if:
//   - An unsupported dialect is specified
//   - SQL rendering fails due to invalid AST nodes
//   - The renderer encounters an unhandled node type
//
// # Usage Example
//
//	import "github.com/denisvmedia/inventario/ptah/core/platform"
//
//	// Generate complete SQL script for PostgreSQL
//	sql := planner.GenerateSchemaDiffSQL(diff, generated, platform.Postgres)
//
//	// Write to migration file
//	if err := os.WriteFile("migration.sql", []byte(sql), 0644); err != nil {
//		log.Fatalf("Failed to write migration file: %v", err)
//	}
//
//	// Or execute as a single transaction
//	if _, err := db.Exec(sql); err != nil {
//		log.Fatalf("Migration failed: %v", err)
//	}
//
// # See Also
//
//   - GenerateSchemaDiffSQLStatements: For individual SQL statements
//   - GenerateSchemaDiffAST: For AST nodes without rendering
func GenerateSchemaDiffSQL(diff *types.SchemaDiff, generated *goschema.Database, dialect string) string {
	astNodes := GenerateSchemaDiffAST(diff, generated, dialect)
	output, err := renderer.RenderSQL(dialect, astNodes...)
	if err != nil {
		panic(err)
	}
	return output
}
