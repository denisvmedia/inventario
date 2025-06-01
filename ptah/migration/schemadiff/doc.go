// Package schemadiff provides comprehensive schema comparison and difference analysis for the Ptah schema management system.
//
// This package implements the core functionality for comparing database schemas and identifying
// differences between a desired schema (generated from Go entity definitions) and the current
// database state. It produces detailed difference reports that can be used for migration planning
// and schema synchronization.
//
// # Overview
//
// The schemadiff package serves as the bridge between schema generation and migration planning.
// It takes two schema representations - one from Go entity parsing and another from database
// introspection - and produces a comprehensive difference analysis that identifies all changes
// needed to synchronize the schemas.
//
// # Key Features
//
//   - Comprehensive schema comparison across all database objects
//   - Detailed difference analysis with change categorization
//   - Support for tables, columns, indexes, enums, and constraints
//   - Proper handling of schema modifications and additions/removals
//   - Integration with migration planning for SQL generation
//
// # Core Functionality
//
// The package provides a single main function for schema comparison:
//
//	func Compare(generated *goschema.Database, database *types.DBSchema) *types.SchemaDiff
//
// This function performs comprehensive comparison and returns a detailed difference report.
//
// # Comparison Categories
//
// The schema comparison covers these main areas:
//
//   - Tables: New, removed, and modified table structures
//   - Columns: Added, removed, and modified column definitions
//   - Indexes: New and removed database indexes
//   - Enums: New, removed, and modified enum type definitions
//   - Constraints: Primary keys, foreign keys, unique constraints, and check constraints
//
// # Usage Example
//
// Basic schema comparison:
//
//	// Parse Go entities to get desired schema
//	generated, err := goschema.ParseDir("./entities")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Connect to database and read current schema
//	conn, err := dbschema.ConnectToDatabase("postgres://user:pass@localhost/db")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer conn.Close()
//
//	database, err := conn.Reader().ReadSchema()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Compare schemas
//	diff := schemadiff.Compare(generated, database)
//
//	// Check if there are any changes
//	if diff.HasChanges() {
//		fmt.Println("Schema differences detected:")
//		// Process differences...
//	}
//
// # Difference Types
//
// The comparison produces different types of changes:
//
//   - TablesAdded: Tables that exist in generated schema but not in database
//   - TablesRemoved: Tables that exist in database but not in generated schema
//   - TablesModified: Tables that exist in both but have structural differences
//   - EnumsAdded/EnumsRemoved/EnumsModified: Enum type changes
//   - IndexesAdded/IndexesRemoved: Index changes
//
// # Table Modifications
//
// For modified tables, the comparison identifies:
//
//   - ColumnsAdded: New columns to be added
//   - ColumnsRemoved: Existing columns to be removed
//   - ColumnsModified: Existing columns with changed properties
//
// # Column Modifications
//
// For modified columns, the comparison tracks changes in:
//
//   - Data type changes
//   - Null/not null constraint changes
//   - Default value changes
//   - Primary key constraint changes
//   - Unique constraint changes
//   - Check constraint changes
//   - Foreign key constraint changes
//
// # Enum Modifications
//
// For modified enums, the comparison identifies:
//
//   - ValuesAdded: New enum values to be added
//   - ValuesRemoved: Existing enum values to be removed
//
// # Internal Architecture
//
// The package is organized with internal comparison modules:
//
//   - internal/compare: Core comparison logic for different schema objects
//   - internal/normalize: Schema normalization utilities
//   - types: Type definitions for difference structures
//
// # Integration with Ptah
//
// This package integrates with other Ptah components:
//
//   - ptah/core/goschema: Consumes generated schema from Go entities
//   - ptah/dbschema/types: Consumes database schema from introspection
//   - ptah/migration/planner: Provides difference data for migration planning
//   - ptah/migration/generator: Used in migration file generation
//
// # Performance Considerations
//
// The comparison algorithm is optimized for:
//
//   - Efficient schema traversal and comparison
//   - Memory-efficient difference storage
//   - Fast lookup operations for schema objects
//   - Minimal computational overhead for large schemas
//
// # Error Handling
//
// The comparison process is designed to be robust:
//
//   - Handles missing or malformed schema objects gracefully
//   - Provides detailed error context for debugging
//   - Continues comparison even when individual objects fail
//   - Produces partial results when possible
//
// # Thread Safety
//
// The comparison functions are thread-safe and can be called concurrently
// from multiple goroutines. The returned difference structures are immutable
// and safe for concurrent access.
//
// # Future Enhancements
//
// Potential areas for future development:
//
//   - Support for view and function comparisons
//   - Advanced constraint comparison (check constraints, triggers)
//   - Schema dependency analysis for complex changes
//   - Performance optimizations for very large schemas
//   - Configurable comparison sensitivity levels
package schemadiff
