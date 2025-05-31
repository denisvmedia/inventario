package normalize

import (
	"strings"
)

// NormalizeType normalizes database type names for cross-platform comparison.
//
// This function converts database-specific type names to standardized forms that can be
// compared across different database systems. It handles the variations in type naming
// conventions between PostgreSQL, MySQL, MariaDB, and other databases.
//
// # Type Normalization Rules
//
//   - VARCHAR variations (VARCHAR, VARCHAR2, etc.) → "varchar"
//   - TEXT variations (TEXT, LONGTEXT, etc.) → "text"
//   - Integer variations (INT, INTEGER, BIGINT, etc.) → "integer"
//   - SERIAL types (SERIAL, BIGSERIAL) → "integer" (for comparison purposes)
//   - Boolean variations (BOOL, BOOLEAN, TINYINT(1)) → "boolean"
//   - Timestamp variations → "timestamp"
//   - Decimal variations (DECIMAL, NUMERIC) → "decimal"
//
// # Database-Specific Handling
//
//   - **MySQL/MariaDB**: TINYINT and TINYINT(1) are treated as BOOLEAN
//   - **PostgreSQL**: SERIAL types are normalized to INTEGER for comparison
//   - **Cross-platform**: Case-insensitive comparison with lowercase normalization
//
// # Example Usage
//
//	// These all normalize to "varchar"
//	NormalizeType("VARCHAR(255)")  // → "varchar"
//	NormalizeType("varchar(100)")  // → "varchar"
//	NormalizeType("VARCHAR2")      // → "varchar"
//
//	// These all normalize to "boolean"
//	NormalizeType("BOOLEAN")       // → "boolean"
//	NormalizeType("TINYINT(1)")    // → "boolean"
//	NormalizeType("BOOL")          // → "boolean"
//
// # Parameters
//
//   - typeName: The database-specific type name to normalize
//
// # Return Value
//
// Returns a normalized type name suitable for cross-database comparison.
func NormalizeType(typeName string) string {
	// Convert to lowercase for case-insensitive comparison
	typeName = strings.ToLower(typeName)

	switch {
	case strings.Contains(typeName, "varchar"):
		return "varchar"
	case strings.Contains(typeName, "text"):
		return "text"
	case strings.Contains(typeName, "serial"):
		// SERIAL types are auto-incrementing integers
		return "integer"
	case strings.Contains(typeName, "tinyint"):
		// MySQL/MariaDB stores BOOLEAN as TINYINT or TINYINT(1)
		return "boolean"
	case strings.Contains(typeName, "int"):
		return "integer"
	case strings.Contains(typeName, "bool"):
		return "boolean"
	case strings.Contains(typeName, "timestamp"):
		return "timestamp"
	case strings.Contains(typeName, "decimal") || strings.Contains(typeName, "numeric"):
		return "decimal"
	default:
		// Return as-is for unrecognized types (enums, custom types, etc.)
		return typeName
	}
}

// NormalizeDefaultValue normalizes default values for cross-database comparison.
//
// This function handles the variations in how different database systems represent
// default values, ensuring that semantically equivalent defaults are recognized
// as identical during schema comparison.
//
// # Normalization Rules
//
//   - Empty/NULL values: Converted to empty string for consistent comparison
//   - Quoted values: Quotes are removed for comparison (both single and double)
//   - Boolean values: MySQL/MariaDB '1'/'0' normalized to 'true'/'false'
//   - NULL literals: Database-specific NULL representations normalized to empty string
//
// # Database-Specific Handling
//
//   - **MySQL/MariaDB**: Returns 'NULL' string for columns without explicit defaults
//   - **PostgreSQL**: Returns actual NULL for columns without defaults
//   - **Boolean types**: Handles '1'/'0' vs 'true'/'false' variations
//
// # Example Usage
//
//	// Boolean normalization
//	NormalizeDefaultValue("1", "boolean")     // → "true"
//	NormalizeDefaultValue("0", "boolean")     // → "false"
//	NormalizeDefaultValue("true", "boolean")  // → "true"
//
//	// Quote removal
//	NormalizeDefaultValue("'hello'", "varchar")  // → "hello"
//	NormalizeDefaultValue("\"world\"", "text")   // → "world"
//
//	// NULL handling
//	NormalizeDefaultValue("NULL", "varchar")     // → ""
//	NormalizeDefaultValue("", "integer")        // → ""
//
// # Parameters
//
//   - defaultValue: The raw default value from database introspection
//   - typeName: The normalized type name (used for type-specific handling)
//
// # Return Value
//
// Returns a normalized default value suitable for cross-database comparison.
func NormalizeDefaultValue(defaultValue, typeName string) string {
	if defaultValue == "" {
		return ""
	}

	// Remove surrounding quotes for comparison (both single and double quotes)
	cleanValue := strings.Trim(defaultValue, "'\"")

	// MariaDB/MySQL returns 'NULL' string for columns without explicit defaults
	// Normalize this to empty string for consistent comparison
	if strings.ToUpper(cleanValue) == "NULL" {
		return ""
	}

	// For boolean types, normalize database-specific representations
	if typeName == "boolean" {
		switch strings.ToLower(cleanValue) {
		case "1", "true":
			return "true"
		case "0", "false":
			return "false"
		}
		// If it's not a recognized boolean value, return as-is
		return cleanValue
	}

	// Return cleaned value for all other types
	return cleanValue
}
