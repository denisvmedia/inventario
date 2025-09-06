package users

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/cmd/inventario/users/create"
	"github.com/denisvmedia/inventario/cmd/inventario/users/deletecmd"
	"github.com/denisvmedia/inventario/cmd/inventario/users/get"
	"github.com/denisvmedia/inventario/cmd/inventario/users/list"
	"github.com/denisvmedia/inventario/cmd/inventario/users/update"
)

// New creates the main users command group
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "User management commands",
		Long: `User management commands for creating and managing users.

This command provides user management capabilities including:
- Create new users with interactive prompts
- Secure password input and validation
- Tenant association and validation
- Support for PostgreSQL databases only

IMPORTANT: These commands ONLY support PostgreSQL databases.
Memory databases are not supported for persistent user operations.

USAGE EXAMPLES:

  Create user interactively:
    inventario users create

  Create user with flags:
    inventario users create --email="admin@acme.com" --tenant="acme" --role="admin"

  Preview user creation:
    inventario users create --dry-run --email="test@example.com" --tenant="acme"

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
	cmd.AddCommand(deletecmd.New(dbConfig).Cmd())
	cmd.AddCommand(get.New(dbConfig).Cmd())
	cmd.AddCommand(list.New(dbConfig).Cmd())
	cmd.AddCommand(update.New(dbConfig).Cmd())

	return cmd
}
