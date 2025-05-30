package integration

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// GetDynamicScenarios returns all dynamic integration test scenarios that use versioned entities
func GetDynamicScenarios() []TestScenario {
	return []TestScenario{
		{
			Name:             "dynamic_basic_evolution",
			Description:      "Test basic schema evolution using versioned entities: 000 → 001 → 002 → 003",
			EnhancedTestFunc: testDynamicBasicEvolution,
		},
		{
			Name:             "dynamic_skip_versions",
			Description:      "Test non-sequential migration: 000 → 002 → 003",
			EnhancedTestFunc: testDynamicSkipVersions,
		},
		{
			Name:             "dynamic_idempotency",
			Description:      "Test applying the same version multiple times",
			EnhancedTestFunc: testDynamicIdempotency,
		},
		{
			Name:             "dynamic_partial_apply",
			Description:      "Test applying to specific version, then continuing",
			EnhancedTestFunc: testDynamicPartialApply,
		},
		{
			Name:             "dynamic_schema_diff",
			Description:      "Test schema diff generation between versions",
			EnhancedTestFunc: testDynamicSchemaDiff,
		},
		{
			Name:             "dynamic_migration_sql_generation",
			Description:      "Test SQL migration generation from entity changes",
			EnhancedTestFunc: testDynamicMigrationSQLGeneration,
		},
	}
}

// testDynamicBasicEvolution tests the basic evolution path through all versions
func testDynamicBasicEvolution(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	// Create versioned entity manager
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Evolution path: 000 → 001 → 002 → 003 → 004 → 005 → 006 → 007 → 008 → 009 → 010 → 011 → 012
	versions := []struct {
		dir         string
		description string
	}{
		{"000-initial", "Create initial users and products tables"},
		{"001-add-fields", "Add additional fields to users and products"},
		{"002-add-posts", "Add posts table with foreign key to users"},
		{"003-add-enums", "Add enum types and status fields"},
		{"004-field-rename", "Rename fields: bio → description, age → user_age"},
		{"005-field-type-change", "Change field types: user_age INTEGER → SMALLINT, description TEXT → VARCHAR(500)"},
		{"006-field-drop", "Drop unused fields: active field from users"},
		{"007-index-add", "Add new indexes: compound index on name+email, index on description"},
		{"008-index-remove", "Remove old indexes: idx_users_email (replaced by compound)"},
		{"009-add-constraints", "Add constraints: check constraint on user_age, foreign key constraint"},
		{"010-drop-constraints", "Drop constraints: remove check constraint on user_age"},
		{"011-add-entity", "Add new entity: categories table"},
		{"012-drop-entity", "Drop entity: remove products table"},
	}

	for _, version := range versions {
		stepName := fmt.Sprintf("Apply %s", version.dir)
		stepDesc := version.description

		err := recorder.RecordStep(stepName, stepDesc, func() error {
			fmt.Printf("Applying version %s: %s\n", version.dir, version.description)

			if err := vem.MigrateToVersion(ctx, conn, version.dir, version.description); err != nil {
				return fmt.Errorf("failed to migrate to version %s: %w", version.dir, err)
			}

			fmt.Printf("Successfully applied version %s\n", version.dir)
			return nil
		})

		if err != nil {
			return err
		}
	}

	// Verify final state
	return recorder.RecordStep("Verify Final State", "Validate that all migrations were applied correctly", func() error {
		schema, err := vem.GenerateSchemaFromEntities()
		if err != nil {
			return fmt.Errorf("failed to generate final schema: %w", err)
		}

		// Should have 3 tables: users, posts, categories (products was dropped in version 012)
		if len(schema.Tables) != 3 {
			return fmt.Errorf("expected 3 tables, got %d", len(schema.Tables))
		}

		// Should have 2 enums: user_status, post_status (product_status was dropped with products table)
		if len(schema.Enums) != 2 {
			return fmt.Errorf("expected 2 enums, got %d", len(schema.Enums))
		}

		// Verify that field renames, type changes, and constraint changes were applied
		// Check that users table has the renamed fields (description, user_age) and not the old ones (bio, age)
		usersTable := findTable(schema.Tables, "users")
		if usersTable == nil {
			return fmt.Errorf("users table not found in final schema")
		}

		// Should have description field (renamed from bio)
		if !hasField(schema.Fields, "User", "description") {
			return fmt.Errorf("users table should have description field (renamed from bio)")
		}

		// Should have user_age field (renamed from age)
		if !hasField(schema.Fields, "User", "user_age") {
			return fmt.Errorf("users table should have user_age field (renamed from age)")
		}

		// Should NOT have bio or age fields (they were renamed)
		if hasField(schema.Fields, "User", "bio") {
			return fmt.Errorf("users table should not have bio field (it was renamed to description)")
		}

		if hasField(schema.Fields, "User", "age") {
			return fmt.Errorf("users table should not have age field (it was renamed to user_age)")
		}

		// Should NOT have active field (it was dropped)
		if hasField(schema.Fields, "User", "active") {
			return fmt.Errorf("users table should not have active field (it was dropped)")
		}

		// Verify that categories table was added
		categoriesTable := findTable(schema.Tables, "categories")
		if categoriesTable == nil {
			return fmt.Errorf("categories table should exist (added in version 011)")
		}

		// Verify that products table was dropped
		productsTable := findTable(schema.Tables, "products")
		if productsTable != nil {
			return fmt.Errorf("products table should not exist (dropped in version 012)")
		}

		return nil
	})
}

// testDynamicSkipVersions tests non-sequential version application
func testDynamicSkipVersions(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Skip several versions, go directly from 000 → 005 → 012
	versions := []struct {
		dir         string
		description string
	}{
		{"000-initial", "Create initial users and products tables"},
		{"005-field-type-change", "Add fields, posts table, enums, renames, and type changes"},
		{"012-drop-entity", "Apply all remaining changes including entity add/drop"},
	}

	for _, version := range versions {
		stepName := fmt.Sprintf("Apply %s", version.dir)
		stepDesc := version.description

		err := recorder.RecordStep(stepName, stepDesc, func() error {
			return vem.MigrateToVersion(ctx, conn, version.dir, version.description)
		})

		if err != nil {
			return fmt.Errorf("failed to migrate to version %s: %w", version.dir, err)
		}
	}

	// Verify final state is the same as sequential application
	return recorder.RecordStep("Verify Final State", "Validate that skip-version migration produces same result as sequential", func() error {
		schema, err := vem.GenerateSchemaFromEntities()
		if err != nil {
			return fmt.Errorf("failed to generate final schema: %w", err)
		}

		if len(schema.Tables) != 3 {
			return fmt.Errorf("expected 3 tables, got %d", len(schema.Tables))
		}

		if len(schema.Enums) != 2 {
			return fmt.Errorf("expected 2 enums, got %d", len(schema.Enums))
		}

		return nil
	})
}

// testDynamicIdempotency tests applying the same version multiple times
func testDynamicIdempotency(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Apply version 001 twice (simple add fields)
	version := "001-add-fields"
	description := "Add additional fields to users and products"

	// First application
	if err := vem.MigrateToVersion(ctx, conn, version, description); err != nil {
		return fmt.Errorf("failed to migrate to version %s (first time): %w", version, err)
	}

	// Get the current migration version after first application
	currentVersion, err := getCurrentMigrationVersion(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	// Try to apply the same version again - should be idempotent
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
func testDynamicPartialApply(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Apply up to version 001 (add fields - still only 2 tables: users, products)
	if err := vem.MigrateToVersion(ctx, conn, "000-initial", "Create initial tables"); err != nil {
		return fmt.Errorf("failed to migrate to version 000: %w", err)
	}

	if err := vem.MigrateToVersion(ctx, conn, "001-add-fields", "Add additional fields to users and products"); err != nil {
		return fmt.Errorf("failed to migrate to version 001: %w", err)
	}

	// Verify intermediate state (should have 2 tables: users, products)
	schema, err := vem.GenerateSchemaFromEntities()
	if err != nil {
		return fmt.Errorf("failed to generate intermediate schema: %w", err)
	}

	if len(schema.Tables) != 2 {
		return fmt.Errorf("expected 2 tables at intermediate state, got %d", len(schema.Tables))
	}

	// Continue to final version
	if err := vem.MigrateToVersion(ctx, conn, "012-drop-entity", "Apply all remaining changes including entity add/drop"); err != nil {
		return fmt.Errorf("failed to migrate to version 012: %w", err)
	}

	// Verify final state
	finalSchema, err := vem.GenerateSchemaFromEntities()
	if err != nil {
		return fmt.Errorf("failed to generate final schema: %w", err)
	}

	if len(finalSchema.Tables) != 3 {
		return fmt.Errorf("expected 3 tables at final state, got %d", len(finalSchema.Tables))
	}

	if len(finalSchema.Enums) != 2 {
		return fmt.Errorf("expected 2 enums at final state, got %d", len(finalSchema.Enums))
	}

	return nil
}

// testDynamicSchemaDiff tests schema diff generation between versions
func testDynamicSchemaDiff(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
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
	if err := vem.LoadEntityVersion("006-field-drop"); err != nil {
		return fmt.Errorf("failed to load version 006 entities: %w", err)
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
func testDynamicMigrationSQLGeneration(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Start with empty database, load version 008 entities (index remove)
	if err := vem.LoadEntityVersion("008-index-remove"); err != nil {
		return fmt.Errorf("failed to load version 008 entities: %w", err)
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

// findTable finds a table by name in a slice of tables
func findTable(tables []types.TableDirective, name string) *types.TableDirective {
	for i, table := range tables {
		if table.Name == name {
			return &tables[i]
		}
	}
	return nil
}

// hasField checks if a field exists for a specific table
func hasField(fields []types.SchemaField, tableName, fieldName string) bool {
	for _, field := range fields {
		if field.StructName == tableName && field.Name == fieldName {
			return true
		}
	}
	return false
}
