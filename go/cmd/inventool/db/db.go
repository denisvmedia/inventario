package db

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/db/bootstrap"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/cmd/inventool/db/migrations"
)

// New creates the main db command.
func New() *cobra.Command {
	var cfg shared.DatabaseConfig

	var cmd = &cobra.Command{
		Use:   "db",
		Short: "Database management commands",
		Long: `Database management commands for PostgreSQL operations.

IMPORTANT: These commands ONLY support PostgreSQL databases.
Other database types are no longer supported in this version.`,
		Args: cobra.NoArgs, // Disallow unknown subcommands
	}

	shared.RegisterDatabaseFlags(cmd, &cfg)

	// Add subcommands
	cmd.AddCommand(bootstrap.New(&cfg))
	cmd.AddCommand(migrations.New(&cfg))

	return cmd
}
