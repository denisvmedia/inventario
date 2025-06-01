// Package migrator provides database migration execution and management for the Ptah schema management system.
//
// This package implements the core migration engine that applies and rolls back database migrations
// with proper version tracking, transaction safety, and comprehensive error handling. It manages
// the migration history table and provides both programmatic and CLI interfaces for migration operations.
//
// # Overview
//
// The migrator package is responsible for executing database migrations in a controlled and safe manner.
// It maintains a migration history table to track applied migrations and provides functionality for
// applying migrations forward (up) or rolling them back (down) to previous versions.
//
// # Key Features
//
//   - Version-based migration tracking with timestamp support
//   - Transaction-safe migration execution with automatic rollback on failure
//   - Support for both up and down migrations
//   - Migration to specific versions (forward or backward)
//   - Comprehensive migration status reporting
//   - SQL file-based and programmatic migration support
//   - Cross-database compatibility (PostgreSQL, MySQL, MariaDB)
//
// # Core Components
//
// The package provides these main types:
//
//   - Migrator: Main migration engine that manages migration execution
//   - Migration: Represents a single database migration with up/down functions
//   - MigrationFunc: Function type for programmatic migrations
//
// # Migration Structure
//
// Each migration consists of:
//
//   - Version: Unique integer identifier (typically timestamp-based)
//   - Description: Human-readable description of the migration
//   - Up: Function to apply the migration
//   - Down: Function to roll back the migration
//
// # Usage Example
//
// Basic migration setup and execution:
//
//	// Create database connection
//	conn, err := dbschema.ConnectToDatabase("postgres://user:pass@localhost/db")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer conn.Close()
//
//	// Create migrator
//	m := migrator.NewMigrator(conn)
//
//	// Register migrations
//	m.Register(&migrator.Migration{
//		Version:     20240101120000,
//		Description: "Create users table",
//		Up:          migrator.MigrationFuncFromSQLFilename("001_create_users.up.sql", fsys),
//		Down:        migrator.MigrationFuncFromSQLFilename("001_create_users.down.sql", fsys),
//	})
//
//	// Apply all pending migrations
//	if err := m.MigrateUp(context.Background()); err != nil {
//		log.Fatal(err)
//	}
//
// # Migration History Table
//
// The migrator automatically creates and manages a `schema_migrations` table:
//
//	CREATE TABLE schema_migrations (
//		version BIGINT PRIMARY KEY,
//		description TEXT NOT NULL,
//		applied_at TIMESTAMP NOT NULL
//	);
//
// This table tracks which migrations have been applied and when.
//
// # Migration Operations
//
// The migrator supports several migration operations:
//
//   - MigrateUp(): Apply all pending migrations
//   - MigrateDown(): Roll back to the previous version
//   - MigrateTo(version): Migrate to a specific version (up or down)
//   - GetCurrentVersion(): Get the current migration version
//   - GetAppliedMigrations(): List all applied migration versions
//   - GetPendingMigrations(): List all pending migration versions
//
// # Transaction Safety
//
// Each migration is executed within its own database transaction:
//
//   - If a migration succeeds, the transaction is committed
//   - If a migration fails, the transaction is rolled back
//   - Migration history is updated only after successful execution
//   - Partial failures leave the database in a consistent state
//
// # SQL File Support
//
// The package provides utilities for SQL file-based migrations:
//
//	// Create migration function from SQL file
//	upFunc := migrator.MigrationFuncFromSQLFilename("migration.up.sql", fsys)
//	downFunc := migrator.MigrationFuncFromSQLFilename("migration.down.sql", fsys)
//
//	migration := &migrator.Migration{
//		Version:     20240101120000,
//		Description: "Add user preferences",
//		Up:          upFunc,
//		Down:        downFunc,
//	}
//
// # SQL Statement Splitting
//
// The migrator properly handles multi-statement SQL files:
//
//   - Uses AST-based parsing to split SQL statements
//   - Properly handles semicolons within string literals and comments
//   - Executes each statement separately for better MySQL compatibility
//   - Provides detailed error reporting for failed statements
//
// # Error Handling
//
// The migrator provides comprehensive error handling:
//
//   - Database connection errors
//   - SQL execution errors with statement context
//   - Transaction management errors
//   - Migration file reading errors
//   - Version tracking errors
//
// # Cross-Database Support
//
// The migrator works with all supported database platforms:
//
//   - PostgreSQL: Full support with proper timestamp handling
//   - MySQL: Compatible with MySQL-specific SQL syntax
//   - MariaDB: Full compatibility with MariaDB features
//
// # Integration with Ptah
//
// This package integrates with other Ptah components:
//
//   - ptah/dbschema: Uses database connections and schema operations
//   - ptah/migration/generator: Applies generated migration files
//   - ptah/core/sqlutil: Uses SQL parsing utilities for statement splitting
//   - ptah/cmd/migrate*: Provides CLI interfaces for migration operations
//
// # Performance Considerations
//
// The migrator is optimized for:
//
//   - Efficient migration execution with minimal overhead
//   - Fast version checking and status queries
//   - Atomic migration operations with proper transaction handling
//   - Memory-efficient SQL file processing
//
// # Thread Safety
//
// Migrator instances are not thread-safe and should not be used concurrently
// from multiple goroutines. However, the migration history table includes
// proper constraints to prevent concurrent migration execution conflicts.
package migrator
