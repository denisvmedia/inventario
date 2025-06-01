// Package postgres provides PostgreSQL-specific migration planning functionality for the Ptah schema management system.
//
// This package implements the PostgreSQL dialect for generating database migration SQL statements
// from schema differences. It transforms high-level schema changes into executable PostgreSQL DDL
// statements while respecting PostgreSQL-specific features and limitations.
//
// # Overview
//
// The postgres package is part of the Ptah migration system's dialect-specific architecture.
// It handles the conversion of schema differences (captured in differtypes.SchemaDiff) into
// PostgreSQL-compatible SQL statements represented as AST nodes.
//
// # Key Features
//
//   - PostgreSQL ENUM type support with CREATE TYPE and ALTER TYPE statements
//   - SERIAL column handling for auto-increment functionality
//   - Proper dependency ordering to avoid constraint violations
//   - Safety warnings for destructive operations
//   - Comprehensive ALTER TABLE operation support
//   - Index management with CREATE INDEX and DROP INDEX statements
//
// # Migration Order
//
// The planner generates SQL statements in a specific order to respect PostgreSQL dependencies:
//
//  1. Create new enum types (required before tables that use them)
//  2. Modify existing enum types (add new values only)
//  3. Create new tables with all columns and constraints
//  4. Modify existing tables (add/modify/remove columns)
//  5. Add new indexes
//  6. Remove indexes (safe operations)
//  7. Remove tables (with CASCADE warnings)
//  8. Remove enum types (with dependency warnings)
//
// # PostgreSQL-Specific Considerations
//
// The planner handles several PostgreSQL-specific features and limitations:
//
//   - ENUM values cannot be easily removed without recreating the entire type
//   - SERIAL columns provide auto-increment functionality
//   - Foreign key constraints require proper ordering
//   - CASCADE options for DROP operations
//   - IF EXISTS clauses for safe operations
//
// # Usage Example
//
// Basic usage with schema differences:
//
//	planner := &postgres.Planner{}
//
//	// Schema differences from comparison
//	diff := &differtypes.SchemaDiff{
//		TablesAdded: []string{"users"},
//		EnumsAdded:  []string{"user_status"},
//	}
//
//	// Target schema from Go struct parsing
//	generated := &goschema.Database{
//		Tables: []goschema.Table{
//			{Name: "users", StructName: "User"},
//		},
//		Enums: []goschema.Enum{
//			{Name: "user_status", Values: []string{"active", "inactive"}},
//		},
//		Fields: []goschema.Field{
//			{Name: "id", Type: "SERIAL", StructName: "User", Primary: true},
//			{Name: "status", Type: "user_status", StructName: "User"},
//		},
//	}
//
//	// Generate migration AST nodes
//	nodes := planner.GenerateMigrationAST(diff, generated)
//
//	// Render to SQL using PostgreSQL renderer
//	renderer := postgresql.NewRenderer()
//	for _, node := range nodes {
//		err := node.Accept(renderer)
//		if err != nil {
//			log.Fatal(err)
//		}
//	}
//	sql := renderer.GetOutput()
//
// # Integration with Ptah System
//
// This package integrates with several other Ptah components:
//
//   - ptah/core/ast: Provides AST node types for SQL representation
//   - ptah/core/convert/fromschema: Converts Go schema fields to AST columns
//   - ptah/schema/differ/differtypes: Defines schema difference structures
//   - ptah/core/goschema: Defines target schema structures
//   - ptah/renderer: Converts AST nodes to dialect-specific SQL
//
// # Safety Features
//
// The planner includes several safety mechanisms:
//
//   - Destructive operations include warning comments
//   - DROP operations use IF EXISTS clauses when possible
//   - CASCADE options are explicitly noted for review
//   - Error comments are generated when field definitions are missing
//   - Enum value removal limitations are documented in output
//
// # Error Handling
//
// The planner handles various error conditions gracefully:
//
//   - Missing field definitions result in error comments rather than failures
//   - Invalid enum references are noted with warnings
//   - Dependency conflicts are avoided through proper ordering
//   - Unsupported operations are documented rather than ignored
//
// # Performance Considerations
//
// The planner is designed for migration generation rather than runtime performance:
//
//   - Linear searches through schema elements are acceptable for migration use cases
//   - Memory usage is optimized for typical schema sizes
//   - AST node creation is efficient for batch processing
//
// # Future Enhancements
//
// Potential areas for future development:
//
//   - Support for PostgreSQL-specific features like partitioning
//   - Enhanced enum modification capabilities
//   - Constraint modification support
//   - View and function migration support
//   - Advanced index options (partial indexes, expression indexes)
package postgres
