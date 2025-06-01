package integration

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/dbschema"
	"github.com/denisvmedia/inventario/ptah/migration/generator"
	"github.com/denisvmedia/inventario/ptah/migration/migrator"
	"github.com/denisvmedia/inventario/ptah/migration/schemadiff"
	difftypes "github.com/denisvmedia/inventario/ptah/migration/schemadiff/types"
)

// testOperationPlanning tests generating detailed operation plans
func testOperationPlanning(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
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
func testSchemaDiff(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
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
	entityResult, err := goschema.ParseDir(entitiesDir)
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
func testFailureDiagnostics(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
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
func testIdempotencyReapply(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
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
func testIdempotencyUpToDate(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
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
func testConcurrencyParallelMigrate(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
	migrationsFS, err := GetMigrationsFS(fixtures, conn, "basic")
	if err != nil {
		return fmt.Errorf("failed to get migrations filesystem: %w", err)
	}

	// Create two separate connections for parallel execution
	conn1, err := dbschema.ConnectToDatabase(conn.Info().URL)
	if err != nil {
		return fmt.Errorf("failed to create first connection: %w", err)
	}
	defer conn1.Close()

	conn2, err := dbschema.ConnectToDatabase(conn.Info().URL)
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
func testPartialFailureRecovery(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS) error {
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

// testMigrationGeneratorValidation tests the migration generator with forward and rollback migrations
// This test validates the correctness of migration generation and application using the ptah/migration/generator module.
// It ensures that the resulting database schema is consistent with goschema, and that schemadiff reports no differences.
func testMigrationGeneratorValidation(ctx context.Context, conn *dbschema.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	// Create versioned entity manager
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	migrationsDir, err := os.MkdirTemp("", "ptah_integration_test_migrations_*")
	if err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}
	migrationsFs := os.DirFS(migrationsDir)
	dh := NewDatabaseHelper(conn)

	err = recorder.RecordStep("1.1 Initial Migration", "Apply migrations from 000-initial", func() error {
		// Step 1: Initial Migration (000-initial)
		if err := vem.LoadEntityVersion("000-initial"); err != nil {
			return err
		}

		_, err := generator.GenerateMigration(generator.GenerateMigrationOptions{
			RootDir:   vem.GetEntitiesDir(),
			DBConn:    conn,
			OutputDir: migrationsDir,
		})
		if err != nil {
			return err
		}
		if err := dh.MigrateUp(ctx, migrationsFs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("1.2 Validate Initial Migration", "Validate schema consistency after initial migration", func() error {
		// Validate Step 1: Database schema matches goschema output for 000-initial
		if err := validateSchemaConsistency(ctx, conn, vem, "000-initial"); err != nil {
			return fmt.Errorf("step 1 validation failed: %w", err)
		}
		return err
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("2.1 Add Fields Migration", "Apply migrations from 001-add-fields", func() error {
		// Step 2: Add Fields (001-add-fields)
		if err := vem.LoadEntityVersion("001-add-fields"); err != nil {
			return err
		}

		_, err = generator.GenerateMigration(generator.GenerateMigrationOptions{
			RootDir:   vem.GetEntitiesDir(),
			DBConn:    conn,
			OutputDir: migrationsDir,
		})
		if err != nil {
			return err
		}
		if err := dh.MigrateUp(ctx, migrationsFs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("2.2 Validate Add Fields Migration", "Validate schema consistency after add fields migration", func() error {
		// Validate Step 2: Database schema matches goschema output for 001-add-fields
		if err := validateSchemaConsistency(ctx, conn, vem, "001-add-fields"); err != nil {
			return fmt.Errorf("step 2 validation failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("3.1 Add Posts Migration", "Apply migrations from 002-add-posts", func() error {
		// Step 3: Add Posts (002-add-posts)
		if err := vem.LoadEntityVersion("002-add-posts"); err != nil {
			return err
		}
		_, err = generator.GenerateMigration(generator.GenerateMigrationOptions{
			RootDir:   vem.GetEntitiesDir(),
			DBConn:    conn,
			OutputDir: migrationsDir,
		})
		if err != nil {
			return err
		}
		if err := dh.MigrateUp(ctx, migrationsFs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("3.2 Validate Add Posts Migration", "Validate schema consistency after add posts migration", func() error {
		// Validate Step 3: Database schema matches goschema output for 002-add-posts
		if err := validateSchemaConsistency(ctx, conn, vem, "002-add-posts"); err != nil {
			return fmt.Errorf("step 3 validation failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("4.1 Rollback to Add Fields", "Rollback to step 2 (001-add-fields)", func() error {
		// Step 4: Rollback to Step 2 (001-add-fields)
		if err := dh.MigrateDown(ctx, migrationsFs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("4.2 Validate Rollback to Add Fields", "Validate schema consistency after rollback to add fields", func() error {
		// Validate Step 4: Database schema matches goschema output for 001-add-fields
		if err := vem.LoadEntityVersion("001-add-fields"); err != nil {
			return err
		}
		if err := validateSchemaConsistency(ctx, conn, vem, "001-add-fields"); err != nil {
			return fmt.Errorf("step 4 validation failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("5.1 Rollback to Initial", "Rollback to step 1 (000-initial)", func() error {
		// Step 5: Rollback to Step 1 (000-initial)
		if err := dh.MigrateDown(ctx, migrationsFs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("5.2 Validate Rollback to Initial", "Validate schema consistency after rollback to initial", func() error {
		// Validate Step 5: Database schema matches goschema output for 000-initial
		if err := vem.LoadEntityVersion("000-initial"); err != nil {
			return err
		}
		if err := validateSchemaConsistency(ctx, conn, vem, "000-initial"); err != nil {
			return fmt.Errorf("step 5 validation failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("6.1 Rollback to Empty State", "Rollback to empty database state", func() error {
		// Step 6: Rollback to Empty State
		if err := dh.MigrateDown(ctx, migrationsFs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("6.2 Validate Empty State", "Validate that database schema is empty", func() error {
		// Validate Step 6: Database schema is empty
		if err := validateEmptySchema(ctx, conn); err != nil {
			return fmt.Errorf("step 6 validation failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("7.1 Apply All Migrations at Once", "Apply all 3 migrations sequentially from empty state", func() error {
		// Step 7: Apply all migrations at once
		if err := dh.MigrateUp(ctx, migrationsFs); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("7.2 Validate Final State", "Validate schema consistency after applying all migrations", func() error {
		// Validate final state: Database schema matches goschema output for 002-add-posts
		if err := validateSchemaConsistency(ctx, conn, vem, "002-add-posts"); err != nil {
			return fmt.Errorf("final state validation failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("7.3 Drop Schema", "Drop all tables to clean up", func() error {
		// Step 7.3: Drop the schema to clean up
		if err := rollbackToEmptyState(ctx, conn); err != nil {
			return fmt.Errorf("failed to drop schema: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = recorder.RecordStep("7.4 Validate Clean State", "Validate that database is clean after dropping schema", func() error {
		// Validate clean state: Database schema is empty
		if err := validateEmptySchema(ctx, conn); err != nil {
			return fmt.Errorf("clean state validation failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// validateSchemaConsistency validates that the database schema matches the goschema output for a given version
// and that schemadiff reports no differences
func validateSchemaConsistency(ctx context.Context, conn *dbschema.DatabaseConnection, vem *VersionedEntityManager, versionDir string) error {
	// Load entities for the specified version
	if err := vem.LoadEntityVersion(versionDir); err != nil {
		return fmt.Errorf("failed to load entity version %s: %w", versionDir, err)
	}

	// Generate expected schema from entities
	expectedSchema, err := vem.GenerateSchemaFromEntities()
	if err != nil {
		return fmt.Errorf("failed to generate schema from entities: %w", err)
	}

	// Read actual database schema
	actualSchema, err := conn.Reader().ReadSchema()
	if err != nil {
		return fmt.Errorf("failed to read database schema: %w", err)
	}

	// Compare schemas using schemadiff
	diff := schemadiff.Compare(expectedSchema, actualSchema)

	// Check if there are any differences
	if hasSchemaChanges(diff) {
		return fmt.Errorf("schema differences detected for version %s: %+v", versionDir, diff)
	}

	return nil
}

// rollbackToVersion performs a rollback to a specific version by generating and applying down migrations
func rollbackToVersion(ctx context.Context, conn *dbschema.DatabaseConnection, vem *VersionedEntityManager, targetVersionDir, description string) error {
	// Load target version entities
	if err := vem.LoadEntityVersion(targetVersionDir); err != nil {
		return fmt.Errorf("failed to load target version %s: %w", targetVersionDir, err)
	}

	// Generate migration SQL to reach target state
	statements, err := vem.GenerateMigrationSQL(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to generate rollback migration SQL: %w", err)
	}

	// If no statements, we're already at the target state
	if len(statements) == 0 {
		return nil
	}

	// Apply the rollback migration
	if err := vem.ApplyMigrationFromEntities(ctx, conn, description); err != nil {
		return fmt.Errorf("failed to apply rollback migration: %w", err)
	}

	return nil
}

// rollbackToEmptyState drops all tables to return to an empty database state
func rollbackToEmptyState(ctx context.Context, conn *dbschema.DatabaseConnection) error {
	// Drop all tables to return to empty state
	if err := conn.Writer().DropAllTables(); err != nil {
		return fmt.Errorf("failed to drop all tables: %w", err)
	}

	return nil
}

// validateEmptySchema validates that the database schema is empty (no tables)
func validateEmptySchema(ctx context.Context, conn *dbschema.DatabaseConnection) error {
	// Read current schema
	schema, err := conn.Reader().ReadSchema()
	if err != nil {
		return fmt.Errorf("failed to read database schema: %w", err)
	}

	// Check that there are no tables
	if len(schema.Tables) > 0 {
		return fmt.Errorf("expected empty schema but found %d tables", len(schema.Tables))
	}

	return nil
}

// hasSchemaChanges checks if a SchemaDiff contains any changes
func hasSchemaChanges(diff *difftypes.SchemaDiff) bool {
	return diff.HasChanges()
}
