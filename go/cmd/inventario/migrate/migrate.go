package migrate

import (
	"context"
	"fmt"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/registry/migrations"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long: `Migrate applies database schema migrations to create or update the database
structure required by Inventario. This command ensures your database has all the
necessary tables, indexes, and constraints needed for the application to function.

Migrations are version-controlled database changes that are applied incrementally.
The system tracks which migrations have been applied, so running this command
multiple times is safe - only new migrations will be executed.

This command must be run before using the application for the first time, and
should be run whenever you update Inventario to a newer version that includes
database schema changes.

USAGE EXAMPLES:

  Migrate a PostgreSQL database:
    inventario migrate --db-dsn="postgres://user:pass@localhost/inventario"

  Migrate a local BoltDB database:
    inventario migrate --db-dsn="boltdb://./inventario.db"

  Preview migrations without applying (dry-run mode):
    inventario migrate --dry-run --db-dsn="postgres://user:pass@localhost/inventario"

FLAG DETAILS:

  --db-dsn (REQUIRED)
    Database connection string specifying which database to migrate.
    Unlike other commands, this flag has no default and must be provided.
    
    Supported database types:
    • PostgreSQL: "postgres://user:password@host:port/database?sslmode=disable"
      - Requires PostgreSQL 12 or later
      - User must have CREATE TABLE, CREATE INDEX, and INSERT permissions
      - Database must already exist (migrations don't create databases)
    
    • BoltDB: "boltdb://path/to/database.db"
      - Creates the database file if it doesn't exist
      - Requires write permissions to the target directory
      - Suitable for single-user deployments and development
    
    • Memory: "memory://"
      - Creates temporary in-memory database
      - Data is lost when the process exits
      - Useful for testing and development only

  --dry-run (default false)
    When enabled, shows what migrations would be applied without making changes.
    Note: Dry-run mode is not yet fully implemented and will return an error.
    For now, use external tools like 'ptah migrate' for dry-run SQL generation.

MIGRATION PROCESS:
  1. Connects to the specified database
  2. Creates a migrations tracking table if it doesn't exist
  3. Compares applied migrations with available migration files
  4. Applies any pending migrations in chronological order
  5. Updates the tracking table to record successful migrations

TYPICAL WORKFLOW:
  1. First-time setup:
     inventario migrate --db-dsn="your-database-url"
     inventario seed --db-dsn="your-database-url"
     inventario run --db-dsn="your-database-url"

  2. After updating Inventario:
     inventario migrate --db-dsn="your-database-url"
     inventario run --db-dsn="your-database-url"

TROUBLESHOOTING:
  • If migrations fail, check database permissions and connectivity
  • Failed migrations may leave the database in an inconsistent state
  • Always backup your database before running migrations in production
  • Check logs for specific error messages if migrations fail

SAFETY NOTES:
  • Migrations are typically safe to run multiple times
  • Some migrations may be irreversible (e.g., dropping columns)
  • Always test migrations on a copy of production data first
  • Consider maintenance windows for production migration runs`,
	RunE:  migrateCommand,
}

const (
	dbDSNFlag  = "db-dsn"
	dryRunFlag = "dry-run"
)

var migrateFlags = map[string]cobraflags.Flag{
	dbDSNFlag: &cobraflags.StringFlag{
		Name:  dbDSNFlag,
		Value: "",
		Usage: "Database DSN (required). Supported types: postgres://, memory://, boltdb://",
	},
	dryRunFlag: &cobraflags.BoolFlag{
		Name:  dryRunFlag,
		Value: false,
		Usage: "Show what migrations would be executed without making actual changes",
	},
}

func NewMigrateCommand() *cobra.Command {
	cobraflags.RegisterMap(migrateCmd, migrateFlags)

	return migrateCmd
}

func migrateCommand(_ *cobra.Command, _ []string) error {
	dsn := migrateFlags[dbDSNFlag].GetString()
	dryRun := migrateFlags[dryRunFlag].GetBool()

	if dsn == "" {
		return fmt.Errorf("database DSN is required")
	}

	if dryRun {
		// log.WithField(dbDSNFlag, dsn).Info("[DRY RUN] Would run migrations")
		// fmt.Println("⚠️  [DRY RUN] Migration dry run mode is not yet fully implemented.")
		// fmt.Println("⚠️  This would run migrations against the database.")
		// fmt.Println("⚠️  For now, use the ptah tool 'migrate' command for dry run migration SQL generation.")
		return fmt.Errorf("dry run mode is not yet implemented")
	}

	log.WithField(dbDSNFlag, dsn).Info("Running migrations")

	// Run migrations using the standardized interface
	err := migrations.RunMigrations(context.Background(), dsn)
	if err != nil {
		log.WithError(err).Error("Failed to run migrations")
		return err
	}

	log.Info("Migrations completed successfully")
	return nil
}
