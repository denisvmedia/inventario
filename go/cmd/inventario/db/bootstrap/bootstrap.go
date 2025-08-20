package bootstrap

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/db/bootstrap/apply"
	"github.com/denisvmedia/inventario/cmd/inventario/db/bootstrap/printcmd"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// New creates the bootstrap command group
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Manage bootstrap database migrations",
		Long: `Bootstrap migrations are special SQL migrations that must be run under privileged
database user credentials before regular Ptah migrations can be executed.

These migrations handle initial database setup that requires elevated privileges,
such as creating extensions, roles, and setting up default privileges.

Bootstrap migrations are:
- Idempotent (can be run multiple times safely)
- PostgreSQL-only
- Prerequisites for regular Ptah migrations
- Executed with elevated database privileges

USAGE EXAMPLES:

  Apply all bootstrap migrations:
    inventario migrate bootstrap apply --db-dsn="postgres://admin:pass@localhost/inventario"

  Preview bootstrap migrations without applying:
    inventario migrate bootstrap print --db-dsn="postgres://admin:pass@localhost/inventario"

IMPORTANT: Bootstrap migrations require a privileged database user (typically with
SUPERUSER or equivalent privileges) to create extensions and manage roles.`,
	}

	// Add subcommands
	cmd.AddCommand(apply.New(dbConfig).Cmd())
	cmd.AddCommand(printcmd.New().Cmd())

	return cmd
}
