package migrations

import (
	"context"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/registry/ptah"
)

// PostgreSQLMigrator implements the Migrator interface for PostgreSQL using Ptah
type PostgreSQLMigrator struct {
	dsn          string
	ptahMigrator *ptah.PtahMigrator
}

// NewPostgreSQLMigrator creates a new PostgreSQLMigrator using Ptah
func NewPostgreSQLMigrator(dsn string) (Migrator, error) {
	// Create Ptah migrator
	ptahMigrator, err := ptah.NewPtahMigrator(nil, dsn, "./models")
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create Ptah migrator")
	}

	return &PostgreSQLMigrator{
		dsn:          dsn,
		ptahMigrator: ptahMigrator,
	}, nil
}

// RunMigrations runs all migrations for PostgreSQL using Ptah
func (m *PostgreSQLMigrator) RunMigrations(ctx context.Context) error {
	// Use Ptah to run migrations
	err := m.ptahMigrator.MigrateUp(ctx, false)
	if err != nil {
		return errkit.Wrap(err, "failed to run Ptah migrations")
	}

	return nil
}

// CheckMigrationsApplied checks if all migrations have been applied for PostgreSQL using Ptah
func (m *PostgreSQLMigrator) CheckMigrationsApplied(ctx context.Context) (bool, error) {
	// Use Ptah to check migration status
	err := m.ptahMigrator.PrintMigrationStatus(ctx, false)
	if err != nil {
		return false, errkit.Wrap(err, "failed to check Ptah migration status")
	}

	// For now, assume migrations are applied if no error occurred
	// In a real implementation, you might want to parse the status output
	return true, nil
}
