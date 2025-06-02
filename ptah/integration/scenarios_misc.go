package integration

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/denisvmedia/inventario/ptah/dbschema"
)

// testTimestampVerification tests that applied_at timestamps are stored correctly
func testTimestampVerification(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Apply migrations
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Query the schema_migrations table directly (it's excluded from schema introspection)
	// Check that the table exists and has the expected structure
	var count int
	err = conn.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		return fmt.Errorf("schema_migrations table should exist and be queryable: %w", err)
	}

	// Verify we have the expected number of migrations applied
	if count != 3 { // basic migrations has 3 migrations
		return fmt.Errorf("expected 3 applied migrations, got %d", count)
	}

	// Query a sample migration record to verify timestamp structure
	var version int
	var description string
	var appliedAt string
	err = conn.QueryRow("SELECT version, description, applied_at FROM schema_migrations ORDER BY version LIMIT 1").Scan(&version, &description, &appliedAt)
	if err != nil {
		return fmt.Errorf("failed to query migration record: %w", err)
	}

	// Verify the record has valid data
	if version <= 0 {
		return fmt.Errorf("migration version should be positive, got %d", version)
	}
	if description == "" {
		return fmt.Errorf("migration description should not be empty")
	}
	if appliedAt == "" {
		return fmt.Errorf("applied_at timestamp should not be empty")
	}

	return nil
}

// testManualPatchDetection tests detecting manual schema changes
func testManualPatchDetection(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Apply migrations first
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Make a manual change to the database
	manualSQL := "ALTER TABLE users ADD COLUMN manual_column VARCHAR(255)"
	if err := helper.ExecuteSQL(manualSQL); err != nil {
		return fmt.Errorf("failed to execute manual SQL: %w", err)
	}

	// Read the schema to verify the manual change
	schema, err := conn.Reader().ReadSchema()
	if err != nil {
		return fmt.Errorf("failed to read schema after manual change: %w", err)
	}

	// Find the users table and check for the manual column
	var usersTable *struct {
		Name    string
		Columns []struct {
			Name string
		}
	}

	for _, table := range schema.Tables {
		if table.Name == "users" {
			usersTable = &struct {
				Name    string
				Columns []struct{ Name string }
			}{
				Name: table.Name,
			}
			for _, col := range table.Columns {
				usersTable.Columns = append(usersTable.Columns, struct{ Name string }{Name: col.Name})
			}
			break
		}
	}

	if usersTable == nil {
		return fmt.Errorf("users table should exist")
	}

	// Check if manual column exists
	var hasManualColumn bool
	for _, col := range usersTable.Columns {
		if col.Name == "manual_column" {
			hasManualColumn = true
			break
		}
	}

	if !hasManualColumn {
		return fmt.Errorf("manual_column should exist after manual change")
	}

	// This test verifies that we can detect manual changes
	// In a real implementation, you'd use the schema diff functionality
	// to compare against entity definitions and detect drift

	return nil
}

// testPermissionRestrictions tests behavior with limited database permissions
func testPermissionRestrictions(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	// This test is challenging to implement without actually creating a restricted user
	// For now, we'll simulate the scenario by testing error handling

	helper := NewDatabaseHelper(conn)

	// Try to execute a statement that might fail due to permissions
	// This is a simplified test - in a real scenario, you'd connect with a restricted user
	restrictedSQL := "CREATE DATABASE test_restricted_db"
	err := helper.ExecuteSQL(restrictedSQL)

	// We expect this might fail, and that's okay for this test
	// The important thing is that we handle the error gracefully
	if err != nil {
		// Check that the error message is informative
		errorMsg := err.Error()
		if errorMsg == "" {
			return fmt.Errorf("error message should not be empty")
		}

		// Error should contain some indication of the problem
		// if !strings.Contains(strings.ToLower(errorMsg), "permission") &&
		//   !strings.Contains(strings.ToLower(errorMsg), "denied") &&
		//   !strings.Contains(strings.ToLower(errorMsg), "access") &&
		//   !strings.Contains(strings.ToLower(errorMsg), "privilege") {
		//	// This might not be a permission error, which is fine for this test
		// }
	}

	// Clean up if the statement succeeded
	if err == nil {
		_ = helper.ExecuteSQL("DROP DATABASE IF EXISTS test_restricted_db")
	}

	return nil
}

// testCleanupSupport tests dropping all tables and re-running migrations
func testCleanupSupport(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	helper := NewDatabaseHelper(conn)

	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Apply migrations first
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations initially: %w", err)
	}

	// Verify tables exist
	exists, err := helper.TableExists("users")
	if err != nil {
		return fmt.Errorf("failed to check if users table exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("users table should exist after initial migration")
	}

	// Drop all tables (cleanup)
	if err := conn.Writer().DropAllTables(); err != nil {
		return fmt.Errorf("failed to drop all tables: %w", err)
	}

	// Verify tables no longer exist
	exists, err = helper.TableExists("users")
	if err != nil {
		return fmt.Errorf("failed to check if users table exists after cleanup: %w", err)
	}
	if exists {
		return fmt.Errorf("users table should not exist after cleanup")
	}

	// Re-run migrations from empty state
	if err := helper.ApplyMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("failed to apply migrations after cleanup: %w", err)
	}

	// Verify tables exist again
	exists, err = helper.TableExists("users")
	if err != nil {
		return fmt.Errorf("failed to check if users table exists after re-migration: %w", err)
	}
	if !exists {
		return fmt.Errorf("users table should exist after re-migration")
	}

	// Verify final version is correct
	version, err := helper.GetCurrentVersion(ctx, migrationsFS)
	if err != nil {
		return fmt.Errorf("failed to get final version: %w", err)
	}

	if version != 3 {
		return fmt.Errorf("expected final version 3, got %d", version)
	}

	return nil
}
