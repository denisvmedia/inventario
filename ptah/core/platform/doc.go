// Package platform provides database platform constants and identifiers for the Ptah schema management system.
//
// This package defines standardized constants for identifying different database platforms
// throughout the Ptah ecosystem. It ensures consistent platform identification across
// all components including parsers, renderers, migrators, and schema generators.
//
// # Supported Platforms
//
// The package defines constants for all supported database platforms:
//
//   - Postgres: PostgreSQL database platform identifier
//   - MySQL: MySQL database platform identifier  
//   - MariaDB: MariaDB database platform identifier
//
// # Usage
//
// Platform constants are used throughout the Ptah system for:
//
//   - Dialect-specific SQL generation
//   - Database connection management
//   - Migration planning and execution
//   - Schema comparison and validation
//
// Example usage:
//
//	import "github.com/denisvmedia/inventario/ptah/core/platform"
//
//	func generateSQL(platformType string) string {
//		switch platformType {
//		case platform.Postgres:
//			return generatePostgreSQLSchema()
//		case platform.MySQL:
//			return generateMySQLSchema()
//		case platform.MariaDB:
//			return generateMariaDBSchema()
//		default:
//			return generateGenericSchema()
//		}
//	}
//
// # Integration with Ptah
//
// This package integrates with other Ptah components:
//
//   - ptah/core/renderer: Uses platform constants for dialect selection
//   - ptah/migration/planner: Uses platform constants for migration planning
//   - ptah/dbschema: Uses platform constants for connection management
//   - ptah/core/goschema: Uses platform constants for schema generation
//
// # Design Principles
//
// The platform constants follow these design principles:
//
//   - Simple string constants for easy comparison and debugging
//   - Lowercase naming for consistency with database driver conventions
//   - Clear, unambiguous names that match common database identifiers
//   - Stable values that won't change across versions
//
// # Extensibility
//
// New database platforms can be added by:
//
//   1. Adding a new constant to this package
//   2. Implementing dialect-specific renderers
//   3. Adding platform support to migration planners
//   4. Updating connection management code
//
// # Backward Compatibility
//
// Platform constant values are considered stable and will not change
// in future versions to maintain backward compatibility with existing
// configurations and code that depends on these values.
package platform
