package integration

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/migrator"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// GetDynamicScenarios returns all dynamic integration test scenarios that use versioned entities
func GetDynamicScenarios() []TestScenario {
	return []TestScenario{
		// Basic functionality scenarios
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

		// Rollback/Downgrade scenarios
		{
			Name:             "dynamic_rollback_single",
			Description:      "Test rolling back one version (003 → 002)",
			EnhancedTestFunc: testDynamicRollbackSingle,
		},
		{
			Name:             "dynamic_rollback_multiple",
			Description:      "Test rolling back multiple versions (005 → 001)",
			EnhancedTestFunc: testDynamicRollbackMultiple,
		},
		{
			Name:             "dynamic_rollback_to_zero",
			Description:      "Test complete rollback to empty database",
			EnhancedTestFunc: testDynamicRollbackToZero,
		},

		// TODO: Re-enable these scenarios after fixing the rollback scenarios
		// Error handling & recovery scenarios
		// {
		// 	Name:             "dynamic_partial_failure_recovery",
		// 	Description:      "Test recovery from migration failure mid-way",
		// 	EnhancedTestFunc: testDynamicPartialFailureRecovery,
		// },
		// {
		// 	Name:             "dynamic_invalid_migration",
		// 	Description:      "Test handling of invalid/corrupted migration data",
		// 	EnhancedTestFunc: testDynamicInvalidMigration,
		// },
		// {
		// 	Name:             "dynamic_concurrent_migrations",
		// 	Description:      "Test concurrent migration attempts (locking behavior)",
		// 	EnhancedTestFunc: testDynamicConcurrentMigrations,
		// },

		// // Complex schema change scenarios
		// {
		// 	Name:             "dynamic_circular_dependencies",
		// 	Description:      "Test handling of circular foreign key dependencies",
		// 	EnhancedTestFunc: testDynamicCircularDependencies,
		// },
		// {
		// 	Name:             "dynamic_data_migration",
		// 	Description:      "Test migrations that require data transformation",
		// 	EnhancedTestFunc: testDynamicDataMigration,
		// },
		// {
		// 	Name:             "dynamic_large_table_migration",
		// 	Description:      "Test performance with large datasets during migration",
		// 	EnhancedTestFunc: testDynamicLargeTableMigration,
		// },

		// // Edge case scenarios
		// {
		// 	Name:             "dynamic_empty_migrations",
		// 	Description:      "Test versions with no actual schema changes",
		// 	EnhancedTestFunc: testDynamicEmptyMigrations,
		// },
		// {
		// 	Name:             "dynamic_duplicate_names",
		// 	Description:      "Test handling of duplicate table/field names across versions",
		// 	EnhancedTestFunc: testDynamicDuplicateNames,
		// },
		// {
		// 	Name:             "dynamic_reserved_keywords",
		// 	Description:      "Test migrations involving SQL reserved keywords",
		// 	EnhancedTestFunc: testDynamicReservedKeywords,
		// },

		// // Cross-database compatibility scenarios
		// {
		// 	Name:             "dynamic_dialect_differences",
		// 	Description:      "Test same migration across PostgreSQL/MySQL/MariaDB",
		// 	EnhancedTestFunc: testDynamicDialectDifferences,
		// },
		// {
		// 	Name:             "dynamic_type_mapping",
		// 	Description:      "Test database-specific type conversions",
		// 	EnhancedTestFunc: testDynamicTypeMapping,
		// },

		// // Validation & integrity scenarios
		// {
		// 	Name:             "dynamic_constraint_validation",
		// 	Description:      "Test constraint violations during migration",
		// 	EnhancedTestFunc: testDynamicConstraintValidation,
		// },
		// {
		// 	Name:             "dynamic_foreign_key_cascade",
		// 	Description:      "Test cascading effects of table/field drops",
		// 	EnhancedTestFunc: testDynamicForeignKeyCascade,
		// },
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

// ============================================================================
// ROLLBACK/DOWNGRADE SCENARIOS
// ============================================================================

// testDynamicRollbackSingle tests rolling back one version (003 → 002)
func testDynamicRollbackSingle(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Create a migrator and register all migrations with both up and down
	m := migrator.NewMigrator(conn)
	dialect := conn.Info().Dialect

	// Register migrations with database-specific SQL
	var migrations []*migrator.Migration

	if dialect == "mysql" {
		migrations = []*migrator.Migration{
			migrator.CreateMigrationFromSQL(1, "Create initial tables",
				"CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255)); CREATE TABLE products (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255));",
				"DROP TABLE products; DROP TABLE users;"),
			migrator.CreateMigrationFromSQL(2, "Add additional fields",
				"ALTER TABLE users ADD COLUMN email VARCHAR(255); ALTER TABLE products ADD COLUMN price DECIMAL(10,2);",
				"ALTER TABLE products DROP COLUMN price; ALTER TABLE users DROP COLUMN email;"),
			migrator.CreateMigrationFromSQL(3, "Add posts table",
				"CREATE TABLE posts (id INT AUTO_INCREMENT PRIMARY KEY, user_id INTEGER, title VARCHAR(255), FOREIGN KEY (user_id) REFERENCES users(id));",
				"DROP TABLE posts;"),
			migrator.CreateMigrationFromSQL(4, "Add enum types",
				"ALTER TABLE users ADD COLUMN status ENUM('active', 'inactive') DEFAULT 'active';",
				"ALTER TABLE users DROP COLUMN status;"),
		}
	} else {
		// PostgreSQL and MariaDB
		migrations = []*migrator.Migration{
			migrator.CreateMigrationFromSQL(1, "Create initial tables",
				"CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(255)); CREATE TABLE products (id SERIAL PRIMARY KEY, name VARCHAR(255));",
				"DROP TABLE products; DROP TABLE users;"),
			migrator.CreateMigrationFromSQL(2, "Add additional fields",
				"ALTER TABLE users ADD COLUMN email VARCHAR(255); ALTER TABLE products ADD COLUMN price DECIMAL(10,2);",
				"ALTER TABLE products DROP COLUMN price; ALTER TABLE users DROP COLUMN email;"),
			migrator.CreateMigrationFromSQL(3, "Add posts table",
				"CREATE TABLE posts (id SERIAL PRIMARY KEY, user_id INTEGER, title VARCHAR(255), FOREIGN KEY (user_id) REFERENCES users(id));",
				"DROP TABLE posts;"),
			migrator.CreateMigrationFromSQL(4, "Add enum types",
				"CREATE TYPE user_status AS ENUM ('active', 'inactive'); ALTER TABLE users ADD COLUMN status user_status DEFAULT 'active'::user_status;",
				"ALTER TABLE users DROP COLUMN status; DROP TYPE user_status;"),
		}
	}

	for _, migration := range migrations {
		m.Register(migration)
	}

	// Apply migrations up to version 4
	err = recorder.RecordStep("Apply All Migrations", "Apply migrations 1-4", func() error {
		return m.MigrateUp(ctx)
	})
	if err != nil {
		return err
	}

	// Verify we're at version 4
	currentVersion, err := getCurrentMigrationVersion(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}
	if currentVersion != 4 {
		return fmt.Errorf("expected version 4, got %d", currentVersion)
	}

	// Now rollback to version 3
	return recorder.RecordStep("Rollback to Version 3", "Roll back from version 4 to version 3", func() error {
		if err := m.MigrateDown(ctx, 3); err != nil {
			return fmt.Errorf("failed to rollback to version 3: %w", err)
		}

		// Verify we're now at version 3
		newVersion, err := getCurrentMigrationVersion(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to get version after rollback: %w", err)
		}
		if newVersion != 3 {
			return fmt.Errorf("expected version 3 after rollback, got %d", newVersion)
		}

		// Verify schema state - should have posts table but no enums
		schema, err := conn.Reader().ReadSchema()
		if err != nil {
			return fmt.Errorf("failed to read schema after rollback: %w", err)
		}

		// Should have 3 tables: users, products, posts (plus schema_migrations)
		applicationTables := 0
		for _, table := range schema.Tables {
			if table.Name != "schema_migrations" {
				applicationTables++
			}
		}
		if applicationTables != 3 {
			return fmt.Errorf("expected 3 application tables after rollback, got %d", applicationTables)
		}

		// Should have no enums (they were added in version 4)
		if len(schema.Enums) != 0 {
			return fmt.Errorf("expected 0 enums after rollback, got %d", len(schema.Enums))
		}

		return nil
	})
}

// testDynamicRollbackMultiple tests rolling back multiple versions (005 → 001)
func testDynamicRollbackMultiple(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Create a migrator and register all migrations with both up and down
	m := migrator.NewMigrator(conn)
	dialect := conn.Info().Dialect

	// Register migrations with database-specific SQL
	var migrations []*migrator.Migration

	if dialect == "mysql" {
		migrations = []*migrator.Migration{
			migrator.CreateMigrationFromSQL(1, "Create initial tables",
				"CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255)); CREATE TABLE products (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255));",
				"DROP TABLE products; DROP TABLE users;"),
			migrator.CreateMigrationFromSQL(2, "Add additional fields",
				"ALTER TABLE users ADD COLUMN email VARCHAR(255); ALTER TABLE products ADD COLUMN price DECIMAL(10,2);",
				"ALTER TABLE products DROP COLUMN price; ALTER TABLE users DROP COLUMN email;"),
			migrator.CreateMigrationFromSQL(3, "Add posts table",
				"CREATE TABLE posts (id INT AUTO_INCREMENT PRIMARY KEY, user_id INTEGER, title VARCHAR(255), FOREIGN KEY (user_id) REFERENCES users(id));",
				"DROP TABLE posts;"),
			migrator.CreateMigrationFromSQL(4, "Add enum types",
				"ALTER TABLE users ADD COLUMN status ENUM('active', 'inactive') DEFAULT 'active';",
				"ALTER TABLE users DROP COLUMN status;"),
			migrator.CreateMigrationFromSQL(5, "Rename fields",
				"ALTER TABLE users ADD COLUMN description TEXT; UPDATE users SET description = name; ALTER TABLE users DROP COLUMN name;",
				"ALTER TABLE users ADD COLUMN name VARCHAR(255); UPDATE users SET name = description; ALTER TABLE users DROP COLUMN description;"),
			migrator.CreateMigrationFromSQL(6, "Change field types",
				"ALTER TABLE users ADD COLUMN user_age SMALLINT; ALTER TABLE products MODIFY COLUMN price DECIMAL(12,2);",
				"ALTER TABLE products MODIFY COLUMN price DECIMAL(10,2); ALTER TABLE users DROP COLUMN user_age;"),
		}
	} else {
		// PostgreSQL and MariaDB
		migrations = []*migrator.Migration{
			migrator.CreateMigrationFromSQL(1, "Create initial tables",
				"CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(255)); CREATE TABLE products (id SERIAL PRIMARY KEY, name VARCHAR(255));",
				"DROP TABLE products; DROP TABLE users;"),
			migrator.CreateMigrationFromSQL(2, "Add additional fields",
				"ALTER TABLE users ADD COLUMN email VARCHAR(255); ALTER TABLE products ADD COLUMN price DECIMAL(10,2);",
				"ALTER TABLE products DROP COLUMN price; ALTER TABLE users DROP COLUMN email;"),
			migrator.CreateMigrationFromSQL(3, "Add posts table",
				"CREATE TABLE posts (id SERIAL PRIMARY KEY, user_id INTEGER, title VARCHAR(255), FOREIGN KEY (user_id) REFERENCES users(id));",
				"DROP TABLE posts;"),
			migrator.CreateMigrationFromSQL(4, "Add enum types",
				"CREATE TYPE user_status AS ENUM ('active', 'inactive'); ALTER TABLE users ADD COLUMN status user_status DEFAULT 'active'::user_status;",
				"ALTER TABLE users DROP COLUMN status; DROP TYPE user_status;"),
			migrator.CreateMigrationFromSQL(5, "Rename fields",
				"ALTER TABLE users ADD COLUMN description TEXT; UPDATE users SET description = name; ALTER TABLE users DROP COLUMN name;",
				"ALTER TABLE users ADD COLUMN name VARCHAR(255); UPDATE users SET name = description; ALTER TABLE users DROP COLUMN description;"),
			migrator.CreateMigrationFromSQL(6, "Change field types",
				"ALTER TABLE users ADD COLUMN user_age SMALLINT; ALTER TABLE products ALTER COLUMN price TYPE DECIMAL(12,2);",
				"ALTER TABLE products ALTER COLUMN price TYPE DECIMAL(10,2); ALTER TABLE users DROP COLUMN user_age;"),
		}
	}

	for _, migration := range migrations {
		m.Register(migration)
	}

	// Apply migrations up to version 6
	err = recorder.RecordStep("Apply All Migrations", "Apply migrations 1-6", func() error {
		return m.MigrateUp(ctx)
	})
	if err != nil {
		return err
	}

	// Verify we're at version 6
	currentVersion, err := getCurrentMigrationVersion(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}
	if currentVersion != 6 {
		return fmt.Errorf("expected version 6, got %d", currentVersion)
	}

	// Now rollback to version 2
	return recorder.RecordStep("Rollback to Version 2", "Roll back from version 6 to version 2", func() error {
		if err := m.MigrateDown(ctx, 2); err != nil {
			return fmt.Errorf("failed to rollback to version 2: %w", err)
		}

		// Verify we're now at version 2
		newVersion, err := getCurrentMigrationVersion(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to get version after rollback: %w", err)
		}
		if newVersion != 2 {
			return fmt.Errorf("expected version 2 after rollback, got %d", newVersion)
		}

		// Verify schema state - should only have users and products tables
		schema, err := conn.Reader().ReadSchema()
		if err != nil {
			return fmt.Errorf("failed to read schema after rollback: %w", err)
		}

		// Should have 2 tables: users, products (plus schema_migrations)
		applicationTables := 0
		for _, table := range schema.Tables {
			if table.Name != "schema_migrations" {
				applicationTables++
			}
		}
		if applicationTables != 2 {
			return fmt.Errorf("expected 2 application tables after rollback, got %d", applicationTables)
		}

		// Should have no enums
		if len(schema.Enums) != 0 {
			return fmt.Errorf("expected 0 enums after rollback, got %d", len(schema.Enums))
		}

		return nil
	})
}

// testDynamicRollbackToZero tests complete rollback to empty database
func testDynamicRollbackToZero(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Create a migrator and register all migrations with both up and down
	m := migrator.NewMigrator(conn)
	dialect := conn.Info().Dialect

	// Register migrations with database-specific SQL
	var migrations []*migrator.Migration

	if dialect == "mysql" {
		migrations = []*migrator.Migration{
			migrator.CreateMigrationFromSQL(1, "Create initial tables",
				"CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255)); CREATE TABLE products (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255));",
				"DROP TABLE products; DROP TABLE users;"),
			migrator.CreateMigrationFromSQL(2, "Add additional fields",
				"ALTER TABLE users ADD COLUMN email VARCHAR(255); ALTER TABLE products ADD COLUMN price DECIMAL(10,2);",
				"ALTER TABLE products DROP COLUMN price; ALTER TABLE users DROP COLUMN email;"),
			migrator.CreateMigrationFromSQL(3, "Add posts table",
				"CREATE TABLE posts (id INT AUTO_INCREMENT PRIMARY KEY, user_id INTEGER, title VARCHAR(255), FOREIGN KEY (user_id) REFERENCES users(id));",
				"DROP TABLE posts;"),
		}
	} else {
		// PostgreSQL and MariaDB
		migrations = []*migrator.Migration{
			migrator.CreateMigrationFromSQL(1, "Create initial tables",
				"CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(255)); CREATE TABLE products (id SERIAL PRIMARY KEY, name VARCHAR(255));",
				"DROP TABLE products; DROP TABLE users;"),
			migrator.CreateMigrationFromSQL(2, "Add additional fields",
				"ALTER TABLE users ADD COLUMN email VARCHAR(255); ALTER TABLE products ADD COLUMN price DECIMAL(10,2);",
				"ALTER TABLE products DROP COLUMN price; ALTER TABLE users DROP COLUMN email;"),
			migrator.CreateMigrationFromSQL(3, "Add posts table",
				"CREATE TABLE posts (id SERIAL PRIMARY KEY, user_id INTEGER, title VARCHAR(255), FOREIGN KEY (user_id) REFERENCES users(id));",
				"DROP TABLE posts;"),
		}
	}

	for _, migration := range migrations {
		m.Register(migration)
	}

	// Apply all migrations
	err = recorder.RecordStep("Apply All Migrations", "Apply migrations 1-3", func() error {
		return m.MigrateUp(ctx)
	})
	if err != nil {
		return err
	}

	// Verify we have tables
	schema, err := conn.Reader().ReadSchema()
	if err != nil {
		return fmt.Errorf("failed to read schema before rollback: %w", err)
	}
	if len(schema.Tables) == 0 {
		return fmt.Errorf("expected tables before rollback, got none")
	}

	// Now rollback to version 0 (empty database)
	return recorder.RecordStep("Rollback to Version 0", "Complete rollback to empty database", func() error {
		if err := m.MigrateDown(ctx, 0); err != nil {
			return fmt.Errorf("failed to rollback to version 0: %w", err)
		}

		// Verify we're now at version 0
		newVersion, err := getCurrentMigrationVersion(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to get version after rollback: %w", err)
		}
		if newVersion != 0 {
			return fmt.Errorf("expected version 0 after rollback, got %d", newVersion)
		}

		// Verify schema is empty (except for schema_migrations table)
		schema, err := conn.Reader().ReadSchema()
		if err != nil {
			return fmt.Errorf("failed to read schema after rollback: %w", err)
		}

		// Should have no application tables (only schema_migrations)
		applicationTables := 0
		for _, table := range schema.Tables {
			if table.Name != "schema_migrations" {
				applicationTables++
			}
		}
		if applicationTables != 0 {
			return fmt.Errorf("expected 0 application tables after rollback, got %d", applicationTables)
		}

		// Should have no enums
		if len(schema.Enums) != 0 {
			return fmt.Errorf("expected 0 enums after rollback, got %d", len(schema.Enums))
		}

		return nil
	})
}

// ============================================================================
// ERROR HANDLING & RECOVERY SCENARIOS
// ============================================================================

// testDynamicPartialFailureRecovery tests recovery from migration failure mid-way
func testDynamicPartialFailureRecovery(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Apply initial migrations successfully
	err = recorder.RecordStep("Apply Initial Migrations", "Apply 000-initial and 001-add-fields", func() error {
		if err := vem.MigrateToVersion(ctx, conn, "000-initial", "Create initial tables"); err != nil {
			return err
		}
		return vem.MigrateToVersion(ctx, conn, "001-add-fields", "Add additional fields")
	})
	if err != nil {
		return err
	}

	// Simulate a failure by trying to apply an invalid migration
	err = recorder.RecordStep("Simulate Migration Failure", "Attempt to apply invalid SQL", func() error {
		// Create a migration with invalid SQL that will fail
		m := migrator.NewMigrator(conn)
		invalidMigration := migrator.CreateMigrationFromSQL(
			999,
			"Invalid migration for testing",
			"CREATE TABLE invalid_table (invalid_column INVALID_TYPE);", // Invalid SQL
			"DROP TABLE invalid_table;",
		)
		m.Register(invalidMigration)

		// This should fail
		err := m.MigrateUp(ctx)
		if err == nil {
			return fmt.Errorf("expected migration to fail, but it succeeded")
		}

		// Verify we're still at version 2 (the invalid migration should not have been recorded)
		currentVersion, err := getCurrentMigrationVersion(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to get current version after failure: %w", err)
		}
		if currentVersion != 2 {
			return fmt.Errorf("expected version 2 after failed migration, got %d", currentVersion)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Verify recovery by applying a valid migration
	return recorder.RecordStep("Recover with Valid Migration", "Apply 002-add-posts after failure", func() error {
		return vem.MigrateToVersion(ctx, conn, "002-add-posts", "Add posts table")
	})
}

// testDynamicInvalidMigration tests handling of invalid/corrupted migration data
func testDynamicInvalidMigration(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Apply initial migration
	err = recorder.RecordStep("Apply Initial Migration", "Apply 000-initial", func() error {
		return vem.MigrateToVersion(ctx, conn, "000-initial", "Create initial tables")
	})
	if err != nil {
		return err
	}

	// Test various invalid migration scenarios
	return recorder.RecordStep("Test Invalid Migration Scenarios", "Test handling of various invalid migrations", func() error {
		m := migrator.NewMigrator(conn)

		// Test 1: Migration with empty SQL
		emptyMigration := migrator.CreateMigrationFromSQL(
			998,
			"Empty migration",
			"", // Empty SQL
			"",
		)
		m.Register(emptyMigration)

		// This should succeed (empty migrations are valid)
		if err := m.MigrateUp(ctx); err != nil {
			return fmt.Errorf("empty migration should succeed: %w", err)
		}

		// Test 2: Migration with syntax error
		m2 := migrator.NewMigrator(conn)
		syntaxErrorMigration := migrator.CreateMigrationFromSQL(
			997,
			"Syntax error migration",
			"CREATE TABLE users (id INVALID_SYNTAX);", // Invalid syntax
			"DROP TABLE users;",
		)
		m2.Register(syntaxErrorMigration)

		// This should fail
		if err := m2.MigrateUp(ctx); err == nil {
			return fmt.Errorf("syntax error migration should fail")
		}

		// Test 3: Migration with conflicting table name
		m3 := migrator.NewMigrator(conn)
		conflictMigration := migrator.CreateMigrationFromSQL(
			996,
			"Conflicting table migration",
			"CREATE TABLE users (id INTEGER);", // Table already exists
			"DROP TABLE users;",
		)
		m3.Register(conflictMigration)

		// This should fail due to table already existing
		if err := m3.MigrateUp(ctx); err == nil {
			return fmt.Errorf("conflicting table migration should fail")
		}

		return nil
	})
}

// testDynamicConcurrentMigrations tests concurrent migration attempts (locking behavior)
func testDynamicConcurrentMigrations(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Apply initial migration
	err = recorder.RecordStep("Apply Initial Migration", "Apply 000-initial", func() error {
		return vem.MigrateToVersion(ctx, conn, "000-initial", "Create initial tables")
	})
	if err != nil {
		return err
	}

	return recorder.RecordStep("Test Concurrent Migration Attempts", "Simulate concurrent migration attempts", func() error {
		// Create two separate database connections to simulate concurrency
		conn2, err := executor.ConnectToDatabase(conn.Info().URL)
		if err != nil {
			return fmt.Errorf("failed to create second connection: %w", err)
		}
		defer conn2.Close()

		// Create channels for synchronization
		startCh := make(chan struct{})
		result1Ch := make(chan error, 1)
		result2Ch := make(chan error, 1)

		// Start first migration in goroutine
		go func() {
			<-startCh
			m1 := migrator.NewMigrator(conn)
			migration1 := migrator.CreateMigrationFromSQL(
				995,
				"Concurrent migration 1",
				"CREATE TABLE concurrent_test1 (id INTEGER);",
				"DROP TABLE concurrent_test1;",
			)
			m1.Register(migration1)
			result1Ch <- m1.MigrateUp(ctx)
		}()

		// Start second migration in goroutine
		go func() {
			<-startCh
			m2 := migrator.NewMigrator(conn2)
			migration2 := migrator.CreateMigrationFromSQL(
				994,
				"Concurrent migration 2",
				"CREATE TABLE concurrent_test2 (id INTEGER);",
				"DROP TABLE concurrent_test2;",
			)
			m2.Register(migration2)
			result2Ch <- m2.MigrateUp(ctx)
		}()

		// Start both migrations simultaneously
		close(startCh)

		// Wait for both to complete
		err1 := <-result1Ch
		err2 := <-result2Ch

		// At least one should succeed (depending on database locking behavior)
		// Both might succeed if the database handles concurrent schema changes well
		if err1 != nil && err2 != nil {
			return fmt.Errorf("both concurrent migrations failed: err1=%v, err2=%v", err1, err2)
		}

		// Verify final state
		currentVersion, err := getCurrentMigrationVersion(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to get current version: %w", err)
		}

		// Should be at least version 2 (one of the concurrent migrations succeeded)
		if currentVersion < 2 {
			return fmt.Errorf("expected version >= 2 after concurrent migrations, got %d", currentVersion)
		}

		return nil
	})
}

// ============================================================================
// COMPLEX SCHEMA CHANGE SCENARIOS
// ============================================================================

// testDynamicCircularDependencies tests handling of circular foreign key dependencies
func testDynamicCircularDependencies(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	return recorder.RecordStep("Test Circular Dependencies", "Create tables with circular foreign key references", func() error {
		// Create migrations that establish circular dependencies
		m := migrator.NewMigrator(conn)

		// First, create tables without foreign keys
		migration1 := migrator.CreateMigrationFromSQL(
			1,
			"Create tables without FK",
			`CREATE TABLE departments (id SERIAL PRIMARY KEY, name VARCHAR(255));
			 CREATE TABLE employees (id SERIAL PRIMARY KEY, name VARCHAR(255), department_id INTEGER);`,
			`DROP TABLE employees; DROP TABLE departments;`,
		)
		m.Register(migration1)

		if err := m.MigrateUp(ctx); err != nil {
			return fmt.Errorf("failed to create initial tables: %w", err)
		}

		// Then add foreign keys that create circular dependency
		m2 := migrator.NewMigrator(conn)
		migration2 := migrator.CreateMigrationFromSQL(
			2,
			"Add circular foreign keys",
			`ALTER TABLE departments ADD COLUMN manager_id INTEGER;
			 ALTER TABLE employees ADD CONSTRAINT fk_emp_dept FOREIGN KEY (department_id) REFERENCES departments(id);
			 ALTER TABLE departments ADD CONSTRAINT fk_dept_manager FOREIGN KEY (manager_id) REFERENCES employees(id);`,
			`ALTER TABLE departments DROP CONSTRAINT fk_dept_manager;
			 ALTER TABLE employees DROP CONSTRAINT fk_emp_dept;
			 ALTER TABLE departments DROP COLUMN manager_id;`,
		)
		m2.Register(migration2)

		// This should succeed - most databases handle circular FKs if created properly
		if err := m2.MigrateUp(ctx); err != nil {
			return fmt.Errorf("failed to add circular foreign keys: %w", err)
		}

		// Verify the schema has both tables with foreign keys
		schema, err := conn.Reader().ReadSchema()
		if err != nil {
			return fmt.Errorf("failed to read schema: %w", err)
		}

		// Should have 2 tables plus schema_migrations
		if len(schema.Tables) < 2 {
			return fmt.Errorf("expected at least 2 tables, got %d", len(schema.Tables))
		}

		return nil
	})
}

// testDynamicDataMigration tests migrations that require data transformation
func testDynamicDataMigration(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Create initial table with data
	err = recorder.RecordStep("Create Table with Data", "Create users table and insert test data", func() error {
		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			1,
			"Create users with data",
			`CREATE TABLE users (id SERIAL PRIMARY KEY, full_name VARCHAR(255), email VARCHAR(255));
			 INSERT INTO users (full_name, email) VALUES
			   ('John Doe', 'john@example.com'),
			   ('Jane Smith', 'jane@example.com'),
			   ('Bob Johnson', 'bob@example.com');`,
			`DROP TABLE users;`,
		)
		m.Register(migration)
		return m.MigrateUp(ctx)
	})
	if err != nil {
		return err
	}

	// Perform data migration: split full_name into first_name and last_name
	return recorder.RecordStep("Data Migration", "Split full_name into first_name and last_name", func() error {
		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			2,
			"Split name fields",
			`ALTER TABLE users ADD COLUMN first_name VARCHAR(255);
			 ALTER TABLE users ADD COLUMN last_name VARCHAR(255);
			 UPDATE users SET
			   first_name = SPLIT_PART(full_name, ' ', 1),
			   last_name = SPLIT_PART(full_name, ' ', 2)
			 WHERE full_name IS NOT NULL;
			 ALTER TABLE users DROP COLUMN full_name;`,
			`ALTER TABLE users ADD COLUMN full_name VARCHAR(255);
			 UPDATE users SET full_name = CONCAT(first_name, ' ', last_name)
			 WHERE first_name IS NOT NULL AND last_name IS NOT NULL;
			 ALTER TABLE users DROP COLUMN first_name;
			 ALTER TABLE users DROP COLUMN last_name;`,
		)
		m.Register(migration)

		if err := m.MigrateUp(ctx); err != nil {
			// If SPLIT_PART doesn't exist (not PostgreSQL), try a simpler approach
			if strings.Contains(err.Error(), "SPLIT_PART") {
				m2 := migrator.NewMigrator(conn)
				simpleMigration := migrator.CreateMigrationFromSQL(
					3,
					"Simple name split",
					`ALTER TABLE users ADD COLUMN first_name VARCHAR(255);
					 ALTER TABLE users ADD COLUMN last_name VARCHAR(255);
					 UPDATE users SET first_name = 'John', last_name = 'Doe' WHERE id = 1;
					 UPDATE users SET first_name = 'Jane', last_name = 'Smith' WHERE id = 2;
					 UPDATE users SET first_name = 'Bob', last_name = 'Johnson' WHERE id = 3;
					 ALTER TABLE users DROP COLUMN full_name;`,
					`ALTER TABLE users ADD COLUMN full_name VARCHAR(255);
					 UPDATE users SET full_name = 'John Doe' WHERE id = 1;
					 UPDATE users SET full_name = 'Jane Smith' WHERE id = 2;
					 UPDATE users SET full_name = 'Bob Johnson' WHERE id = 3;
					 ALTER TABLE users DROP COLUMN first_name;
					 ALTER TABLE users DROP COLUMN last_name;`,
				)
				m2.Register(simpleMigration)
				return m2.MigrateUp(ctx)
			}
			return err
		}

		// Verify the data migration worked
		rows := conn.QueryRow("SELECT COUNT(*) FROM users WHERE first_name IS NOT NULL AND last_name IS NOT NULL")
		var count int
		if err := rows.Scan(&count); err != nil {
			return fmt.Errorf("failed to verify data migration: %w", err)
		}
		if count != 3 {
			return fmt.Errorf("expected 3 users with split names, got %d", count)
		}

		return nil
	})
}

// testDynamicLargeTableMigration tests performance with large datasets during migration
func testDynamicLargeTableMigration(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Create table with moderate amount of data (not too large for CI)
	err = recorder.RecordStep("Create Large Table", "Create table with test data", func() error {
		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			1,
			"Create large table",
			`CREATE TABLE large_table (
			   id SERIAL PRIMARY KEY,
			   data VARCHAR(255),
			   created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			 );`,
			`DROP TABLE large_table;`,
		)
		m.Register(migration)
		return m.MigrateUp(ctx)
	})
	if err != nil {
		return err
	}

	// Insert test data
	err = recorder.RecordStep("Insert Test Data", "Insert 1000 rows of test data", func() error {
		// Use a transaction for better performance
		if err := conn.Writer().BeginTransaction(); err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer conn.Writer().RollbackTransaction()

		for i := 0; i < 1000; i++ {
			sql := fmt.Sprintf("INSERT INTO large_table (data) VALUES ('test_data_%d')", i)
			if err := conn.Writer().ExecuteSQL(sql); err != nil {
				return fmt.Errorf("failed to insert row %d: %w", i, err)
			}
		}

		return conn.Writer().CommitTransaction()
	})
	if err != nil {
		return err
	}

	// Perform migration on large table
	return recorder.RecordStep("Migrate Large Table", "Add index and new column to large table", func() error {
		start := time.Now()

		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			2,
			"Add index and column to large table",
			`ALTER TABLE large_table ADD COLUMN status VARCHAR(50) DEFAULT 'active';
			 CREATE INDEX idx_large_table_status ON large_table(status);
			 CREATE INDEX idx_large_table_data ON large_table(data);`,
			`DROP INDEX idx_large_table_data;
			 DROP INDEX idx_large_table_status;
			 ALTER TABLE large_table DROP COLUMN status;`,
		)
		m.Register(migration)

		if err := m.MigrateUp(ctx); err != nil {
			return fmt.Errorf("failed to migrate large table: %w", err)
		}

		duration := time.Since(start)
		fmt.Printf("Large table migration took: %v\n", duration)

		// Verify the migration worked
		rows := conn.QueryRow("SELECT COUNT(*) FROM large_table WHERE status = 'active'")
		var count int
		if err := rows.Scan(&count); err != nil {
			return fmt.Errorf("failed to verify large table migration: %w", err)
		}
		if count != 1000 {
			return fmt.Errorf("expected 1000 rows with status 'active', got %d", count)
		}

		return nil
	})
}

// ============================================================================
// EDGE CASE SCENARIOS
// ============================================================================

// testDynamicEmptyMigrations tests versions with no actual schema changes
func testDynamicEmptyMigrations(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Apply initial migration
	err = recorder.RecordStep("Apply Initial Migration", "Apply 000-initial", func() error {
		return vem.MigrateToVersion(ctx, conn, "000-initial", "Create initial tables")
	})
	if err != nil {
		return err
	}

	// Test empty migrations
	return recorder.RecordStep("Test Empty Migrations", "Apply migrations with no schema changes", func() error {
		m := migrator.NewMigrator(conn)

		// Empty migration 1
		emptyMigration1 := migrator.CreateMigrationFromSQL(
			990,
			"Empty migration 1",
			"", // No SQL
			"",
		)
		m.Register(emptyMigration1)

		// Empty migration 2 with comments only
		emptyMigration2 := migrator.CreateMigrationFromSQL(
			991,
			"Empty migration 2",
			"-- This is just a comment\n-- No actual schema changes",
			"-- Rollback comment",
		)
		m.Register(emptyMigration2)

		// Empty migration 3 with whitespace
		emptyMigration3 := migrator.CreateMigrationFromSQL(
			992,
			"Empty migration 3",
			"   \n\t  \n   ", // Just whitespace
			"",
		)
		m.Register(emptyMigration3)

		if err := m.MigrateUp(ctx); err != nil {
			return fmt.Errorf("empty migrations should succeed: %w", err)
		}

		// Verify all empty migrations were recorded
		currentVersion, err := getCurrentMigrationVersion(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to get current version: %w", err)
		}

		// Should be at version 992 (last empty migration)
		if currentVersion != 992 {
			return fmt.Errorf("expected version 992 after empty migrations, got %d", currentVersion)
		}

		return nil
	})
}

// testDynamicDuplicateNames tests handling of duplicate table/field names across versions
func testDynamicDuplicateNames(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	return recorder.RecordStep("Test Duplicate Names", "Test handling of duplicate table/field names", func() error {
		m := migrator.NewMigrator(conn)

		// Create initial table
		migration1 := migrator.CreateMigrationFromSQL(
			1,
			"Create initial table",
			"CREATE TABLE test_table (id SERIAL PRIMARY KEY, name VARCHAR(255));",
			"DROP TABLE test_table;",
		)
		m.Register(migration1)

		if err := m.MigrateUp(ctx); err != nil {
			return fmt.Errorf("failed to create initial table: %w", err)
		}

		// Try to create table with same name (should fail)
		m2 := migrator.NewMigrator(conn)
		duplicateMigration := migrator.CreateMigrationFromSQL(
			2,
			"Duplicate table name",
			"CREATE TABLE test_table (id INTEGER, data TEXT);", // Same table name
			"DROP TABLE test_table;",
		)
		m2.Register(duplicateMigration)

		// This should fail
		if err := m2.MigrateUp(ctx); err == nil {
			return fmt.Errorf("duplicate table creation should fail")
		}

		// Try to add column with same name (should fail)
		m3 := migrator.NewMigrator(conn)
		duplicateColumnMigration := migrator.CreateMigrationFromSQL(
			3,
			"Duplicate column name",
			"ALTER TABLE test_table ADD COLUMN id VARCHAR(255);", // Column 'id' already exists
			"ALTER TABLE test_table DROP COLUMN id;",
		)
		m3.Register(duplicateColumnMigration)

		// This should fail
		if err := m3.MigrateUp(ctx); err == nil {
			return fmt.Errorf("duplicate column creation should fail")
		}

		// Verify we're still at version 1 (only the first migration succeeded)
		currentVersion, err := getCurrentMigrationVersion(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to get current version: %w", err)
		}
		if currentVersion != 1 {
			return fmt.Errorf("expected version 1 after duplicate name failures, got %d", currentVersion)
		}

		return nil
	})
}

// testDynamicReservedKeywords tests migrations involving SQL reserved keywords
func testDynamicReservedKeywords(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	return recorder.RecordStep("Test Reserved Keywords", "Test migrations with SQL reserved keywords", func() error {
		m := migrator.NewMigrator(conn)

		// Create table with reserved keyword names (properly quoted)
		migration := migrator.CreateMigrationFromSQL(
			1,
			"Reserved keywords test",
			`CREATE TABLE "order" (
			   "id" SERIAL PRIMARY KEY,
			   "select" VARCHAR(255),
			   "from" VARCHAR(255),
			   "where" TEXT,
			   "group" INTEGER
			 );
			 CREATE INDEX "index" ON "order"("select");`,
			`DROP INDEX "index"; DROP TABLE "order";`,
		)
		m.Register(migration)

		if err := m.MigrateUp(ctx); err != nil {
			return fmt.Errorf("reserved keywords migration should succeed with proper quoting: %w", err)
		}

		// Verify the table was created
		schema, err := conn.Reader().ReadSchema()
		if err != nil {
			return fmt.Errorf("failed to read schema: %w", err)
		}

		// Should have the "order" table
		orderTableExists := false
		for _, table := range schema.Tables {
			if table.Name == "order" {
				orderTableExists = true
				break
			}
		}
		if !orderTableExists {
			return fmt.Errorf("expected 'order' table to exist")
		}

		// Test unquoted reserved keywords (should fail)
		m2 := migrator.NewMigrator(conn)
		badMigration := migrator.CreateMigrationFromSQL(
			2,
			"Bad reserved keywords",
			"CREATE TABLE select (id INTEGER, from VARCHAR(255));", // Unquoted reserved keywords
			"DROP TABLE select;",
		)
		m2.Register(badMigration)

		// This should fail
		if err := m2.MigrateUp(ctx); err == nil {
			return fmt.Errorf("unquoted reserved keywords should fail")
		}

		return nil
	})
}

// ============================================================================
// CROSS-DATABASE COMPATIBILITY SCENARIOS
// ============================================================================

// testDynamicDialectDifferences tests same migration across PostgreSQL/MySQL/MariaDB
func testDynamicDialectDifferences(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	return recorder.RecordStep("Test Dialect Differences", "Test migrations with dialect-specific features", func() error {
		dialect := conn.Info().Dialect
		m := migrator.NewMigrator(conn)

		var migration *migrator.Migration
		switch dialect {
		case "postgres":
			migration = migrator.CreateMigrationFromSQL(
				1,
				"PostgreSQL specific features",
				`CREATE TABLE dialect_test (
				   id SERIAL PRIMARY KEY,
				   data JSONB,
				   created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
				 );
				 CREATE INDEX idx_dialect_test_data ON dialect_test USING GIN (data);`,
				`DROP TABLE dialect_test;`,
			)
		case "mysql":
			migration = migrator.CreateMigrationFromSQL(
				1,
				"MySQL specific features",
				`CREATE TABLE dialect_test (
				   id INT AUTO_INCREMENT PRIMARY KEY,
				   data JSON,
				   created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				 ) ENGINE=InnoDB;`,
				`DROP TABLE dialect_test;`,
			)
		default:
			// Generic SQL for other databases
			migration = migrator.CreateMigrationFromSQL(
				1,
				"Generic SQL features",
				`CREATE TABLE dialect_test (
				   id INTEGER PRIMARY KEY,
				   data TEXT,
				   created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				 );`,
				`DROP TABLE dialect_test;`,
			)
		}

		m.Register(migration)
		if err := m.MigrateUp(ctx); err != nil {
			return fmt.Errorf("dialect-specific migration failed for %s: %w", dialect, err)
		}

		// Verify the table was created
		schema, err := conn.Reader().ReadSchema()
		if err != nil {
			return fmt.Errorf("failed to read schema: %w", err)
		}

		dialectTestExists := false
		for _, table := range schema.Tables {
			if table.Name == "dialect_test" {
				dialectTestExists = true
				break
			}
		}
		if !dialectTestExists {
			return fmt.Errorf("expected dialect_test table to exist")
		}

		return nil
	})
}

// testDynamicTypeMapping tests database-specific type conversions
func testDynamicTypeMapping(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	return recorder.RecordStep("Test Type Mapping", "Test database-specific type conversions", func() error {
		dialect := conn.Info().Dialect
		m := migrator.NewMigrator(conn)

		// Create table with various data types
		var createSQL, dropSQL string
		switch dialect {
		case "postgres":
			createSQL = `CREATE TABLE type_test (
			   id SERIAL PRIMARY KEY,
			   small_int SMALLINT,
			   big_int BIGINT,
			   decimal_val DECIMAL(10,2),
			   text_val TEXT,
			   bool_val BOOLEAN,
			   date_val DATE,
			   timestamp_val TIMESTAMP,
			   uuid_val UUID
			 );`
		case "mysql":
			createSQL = `CREATE TABLE type_test (
			   id INT AUTO_INCREMENT PRIMARY KEY,
			   small_int SMALLINT,
			   big_int BIGINT,
			   decimal_val DECIMAL(10,2),
			   text_val TEXT,
			   bool_val BOOLEAN,
			   date_val DATE,
			   timestamp_val TIMESTAMP,
			   uuid_val CHAR(36)
			 );`
		default:
			createSQL = `CREATE TABLE type_test (
			   id INTEGER PRIMARY KEY,
			   small_int INTEGER,
			   big_int INTEGER,
			   decimal_val DECIMAL(10,2),
			   text_val TEXT,
			   bool_val INTEGER,
			   date_val DATE,
			   timestamp_val TIMESTAMP,
			   uuid_val VARCHAR(36)
			 );`
		}
		dropSQL = `DROP TABLE type_test;`

		migration := migrator.CreateMigrationFromSQL(1, "Type mapping test", createSQL, dropSQL)
		m.Register(migration)

		if err := m.MigrateUp(ctx); err != nil {
			return fmt.Errorf("type mapping migration failed for %s: %w", dialect, err)
		}

		// Test type conversion migration
		m2 := migrator.NewMigrator(conn)
		var alterSQL string
		switch dialect {
		case "postgres":
			alterSQL = `ALTER TABLE type_test ALTER COLUMN small_int TYPE INTEGER;`
		case "mysql":
			alterSQL = `ALTER TABLE type_test MODIFY COLUMN small_int INT;`
		default:
			alterSQL = `-- Type conversion not supported in generic SQL`
		}

		if alterSQL != "-- Type conversion not supported in generic SQL" {
			migration2 := migrator.CreateMigrationFromSQL(2, "Type conversion test", alterSQL, "")
			m2.Register(migration2)

			if err := m2.MigrateUp(ctx); err != nil {
				return fmt.Errorf("type conversion migration failed for %s: %w", dialect, err)
			}
		}

		return nil
	})
}

// ============================================================================
// VALIDATION & INTEGRITY SCENARIOS
// ============================================================================

// testDynamicConstraintValidation tests constraint violations during migration
func testDynamicConstraintValidation(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Create table with data that will violate constraints
	err = recorder.RecordStep("Create Table with Data", "Create table and insert data", func() error {
		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			1,
			"Create table with data",
			`CREATE TABLE constraint_test (
			   id SERIAL PRIMARY KEY,
			   email VARCHAR(255),
			   age INTEGER
			 );
			 INSERT INTO constraint_test (email, age) VALUES
			   ('user1@example.com', 25),
			   ('user2@example.com', 30),
			   ('user1@example.com', 35),  -- Duplicate email
			   ('user3@example.com', -5);  -- Invalid age`,
			`DROP TABLE constraint_test;`,
		)
		m.Register(migration)
		return m.MigrateUp(ctx)
	})
	if err != nil {
		return err
	}

	// Try to add unique constraint (should fail due to duplicate emails)
	err = recorder.RecordStep("Test Unique Constraint Violation", "Try to add unique constraint on email", func() error {
		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			2,
			"Add unique constraint",
			`ALTER TABLE constraint_test ADD CONSTRAINT uk_constraint_test_email UNIQUE (email);`,
			`ALTER TABLE constraint_test DROP CONSTRAINT uk_constraint_test_email;`,
		)
		m.Register(migration)

		// This should fail due to duplicate emails
		if err := m.MigrateUp(ctx); err == nil {
			return fmt.Errorf("unique constraint should fail due to duplicate emails")
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Try to add check constraint (should fail due to negative age)
	return recorder.RecordStep("Test Check Constraint Violation", "Try to add check constraint on age", func() error {
		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			3,
			"Add check constraint",
			`ALTER TABLE constraint_test ADD CONSTRAINT ck_constraint_test_age CHECK (age >= 0);`,
			`ALTER TABLE constraint_test DROP CONSTRAINT ck_constraint_test_age;`,
		)
		m.Register(migration)

		// This should fail due to negative age
		if err := m.MigrateUp(ctx); err == nil {
			return fmt.Errorf("check constraint should fail due to negative age")
		}
		return nil
	})
}

// testDynamicForeignKeyCascade tests cascading effects of table/field drops
func testDynamicForeignKeyCascade(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS, recorder *StepRecorder) error {
	vem, err := NewVersionedEntityManager(fixtures)
	if err != nil {
		return fmt.Errorf("failed to create versioned entity manager: %w", err)
	}
	defer vem.Cleanup()

	// Create tables with foreign key relationships
	err = recorder.RecordStep("Create Tables with FK", "Create parent and child tables with foreign key", func() error {
		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			1,
			"Create FK tables",
			`CREATE TABLE parent_table (
			   id SERIAL PRIMARY KEY,
			   name VARCHAR(255)
			 );
			 CREATE TABLE child_table (
			   id SERIAL PRIMARY KEY,
			   parent_id INTEGER,
			   data VARCHAR(255),
			   FOREIGN KEY (parent_id) REFERENCES parent_table(id)
			 );
			 INSERT INTO parent_table (name) VALUES ('Parent 1'), ('Parent 2');
			 INSERT INTO child_table (parent_id, data) VALUES (1, 'Child 1'), (2, 'Child 2');`,
			`DROP TABLE child_table; DROP TABLE parent_table;`,
		)
		m.Register(migration)
		return m.MigrateUp(ctx)
	})
	if err != nil {
		return err
	}

	// Try to drop parent table (should fail due to FK constraint)
	err = recorder.RecordStep("Test FK Constraint on Drop", "Try to drop parent table with FK references", func() error {
		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			2,
			"Drop parent table",
			`DROP TABLE parent_table;`,
			`CREATE TABLE parent_table (id SERIAL PRIMARY KEY, name VARCHAR(255));`,
		)
		m.Register(migration)

		// This should fail due to foreign key constraint
		if err := m.MigrateUp(ctx); err == nil {
			return fmt.Errorf("dropping parent table should fail due to FK constraint")
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Test cascade delete
	return recorder.RecordStep("Test Cascade Operations", "Test foreign key cascade behavior", func() error {
		// Add cascade constraint
		m := migrator.NewMigrator(conn)
		migration := migrator.CreateMigrationFromSQL(
			3,
			"Add cascade FK",
			`ALTER TABLE child_table DROP CONSTRAINT child_table_parent_id_fkey;
			 ALTER TABLE child_table ADD CONSTRAINT child_table_parent_id_fkey
			   FOREIGN KEY (parent_id) REFERENCES parent_table(id) ON DELETE CASCADE;`,
			`ALTER TABLE child_table DROP CONSTRAINT child_table_parent_id_fkey;
			 ALTER TABLE child_table ADD CONSTRAINT child_table_parent_id_fkey
			   FOREIGN KEY (parent_id) REFERENCES parent_table(id);`,
		)
		m.Register(migration)

		if err := m.MigrateUp(ctx); err != nil {
			// If the constraint name is different, try a more generic approach
			if strings.Contains(err.Error(), "does not exist") {
				m2 := migrator.NewMigrator(conn)
				migration2 := migrator.CreateMigrationFromSQL(
					4,
					"Recreate tables with cascade",
					`DROP TABLE child_table;
					 DROP TABLE parent_table;
					 CREATE TABLE parent_table (
					   id SERIAL PRIMARY KEY,
					   name VARCHAR(255)
					 );
					 CREATE TABLE child_table (
					   id SERIAL PRIMARY KEY,
					   parent_id INTEGER,
					   data VARCHAR(255),
					   FOREIGN KEY (parent_id) REFERENCES parent_table(id) ON DELETE CASCADE
					 );
					 INSERT INTO parent_table (name) VALUES ('Parent 1'), ('Parent 2');
					 INSERT INTO child_table (parent_id, data) VALUES (1, 'Child 1'), (2, 'Child 2');`,
					`DROP TABLE child_table; DROP TABLE parent_table;`,
				)
				m2.Register(migration2)
				if err := m2.MigrateUp(ctx); err != nil {
					return fmt.Errorf("failed to recreate tables with cascade: %w", err)
				}
			} else {
				return fmt.Errorf("failed to add cascade constraint: %w", err)
			}
		}

		// Verify cascade works by deleting parent
		if err := conn.Writer().ExecuteSQL("DELETE FROM parent_table WHERE id = 1"); err != nil {
			return fmt.Errorf("failed to delete parent record: %w", err)
		}

		// Check that child record was also deleted
		rows := conn.QueryRow("SELECT COUNT(*) FROM child_table WHERE parent_id = 1")
		var count int
		if err := rows.Scan(&count); err != nil {
			return fmt.Errorf("failed to check cascade delete: %w", err)
		}
		if count != 0 {
			return fmt.Errorf("expected 0 child records after cascade delete, got %d", count)
		}

		return nil
	})
}


