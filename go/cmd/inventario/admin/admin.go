// Package admin is the CLI command group for platform-administrative
// operations: granting and revoking the system-admin flag and listing
// current system administrators. Issue #1745 introduces the foundation;
// later issues under the #1744 umbrella will hang additional admin
// subcommands (impersonation #1750, etc.) off the same group.
//
// All operations require a PostgreSQL DSN — the in-memory backend has no
// persistence, so an `admin grant-system-admin` against memory:// would
// be lost the moment the CLI exits.
package admin

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/admin/grant"
	"github.com/denisvmedia/inventario/cmd/inventario/admin/listcmd"
	"github.com/denisvmedia/inventario/cmd/inventario/admin/revoke"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// New creates the parent `admin` command and registers its subcommands.
// Mounted from cmd/inventario/inventario.go alongside `users`, `tenants`,
// etc. — the dbConfig flagset is supplied by the root command.
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Platform-administrative commands (system administrators)",
		Long: `Platform-administrative commands for managing system administrators.

System administrators have cross-tenant administrative privileges and can
access the /api/v1/admin/* surfaces. They are NOT automatically members
of any group — system-admin is distinct from per-group GroupRoleAdmin.

IMPORTANT: These commands ONLY support PostgreSQL databases. Memory
databases cannot persist the system-admin flag across restarts.

USAGE EXAMPLES:

  Grant system-admin to a user:
    inventario admin grant-system-admin --email admin@acme.com

  Revoke system-admin (refuses to revoke the last admin):
    inventario admin revoke-system-admin --email admin@acme.com

  Revoke even if it would leave zero admins (deliberate shutdown):
    inventario admin revoke-system-admin --email admin@acme.com --allow-zero

  List current system administrators:
    inventario admin list-system-admins`,
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(grant.New(dbConfig).Cmd())
	cmd.AddCommand(revoke.New(dbConfig).Cmd())
	cmd.AddCommand(listcmd.New(dbConfig).Cmd())

	return cmd
}
