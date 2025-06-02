// Package planner provides migration planning and SQL generation for the Ptah schema management system.
//
// This package implements the core functionality for converting schema differences into executable
// SQL statements. It serves as the bridge between schema comparison results and actual database
// migration execution, providing dialect-specific SQL generation with proper dependency ordering
// and safety considerations.
//
// # Overview
//
// The planner package takes schema differences identified by the schemadiff package and converts
// them into database-specific SQL statements that can be executed to synchronize schemas. It
// provides both AST-based and string-based SQL generation with support for multiple database
// dialects.
//
// # Key Features
//
//   - Dialect-specific migration planning for PostgreSQL, MySQL, and MariaDB
//   - AST-based SQL generation for type safety and consistency
//   - Proper dependency ordering to avoid constraint violations
//   - Safety checks and warnings for destructive operations
//   - Support for complex schema changes including tables, columns, indexes, and enums
//
// # Core Interface
//
// The package provides a Planner interface for extensible dialect support:
//
//	type Planner interface {
//		GenerateMigrationAST(diff *types.SchemaDiff, generated *goschema.Database) []ast.Node
//	}
//
// # Main Functions
//
// The package provides several convenience functions for SQL generation:
//
//   - GenerateSchemaDiffAST(): Generates AST nodes from schema differences
//   - GenerateSchemaDiffSQL(): Generates complete SQL string from schema differences
//   - GenerateSchemaDiffSQLStatements(): Generates individual SQL statements as string slice
//   - GetPlanner(): Factory function to get dialect-specific planners
//
// # Usage Example
//
// Basic migration planning:
//
//	// Compare schemas to get differences
//	diff := schemadiff.Compare(generated, database)
//
//	// Generate SQL statements for PostgreSQL
//	statements := planner.GenerateSchemaDiffSQLStatements(diff, generated, "postgres")
//
//	// Execute statements
//	for _, stmt := range statements {
//		if err := conn.Writer().ExecuteSQL(stmt); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// # Dialect-Specific Planning
//
// The package includes dialect-specific planners in the dialects subdirectory:
//
//   - dialects/postgres: PostgreSQL-specific migration planning
//   - dialects/mysql: MySQL-specific migration planning
//   - dialects/mariadb: MariaDB-specific migration planning (planned)
//
// Each dialect planner handles platform-specific features and limitations:
//
//   - PostgreSQL: ENUM types, SERIAL columns, advanced constraints
//   - MySQL: AUTO_INCREMENT, ENGINE specifications, charset handling
//   - MariaDB: MariaDB-specific extensions and optimizations
//
// # Migration Order
//
// The planner generates SQL statements in a specific order to respect database dependencies:
//
//  1. Create new enum types (PostgreSQL requirement)
//  2. Modify existing enum types (add new values only)
//  3. Create new tables with all columns and constraints
//  4. Modify existing tables (add/modify/remove columns)
//  5. Add new indexes
//  6. Remove indexes (safe operations)
//  7. Remove tables (with CASCADE warnings)
//  8. Remove enum types (with dependency warnings)
//
// # AST-Based Generation
//
// The planner uses AST-based SQL generation for several benefits:
//
//   - Type safety and validation during SQL construction
//   - Consistent formatting across different dialects
//   - Easier testing and debugging of generated SQL
//   - Extensibility for new SQL constructs and dialects
//
// # Safety Features
//
// The planner includes several safety mechanisms:
//
//   - Destructive operations include warning comments
//   - DROP operations use IF EXISTS clauses when possible
//   - CASCADE options are explicitly noted for review
//   - Proper dependency ordering to avoid constraint violations
//
// # SQL Statement Splitting
//
// The planner properly handles multi-statement SQL generation:
//
//   - Uses AST-based parsing to split SQL statements
//   - Properly handles semicolons within string literals and comments
//   - Generates individual statements for better execution control
//   - Provides detailed error context for failed statements
//
// # Integration with Ptah
//
// This package integrates with other Ptah components:
//
//   - ptah/migration/schemadiff/types: Consumes schema difference data
//   - ptah/core/goschema: Uses generated schema information
//   - ptah/core/ast: Generates AST nodes for SQL representation
//   - ptah/core/renderer: Converts AST nodes to dialect-specific SQL
//   - ptah/core/sqlutil: Uses SQL parsing utilities for statement handling
//   - ptah/migration/generator: Used in migration file generation
//
// # Error Handling
//
// The planner provides comprehensive error handling:
//
//   - Validation of schema differences before SQL generation
//   - Detailed error messages with context information
//   - Graceful handling of unsupported operations
//   - Proper error propagation for debugging
//
// # Performance Considerations
//
// The planner is optimized for:
//
//   - Efficient AST node generation and manipulation
//   - Fast SQL rendering through optimized visitor patterns
//   - Memory-efficient handling of large schema differences
//   - Minimal computational overhead for complex migrations
//
// # Extensibility
//
// New database dialects can be added by:
//
//  1. Implementing the Planner interface
//  2. Creating a new dialect package under dialects/
//  3. Adding the dialect to the GetPlanner() factory function
//  4. Implementing dialect-specific SQL generation logic
//
// # Thread Safety
//
// The planner functions are thread-safe and can be called concurrently
// from multiple goroutines. The generated AST nodes and SQL statements
// are immutable and safe for concurrent access.
package planner
