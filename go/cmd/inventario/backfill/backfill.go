// Package backfill is the parent command group for one-shot data
// migration subcommands. The first subcommand is `blobs` (issue #1793)
// — future tenant-scoped data backfills hang off the same group so the
// pattern stays predictable for operators.
package backfill

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/backfill/blobs"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// New constructs the parent `backfill` command and registers its
// subcommands.
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "One-shot data migrations",
		Long: `Parent command for one-shot data migrations that aren't expressible as
SQL schema migrations.

Subcommands operate against a configured PostgreSQL database and the
running deployment's upload bucket. They are designed to be safe to
re-run: a successful pass produces no changes on the second run; a
partial pass (e.g. interrupted by SIGTERM) is picked up by the next
invocation.

Examples:
  inventario backfill blobs --upload-location file:///srv/uploads`,
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(blobs.New(dbConfig).Cmd())
	return cmd
}
