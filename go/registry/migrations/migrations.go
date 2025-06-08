package migrations

import (
	"context"
	"maps"
	"net/url"

	"github.com/denisvmedia/inventario/internal/errkit"
)

// Migrator is the interface that wraps the basic migration methods.
type Migrator interface {
	// RunMigrations runs all migrations
	RunMigrations(ctx context.Context) error

	// CheckMigrationsApplied checks if all migrations have been applied
	CheckMigrationsApplied(ctx context.Context) (bool, error)
}

// MigratorFunc is a function that creates a new migrator for a given DSN
type MigratorFunc func(dsn string) (Migrator, error)

var migrators = make(map[string]MigratorFunc)

// Register registers a new migrator function for a database type.
// It panics if the name is already registered.
// It is intended to be called from the init function in the migrator package.
// It is NOT safe for concurrent use.
func Register(name string, f MigratorFunc) {
	if _, ok := migrators[name]; ok {
		panic("migrations: duplicate migrator name")
	}

	migrators[name] = f
}

// Unregister unregisters a migrator function.
// It is intended to be called from tests.
// It is NOT safe for concurrent use.
func Unregister(name string) {
	delete(migrators, name)
}

// Migrators returns a map of registered migrator functions.
// It can be used concurrently with itself, but not with Register or Unregister.
func Migrators() map[string]MigratorFunc {
	return maps.Clone(migrators)
}

// MigratorNames returns a slice of registered migrator names.
func MigratorNames() []string {
	names := make([]string, 0, len(migrators))
	for name := range migrators {
		names = append(names, name)
	}

	return names
}

// GetMigrator returns a migrator function for a given DSN.
func GetMigrator(dsn string) (MigratorFunc, bool) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return nil, false
	}

	m, ok := migrators[parsed.Scheme]
	if !ok {
		return nil, false
	}

	return m, true
}

// RunMigrations runs migrations for a given DSN.
func RunMigrations(ctx context.Context, dsn string) error {
	migratorFn, ok := GetMigrator(dsn)
	if !ok {
		return errkit.Wrap(ErrUnknownDatabaseType, "failed to get migrator")
	}

	migrator, err := migratorFn(dsn)
	if err != nil {
		return errkit.Wrap(err, "failed to create migrator")
	}

	return migrator.RunMigrations(ctx)
}

// CheckMigrationsApplied checks if migrations are applied for a given DSN.
func CheckMigrationsApplied(ctx context.Context, dsn string) (bool, error) {
	migratorFn, ok := GetMigrator(dsn)
	if !ok {
		return false, errkit.Wrap(ErrUnknownDatabaseType, "failed to get migrator")
	}

	migrator, err := migratorFn(dsn)
	if err != nil {
		return false, errkit.Wrap(err, "failed to create migrator")
	}

	return migrator.CheckMigrationsApplied(ctx)
}
