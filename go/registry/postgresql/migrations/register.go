package migrations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisvmedia/inventario/internal/errkit"
)

// RegisterMigrations registers all migrations with the migrator
func RegisterMigrations(migrator *Migrator) {
	// Register migrations in order
	migrator.Register(InitialSchemaMigration())
	// Add more migrations here as needed
}

// RunMigrations runs all migrations
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrator := NewMigrator(pool)
	RegisterMigrations(migrator)

	if err := migrator.MigrateUp(ctx); err != nil {
		return errkit.Wrap(err, "failed to run migrations")
	}

	return nil
}

// CheckMigrationsApplied checks if all migrations have been applied
func CheckMigrationsApplied(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	migrator := NewMigrator(pool)
	RegisterMigrations(migrator)

	upToDate, err := migrator.CheckMigrationsUpToDate(ctx)
	if err != nil {
		return false, errkit.Wrap(err, "failed to check migrations")
	}

	return upToDate, nil
}
