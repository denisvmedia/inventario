// Package jobs is the CLI command group for background jobs (#1307). Its
// one subgroup, `failed`, lists, retries and discards the jobs that failed
// for good, across every tenant, the way the back office does.
//
// The commands need PostgreSQL: the failed jobs live in the tables the
// running workers share, and an in-memory store is per process.
package jobs

import (
	"github.com/spf13/cobra"

	"go.5x5.cz/inventario/cmd/inventario/jobs/failed"
	"go.5x5.cz/inventario/cmd/inventario/shared"
)

// New creates the `jobs` command and its subcommands.
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Inspect and act on background jobs",
		Long: `Inspect and act on Inventario's background jobs.

A job that fails for good stays in its own table with its error. The
"failed" commands list such jobs across every tenant, put one back in its
queue, or discard it. Discarding keeps the row and its error; the job
only leaves the failed list and the inventario_jobs_dead_letter_depth
gauge.

These commands ONLY support PostgreSQL databases.

USAGE EXAMPLES:

  List the failed jobs of every tenant:
    inventario jobs failed list

  List one tenant's failed exports:
    inventario jobs failed list --type export --tenant acme

  Retry a failed thumbnail:
    inventario jobs failed retry --type thumbnail <job-id>

  Discard a failed restore:
    inventario jobs failed discard --type restore <job-id>`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(failed.New(dbConfig))
	return cmd
}
