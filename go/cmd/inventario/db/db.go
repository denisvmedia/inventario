package db

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/db/bootstrap"
	"github.com/denisvmedia/inventario/cmd/inventario/db/drop"
	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate"
	"github.com/denisvmedia/inventario/cmd/inventario/db/reset"
	"github.com/denisvmedia/inventario/cmd/inventario/db/status"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// New creates the main db command.
func New() *cobra.Command {
	var cfg shared.DatabaseConfig

	var cmd = &cobra.Command{
		Use:   "db",
		Short: "Database management commands",
		Long: `Database management commands for PostgreSQL operations.

This command provides database management capabilities including:
- Bootstrap migrations for initial setup
- Schema migrations using Ptah
- Database maintenance operations

IMPORTANT: These commands ONLY support PostgreSQL databases.
Other database types are no longer supported in this version.`,
		Args: cobra.NoArgs, // Disallow unknown subcommands
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	shared.RegisterDatabaseFlags(cmd, &cfg)

	// Add subcommands
	cmd.AddCommand(bootstrap.New(&cfg))
	cmd.AddCommand(migrate.New(&cfg))
	cmd.AddCommand(drop.New(&cfg).Cmd())
	cmd.AddCommand(reset.New(&cfg).Cmd())
	cmd.AddCommand(status.New(&cfg).Cmd())

	return cmd
}
