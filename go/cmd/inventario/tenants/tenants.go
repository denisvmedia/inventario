package tenants

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/cmd/inventario/tenants/create"
	"github.com/denisvmedia/inventario/cmd/inventario/tenants/delete"
	"github.com/denisvmedia/inventario/cmd/inventario/tenants/get"
	"github.com/denisvmedia/inventario/cmd/inventario/tenants/list"
	"github.com/denisvmedia/inventario/cmd/inventario/tenants/update"
)

// New creates the main tenants command group
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenants",
		Short: "Tenant management commands",
		Long: `Tenant management commands for creating and managing tenants.

This command provides tenant management capabilities including:
- Create new tenants with interactive prompts
- Validate tenant data and uniqueness
- Support for PostgreSQL databases only

IMPORTANT: These commands ONLY support PostgreSQL databases.
Memory databases are not supported for persistent tenant operations.

USAGE EXAMPLES:

  Create tenant interactively:
    inventario tenants create

  Create tenant with flags:
    inventario tenants create --name="Acme Corp" --slug="acme" --domain="acme.com"

  Preview tenant creation:
    inventario tenants create --dry-run --name="Test Org"

CONFIGURATION:

  The command reads database configuration from:
  1. Command line flag: --db-dsn
  2. Environment variable: DB_DSN
  3. Configuration file: db-dsn setting

  PostgreSQL connection string format:
    postgres://user:password@host:port/database?sslmode=disable`,
		Args: cobra.NoArgs, // Disallow unknown subcommands
	}

	// Add subcommands
	cmd.AddCommand(create.New(dbConfig).Cmd())
	cmd.AddCommand(delete.New(dbConfig).Cmd())
	cmd.AddCommand(get.New(dbConfig).Cmd())
	cmd.AddCommand(list.New(dbConfig).Cmd())
	cmd.AddCommand(update.New(dbConfig).Cmd())

	return cmd
}
