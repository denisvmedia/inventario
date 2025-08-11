package migrate

import (
	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/migrate/down"
	"github.com/denisvmedia/inventario/cmd/inventario/migrate/drop"
	"github.com/denisvmedia/inventario/cmd/inventario/migrate/generate"
	"github.com/denisvmedia/inventario/cmd/inventario/migrate/reset"
	"github.com/denisvmedia/inventario/cmd/inventario/migrate/status"
	"github.com/denisvmedia/inventario/cmd/inventario/migrate/up"
)

const (
	dbDSNFlag = "db-dsn"
)

var migrateFlags = map[string]cobraflags.Flag{
	dbDSNFlag: &cobraflags.StringFlag{
		Name:       dbDSNFlag,
		Value:      "", // No default for migrate command - must be explicitly provided
		Usage:      "PostgreSQL database connection string (required)",
		Persistent: true, // Make this flag available to all subcommands
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "PostgreSQL database migration management",
	Long: `Advanced PostgreSQL database migration management using Ptah.

This command provides comprehensive PostgreSQL migration capabilities including:
- Apply pending migrations (up)
- Rollback migrations (down)
- Check migration status
- Dry run mode for safe testing

All migrations are embedded in the binary and support PostgreSQL-specific features
like triggers, functions, JSONB operators, and advanced indexing.

IMPORTANT: This migration system ONLY supports PostgreSQL databases.
Other database types are no longer supported in this version.

USAGE EXAMPLES:

  Apply all pending migrations:
    inventario migrate
    inventario migrate up

  Rollback to specific version:
    inventario migrate down 5

  Check migration status:
    inventario migrate status

  Preview migrations without applying:
    inventario migrate up --dry-run

CONFIGURATION:

  The command reads database configuration from:
  1. Command line flag: --db-dsn
  2. Environment variable: DB_DSN
  3. Configuration file: db-dsn setting

  PostgreSQL connection string format:
    postgres://user:password@host:port/database?sslmode=disable

MIGRATION SAFETY:

  • Each migration runs in its own transaction
  • Failed migrations are automatically rolled back
  • Migration state is tracked in schema_migrations table
  • Always backup your database before running migrations in production`,
	RunE: cobra.NoArgs,
}

// New creates the main migrate command using Ptah.
func New() *cobra.Command {
	// Register cobraflags which automatically handles viper binding and persistent flags
	cobraflags.RegisterMap(migrateCmd, migrateFlags)

	// Add subcommands
	migrateCmd.AddCommand(up.New(migrateFlags[dbDSNFlag]))
	migrateCmd.AddCommand(down.New(migrateFlags[dbDSNFlag]))
	migrateCmd.AddCommand(status.New(migrateFlags[dbDSNFlag]))
	migrateCmd.AddCommand(generate.New(migrateFlags[dbDSNFlag]))
	migrateCmd.AddCommand(reset.New(migrateFlags[dbDSNFlag]))
	migrateCmd.AddCommand(drop.New(migrateFlags[dbDSNFlag]))

	return migrateCmd
}
