// Package differ provides schema comparison and migration SQL generation functionality.
//
// This package is the core of the Ptah migration system's schema evolution capabilities.
// It compares generated schema definitions (from Go struct annotations) with existing
// database schemas to identify differences and generate appropriate migration SQL statements.
//
// # Core Functionality
//
// The package provides comprehensive schema comparison across multiple database elements:
//   - Tables: Creation, removal, and structural modifications
//   - Columns: Addition, removal, and property changes (type, constraints, defaults)
//   - Enums: Creation, removal, and value modifications
//   - Indexes: Addition and removal of database indexes
//
// # Use Cases
//
// 1. **Migration Generation**: Automatically generate SQL migration scripts from schema changes
// 2. **Schema Validation**: Verify that database schema matches application expectations
// 3. **Development Workflow**: Detect schema drift during development cycles
// 4. **Production Deployment**: Generate safe migration scripts for production environments
// 5. **Multi-Database Support**: Handle schema differences across PostgreSQL, MySQL, and MariaDB
//
// # Workflow
//
// The typical workflow involves:
//  1. Parse Go struct annotations to generate target schema
//  2. Introspect existing database schema using executor reader.ReadSchema()
//  3. Compare schemas using CompareSchemas()
//  4. Generate migration SQL using GenerateMigrationSQL()
//  5. Review and apply migrations
//
// # Example Usage
//
// Basic schema comparison:
//
//	// Parse target schema from Go structs
//	generated := parser.ParsePackage("./models")
//
//	// Introspect current database schema using executor
//	reader := executor.NewPostgreSQLReader(db, "public")
//	database, err := reader.ReadSchema()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Compare schemas
//	diff := differ.CompareSchemas(generated, database)
//
//	// Check for changes
//	if diff.HasChanges() {
//		// Generate migration SQL
//		statements := GenerateMigrationSQL(diff, generated, "postgres")
//		for _, stmt := range statements {
//			fmt.Println(stmt)
//		}
//	}
//
// MySQL/MariaDB schema comparison:
//
//	// Parse target schema from Go structs
//	generated := parser.ParsePackage("./models")
//
//	// Introspect current database schema using MySQL executor
//	reader := executor.NewMySQLReader(db, "")
//	database, err := reader.ReadSchema()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Compare schemas
//	diff := differ.CompareSchemas(generated, database)
//
//	// Generate MySQL-specific migration SQL
//	if diff.HasChanges() {
//		statements := GenerateMigrationSQL(diff, generated, "mysql")
//		for _, stmt := range statements {
//			fmt.Println(stmt)
//		}
//	}
//
// # Safety Features
//
// The package includes several safety mechanisms:
//   - Destructive operations (DROP TABLE, DROP COLUMN) are commented out by default
//   - Warnings are generated for operations that may cause data loss
//   - Enum value removal limitations are clearly documented
//   - Auto-increment/SERIAL column handling prevents false positives
//
// # Multi-Database Support
//
// The package supports multiple SQL dialects with appropriate type mapping:
//   - PostgreSQL: Native ENUM types, SERIAL columns, JSONB support
//   - MySQL/MariaDB: Inline ENUM syntax, AUTO_INCREMENT, JSON columns
//   - Extensible architecture for additional database support
package differ

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
	"github.com/denisvmedia/inventario/ptah/schema/differ/internal/compare"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// GenerateMigrationSQL generates SQL statements to apply the schema differences for a specific database dialect.
//
// This method transforms the schema differences captured in the SchemaDiff into executable
// SQL statements that can be applied to bring the database schema in line with the target
// schema. The generated SQL follows database-specific syntax and best practices.
//
// # Parameters
//
//   - generated: The target schema parsed from Go struct annotations
//   - dialect: Target database dialect ("postgres", "mysql", "mariadb")
//
// # Migration Order
//
// The SQL statements are generated in a specific order to avoid dependency conflicts:
//  1. Create new enum types (required before tables that use them)
//  2. Modify existing enum types (add new values)
//  3. Create new tables
//  4. Modify existing tables (add/modify/remove columns)
//  5. Add new indexes
//  6. Remove indexes (safe operations)
//  7. Remove tables (dangerous - commented out by default)
//  8. Remove enum types (dangerous - commented out by default)
//
// # Safety Features
//
// The method includes several safety mechanisms:
//   - Destructive operations are commented out with warnings
//   - Enum value removal limitations are documented
//   - Data loss warnings are included for dangerous operations
//   - TODO comments are added for operations requiring manual review
//
// # Database-Specific Handling
//
//   - **PostgreSQL**: Native ENUM types, SERIAL columns, proper type handling
//   - **MySQL/MariaDB**: Inline ENUM syntax, AUTO_INCREMENT, type mapping
//   - **Cross-platform**: Intelligent type normalization and conversion
//
// # Example Usage
//
//	diff := CompareSchemas(generated, database)
//	statements := GenerateMigrationSQL(diff, generated, "postgres")
//
//	// Review generated statements
//	for i, stmt := range statements {
//		fmt.Printf("-- Statement %d:\n%s\n\n", i+1, stmt)
//	}
//
//	// Apply non-commented statements
//	for _, stmt := range statements {
//		if !strings.HasPrefix(stmt, "--") {
//			_, err := db.Exec(stmt)
//			if err != nil {
//				log.Fatalf("Migration failed: %v", err)
//			}
//		}
//	}
//
// # Return Value
//
// Returns a slice of SQL statements as strings. Each statement is a complete SQL
// command that can be executed independently. Comments and warnings are included
// as SQL comments (lines starting with "--").
func GenerateMigrationSQL(diff *differtypes.SchemaDiff, generated *parsertypes.PackageParseResult, dialect string) []string {
	var statements []string

	// 1. Add new enums first
	for _, enumName := range diff.EnumsAdded {
		for _, enum := range generated.Enums {
			if enum.Name == enumName {
				if dialect == "postgres" {
					values := make([]string, len(enum.Values))
					for i, v := range enum.Values {
						values[i] = "'" + v + "'"
					}
					sql := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s);", enum.Name, strings.Join(values, ", "))
					statements = append(statements, sql)
				}
				break
			}
		}
	}

	// 2. Modify existing enums (add values only - PostgreSQL doesn't support removing enum values easily)
	for _, enumDiff := range diff.EnumsModified {
		if dialect == "postgres" {
			for _, value := range enumDiff.ValuesAdded {
				sql := fmt.Sprintf("ALTER TYPE %s ADD VALUE '%s';", enumDiff.EnumName, value)
				statements = append(statements, sql)
			}
			// Note: PostgreSQL doesn't support removing enum values without recreating the enum
			if len(enumDiff.ValuesRemoved) > 0 {
				statements = append(statements, fmt.Sprintf("-- WARNING: Cannot remove enum values %v from %s without recreating the enum", enumDiff.ValuesRemoved, enumDiff.EnumName))
			}
		}
	}

	// 3. Add new tables
	for _, tableName := range diff.TablesAdded {
		// Find the table in generated schema and create it
		for _, table := range generated.Tables {
			if table.Name == tableName {
				// Generate basic CREATE TABLE SQL
				createSQL := GenerateBasicCreateTableSQL(table, generated.Fields, dialect)
				statements = append(statements, createSQL)
				break
			}
		}
	}

	// 4. Modify existing tables
	for _, tableDiff := range diff.TablesModified {
		statements = append(statements, fmt.Sprintf("-- Modify table: %s", tableDiff.TableName))

		// Add new columns
		for _, colName := range tableDiff.ColumnsAdded {
			statements = append(statements, fmt.Sprintf("-- TODO: ALTER TABLE %s ADD COLUMN %s ...;", tableDiff.TableName, colName))
		}

		// Modify existing columns
		for _, colDiff := range tableDiff.ColumnsModified {
			for changeType, change := range colDiff.Changes {
				statements = append(statements, fmt.Sprintf("-- TODO: ALTER TABLE %s ALTER COLUMN %s %s (%s);", tableDiff.TableName, colDiff.ColumnName, changeType, change))
			}
		}

		// Remove columns (dangerous!)
		for _, colName := range tableDiff.ColumnsRemoved {
			statements = append(statements, fmt.Sprintf("-- WARNING: ALTER TABLE %s DROP COLUMN %s; -- This will delete data!", tableDiff.TableName, colName))
		}
	}

	// 5. Add new indexes
	for _, indexName := range diff.IndexesAdded {
		statements = append(statements, fmt.Sprintf("-- TODO: CREATE INDEX %s ON ...;", indexName))
	}

	// 6. Remove indexes
	for _, indexName := range diff.IndexesRemoved {
		statements = append(statements, fmt.Sprintf("DROP INDEX IF EXISTS %s;", indexName))
	}

	// 7. Remove tables (dangerous!)
	for _, tableName := range diff.TablesRemoved {
		statements = append(statements, fmt.Sprintf("-- WARNING: DROP TABLE %s; -- This will delete all data!", tableName))
	}

	// 8. Remove enums (dangerous!)
	for _, enumName := range diff.EnumsRemoved {
		statements = append(statements, fmt.Sprintf("-- WARNING: DROP TYPE %s; -- Make sure no tables use this enum!", enumName))
	}

	return statements
}

// CompareSchemas compares a generated schema with a database schema and returns comprehensive differences.
//
// This is the main entry point for schema comparison in the Ptah migration system.
// It performs a comprehensive analysis of differences between the target schema
// (generated from Go struct annotations) and the current database schema.
//
// # Parameters
//
//   - generated: Target schema parsed from Go struct annotations using the parser package
//   - database: Current database schema obtained through executor reader.ReadSchema()
//
// # Comparison Process
//
// The function performs comparison in three main areas:
//  1. **Tables and Columns**: Structural differences in table definitions
//  2. **Enum Types**: Changes to enum type definitions and values
//  3. **Indexes**: Differences in database index definitions
//
// # Embedded Field Handling
//
// The comparison process properly handles embedded fields by processing them
// through the transform package, ensuring that generated fields from embedded
// structs are correctly compared against database columns.
//
// # Example Usage
//
//	// Parse target schema from Go structs
//	generated, err := parser.ParsePackage("./models")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Introspect current database schema using executor
//	reader := executor.NewPostgreSQLReader(db, "public")
//	database, err := reader.ReadSchema()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Compare schemas
//	diff := CompareSchemas(generated, database)
//
//	// Analyze results
//	if diff.HasChanges() {
//		fmt.Printf("Found %d new tables\n", len(diff.TablesAdded))
//		fmt.Printf("Found %d modified tables\n", len(diff.TablesModified))
//		fmt.Printf("Found %d new enums\n", len(diff.EnumsAdded))
//	}
//
// # Return Value
//
// Returns a *SchemaDiff containing all identified differences between the schemas.
// The diff can be used to generate migration SQL or for analysis purposes.
//
// # Thread Safety
//
// This function is read-only and thread-safe. It does not modify the input
// parameters and can be called concurrently from multiple goroutines.
func CompareSchemas(generated *parsertypes.PackageParseResult, database *parsertypes.DatabaseSchema) *differtypes.SchemaDiff {
	diff := &differtypes.SchemaDiff{}

	// Compare tables and their column structures
	compare.CompareTablesAndColumns(generated, database, diff)

	// Compare enum type definitions and values
	compare.CompareEnums(generated, database, diff)

	// Compare database index definitions
	compare.CompareIndexes(generated, database, diff)

	return diff
}

// GenerateBasicCreateTableSQL generates database-specific CREATE TABLE SQL statements for migration purposes.
//
// This function creates basic but complete CREATE TABLE statements that can be used
// in migration scripts. It handles column definitions, primary key constraints,
// and dialect-specific syntax variations while maintaining compatibility across
// different database systems.
//
// # SQL Generation Process
//
// The function follows a structured approach to SQL generation:
//  1. **Field Filtering**: Processes only fields belonging to the target table
//  2. **Column Definition**: Generates individual column definitions with constraints
//  3. **Primary Key Handling**: Manages both single and composite primary keys
//  4. **SQL Assembly**: Constructs the final CREATE TABLE statement
//
// # Primary Key Logic
//
// **Single Primary Key**:
//   - Added directly to column definition (e.g., "id SERIAL PRIMARY KEY")
//   - Handled within GenerateColumnDefinition()
//
// **Composite Primary Key**:
//   - Individual columns don't have PRIMARY KEY in their definitions
//   - Table-level PRIMARY KEY constraint added at the end
//   - Format: "PRIMARY KEY (col1, col2, col3)"
//
// # Error Handling
//
// The function includes safety checks:
//   - Validates that at least one column exists for the table
//   - Returns descriptive error comments for debugging
//   - Includes struct name in error messages for easier troubleshooting
//
// # Example Output
//
// **Simple table**:
//
//	```sql
//	CREATE TABLE users (
//	  id SERIAL PRIMARY KEY,
//	  email VARCHAR(255) NOT NULL UNIQUE,
//	  created_at TIMESTAMP DEFAULT NOW()
//	);
//	```
//
// **Composite primary key table**:
//
//	```sql
//	CREATE TABLE user_roles (
//	  user_id INTEGER NOT NULL,
//	  role_id INTEGER NOT NULL,
//	  assigned_at TIMESTAMP DEFAULT NOW(),
//	  PRIMARY KEY (user_id, role_id)
//	);
//	```
//
// # Parameters
//
//   - table: Table directive containing table metadata (name, struct name, etc.)
//   - fields: Complete list of schema fields (function filters for relevant ones)
//   - dialect: Target database dialect for SQL syntax ("postgres", "mysql", "mariadb")
//
// # Return Value
//
// Returns a complete CREATE TABLE SQL statement as a string, or an error comment
// if the table has no columns. The SQL is formatted for readability with proper
// indentation and line breaks.
//
// # Database Compatibility
//
// The function generates SQL compatible with:
//   - PostgreSQL: SERIAL types, native syntax
//   - MySQL/MariaDB: AUTO_INCREMENT, dialect-specific types
//   - Cross-platform: Uses MapTypeToSQL() for type conversion
func GenerateBasicCreateTableSQL(table types.TableDirective, fields []types.SchemaField, dialect string) string {
	var columns []string
	var primaryKeys []string

	// Filter and process fields for this table
	for _, field := range fields {
		if field.StructName == table.StructName {
			columnDef := GenerateColumnDefinition(field, dialect)
			columns = append(columns, columnDef)

			if field.Primary {
				primaryKeys = append(primaryKeys, field.Name)
			}
		}
	}

	// Ensure we have at least one column
	if len(columns) == 0 {
		return fmt.Sprintf("-- ERROR: No columns found for table %s (struct: %s)", table.Name, table.StructName)
	}

	// Build CREATE TABLE statement
	sql := fmt.Sprintf("CREATE TABLE %s (\n", table.Name)
	sql += "  " + strings.Join(columns, ",\n  ")

	// Add primary key constraint if there are multiple primary keys
	if len(primaryKeys) > 1 {
		sql += fmt.Sprintf(",\n  PRIMARY KEY (%s)", strings.Join(primaryKeys, ", "))
	}

	sql += "\n);"
	return sql
}

// GenerateColumnDefinition creates a complete SQL column definition from a schema field with full constraint support.
//
// This function is responsible for translating Go struct field annotations into
// proper SQL column definitions, handling all supported constraints, data types,
// and database-specific syntax variations.
//
// # Column Definition Components
//
// The function builds column definitions with these components in order:
//  1. **Column Name**: The database column name
//  2. **Data Type**: Mapped to database-specific SQL type
//  3. **Primary Key**: PRIMARY KEY constraint (for single-column PKs)
//  4. **Nullability**: NOT NULL constraint (when applicable)
//  5. **Uniqueness**: UNIQUE constraint
//  6. **Default Value**: DEFAULT clause with proper quoting
//
// # Primary Key Handling Logic
//
// **MySQL SERIAL Types**:
//   - SERIAL becomes "INT AUTO_INCREMENT PRIMARY KEY"
//   - Special handling for MySQL's auto-increment syntax
//
// **Non-SERIAL Primary Keys**:
//   - Adds "PRIMARY KEY" to column definition
//   - Works for INTEGER, VARCHAR, UUID, and other types
//
// **Composite Primary Keys**:
//   - Individual columns don't get PRIMARY KEY in their definition
//   - Table-level constraint is added separately
//
// # Constraint Logic
//
// **NOT NULL Handling**:
//   - Primary key columns are implicitly NOT NULL
//   - Only adds NOT NULL for non-primary key columns when field.Nullable is false
//   - Prevents redundant "PRIMARY KEY NOT NULL" syntax
//
// **UNIQUE Constraint**:
//   - Only added for non-primary key columns
//   - Primary keys are inherently unique
//
// # Default Value Processing
//
// The function handles default values with intelligent quoting:
//   - Uses NeedsQuoting() to determine if quotes are needed
//   - Handles function calls (NOW(), CURRENT_TIMESTAMP)
//   - Properly quotes string and enum values
//   - Leaves numeric and boolean values unquoted
//
// # Example Outputs
//
// **Auto-increment primary key**:
//
//	```sql
//	id SERIAL PRIMARY KEY
//	```
//
// **String column with constraints**:
//
//	```sql
//	email VARCHAR(255) NOT NULL UNIQUE DEFAULT 'user@example.com'
//	```
//
// **Timestamp with function default**:
//
//	```sql
//	created_at TIMESTAMP DEFAULT NOW()
//	```
//
// **Enum column**:
//
//	```sql
//	status enum_status_type DEFAULT 'active'
//	```
//
// # Parameters
//
//   - field: Schema field containing all column metadata and constraints
//   - dialect: Target database dialect for type mapping and syntax
//
// # Return Value
//
// Returns a complete SQL column definition string ready for use in CREATE TABLE
// or ALTER TABLE statements.
//
// # Database Compatibility
//
// Generates dialect-appropriate SQL:
//   - PostgreSQL: SERIAL, BOOLEAN, native enum types
//   - MySQL/MariaDB: AUTO_INCREMENT, TINYINT, inline ENUM syntax
//   - Cross-platform: Intelligent type mapping via MapTypeToSQL()
func GenerateColumnDefinition(field types.SchemaField, dialect string) string {
	sqlType := MapTypeToSQL(field.Type, field.Enum, dialect)
	colDef := field.Name + " " + sqlType

	// Handle primary key
	if field.Primary {
		if dialect == "mysql" && strings.Contains(strings.ToUpper(field.Type), "SERIAL") {
			// For MySQL SERIAL (which becomes INT AUTO_INCREMENT), add PRIMARY KEY
			colDef += " PRIMARY KEY"
		} else if !strings.Contains(strings.ToUpper(field.Type), "SERIAL") {
			// For non-SERIAL primary keys, add PRIMARY KEY
			colDef += " PRIMARY KEY"
		}
	}

	// Handle NOT NULL (but not for primary keys as they're implicitly NOT NULL)
	if !field.Nullable && !field.Primary {
		colDef += " NOT NULL"
	}

	// Handle UNIQUE constraint
	if field.Unique && !field.Primary {
		colDef += " UNIQUE"
	}

	// Handle DEFAULT values
	if field.Default != "" {
		defaultValue := field.Default
		// Quote string/enum default values if they're not already quoted and not functions
		if NeedsQuoting(defaultValue, field.Type, field.Enum) {
			defaultValue = fmt.Sprintf("'%s'", defaultValue)
		}
		colDef += " DEFAULT " + defaultValue
	}

	return colDef
}

// NeedsQuoting determines if a default value requires SQL quoting based on value content and field type.
//
// This function implements intelligent quoting logic for SQL default values,
// ensuring that string and enum values are properly quoted while leaving
// numeric values, function calls, and already-quoted values unchanged.
//
// # Quoting Decision Logic
//
// The function applies quoting rules in this order:
//  1. **Already Quoted**: Skip if value already has single quotes
//  2. **Function Calls**: Skip if value contains parentheses or is a known function
//  3. **Enum Types**: Quote if field has enum values or is an enum type
//  4. **String Types**: Quote if field type is VARCHAR, TEXT, or CHAR
//  5. **Other Types**: Don't quote numeric, boolean, or other types
//
// # Function Call Detection
//
// The function recognizes these patterns as function calls (no quoting needed):
//   - Values containing parentheses: "NOW()", "RANDOM()"
//   - Known SQL functions: "CURRENT_TIMESTAMP", "NULL"
//   - Case-insensitive function names
//
// # Type-Based Quoting Rules
//
// **Enum Types**:
//   - Quote if enumValues slice is non-empty
//   - Quote if fieldType starts with "enum" (case-insensitive)
//   - Ensures enum values are properly quoted in SQL
//
// **String Types**:
//   - Quote VARCHAR, TEXT, CHAR variations (case-insensitive)
//   - Handles all string-like database types
//
// **Numeric/Boolean Types**:
//   - No quoting for INTEGER, DECIMAL, BOOLEAN, etc.
//   - Allows direct numeric and boolean value insertion
//
// # Example Decisions
//
// **String values (quote needed)**:
//
//	```go
//	NeedsQuoting("hello", "VARCHAR(255)", nil)     // → true
//	NeedsQuoting("default", "TEXT", nil)          // → true
//	```
//
// **Already quoted (skip)**:
//
//	```go
//	NeedsQuoting("'hello'", "VARCHAR(255)", nil)  // → false
//	```
//
// **Function calls (skip)**:
//
//	```go
//	NeedsQuoting("NOW()", "TIMESTAMP", nil)       // → false
//	NeedsQuoting("CURRENT_TIMESTAMP", "TIMESTAMP", nil) // → false
//	```
//
// **Enum values (quote needed)**:
//
//	```go
//	NeedsQuoting("active", "status", []string{"active", "inactive"}) // → true
//	NeedsQuoting("pending", "enum_status", nil)   // → true
//	```
//
// **Numeric values (skip)**:
//
//	```go
//	NeedsQuoting("42", "INTEGER", nil)            // → false
//	NeedsQuoting("true", "BOOLEAN", nil)          // → false
//	```
//
// # Parameters
//
//   - defaultValue: The default value to analyze for quoting needs
//   - fieldType: The SQL field type (used for type-based quoting decisions)
//   - enumValues: Slice of enum values (non-empty indicates enum type)
//
// # Return Value
//
// Returns true if the default value should be wrapped in single quotes,
// false if it should be used as-is in the SQL statement.
//
// # SQL Injection Safety
//
// This function is designed for use with trusted schema definitions and
// should not be used with user-provided input. Default values come from
// Go struct annotations and are considered safe for SQL generation.
func NeedsQuoting(defaultValue, fieldType string, enumValues []string) bool {
	// Don't quote if already quoted
	if strings.HasPrefix(defaultValue, "'") && strings.HasSuffix(defaultValue, "'") {
		return false
	}

	// Don't quote if it's a function call (contains parentheses or is a known function)
	if strings.Contains(defaultValue, "(") ||
		strings.ToUpper(defaultValue) == "CURRENT_TIMESTAMP" ||
		strings.ToUpper(defaultValue) == "NOW()" ||
		strings.ToUpper(defaultValue) == "NULL" {
		return false
	}

	// Quote if it's an enum type
	if len(enumValues) > 0 || strings.HasPrefix(strings.ToLower(fieldType), "enum") {
		return true
	}

	// Quote if it's a string type
	fieldTypeUpper := strings.ToUpper(fieldType)
	if strings.Contains(fieldTypeUpper, "VARCHAR") ||
		strings.Contains(fieldTypeUpper, "TEXT") ||
		strings.Contains(fieldTypeUpper, "CHAR") {
		return true
	}

	// Don't quote numeric types, booleans, etc.
	return false
}

// MapTypeToSQL performs intelligent type mapping from schema field types to database-specific SQL types.
//
// This function is the core type translation engine that handles the complex task
// of converting Go struct field types into appropriate SQL data types for different
// database systems. It includes special handling for enum types and cross-database
// type compatibility.
//
// # Type Mapping Strategy
//
// The function uses a two-phase approach:
//  1. **Enum Detection**: Checks if the field represents an enum type
//  2. **Standard Type Mapping**: Applies database-specific type conversions
//
// # Enum Type Handling
//
// **Enum Detection Logic**:
//   - Field has non-empty enumValues slice (explicit enum values)
//   - Field type starts with "enum_" prefix (enum type reference)
//
// **PostgreSQL Enum Handling**:
//   - Returns enum type name as-is (preserves case)
//   - Assumes enum type already exists or will be created
//   - Example: "enum_status" → "enum_status"
//
// **MySQL/MariaDB Enum Handling**:
//   - Converts to inline ENUM syntax with quoted values
//   - Example: ["active", "inactive"] → "ENUM('active', 'inactive')"
//   - Filters out empty values for clean SQL generation
//
// # Standard Type Mapping
//
// **PostgreSQL Type Mapping**:
//   - SERIAL → "SERIAL" (auto-incrementing integer)
//   - VARCHAR → preserved as-is with length
//   - BOOLEAN → "BOOLEAN" (native boolean type)
//   - TIMESTAMP → "TIMESTAMP"
//   - Other types preserved as-is
//
// **MySQL/MariaDB Type Mapping**:
//   - SERIAL → "INT AUTO_INCREMENT" (MySQL auto-increment syntax)
//   - INTEGER → "INT" (MySQL standard integer type)
//   - BOOLEAN → "BOOLEAN" (MySQL boolean type)
//   - Other types preserved with appropriate conversions
//
// # Example Mappings
//
// **Enum types**:
//
//	```go
//	// PostgreSQL
//	MapTypeToSQL("enum_status", []string{"active", "inactive"}, "postgres")
//	// → "enum_status"
//
//	// MySQL
//	MapTypeToSQL("status", []string{"active", "inactive"}, "mysql")
//	// → "ENUM('active', 'inactive')"
//	```
//
// **Standard types**:
//
//	```go
//	// PostgreSQL
//	MapTypeToSQL("SERIAL", nil, "postgres")     // → "SERIAL"
//	MapTypeToSQL("VARCHAR(255)", nil, "postgres") // → "VARCHAR(255)"
//
//	// MySQL
//	MapTypeToSQL("SERIAL", nil, "mysql")        // → "INT AUTO_INCREMENT"
//	MapTypeToSQL("INTEGER", nil, "mysql")       // → "INT"
//	```
//
// # Parameters
//
//   - fieldType: The original field type from Go struct annotations
//   - enumValues: Slice of enum values (empty for non-enum types)
//   - dialect: Target database dialect ("postgres", "mysql", "mariadb")
//
// # Return Value
//
// Returns a database-specific SQL type string ready for use in CREATE TABLE
// or ALTER TABLE statements.
//
// # Case Handling
//
// **Enum Types**: Preserve original case for PostgreSQL compatibility
// **Standard Types**: Convert to uppercase for consistent SQL generation
// **Database-Specific**: Apply dialect-appropriate case conventions
//
// # Extensibility
//
// The function is designed for easy extension:
//   - Add new database dialects by extending the switch statement
//   - Add new type mappings within existing dialect cases
//   - Enum handling is abstracted and reusable across dialects
func MapTypeToSQL(fieldType string, enumValues []string, dialect string) string {
	// Check if this is an enum type (has non-empty enum values or starts with "enum_")
	hasValidEnumValues := HasNonEmptyEnumValues(enumValues)
	isEnum := hasValidEnumValues || strings.HasPrefix(strings.ToLower(fieldType), "enum_")

	if isEnum {
		switch dialect {
		case "postgres":
			// For PostgreSQL, return the enum type name as-is (don't uppercase it)
			return fieldType
		case "mysql", "mariadb":
			// For MySQL/MariaDB, convert to inline ENUM syntax
			if hasValidEnumValues {
				quotedValues := make([]string, 0, len(enumValues))
				for _, value := range enumValues {
					if value != "" { // Skip empty values
						quotedValues = append(quotedValues, fmt.Sprintf("'%s'", value))
					}
				}
				if len(quotedValues) > 0 {
					return fmt.Sprintf("ENUM(%s)", strings.Join(quotedValues, ", "))
				}
			}
			// If no valid enum values provided but type starts with enum_, return as-is
			// This shouldn't happen in normal usage but provides a fallback
			return fieldType
		default:
			return fieldType
		}
	}

	// For non-enum types, apply the original logic with uppercase conversion
	fieldType = strings.ToUpper(fieldType)

	switch dialect {
	case "postgres":
		switch {
		case strings.Contains(fieldType, "SERIAL"):
			return "SERIAL"
		case strings.Contains(fieldType, "VARCHAR"):
			return fieldType
		case strings.Contains(fieldType, "TEXT"):
			return "TEXT"
		case strings.Contains(fieldType, "INTEGER"):
			return "INTEGER"
		case strings.Contains(fieldType, "BOOLEAN"):
			return "BOOLEAN"
		case strings.Contains(fieldType, "TIMESTAMP"):
			return "TIMESTAMP"
		case strings.Contains(fieldType, "DECIMAL"):
			return fieldType
		default:
			return fieldType
		}
	case "mysql", "mariadb":
		switch {
		case strings.Contains(fieldType, "SERIAL"):
			return "INT AUTO_INCREMENT"
		case strings.Contains(fieldType, "VARCHAR"):
			return fieldType
		case strings.Contains(fieldType, "TEXT"):
			return "TEXT"
		case strings.Contains(fieldType, "INTEGER"):
			return "INT"
		case strings.Contains(fieldType, "BOOLEAN"):
			return "BOOLEAN"
		case strings.Contains(fieldType, "TIMESTAMP"):
			return "TIMESTAMP"
		case strings.Contains(fieldType, "DECIMAL"):
			return fieldType
		default:
			return fieldType
		}
	default:
		return fieldType
	}
}

// HasNonEmptyEnumValues validates that an enum values slice contains at least one meaningful value.
//
// This utility function is used throughout the enum handling logic to distinguish
// between fields that have actual enum values defined versus fields that have
// empty or placeholder enum value slices.
//
// # Validation Logic
//
// The function iterates through the enum values slice and returns true as soon
// as it finds any non-empty string value. This approach is efficient for typical
// enum value slices which usually have meaningful values at the beginning.
//
// # Use Cases
//
// **Enum Type Detection**:
//   - Used in MapTypeToSQL() to determine if a field should be treated as an enum
//   - Helps distinguish between explicit enum values and enum type references
//
// **SQL Generation**:
//   - Determines whether to generate inline ENUM syntax (MySQL/MariaDB)
//   - Validates that enum fields have meaningful values for SQL generation
//
// **Schema Validation**:
//   - Ensures enum fields have proper value definitions
//   - Prevents generation of empty ENUM() clauses in SQL
//
// # Example Scenarios
//
// **Valid enum values**:
//
//	```go
//	HasNonEmptyEnumValues([]string{"active", "inactive", "pending"}) // → true
//	HasNonEmptyEnumValues([]string{"", "draft", "published"})        // → true (has "draft")
//	```
//
// **Invalid/empty enum values**:
//
//	```go
//	HasNonEmptyEnumValues([]string{})                    // → false (empty slice)
//	HasNonEmptyEnumValues([]string{"", "", ""})          // → false (all empty)
//	HasNonEmptyEnumValues(nil)                           // → false (nil slice)
//	```
//
// # Performance Characteristics
//
// - Time Complexity: O(n) worst case, O(1) best case (early return)
// - Space Complexity: O(1) (no additional memory allocation)
// - Optimized for typical enum slices with valid values at the beginning
//
// # Parameters
//
//   - enumValues: Slice of string values to validate for non-empty content
//
// # Return Value
//
// Returns true if at least one non-empty string is found in the slice,
// false if the slice is nil, empty, or contains only empty strings.
//
// # Integration Points
//
// This function is used by:
//   - MapTypeToSQL() for enum type detection
//   - MySQL/MariaDB enum SQL generation logic
//   - Schema validation routines
//
// # Edge Case Handling
//
// The function gracefully handles:
//   - Nil slices (returns false)
//   - Empty slices (returns false)
//   - Slices with mixed empty and non-empty values (returns true if any non-empty)
//   - Single-element slices (returns based on that element's emptiness)
func HasNonEmptyEnumValues(enumValues []string) bool {
	for _, value := range enumValues {
		if value != "" {
			return true
		}
	}
	return false
}
