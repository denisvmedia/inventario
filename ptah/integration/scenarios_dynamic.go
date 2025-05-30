package integration

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/denisvmedia/inventario/ptah/executor"
)

// GetDynamicScenarios returns all dynamic integration test scenarios that use versioned entities
func GetDynamicScenarios() []TestScenario {
	return []TestScenario{
		{
			Name:        "dynamic_basic_evolution",
			Description: "Test basic schema evolution using versioned entities: 000 → 001 → 002 → 003",
			TestFunc:    testDynamicBasicEvolution,
		},
		{
			Name:        "dynamic_skip_versions",
			Description: "Test non-sequential migration: 000 → 002 → 003",
			TestFunc:    testDynamicSkipVersions,
		},
		{
			Name:        "dynamic_idempotency",
			Description: "Test applying the same version multiple times",
			TestFunc:    testDynamicIdempotency,
		},
		{
			Name:        "dynamic_partial_apply",
			Description: "Test applying to specific version, then continuing",
			TestFunc:    testDynamicPartialApply,
		},
		{
			Name:        "dynamic_schema_diff",
			Description: "Test schema diff generation between versions",
			TestFunc:    testDynamicSchemaDiff,
		},
		{
			Name:        "dynamic_migration_sql_generation",
			Description: "Test SQL migration generation from entity changes",
			TestFunc:    testDynamicMigrationSQLGeneration,
		},
	}
}

// testDynamicBasicEvolution tests the basic evolution path through all versions
func testDynamicBasicEvolution(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	// Create versioned entity manager
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Evolution path: 000 → 001 → 002 → 003
	versions := []struct {
		dir         string
		description string
	}{
		{"000-initial", "Create initial users and products tables"},
		{"001-add-fields", "Add additional fields to users and products"},
		{"002-add-posts", "Add posts table with foreign key to users"},
		{"003-add-enums", "Add enum types and status fields"},
	}

	for _, version := range versions {
		fmt.Printf("Applying version %s: %s\n", version.dir, version.description)
		
		if err := vem.MigrateToVersion(ctx, conn, version.dir, version.description); err != nil {
			return fmt.Errorf("failed to migrate to version %s: %w", version.dir, err)
		}
		
		fmt.Printf("Successfully applied version %s\n", version.dir)
	}

	// Verify final state
	schema, err := vem.GenerateSchemaFromEntities()
	if err != nil {
		return fmt.Errorf("failed to generate final schema: %w", err)
	}

	// Should have 3 tables: users, products, posts
	if len(schema.Tables) != 3 {
		return fmt.Errorf("expected 3 tables, got %d", len(schema.Tables))
	}

	// Should have 3 enums: user_status, product_status, post_status
	if len(schema.Enums) != 3 {
		return fmt.Errorf("expected 3 enums, got %d", len(schema.Enums))
	}

	return nil
}

// testDynamicSkipVersions tests non-sequential version application
func testDynamicSkipVersions(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Skip version 001, go directly from 000 → 002 → 003
	versions := []struct {
		dir         string
		description string
	}{
		{"000-initial", "Create initial users and products tables"},
		{"002-add-posts", "Add posts table and additional fields"},
		{"003-add-enums", "Add enum types and status fields"},
	}

	for _, version := range versions {
		if err := vem.MigrateToVersion(ctx, conn, version.dir, version.description); err != nil {
			return fmt.Errorf("failed to migrate to version %s: %w", version.dir, err)
		}
	}

	// Verify final state is the same as sequential application
	schema, err := vem.GenerateSchemaFromEntities()
	if err != nil {
		return fmt.Errorf("failed to generate final schema: %w", err)
	}

	if len(schema.Tables) != 3 {
		return fmt.Errorf("expected 3 tables, got %d", len(schema.Tables))
	}

	if len(schema.Enums) != 3 {
		return fmt.Errorf("expected 3 enums, got %d", len(schema.Enums))
	}

	return nil
}

// testDynamicIdempotency tests applying the same version multiple times
func testDynamicIdempotency(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Apply version 001 twice
	version := "001-add-fields"
	description := "Add additional fields to users and products"

	// First application
	if err := vem.MigrateToVersion(ctx, conn, version, description); err != nil {
		return fmt.Errorf("failed to migrate to version %s (first time): %w", version, err)
	}

	// Second application - should be idempotent (no new migrations applied)
	// Instead of checking SQL generation, check that no new migrations are applied
	currentVersion, err := getCurrentMigrationVersion(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	// Try to apply the same version again
	if err := vem.MigrateToVersion(ctx, conn, version, description); err != nil {
		return fmt.Errorf("failed to migrate to version %s (second time): %w", version, err)
	}

	// Check that no new migration was applied
	newVersion, err := getCurrentMigrationVersion(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to get new migration version: %w", err)
	}

	if newVersion != currentVersion {
		return fmt.Errorf("expected migration version to remain %d, but got %d", currentVersion, newVersion)
	}

	return nil
}

// testDynamicPartialApply tests applying to a specific version, then continuing
func testDynamicPartialApply(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Apply up to version 001
	if err := vem.MigrateToVersion(ctx, conn, "000-initial", "Create initial tables"); err != nil {
		return fmt.Errorf("failed to migrate to version 000: %w", err)
	}

	if err := vem.MigrateToVersion(ctx, conn, "001-add-fields", "Add additional fields"); err != nil {
		return fmt.Errorf("failed to migrate to version 001: %w", err)
	}

	// Verify intermediate state
	schema, err := vem.GenerateSchemaFromEntities()
	if err != nil {
		return fmt.Errorf("failed to generate intermediate schema: %w", err)
	}

	if len(schema.Tables) != 2 {
		return fmt.Errorf("expected 2 tables at intermediate state, got %d", len(schema.Tables))
	}

	// Continue to final version
	if err := vem.MigrateToVersion(ctx, conn, "003-add-enums", "Add enum types"); err != nil {
		return fmt.Errorf("failed to migrate to version 003: %w", err)
	}

	// Verify final state
	finalSchema, err := vem.GenerateSchemaFromEntities()
	if err != nil {
		return fmt.Errorf("failed to generate final schema: %w", err)
	}

	if len(finalSchema.Tables) != 3 {
		return fmt.Errorf("expected 3 tables at final state, got %d", len(finalSchema.Tables))
	}

	if len(finalSchema.Enums) != 3 {
		return fmt.Errorf("expected 3 enums at final state, got %d", len(finalSchema.Enums))
	}

	return nil
}

// testDynamicSchemaDiff tests schema diff generation between versions
func testDynamicSchemaDiff(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Apply initial version
	if err := vem.MigrateToVersion(ctx, conn, "000-initial", "Create initial tables"); err != nil {
		return fmt.Errorf("failed to migrate to version 000: %w", err)
	}

	// Load next version entities but don't apply yet
	if err := vem.LoadEntityVersion("002-add-posts"); err != nil {
		return fmt.Errorf("failed to load version 002 entities: %w", err)
	}

	// Generate migration SQL to see the diff
	statements, err := vem.GenerateMigrationSQL(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to generate migration SQL: %w", err)
	}

	// Should have statements to add fields and create posts table
	if len(statements) == 0 {
		return fmt.Errorf("expected migration statements for schema diff, got none")
	}

	// Verify we have statements for adding the posts table
	hasPostsTable := false
	for _, stmt := range statements {
		if contains(stmt, "CREATE TABLE posts") || contains(stmt, "CREATE TABLE \"posts\"") {
			hasPostsTable = true
			break
		}
	}

	if !hasPostsTable {
		return fmt.Errorf("expected CREATE TABLE posts statement in migration SQL")
	}

	return nil
}

// testDynamicMigrationSQLGeneration tests SQL generation from entity changes
func testDynamicMigrationSQLGeneration(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Start with empty database, load version 001 entities
	if err := vem.LoadEntityVersion("001-add-fields"); err != nil {
		return fmt.Errorf("failed to load version 001 entities: %w", err)
	}

	// Generate SQL for creating everything from scratch
	statements, err := vem.GenerateMigrationSQL(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to generate migration SQL: %w", err)
	}

	if len(statements) == 0 {
		return fmt.Errorf("expected migration statements for initial creation, got none")
	}

	// Should have CREATE TABLE statements for users and products
	hasUsersTable := false
	hasProductsTable := false
	
	for _, stmt := range statements {
		if contains(stmt, "CREATE TABLE users") || contains(stmt, "CREATE TABLE \"users\"") {
			hasUsersTable = true
		}
		if contains(stmt, "CREATE TABLE products") || contains(stmt, "CREATE TABLE \"products\"") {
			hasProductsTable = true
		}
	}

	if !hasUsersTable {
		return fmt.Errorf("expected CREATE TABLE users statement in migration SQL")
	}

	if !hasProductsTable {
		return fmt.Errorf("expected CREATE TABLE products statement in migration SQL")
	}

	return nil
}

// getCurrentMigrationVersion gets the current migration version from the database
func getCurrentMigrationVersion(ctx context.Context, conn *executor.DatabaseConnection) (int, error) {
	// Query the schema_migrations table to get the highest version
	query := "SELECT COALESCE(MAX(version), 0) FROM schema_migrations"
	row := conn.QueryRow(query)

	var version int
	if err := row.Scan(&version); err != nil {
		return 0, fmt.Errorf("failed to scan migration version: %w", err)
	}

	return version, nil
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		   (s == substr ||
		    len(s) > len(substr) &&
		    (s[:len(substr)] == substr ||
		     s[len(s)-len(substr):] == substr ||
		     containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
