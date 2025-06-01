package integration

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/dbschema"
	migrator2 "github.com/denisvmedia/inventario/ptah/migration/migrator"
)

// GetAllScenarios returns all integration test scenarios
func GetAllScenarios() []TestScenario {
	scenarios := []TestScenario{
		// Basic Functionality
		{
			Name:        "apply_incremental_migrations",
			Description: "Apply multiple sequential migrations to a fresh database",
			TestFunc:    testApplyIncrementalMigrations,
		},
		{
			Name:        "rollback_migrations",
			Description: "Roll back migrations in reverse order",
			TestFunc:    testRollbackMigrations,
		},
		{
			Name:        "upgrade_to_specific_version",
			Description: "Apply migrations up to a defined version",
			TestFunc:    testUpgradeToSpecificVersion,
		},
		{
			Name:        "check_current_version",
			Description: "Query current migration version",
			TestFunc:    testCheckCurrentVersion,
		},
		{
			Name:        "generate_desired_schema",
			Description: "Extract expected schema from entity definitions",
			TestFunc:    testGenerateDesiredSchema,
		},
		{
			Name:        "read_actual_db_schema",
			Description: "Introspect current schema from the database",
			TestFunc:    testReadActualDBSchema,
		},
		{
			Name:        "dry_run_support",
			Description: "Simulate migrations without executing SQL",
			TestFunc:    testDryRunSupport,
		},
		{
			Name:        "operation_planning",
			Description: "Generate detailed plan of operations",
			TestFunc:    testOperationPlanning,
		},
		{
			Name:        "schema_diff",
			Description: "Compare DB schema with entity definitions",
			TestFunc:    testSchemaDiff,
		},
		{
			Name:        "failure_diagnostics",
			Description: "Simulate a failing migration and capture error",
			TestFunc:    testFailureDiagnostics,
		},

		// Idempotency
		{
			Name:        "idempotency_reapply",
			Description: "Re-apply already applied migrations",
			TestFunc:    testIdempotencyReapply,
		},
		{
			Name:        "idempotency_up_to_date",
			Description: "Run migrate up when database is already up-to-date",
			TestFunc:    testIdempotencyUpToDate,
		},

		// Concurrency
		{
			Name:        "concurrency_parallel_migrate",
			Description: "Launch two migrate up processes in parallel",
			TestFunc:    testConcurrencyParallelMigrate,
		},

		// Partial Failure Recovery
		{
			Name:        "partial_failure_recovery",
			Description: "Handle multi-step migration with intentional failure",
			TestFunc:    testPartialFailureRecovery,
		},

		// Timestamp Verification
		{
			Name:        "timestamp_verification",
			Description: "Check that applied_at timestamps are stored correctly",
			TestFunc:    testTimestampVerification,
		},

		// Manual Patch Detection
		{
			Name:        "manual_patch_detection",
			Description: "Detect manual schema changes via schema diff",
			TestFunc:    testManualPatchDetection,
		},

		// Permission Restrictions
		{
			Name:        "permission_restrictions",
			Description: "Test with read-only privileges",
			TestFunc:    testPermissionRestrictions,
		},

		// Cleanup Support
		{
			Name:        "cleanup_support",
			Description: "Test drop and re-run from empty state",
			TestFunc:    testCleanupSupport,
		},
	}

	// Add dynamic scenarios that use versioned entities
	scenarios = append(scenarios, GetDynamicScenarios()...)

	return scenarios
}

// testApplyIncrementalMigrations tests applying multiple sequential migrations
func testApplyIncrementalMigrations(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	// Get the basic migrations filesystem
	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Apply all migrations
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Verify final version
	version, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if version != 3 { // Expecting 3 migrations in basic set
		return fmt.Errorf("expected version 3, got %d", version)
	}

	// Verify tables exist
	tables := []string{"users", "posts", "comments"}
	for _, table := range tables {
		exists, err := helper.TableExists(table)
		if err != nil {
			return fmt.Errorf("failed to check if table %s exists: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("table %s should exist after migrations", table)
		}
	}

	return nil
}

// testRollbackMigrations tests rolling back migrations
func testRollbackMigrations(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Apply all migrations first
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Rollback to version 1
	if err := helper.RollbackToVersion(ctx, migrationsFS, 1); err != nil {
		return fmt.Errorf("failed to rollback to version 1: %w", err)
	}

	// Verify version
	version, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if version != 1 {
		return fmt.Errorf("expected version 1 after rollback, got %d", version)
	}

	// Verify only users table exists
	exists, err := helper.TableExists("users")
	if err != nil {
		return fmt.Errorf("failed to check if users table exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("users table should exist after rollback to version 1")
	}

	// Verify posts table doesn't exist
	exists, err = helper.TableExists("posts")
	if err != nil {
		return fmt.Errorf("failed to check if posts table exists: %w", err)
	}
	if exists {
		return fmt.Errorf("posts table should not exist after rollback to version 1")
	}

	return nil
}

// testUpgradeToSpecificVersion tests upgrading to a specific version
func testUpgradeToSpecificVersion(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Create migrator and register migrations
	m := migrator2.NewMigrator(conn)
	if err := migrator2.RegisterMigrations(m, migrationsFS); err != nil {
		return fmt.Errorf("failed to register migrations: %w", err)
	}

	// Migrate to version 2 only
	if err := m.MigrateTo(ctx, 2); err != nil {
		return fmt.Errorf("failed to migrate to version 2: %w", err)
	}

	// Verify version
	version, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if version != 2 {
		return fmt.Errorf("expected version 2, got %d", version)
	}

	// Verify correct tables exist
	tables := map[string]bool{
		"users":    true,
		"posts":    true,
		"comments": false, // Should not exist at version 2
	}

	for table, shouldExist := range tables {
		exists, err := helper.TableExists(table)
		if err != nil {
			return fmt.Errorf("failed to check if table %s exists: %w", table, err)
		}
		if exists != shouldExist {
			return fmt.Errorf("table %s existence mismatch: expected %v, got %v", table, shouldExist, exists)
		}
	}

	return nil
}

// testCheckCurrentVersion tests querying the current migration version
func testCheckCurrentVersion(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Initially should be version 0
	version, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get initial version: %w", err)
	}

	if version != 0 {
		return fmt.Errorf("expected initial version 0, got %d", version)
	}

	// Apply first migration
	m := migrator2.NewMigrator(conn)
	if err := migrator2.RegisterMigrations(m, migrationsFS); err != nil {
		return fmt.Errorf("failed to register migrations: %w", err)
	}

	if err := m.MigrateTo(ctx, 1); err != nil {
		return fmt.Errorf("failed to migrate to version 1: %w", err)
	}

	// Should now be version 1
	version, err = helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get version after migration: %w", err)
	}

	if version != 1 {
		return fmt.Errorf("expected version 1 after migration, got %d", version)
	}

	return nil
}

// testGenerateDesiredSchema tests extracting schema from entity definitions
func testGenerateDesiredSchema(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	// Get the entities directory
	entitiesDir := filepath.Join("fixtures", "entities")

	// Parse entities
	result, err := goschema.ParseDir(entitiesDir)
	if err != nil {
		return fmt.Errorf("failed to parse entities: %w", err)
	}

	// Verify we got some tables
	if len(result.Tables) == 0 {
		return fmt.Errorf("expected to find tables in entity definitions")
	}

	// Verify we got some fields
	if len(result.Fields) == 0 {
		return fmt.Errorf("expected to find fields in entity definitions")
	}

	// Check for expected entities
	expectedTables := map[string]bool{
		"users":    false,
		"products": false,
	}

	for _, table := range result.Tables {
		if _, exists := expectedTables[table.Name]; exists {
			expectedTables[table.Name] = true
		}
	}

	for tableName, found := range expectedTables {
		if !found {
			return fmt.Errorf("expected to find table %s in entity definitions", tableName)
		}
	}

	return nil
}

// testReadActualDBSchema tests introspecting current schema from database
func testReadActualDBSchema(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Apply some migrations first
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Read the actual schema
	schema, err := conn.Reader().ReadSchema()
	if err != nil {
		return fmt.Errorf("failed to read database schema: %w", err)
	}

	// Verify we got tables
	if len(schema.Tables) == 0 {
		return fmt.Errorf("expected to find tables in database schema")
	}

	// Check for expected tables
	expectedTables := []string{"users", "posts", "comments"}
	foundTables := make(map[string]bool)

	for _, table := range schema.Tables {
		foundTables[table.Name] = true
	}

	for _, expectedTable := range expectedTables {
		if !foundTables[expectedTable] {
			return fmt.Errorf("expected to find table %s in database schema", expectedTable)
		}
	}

	return nil
}

// testDryRunSupport tests dry run functionality
func testDryRunSupport(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Enable dry run mode
	helper.SetDryRun(true)

	// Verify dry run is enabled
	if !helper.IsDryRun() {
		return fmt.Errorf("dry run mode should be enabled")
	}

	// Try to apply migrations in dry run mode
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("dry run migration failed: %w", err)
	}

	// Verify no tables were actually created
	exists, err := helper.TableExists("users")
	if err != nil {
		return fmt.Errorf("failed to check if users table exists: %w", err)
	}
	if exists {
		return fmt.Errorf("users table should not exist after dry run")
	}

	// Disable dry run and apply for real
	helper.SetDryRun(false)
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("real migration failed: %w", err)
	}

	// Now table should exist
	exists, err = helper.TableExists("users")
	if err != nil {
		return fmt.Errorf("failed to check if users table exists after real migration: %w", err)
	}
	if !exists {
		return fmt.Errorf("users table should exist after real migration")
	}

	return nil
}
