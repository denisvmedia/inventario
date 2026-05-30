// Package workers is the CLI command group for the background-worker
// soft-pause control plane (#1308): pausing, resuming, and inspecting
// the pause state of the polling background workers (export, import,
// thumbnails, reminders, etc.).
//
// Soft-pause means the worker's run loop keeps ticking but skips its
// unit of work while paused, so in-flight jobs finish, no new jobs are
// claimed, and resuming is immediate without a process restart. The
// state is persisted in the worker_control table and shared across every
// replica via the database, so a single `inventario workers pause`
// coordinates the whole fleet.
//
// NOTE: this is a SEPARATE top-level command from `inventario run
// workers` (which STARTS the worker process). This group MUTATES pause
// state in the shared database and is intended for ops use against a
// running deployment.
//
// All operations require a PostgreSQL DSN — the in-memory backend has no
// persistence and is not shared with the worker process, so a pause
// against memory:// would be lost and never seen by the workers.
package workers

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/cmd/inventario/workers/pause"
	"github.com/denisvmedia/inventario/cmd/inventario/workers/resume"
	"github.com/denisvmedia/inventario/cmd/inventario/workers/status"
)

// New creates the parent `workers` command and registers its
// subcommands. Mounted from cmd/inventario/inventario.go alongside
// `admin`, `users`, `tenants`, etc. — the dbConfig flagset is supplied
// by the root command.
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workers",
		Short: "Soft-pause / resume background workers (#1308)",
		Long: `Soft-pause, resume, and inspect Inventario's background workers.

Soft-pause keeps each worker's run loop ticking but skips its unit of
work while paused: in-flight jobs finish, no new jobs are claimed, and
resuming takes effect on the next tick without a process restart. The
pause state is stored in the database and shared across every replica,
so one command coordinates the whole fleet.

This is NOT the same as "inventario run workers", which STARTS the
worker process. These commands MUTATE the shared pause state of an
already-running deployment.

IMPORTANT: These commands ONLY support PostgreSQL databases. The
in-memory backend has no persistence and is not shared with the worker
process, so a pause against memory:// would never be seen.

USAGE EXAMPLES:

  Pause the export worker with a reason:
    inventario workers pause --type export --reason "maintenance window"

  Resume the export worker:
    inventario workers resume --type export

  Show the pause state of every worker:
    inventario workers status`,
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(pause.New(dbConfig).Cmd())
	cmd.AddCommand(resume.New(dbConfig).Cmd())
	cmd.AddCommand(status.New(dbConfig).Cmd())

	return cmd
}
