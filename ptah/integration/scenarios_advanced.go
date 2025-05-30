package integration

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/migrator"
	"github.com/denisvmedia/inventario/ptah/schema/parser"
)

// testOperationPlanning tests generating detailed operation plans
func testOperationPlanning(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}
	
	// Get migration status to see the plan
	status, err := migrator.GetMigrationStatus(ctx, conn, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}
	
	// Verify we have pending migrations
	if len(status.PendingMigrations) == 0 {
		return fmt.Errorf("expected to have pending migrations")
	}
	
	// Verify current version is 0
	if status.CurrentVersion != 0 {
		return fmt.Errorf("expected current version 0, got %d", status.CurrentVersion)
	}
	
	// Verify total migrations count
	if status.TotalMigrations != 3 {
		return fmt.Errorf("expected 3 total migrations, got %d", status.TotalMigrations)
	}
	
	return nil
}

// testSchemaDiff tests comparing DB schema with entity definitions
func testSchemaDiff(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	// Apply some migrations to create tables
	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}
	
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	
	// Read current schema
	currentSchema, err := conn.Reader().ReadSchema()
	if err != nil {
		return fmt.Errorf("failed to read current schema: %w", err)
	}
	
	// Parse entity definitions
	entitiesDir := filepath.Join("fixtures", "entities")
	entityResult, err := parser.ParsePackageRecursively(entitiesDir)
	if err != nil {
		return fmt.Errorf("failed to parse entities: %w", err)
	}
	
	// Basic validation that we have both schemas
	if len(currentSchema.Tables) == 0 {
		return fmt.Errorf("current schema should have tables")
	}
	
	if len(entityResult.Tables) == 0 {
		return fmt.Errorf("entity schema should have tables")
	}
	
	// This is a basic test - in a real implementation, you'd use the differ package
	// to compare schemas and generate a detailed diff
	
	return nil
}

// testFailureDiagnostics tests handling of failing migrations
func testFailureDiagnostics(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	// Use the failing migrations set
	migrationsFS, err := GetMigrationsFS(fixtures, conn, "failing")
	if err != nil {
		return fmt.Errorf("failed to get failing migrations filesystem: %w", err)
	}
	
	helper := NewDatabaseHelper(conn)
	
	// Try to apply migrations - this should fail
	err = helper.ApplyMigrations(ctx, migrationsFS)
	if err == nil {
		return fmt.Errorf("expected migration to fail, but it succeeded")
	}
	
	// Verify the error contains useful information
	errorMsg := err.Error()
	if errorMsg == "" {
		return fmt.Errorf("error message should not be empty")
	}
	
	// Check that we can still query the database state
	version, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("should be able to query version even after failure: %w", err)
	}
	
	// Version should be 0 or 1 depending on where the failure occurred
	if version < 0 || version > 1 {
		return fmt.Errorf("unexpected version after failure: %d", version)
	}
	
	return nil
}

// testIdempotencyReapply tests re-applying already applied migrations
func testIdempotencyReapply(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}
	
	// Apply migrations first time
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations first time: %w", err)
	}
	
	// Get version after first application
	version1, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get version after first application: %w", err)
	}
	
	// Apply migrations second time - should be idempotent
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations second time: %w", err)
	}
	
	// Get version after second application
	version2, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get version after second application: %w", err)
	}
	
	// Versions should be the same
	if version1 != version2 {
		return fmt.Errorf("versions should be the same after idempotent application: %d vs %d", version1, version2)
	}
	
	return nil
}

// testIdempotencyUpToDate tests running migrate up when already up-to-date
func testIdempotencyUpToDate(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}
	
	// Apply all migrations
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	
	// Check status - should have no pending changes
	status, err := migrator.GetMigrationStatus(ctx, conn, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}
	
	if status.HasPendingChanges {
		return fmt.Errorf("should not have pending changes after applying all migrations")
	}
	
	if len(status.PendingMigrations) != 0 {
		return fmt.Errorf("should have no pending migrations, got %d", len(status.PendingMigrations))
	}
	
	// Try to apply again - should be no-op
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations when up-to-date: %w", err)
	}
	
	return nil
}

// testConcurrencyParallelMigrate tests parallel migration execution
func testConcurrencyParallelMigrate(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}
	
	// Create two separate connections for parallel execution
	conn1, err := executor.ConnectToDatabase(conn.Info().URL)
	if err != nil {
		return fmt.Errorf("failed to create first connection: %w", err)
	}
	defer conn1.Close()
	
	conn2, err := executor.ConnectToDatabase(conn.Info().URL)
	if err != nil {
		return fmt.Errorf("failed to create second connection: %w", err)
	}
	defer conn2.Close()
	
	var wg sync.WaitGroup
	var err1, err2 error
	
	// Launch two parallel migrations
	wg.Add(2)
	
	go func() {
		defer wg.Done()
		err1 = migrator.RunMigrations(ctx, conn1, migrationsFS)
	}()
	
	go func() {
		defer wg.Done()
		err2 = migrator.RunMigrations(ctx, conn2, migrationsFS)
	}()
	
	wg.Wait()
	
	// At least one should succeed, and if both succeed, that's also fine (idempotent)
	if err1 != nil && err2 != nil {
		return fmt.Errorf("both parallel migrations failed: err1=%v, err2=%v", err1, err2)
	}
	
	// Verify final state is correct
	helper := NewDatabaseHelper(conn)
	version, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get final version: %w", err)
	}
	
	if version != 3 {
		return fmt.Errorf("expected final version 3, got %d", version)
	}
	
	return nil
}

// testPartialFailureRecovery tests handling of partial migration failures
func testPartialFailureRecovery(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	// Use the partial failure migrations set
	migrationsFS, err := GetMigrationsFS(fixtures, conn, "partial_failure")
	if err != nil {
		return fmt.Errorf("failed to get partial failure migrations filesystem: %w", err)
	}
	
	helper := NewDatabaseHelper(conn)
	
	// Try to apply migrations - should fail partway through
	err = helper.ApplyMigrations(ctx, migrationsFS)
	if err == nil {
		return fmt.Errorf("expected migration to fail, but it succeeded")
	}
	
	// Check what was applied before failure
	version, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("should be able to query version after partial failure: %w", err)
	}
	
	// Should have applied some but not all migrations
	if version == 0 {
		return fmt.Errorf("expected some migrations to be applied before failure")
	}
	
	// Verify we can still interact with the database
	exists, err := helper.TableExists("users")
	if err != nil {
		return fmt.Errorf("should be able to check table existence after partial failure: %w", err)
	}
	
	if !exists {
		return fmt.Errorf("users table should exist after partial failure")
	}
	
	return nil
}
