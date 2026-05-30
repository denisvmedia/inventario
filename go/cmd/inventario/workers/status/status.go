// Package status implements `inventario workers status`.
package status

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/admin"
)

// Command is the `workers status` cobra wrapper. It carries no flags —
// status renders every worker type unconditionally.
type Command struct {
	command.Base
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "status",
		Short: "Show the pause state of every background worker",
		Long: `Show the soft-pause state of every background worker.

Every canonical worker type is listed. A worker with no control row is
rendered as "running"; a paused worker shows who paused it, when, and
the optional reason.

Examples:
  inventario workers status`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run(dbConfig)
		},
	})

	return c
}

func (c *Command) run(dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	// Reject memory:// BEFORE the generic Validate(): Validate() already
	// rejects non-postgres DSNs, so the worker-specific message would be
	// unreachable for the default memory DSN if checked afterwards.
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return errors.New("worker pause commands are not supported for memory databases: the pause state must persist in a database shared with the worker process; use PostgreSQL")
	}
	if err := dbConfig.Validate(); err != nil {
		return errxtrace.Wrap("database configuration error", err)
	}

	adminService, err := admin.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	controls, err := adminService.ListWorkerControls(c.Cmd().Context())
	if err != nil {
		return errxtrace.Wrap("failed to list worker controls", err)
	}

	// Index the control rows by worker type so the canonical-order walk
	// below can look up state in O(1). Absent => the worker is running.
	byType := make(map[models.WorkerType]*models.WorkerControl, len(controls))
	for _, ctrl := range controls {
		if ctrl != nil {
			byType[ctrl.WorkerType] = ctrl
		}
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "WORKER\tSTATE\tPAUSED_BY\tPAUSED_AT\tREASON")
	for _, wt := range models.AllWorkerTypes() {
		state := "running"
		pausedBy := "-"
		pausedAt := "-"
		reason := "-"
		if ctrl, ok := byType[wt]; ok && ctrl.Paused {
			state = "paused"
			if ctrl.PausedBy != nil && *ctrl.PausedBy != "" {
				pausedBy = *ctrl.PausedBy
			}
			if ctrl.PausedAt != nil {
				pausedAt = ctrl.PausedAt.Format("2006-01-02 15:04:05")
			}
			if ctrl.Reason != nil && *ctrl.Reason != "" {
				reason = *ctrl.Reason
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", wt, state, pausedBy, pausedAt, reason)
	}
	return nil
}
