package migrations

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisvmedia/inventario/internal/errkit"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          func(ctx context.Context, tx pgx.Tx) error
	Down        func(ctx context.Context, tx pgx.Tx) error
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
	_, err := m.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP NOT NULL
		);
	`)
	if err != nil {
		return errkit.Wrap(err, "failed to create schema_migrations table")
	}
	return nil
}

// GetCurrentVersion gets the current schema version
func (m *Migrator) GetCurrentVersion(ctx context.Context) (int, error) {
	var version int
	err := m.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM schema_migrations
	`).Scan(&version)
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

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	// Apply migrations
	for _, migration := range m.migrations {
		if migration.Version <= currentVersion {
			continue
		}

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
		_, err = tx.Exec(ctx, `
			INSERT INTO schema_migrations (version, description, applied_at)
			VALUES ($1, $2, $3)
		`, migration.Version, migration.Description, time.Now())
		if err != nil {
			_ = tx.Rollback(ctx)
			return errkit.Wrap(err, fmt.Sprintf("failed to record migration %d", migration.Version))
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to commit transaction for migration %d", migration.Version))
		}

		fmt.Printf("Applied migration %d: %s\n", migration.Version, migration.Description)
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
		_, err = tx.Exec(ctx, `
			DELETE FROM schema_migrations
			WHERE version = $1
		`, migration.Version)
		if err != nil {
			_ = tx.Rollback(ctx)
			return errkit.Wrap(err, fmt.Sprintf("failed to record migration reversion %d", migration.Version))
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to commit transaction for migration reversion %d", migration.Version))
		}

		fmt.Printf("Reverted migration %d: %s\n", migration.Version, migration.Description)
	}

	return nil
}
