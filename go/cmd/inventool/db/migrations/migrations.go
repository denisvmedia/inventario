package migrations

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate/list"
	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate/up"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/cmd/inventool/db/migrations/generate"
)

// New creates the migrations command group
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrations",
		Short: "PostgreSQL database migration management",
		Long: `TODO: write long description
`,
		Args: cobra.NoArgs, // Disallow unknown subcommands
	}

	// Add subcommands
	cmd.AddCommand(up.New(dbConfig).Cmd())
	cmd.AddCommand(list.New(dbConfig).Cmd())
	cmd.AddCommand(generate.New(dbConfig).Cmd())

	return cmd
}
