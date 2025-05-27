package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisvmedia/inventario/internal/errkit"
)

var (
	//go:embed base/schema.sql
	migrationsSchemaSQL string

	//go:embed base/get_version.sql
	getVersionSQL string

	//go:embed base/record_migration.sql
	recordMigrationSQL string

	//go:embed base/delete_migration.sql
	deleteMigrationSQL string
)

//go:embed source
var source embed.FS

func GetMigrations() embed.FS {
	return source
}

// MigrationFunc represents a migration function.
type MigrationFunc func(context.Context, pgx.Tx) error

// MigrationFuncFromSQLFilename returns a migration function that reads the SQL from a file
// in the provided filesystem and executes it.
func MigrationFuncFromSQLFilename(filename string, fsys fs.FS) MigrationFunc {
	return func(ctx context.Context, tx pgx.Tx) error {
		sql, err := fs.ReadFile(fsys, filename)
		if err != nil {
			return errkit.Wrap(err, "failed to read migration file")
		}
		_, err = tx.Exec(ctx, string(sql))
		return err
	}
}

// NoopMigrationFunc is a no-op migration function.
func NoopMigrationFunc(_ctx context.Context, _tx pgx.Tx) error {
	return nil
}

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          MigrationFunc
	Down        MigrationFunc
}

// Migrator handles database migrations
type Migrator struct {
	pool       *pgxpool.Pool
	migrations []*Migration
}

// NewMigrator creates a new migrator
func NewMigrator(pool *pgxpool.Pool) *Migrator {
	return &Migrator{
		pool:       pool,
		migrations: make([]*Migration, 0),
	}
}

// Register registers a migration
func (m *Migrator) Register(migration *Migration) {
	m.migrations = append(m.migrations, migration)
}

// Initialize initializes the migrations table if it doesn't exist
func (m *Migrator) Initialize(ctx context.Context) error {
	_, err := m.pool.Exec(ctx, migrationsSchemaSQL)
	if err != nil {
		return errkit.Wrap(err, "failed to create schema_migrations table")
	}
	return nil
}

// GetCurrentVersion gets the current schema version
func (m *Migrator) GetCurrentVersion(ctx context.Context) (int, error) {
	var version int
	err := m.pool.QueryRow(ctx, getVersionSQL).Scan(&version)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to get current schema version")
	}
	return version, nil
}

// MigrateUp migrates the database up to the latest version
func (m *Migrator) MigrateUp(ctx context.Context) error {
	// Initialize the migrations table
	if err := m.Initialize(ctx); err != nil {
		return err
	}

	// Get the current version
	currentVersion, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Current schema version: %d\n", currentVersion)  //nolint:forbidigo // Migration progress output is intentional
	fmt.Printf("Available migrations: %d\n", len(m.migrations)) //nolint:forbidigo // Migration progress output is intentional

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	// Apply migrations
	for _, migration := range m.migrations {
		if migration.Version <= currentVersion {
			fmt.Printf("Skipping migration %d: %s (already applied)\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional
			continue
		}
		fmt.Printf("Applying migration %d: %s\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional

		// Begin transaction
		tx, err := m.pool.Begin(ctx)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to begin transaction for migration %d", migration.Version))
		}

		// Apply migration
		if err := migration.Up(ctx, tx); err != nil {
			_ = tx.Rollback(ctx)
			return errkit.Wrap(err, fmt.Sprintf("failed to apply migration %d", migration.Version))
		}

		// Record migration
		_, err = tx.Exec(ctx, recordMigrationSQL, migration.Version, migration.Description, time.Now())
		if err != nil {
			_ = tx.Rollback(ctx)
			return errkit.Wrap(err, fmt.Sprintf("failed to record migration %d", migration.Version))
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to commit transaction for migration %d", migration.Version))
		}

		fmt.Printf("Applied migration %d: %s\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional
	}

	return nil
}

// CheckMigrationsUpToDate checks if all migrations have been applied
func (m *Migrator) CheckMigrationsUpToDate(ctx context.Context) (bool, error) {
	// Initialize the migrations table
	if err := m.Initialize(ctx); err != nil {
		return false, err
	}

	// Get the current version
	currentVersion, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return false, err
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	// Check if there are any migrations that haven't been applied
	for _, migration := range m.migrations {
		if migration.Version > currentVersion {
			return false, nil
		}
	}

	return true, nil
}

// MigrateDown migrates the database down to a specific version
func (m *Migrator) MigrateDown(ctx context.Context, targetVersion int) error {
	// Initialize the migrations table
	if err := m.Initialize(ctx); err != nil {
		return err
	}

	// Get the current version
	currentVersion, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return err
	}

	// Sort migrations by version in descending order
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version > m.migrations[j].Version
	})

	// Apply migrations
	for _, migration := range m.migrations {
		if migration.Version <= targetVersion || migration.Version > currentVersion {
			continue
		}

		// Begin transaction
		tx, err := m.pool.Begin(ctx)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to begin transaction for migration %d", migration.Version))
		}

		// Apply migration
		if err := migration.Down(ctx, tx); err != nil {
			_ = tx.Rollback(ctx)
			return errkit.Wrap(err, fmt.Sprintf("failed to revert migration %d", migration.Version))
		}

		// Record migration
		_, err = tx.Exec(ctx, deleteMigrationSQL, migration.Version)
		if err != nil {
			_ = tx.Rollback(ctx)
			return errkit.Wrap(err, fmt.Sprintf("failed to record migration reversion %d", migration.Version))
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to commit transaction for migration reversion %d", migration.Version))
		}

		fmt.Printf("Reverted migration %d: %s\n", migration.Version, migration.Description) //nolint:forbidigo // Migration progress output is intentional
	}

	return nil
}
