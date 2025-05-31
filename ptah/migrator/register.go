package migrator

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/denisvmedia/inventario/ptah/executor"
)

// RegisterMigrations registers all migrations with the migrator by scanning
// the provided filesystem for SQL files. The filesystem should have migrations
// in the root directory (it's the caller's responsibility to prepare it).
func RegisterMigrations(migrator *Migrator, migrationsFS fs.FS) error {
	migrationsMap := make(map[int]*Migration) // version -> migration

	err := fs.WalkDir(migrationsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Parse migration filename
		migrationFile, err := ParseMigrationFileName(d.Name())
		if err != nil {
			// Skip files that don't match migration pattern
			return nil
		}

		// Initialize migration if it doesn't exist
		if _, exists := migrationsMap[migrationFile.Version]; !exists {
			migrationsMap[migrationFile.Version] = &Migration{
				Version:     migrationFile.Version,
				Description: migrationFile.Name,
				Up:          NoopMigrationFunc,
				Down:        NoopMigrationFunc,
			}
		}

		// Set the appropriate migration function based on direction
		switch migrationFile.Direction {
		case "up":
			migrationsMap[migrationFile.Version].Up = MigrationFuncFromSQLFilename(path, migrationsFS)
		case "down":
			migrationsMap[migrationFile.Version].Down = MigrationFuncFromSQLFilename(path, migrationsFS)
		default:
			return fmt.Errorf("invalid migration direction: %s", migrationFile.Direction)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan migrations directory: %w", err)
	}

	// Validate that all migrations have both up and down functions
	var incompleteMigrations []int
	for version, migration := range migrationsMap {
		// Check if both up and down are still noop (meaning files weren't found)
		if migration.Up == nil || migration.Down == nil {
			incompleteMigrations = append(incompleteMigrations, version)
		}
	}

	if len(incompleteMigrations) > 0 {
		return fmt.Errorf("incomplete migrations found (missing up or down files): %v", incompleteMigrations)
	}

	// Register all migrations with the migrator
	// The migrator will sort them by version, so we don't need to sort them here
	for _, migration := range migrationsMap {
		migrator.Register(migration)
	}

	return nil
}

// RunMigrations runs all pending migrations up to the latest version using the provided filesystem
func RunMigrations(ctx context.Context, conn *executor.DatabaseConnection, migrationsFS fs.FS) error {
	migrator := NewMigrator(conn)

	if err := RegisterMigrations(migrator, migrationsFS); err != nil {
		return fmt.Errorf("failed to register migrations: %w", err)
	}

	if err := migrator.MigrateUp(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// RunMigrationsDown runs down migrations to the specified target version using the provided filesystem
func RunMigrationsDown(ctx context.Context, conn *executor.DatabaseConnection, targetVersion int, migrationsFS fs.FS) error {
	migrator := NewMigrator(conn)

	if err := RegisterMigrations(migrator, migrationsFS); err != nil {
		return fmt.Errorf("failed to register migrations: %w", err)
	}

	if err := migrator.MigrateDown(ctx, targetVersion); err != nil {
		return fmt.Errorf("failed to run down migrations: %w", err)
	}

	return nil
}

// GetMigrationStatus returns information about the current migration status using the provided filesystem
func GetMigrationStatus(ctx context.Context, conn *executor.DatabaseConnection, migrationsFS fs.FS) (*MigrationStatus, error) {
	migrator := NewMigrator(conn)

	if err := RegisterMigrations(migrator, migrationsFS); err != nil {
		return nil, fmt.Errorf("failed to register migrations: %w", err)
	}

	currentVersion, err := migrator.GetCurrentVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	pendingMigrations, err := migrator.GetPendingMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending migrations: %w", err)
	}

	return &MigrationStatus{
		CurrentVersion:    currentVersion,
		PendingMigrations: pendingMigrations,
		TotalMigrations:   len(migrator.migrations),
		HasPendingChanges: len(pendingMigrations) > 0,
	}, nil
}

// MigrationStatus represents the current state of migrations
type MigrationStatus struct {
	CurrentVersion    int   `json:"current_version"`
	PendingMigrations []int `json:"pending_migrations"`
	TotalMigrations   int   `json:"total_migrations"`
	HasPendingChanges bool  `json:"has_pending_changes"`
}

// RegisterGoMigration allows registering a migration defined in Go code
// rather than SQL files. This is useful for complex data migrations.
func RegisterGoMigration(migrator *Migrator, version int, description string, up, down MigrationFunc) {
	migration := &Migration{
		Version:     version,
		Description: description,
		Up:          up,
		Down:        down,
	}
	migrator.Register(migration)
}

// CreateMigrationFromSQL creates a migration from SQL strings
// This is useful for programmatically creating migrations
func CreateMigrationFromSQL(version int, description, upSQL, downSQL string) *Migration {
	upFunc := func(ctx context.Context, conn *executor.DatabaseConnection) error {
		return executeSQLStatements(conn, upSQL)
	}

	downFunc := func(ctx context.Context, conn *executor.DatabaseConnection) error {
		return executeSQLStatements(conn, downSQL)
	}

	return &Migration{
		Version:     version,
		Description: description,
		Up:          upFunc,
		Down:        downFunc,
	}
}

// executeSQLStatements splits SQL into individual statements and executes them
func executeSQLStatements(conn *executor.DatabaseConnection, sql string) error {
	// Split SQL by semicolons and execute each statement
	statements := SplitSQLStatements(sql)

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue // Skip empty statements and comments
		}

		_, err := conn.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to execute SQL statement: %w\nSQL: %s", err, stmt)
		}
	}

	return nil
}

// ValidateMigrations validates that all registered migrations are complete
// and that there are no gaps in version numbers
func ValidateMigrations(migrator *Migrator) error {
	if len(migrator.migrations) == 0 {
		return nil // No migrations to validate
	}

	// Check for missing up or down functions
	for _, migration := range migrator.migrations {
		if migration.Up == nil {
			return fmt.Errorf("migration %d is missing up function", migration.Version)
		}
		if migration.Down == nil {
			return fmt.Errorf("migration %d is missing down function", migration.Version)
		}
	}

	// Extract versions and check for duplicates
	versions := make([]int, len(migrator.migrations))
	versionSet := make(map[int]bool)

	for i, migration := range migrator.migrations {
		if versionSet[migration.Version] {
			return fmt.Errorf("duplicate migration version: %d", migration.Version)
		}
		versions[i] = migration.Version
		versionSet[migration.Version] = true
	}

	// Check for gaps (optional - you might want to allow gaps)
	gaps := FindMigrationGaps(versions)
	if len(gaps) > 0 {
		// This is just a warning - gaps might be intentional
		// You could make this an error if you want strict version sequences
		for _, gap := range gaps {
			fmt.Printf("Warning: Gap in migration versions at %d\n", gap) //nolint:forbidigo // Migration validation output is intentional
		}
	}

	return nil
}

// RegisterMigrationsFromDirectory registers migrations from a directory on disk
func RegisterMigrationsFromDirectory(migrator *Migrator, dirPath string) error {
	migrationsFS := os.DirFS(dirPath)
	return RegisterMigrations(migrator, migrationsFS)
}
