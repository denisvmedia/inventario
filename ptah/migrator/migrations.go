package migrator

import (
	"context"
	_ "embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/denisvmedia/inventario/ptah/core/sqlutil"
	"github.com/denisvmedia/inventario/ptah/executor"
)

//go:embed base/schema.sql
var migrationsSchemaSQL string

//go:embed base/get_version.sql
var getVersionSQL string

//go:embed base/record_migration.sql
var recordMigrationSQL string

//go:embed base/delete_migration.sql
var deleteMigrationSQL string

// MigrationFunc represents a migration function that operates on a database connection
type MigrationFunc func(context.Context, *executor.DatabaseConnection) error

// SplitSQLStatements splits a SQL string into individual statements using AST-based parsing.
// This is needed because MySQL doesn't handle multiple statements in a single ExecuteSQL call.
// Unlike simple string splitting, this properly handles semicolons within string literals and comments.
func SplitSQLStatements(sql string) []string {
	return sqlutil.SplitSQLStatements(sqlutil.StripComments(sql))
}

// MigrationFuncFromSQLFilename returns a migration function that reads SQL from a file
// in the provided filesystem and executes it using the database connection
func MigrationFuncFromSQLFilename(filename string, fsys fs.FS) MigrationFunc {
	return func(ctx context.Context, conn *executor.DatabaseConnection) error {
		sql, err := fs.ReadFile(fsys, filename)
		if err != nil {
			return fmt.Errorf("failed to read migration file: %w", err)
		}

		// Split SQL into individual statements for better MySQL compatibility
		statements := SplitSQLStatements(string(sql))

		// Execute each statement separately
		for _, stmt := range statements {
			if err := conn.Writer().ExecuteSQL(stmt); err != nil {
				return fmt.Errorf("failed to execute migration SQL: %w", err)
			}
		}

		return nil
	}
}

// NoopMigrationFunc is a no-op migration function
func NoopMigrationFunc(_ctx context.Context, _conn *executor.DatabaseConnection) error {
	return nil
}

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          MigrationFunc
	Down        MigrationFunc
}

// Migrator handles database migrations for ptah
type Migrator struct {
	conn        *executor.DatabaseConnection
	migrations  []*Migration
	initialized bool
}

// NewMigrator creates a new migrator with the given database connection
func NewMigrator(conn *executor.DatabaseConnection) *Migrator {
	return &Migrator{
		conn:       conn,
		migrations: make([]*Migration, 0),
	}
}

// Register registers a migration with the migrator
func (m *Migrator) Register(migration *Migration) {
	m.migrations = append(m.migrations, migration)
}

// sortMigrations sorts migrations by version in ascending order
func (m *Migrator) sortMigrations() {
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})
}

// Initialize creates the migrations table if it doesn't exist
func (m *Migrator) Initialize(_ctx context.Context) error {
	// Skip if already initialized
	if m.initialized {
		return nil
	}

	// Execute the schema creation SQL directly on the database connection
	// This avoids transaction conflicts with the PostgreSQL writer
	_, err := m.conn.Exec(migrationsSchemaSQL)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Mark as initialized
	m.initialized = true
	return nil
}

// GetCurrentVersion returns the current migration version from the database
func (m *Migrator) GetCurrentVersion(ctx context.Context) (int, error) {
	// First ensure the migrations table exists
	if err := m.Initialize(ctx); err != nil {
		return 0, fmt.Errorf("failed to initialize migrations table: %w", err)
	}

	// Query the current version
	var version int
	row := m.conn.QueryRow(getVersionSQL)
	if err := row.Scan(&version); err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}

	return version, nil
}

// MigrateUp migrates the database up to the latest version
func (m *Migrator) MigrateUp(ctx context.Context) error {
	// Initialize the migrations table
	if err := m.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize migrations table: %w", err)
	}

	// Get the current version
	currentVersion, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	fmt.Printf("Current schema version: %d\n", currentVersion)  //nolint:forbidigo // Migration progress output is intentional
	fmt.Printf("Available migrations: %d\n", len(m.migrations)) //nolint:forbidigo // Migration progress output is intentional

	// Sort migrations by version
	m.sortMigrations()

	// Apply migrations that are newer than current version
	for _, migration := range m.migrations {
		if migration.Version <= currentVersion {
			fmt.Printf("Skipping migration %d: %s (already applied)\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional
			continue
		}

		fmt.Printf("Applying migration %d: %s\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional

		// Begin transaction for this migration
		if err := m.conn.Writer().BeginTransaction(); err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		// Apply migration
		if err := migration.Up(ctx, m.conn); err != nil {
			_ = m.conn.Writer().RollbackTransaction()
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}

		// Record migration
		timestamp := FormatTimestampForDatabase(m.conn.Info().Dialect)
		recordSQL := fmt.Sprintf(recordMigrationSQL, migration.Version, migration.Description, timestamp)
		if err := m.conn.Writer().ExecuteSQL(recordSQL); err != nil {
			_ = m.conn.Writer().RollbackTransaction()
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		// Commit transaction
		if err := m.conn.Writer().CommitTransaction(); err != nil {
			return fmt.Errorf("failed to commit transaction for migration %d: %w", migration.Version, err)
		}

		fmt.Printf("Applied migration %d: %s\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional
	}

	fmt.Println("All migrations applied successfully") //nolint:forbidigo // Migration progress output is intentional
	return nil
}

// MigrateDown migrates the database down to the specified target version
func (m *Migrator) MigrateDown(ctx context.Context, targetVersion int) error {
	// Initialize the migrations table
	if err := m.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize migrations table: %w", err)
	}

	// Get the current version
	currentVersion, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if targetVersion >= currentVersion {
		fmt.Printf("Target version %d is not less than current version %d\n", targetVersion, currentVersion) //nolint:forbidigo // Migration progress output is intentional
		return nil
	}

	fmt.Printf("Current schema version: %d\n", currentVersion)  //nolint:forbidigo // Migration progress output is intentional
	fmt.Printf("Target schema version: %d\n", targetVersion)    //nolint:forbidigo // Migration progress output is intentional
	fmt.Printf("Available migrations: %d\n", len(m.migrations)) //nolint:forbidigo // Migration progress output is intentional

	// Sort migrations by version in descending order for rollback
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version > m.migrations[j].Version
	})

	// Apply down migrations for versions greater than target
	for _, migration := range m.migrations {
		if migration.Version <= targetVersion || migration.Version > currentVersion {
			continue
		}

		fmt.Printf("Rolling back migration %d: %s\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional

		// Begin transaction for this migration rollback
		if err := m.conn.Writer().BeginTransaction(); err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		// Apply down migration
		if err := migration.Down(ctx, m.conn); err != nil {
			_ = m.conn.Writer().RollbackTransaction()
			return fmt.Errorf("failed to revert migration %d: %w", migration.Version, err)
		}

		// Remove migration record
		deleteSQL := fmt.Sprintf(deleteMigrationSQL, migration.Version)
		if err := m.conn.Writer().ExecuteSQL(deleteSQL); err != nil {
			_ = m.conn.Writer().RollbackTransaction()
			return fmt.Errorf("failed to record migration reversion %d: %w", migration.Version, err)
		}

		// Commit transaction
		if err := m.conn.Writer().CommitTransaction(); err != nil {
			return fmt.Errorf("failed to commit transaction for migration %d: %w", migration.Version, err)
		}

		fmt.Printf("Rolled back migration %d: %s\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional
	}

	fmt.Printf("Successfully migrated down to version %d\n", targetVersion) //nolint:forbidigo // Migration progress output is intentional
	return nil
}

// MigrateTo migrates the database to a specific version (up or down)
func (m *Migrator) MigrateTo(ctx context.Context, targetVersion int) error {
	// Initialize the migrations table
	if err := m.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize migrations table: %w", err)
	}

	// Get the current version
	currentVersion, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if targetVersion == currentVersion {
		fmt.Printf("Already at target version %d\n", targetVersion) //nolint:forbidigo // Migration progress output is intentional
		return nil
	}

	if targetVersion > currentVersion {
		// Migrate up to target version
		return m.migrateUpTo(ctx, targetVersion)
	} else {
		// Migrate down to target version
		return m.MigrateDown(ctx, targetVersion)
	}
}

// migrateUpTo migrates the database up to a specific version
func (m *Migrator) migrateUpTo(ctx context.Context, targetVersion int) error {
	// Sort migrations by version
	m.sortMigrations()

	// Get the current version
	currentVersion, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	fmt.Printf("Migrating from version %d to %d\n", currentVersion, targetVersion) //nolint:forbidigo // Migration progress output is intentional

	// Apply migrations up to target version
	for _, migration := range m.migrations {
		if migration.Version <= currentVersion || migration.Version > targetVersion {
			continue
		}

		fmt.Printf("Applying migration %d: %s\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional

		// Begin transaction for this migration
		if err := m.conn.Writer().BeginTransaction(); err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		// Apply migration
		if err := migration.Up(ctx, m.conn); err != nil {
			_ = m.conn.Writer().RollbackTransaction()
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}

		// Record migration
		timestamp := FormatTimestampForDatabase(m.conn.Info().Dialect)
		recordSQL := fmt.Sprintf(recordMigrationSQL, migration.Version, migration.Description, timestamp)
		if err := m.conn.Writer().ExecuteSQL(recordSQL); err != nil {
			_ = m.conn.Writer().RollbackTransaction()
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		// Commit transaction
		if err := m.conn.Writer().CommitTransaction(); err != nil {
			return fmt.Errorf("failed to commit transaction for migration %d: %w", migration.Version, err)
		}

		fmt.Printf("Applied migration %d: %s\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional
	}

	fmt.Printf("Successfully migrated to version %d\n", targetVersion) //nolint:forbidigo // Migration progress output is intentional
	return nil
}

// GetAppliedMigrations returns a list of applied migration versions
func (m *Migrator) GetAppliedMigrations(ctx context.Context) ([]int, error) {
	// First ensure the migrations table exists
	if err := m.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize migrations table: %w", err)
	}

	// Query all applied migration versions
	rows, err := m.conn.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	var applied []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied = append(applied, version)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migration rows: %w", err)
	}

	return applied, nil
}

// GetPendingMigrations returns a list of pending migration versions
func (m *Migrator) GetPendingMigrations(ctx context.Context) ([]int, error) {
	currentVersion, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return nil, err
	}

	var pending []int
	for _, migration := range m.migrations {
		if migration.Version > currentVersion {
			pending = append(pending, migration.Version)
		}
	}

	sort.Ints(pending)
	return pending, nil
}
