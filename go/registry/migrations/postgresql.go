package migrations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisvmedia/inventario/internal/errkit"
	pgmigrations "github.com/denisvmedia/inventario/registry/postgres/migrations"
)

// PostgreSQLMigrator implements the Migrator interface for PostgreSQL
type PostgreSQLMigrator struct {
	dsn string
}

// NewPostgreSQLMigrator creates a new PostgreSQLMigrator
func NewPostgreSQLMigrator(dsn string) (Migrator, error) {
	return &PostgreSQLMigrator{
		dsn: dsn,
	}, nil
}

// RunMigrations runs all migrations for PostgreSQL
func (m *PostgreSQLMigrator) RunMigrations(ctx context.Context) error {
	// Connect to the database
	pool, err := pgxpool.New(ctx, m.dsn)
	if err != nil {
		return errkit.Wrap(err, "failed to connect to database")
	}
	defer pool.Close()

	// Run migrations
	err = pgmigrations.RunMigrations(ctx, pool)
	if err != nil {
		return errkit.Wrap(err, "failed to run migrations")
	}

	return nil
}

// CheckMigrationsApplied checks if all migrations have been applied for PostgreSQL
func (m *PostgreSQLMigrator) CheckMigrationsApplied(ctx context.Context) (bool, error) {
	// Connect to the database
	pool, err := pgxpool.New(ctx, m.dsn)
	if err != nil {
		return false, errkit.Wrap(err, "failed to connect to database")
	}
	defer pool.Close()

	// Check migrations
	return pgmigrations.CheckMigrationsApplied(ctx, pool)
}
