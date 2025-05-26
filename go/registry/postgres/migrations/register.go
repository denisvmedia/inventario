package migrations

import (
	"context"
	"io/fs"

	"github.com/go-extras/go-kit/must"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisvmedia/inventario/internal/errkit"
)

// RegisterMigrations registers all migrations with the migrator
func RegisterMigrations(migrator *Migrator) {
	migrationsMap := make(map[int]*Migration) // version -> migration
	migrationsFS := must.Must(fs.Sub(GetMigrations(), "source"))

	// error is impossible for embedded filesystem
	_ = fs.WalkDir(migrationsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return errkit.Wrap(errkit.WithFields(err, "path", path), "failed to walk migrations directory")
		}
		if d.IsDir() {
			// no error, just skip the entry and continue
			return nil
		}

		migrationFile, err := ParseMigrationFileName(path)
		if err != nil {
			panic(errkit.Wrap(err, "failed to parse migration file name"))
		}

		if migrationsMap[migrationFile.Version] == nil {
			migrationsMap[migrationFile.Version] = &Migration{
				Version:     migrationFile.Version,
				Description: migrationFile.Name,
				Up:          NoopMigrationFunc,
				Down:        NoopMigrationFunc,
			}
		}

		switch migrationFile.Direction {
		case "up":
			migrationsMap[migrationFile.Version].Up = MigrationFuncFromSQLFilename(path, migrationsFS)
		case "down":
			migrationsMap[migrationFile.Version].Down = MigrationFuncFromSQLFilename(path, migrationsFS)
		default:
			panic("invalid migration direction")
		}

		return nil
	})

	// Migrator sorts migrations by version, so we don't need to sort them here.
	for _, migration := range migrationsMap {
		migrator.Register(migration)
	}
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
