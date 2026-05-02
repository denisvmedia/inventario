package migrate

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate/data"
	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate/down"
	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate/list"
	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate/up"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// New creates the migrate command group
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "PostgreSQL database migration management",
		Long: `Advanced PostgreSQL database migration management using Ptah.

This command provides comprehensive PostgreSQL migration capabilities including:
- Apply pending migrations (up)
- List available migration files (list)
- Rollback migrations (down)
- Setup initial dataset with tenant/user structure (data)
- Check migration status
- Dry run mode for safe testing

All migrations are embedded in the binary and support PostgreSQL-specific features
like triggers, functions, JSONB operators, and advanced indexing.

IMPORTANT: This migration system ONLY supports PostgreSQL databases.
Other database types are no longer supported in this version.

USAGE EXAMPLES:

  Apply all pending migrations:
    inventario migrate up

  List available migration files:
    inventario migrate list

  Rollback to specific version:
    inventario migrate down 5

  Check migration status:
    inventario migrate status

  Preview migrations without applying:
    inventario migrate up --dry-run

  Setup initial dataset with tenant/user structure:
    inventario migrate data --dry-run

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
		Args: cobra.NoArgs, // Disallow unknown subcommands
	}

	// Add subcommands
	cmd.AddCommand(up.New(dbConfig).Cmd())
	cmd.AddCommand(list.New(dbConfig).Cmd())
	cmd.AddCommand(down.New(dbConfig).Cmd())
	cmd.AddCommand(data.New(dbConfig).Cmd())

	return cmd
}
