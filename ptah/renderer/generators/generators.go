// Package generators provides database-specific SQL generation capabilities for the Ptah migration system.
//
// This package serves as the main entry point for generating SQL statements across different database dialects.
// It implements a factory pattern to create appropriate dialect generators and provides convenience functions
// for backward compatibility with existing code.
//
// The package supports multiple database platforms including PostgreSQL, MySQL, MariaDB, and provides
// a generic fallback for unknown dialects. Each dialect generator implements the DialectGenerator interface
// to ensure consistent behavior across different database systems.
//
// Example usage:
//
//	generator := generators.GetDialectGenerator("postgresql")
//	sql := generator.GenerateCreateTable(table, fields, indexes, enums)
//
// Or using the convenience functions:
//
//	sql := generators.GenerateCreateTable(table, fields, indexes, enums, "postgresql")
package generators

import (
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/platform"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/generic"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/mariadb"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/mysql"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/postgresql"
	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
)

// DialectGenerator defines the interface for database-specific SQL generation.
//
// This interface abstracts the generation of SQL statements across different database dialects,
// allowing the migration system to support multiple database platforms with dialect-specific
// optimizations and syntax variations.
//
// Implementations of this interface should handle:
//   - Database-specific data type mappings
//   - Dialect-specific SQL syntax variations
//   - Platform-specific constraints and limitations
//   - Proper escaping and quoting for the target database
type DialectGenerator interface {
	// GenerateCreateTable generates a CREATE TABLE SQL statement for the specific dialect.
	//
	// This method creates a complete table definition including columns, constraints,
	// and indexes. It handles dialect-specific data type mappings and syntax variations.
	//
	// Parameters:
	//   - table: Table metadata including name and configuration
	//   - fields: DBColumn definitions with types, constraints, and metadata
	//   - indexes: DBIndex definitions for the table
	//   - enums: Global enum definitions that may be referenced by fields
	//
	// Returns a complete CREATE TABLE SQL statement ready for execution.
	GenerateCreateTable(table goschema.Table, fields []goschema.Field, indexes []goschema.Index, enums []goschema.Enum) string

	// GenerateCreateTableWithEmbedded generates a CREATE TABLE SQL statement with embedded field support.
	//
	// This method extends GenerateCreateTable to handle embedded fields, which are fields
	// from other structs that are embedded into the current table structure. This is useful
	// for composition patterns where common fields are shared across multiple tables.
	//
	// Parameters:
	//   - table: Table metadata including name and configuration
	//   - fields: DBColumn definitions with types, constraints, and metadata
	//   - indexes: DBIndex definitions for the table
	//   - enums: Global enum definitions that may be referenced by fields
	//   - embeddedFields: Fields from embedded structs to be included in the table
	//
	// Returns a complete CREATE TABLE SQL statement with embedded fields included.
	GenerateCreateTableWithEmbedded(table goschema.Table, fields []goschema.Field, indexes []goschema.Index, enums []goschema.Enum, embeddedFields []goschema.EmbeddedField) string

	// GenerateAlterStatements generates ALTER TABLE SQL statements for schema migrations.
	//
	// This method compares the old and new field definitions and generates the necessary
	// ALTER statements to transform the table schema. It handles adding, dropping, and
	// modifying columns while respecting dialect-specific constraints and limitations.
	//
	// Parameters:
	//   - oldFields: Current field definitions in the database
	//   - newFields: Target field definitions after migration
	//
	// Returns a series of ALTER TABLE statements to migrate from old to new schema.
	GenerateAlterStatements(oldFields, newFields []goschema.Field) string

	// GenerateMigrationSQL generates SQL statements to apply schema differences for migration.
	//
	// This method transforms the schema differences captured in the SchemaDiff into executable
	// SQL statements that can be applied to bring the database schema in line with the target
	// schema. The generated SQL follows database-specific syntax and best practices.
	//
	// Parameters:
	//   - diff: The schema differences to be applied
	//   - generated: The target schema parsed from Go struct annotations
	//
	// Returns a slice of SQL statements as strings. Each statement is a complete SQL
	// command that can be executed independently. Comments and warnings are included
	// as SQL comments (lines starting with "--").
	GenerateMigrationSQL(diff *differtypes.SchemaDiff, generated *goschema.Database) []string

	// GetDialectName returns the name identifier of the database dialect.
	//
	// This method provides a way to identify which dialect generator is being used,
	// which can be useful for logging, debugging, or conditional logic based on
	// the target database platform.
	//
	// Returns a string identifier for the dialect (e.g., "postgresql", "mysql", "mariadb").
	GetDialectName() string
}

// GetDialectGenerator returns the appropriate dialect generator for the given dialect name.
//
// This function implements a factory pattern to create dialect-specific generators based on
// the provided dialect identifier. It supports the following database platforms:
//   - PostgreSQL: Returns a PostgreSQL-specific generator
//   - MySQL: Returns a MySQL-specific generator
//   - MariaDB: Returns a MariaDB-specific generator
//   - Unknown dialects: Returns a generic generator as fallback
//
// The generic generator is used for unknown dialects and provides basic SQL generation
// without dialect-specific optimizations or transformations.
//
// Parameters:
//   - dialect: The database dialect identifier (e.g., "postgresql", "mysql", "mariadb")
//
// Returns a DialectGenerator implementation appropriate for the specified dialect.
func GetDialectGenerator(dialect string) DialectGenerator {
	switch dialect {
	case platform.Postgres:
		return postgresql.New()
	case platform.MySQL:
		return mysql.New()
	case platform.MariaDB:
		return mariadb.New()
	default:
		// For unknown dialects, use a generic generator that doesn't apply dialect-specific transformations
		return generic.New(dialect)
	}
}

// GenerateCreateTable generates CREATE TABLE SQL for the given dialect.
//
// This is a convenience function that provides backward compatibility with existing code
// that doesn't use the DialectGenerator interface directly. It internally creates the
// appropriate dialect generator and delegates the SQL generation to it.
//
// For new code, consider using GetDialectGenerator() and calling the method directly
// on the returned generator, especially when generating multiple statements for the
// same dialect to avoid repeated generator creation.
//
// Parameters:
//   - table: Table metadata including name and configuration
//   - fields: Column definitions with types, constraints, and metadata
//   - indexes: Index definitions for the table
//   - enums: Global enum definitions that may be referenced by fields
//   - dialect: The database dialect identifier
//
// Returns a complete CREATE TABLE SQL statement for the specified dialect.
func GenerateCreateTable(table goschema.Table, fields []goschema.Field, indexes []goschema.Index, enums []goschema.Enum, dialect string) string {
	generator := GetDialectGenerator(dialect)
	return generator.GenerateCreateTable(table, fields, indexes, enums)
}

// GenerateCreateTableWithEmbedded generates CREATE TABLE SQL with embedded field support.
//
// This is a convenience function that provides backward compatibility with existing code
// that doesn't use the DialectGenerator interface directly. It handles embedded fields
// by including them in the table definition alongside regular fields.
//
// For new code, consider using GetDialectGenerator() and calling the method directly
// on the returned generator, especially when generating multiple statements for the
// same dialect to avoid repeated generator creation.
//
// Parameters:
//   - table: Table metadata including name and configuration
//   - fields: Column definitions with types, constraints, and metadata
//   - indexes: Index definitions for the table
//   - enums: Global enum definitions that may be referenced by fields
//   - embeddedFields: Fields from embedded structs to be included in the table
//   - dialect: The database dialect identifier
//
// Returns a complete CREATE TABLE SQL statement with embedded fields for the specified dialect.
func GenerateCreateTableWithEmbedded(table goschema.Table, fields []goschema.Field, indexes []goschema.Index, enums []goschema.Enum, embeddedFields []goschema.EmbeddedField, dialect string) string {
	generator := GetDialectGenerator(dialect)
	return generator.GenerateCreateTableWithEmbedded(table, fields, indexes, enums, embeddedFields)
}

// GenerateAlterStatements generates ALTER TABLE SQL statements for the given dialect.
//
// This is a convenience function that provides backward compatibility with existing code
// that doesn't use the DialectGenerator interface directly. It compares old and new field
// definitions and generates the necessary ALTER statements for schema migration.
//
// For new code, consider using GetDialectGenerator() and calling the method directly
// on the returned generator, especially when generating multiple statements for the
// same dialect to avoid repeated generator creation.
//
// Parameters:
//   - oldFields: Current field definitions in the database
//   - newFields: Target field definitions after migration
//   - dialect: The database dialect identifier
//
// Returns a series of ALTER TABLE statements to migrate from old to new schema for the specified dialect.
func GenerateAlterStatements(oldFields, newFields []goschema.Field, dialect string) string {
	generator := GetDialectGenerator(dialect)
	return generator.GenerateAlterStatements(oldFields, newFields)
}

// GenerateMigrationSQL generates migration SQL statements for the given dialect.
//
// This is a convenience function that provides backward compatibility with existing code
// that doesn't use the DialectGenerator interface directly. It transforms schema differences
// into executable SQL statements for database migration.
//
// For new code, consider using GetDialectGenerator() and calling the method directly
// on the returned generator, especially when generating multiple statements for the
// same dialect to avoid repeated generator creation.
//
// Parameters:
//   - diff: The schema differences to be applied
//   - generated: The target schema parsed from Go struct annotations
//   - dialect: The database dialect identifier
//
// Returns a slice of SQL statements for the specified dialect.
func GenerateMigrationSQL(diff *differtypes.SchemaDiff, generated *goschema.Database, dialect string) []string {
	generator := GetDialectGenerator(dialect)
	return generator.GenerateMigrationSQL(diff, generated)
}
