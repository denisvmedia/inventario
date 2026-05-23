// Package backoffice is the CLI command group for managing platform-
// operator identities used by the back-office auth plane (issue #1785).
//
// Back-office identities live OUTSIDE the tenant model: a row in
// backoffice_users has no tenant_id, no group membership, and no RLS
// scoping. The whole point of the #1785 epic is to keep platform
// operators (support agents, platform admins) on a separate auth plane
// so impersonating a customer or escalating to system-admin cannot
// happen by accidentally flipping a column on a regular user row.
//
// Phase 1 (this CLI) ships only the `bootstrap` subcommand — enough to
// stamp the first platform_admin into a fresh deployment. Later phases
// will hang `create`, `list`, and `block` off this same group.
//
// All operations require a PostgreSQL DSN — the in-memory backend has
// no persistence, so any write would vanish the moment the CLI exits.
package backoffice

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/backoffice/bootstrap"
	"github.com/denisvmedia/inventario/cmd/inventario/backoffice/mfa"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// New creates the parent `backoffice` command and registers its
// subcommands. Mounted from cmd/inventario/inventario.go alongside
// `admin`, `tenants`, `users` — the dbConfig flagset is supplied by
// the root command.
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backoffice",
		Short: "Manage back-office (platform-operator) identities",
		Long: `Back-office commands manage platform-operator identities used by the
back-office auth plane (issue #1785). Back-office users live OUTSIDE
the tenant model: they have no tenant_id, no group membership, and no
RLS scoping. They are distinct from regular tenant users (and from the
legacy users.is_system_admin flag, which Phase 3 of #1785 retires).

IMPORTANT: These commands ONLY support PostgreSQL databases. Memory
databases cannot persist back-office identities across restarts.

USAGE EXAMPLES:

  Bootstrap the first platform_admin (auto-generates a password):
    inventario backoffice bootstrap --email admin@example.com --name "Ops Admin"

  Bootstrap with an explicit password:
    inventario backoffice bootstrap --email admin@example.com --name "Ops" --password 'S3cret-Pass'

  Add a second back-office user after the first has been created:
    inventario backoffice bootstrap --email second@example.com --name "Second" --force`,
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(bootstrap.New(dbConfig).Cmd())
	cmd.AddCommand(mfa.New(dbConfig))

	return cmd
}
