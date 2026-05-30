// Package pause implements `inventario workers pause`.
package pause

import (
	"errors"
	"fmt"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/admin"
)

// Config carries the pause command's flags.
type Config struct {
	Type   string `yaml:"type" env:"TYPE"`
	Reason string `yaml:"reason" env:"REASON"`
}

// Command is the `workers pause` cobra wrapper.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("workers.pause", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "pause",
		Short: "Soft-pause a background worker",
		Long: `Soft-pause the background worker named by --type.

Soft-pause keeps the worker's run loop ticking but skips its unit of
work: in-flight jobs finish, no new jobs are claimed, and the state
persists across restarts. Resuming takes effect on the next tick.

The operation is idempotent: re-pausing an already-paused worker
updates the reason but preserves the original pause time.

Known worker types: ` + knownWorkerTypes() + `

Examples:
  inventario workers pause --type export
  inventario workers pause --type thumbnail --reason "GPU maintenance"`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// `--type` is "required" but enforced inside run() (with a friendlier
	// error than cobra's default) so the same shape is reached whether the
	// command is invoked through the root binary or directly from tests.
	c.Cmd().Flags().StringVar(&c.config.Type, "type", c.config.Type, "Worker type to pause (required)")
	c.Cmd().Flags().StringVar(&c.config.Reason, "reason", c.config.Reason, "Optional reason recorded with the pause")
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	// Reject memory:// BEFORE the generic Validate(): Validate() already
	// rejects non-postgres DSNs, so the worker-specific message would be
	// unreachable for the default memory DSN if checked afterwards.
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		// The pause state must live in a persistent database shared with
		// the worker process; an in-memory store is per-process and would
		// never be seen by the workers.
		return errors.New("worker pause commands are not supported for memory databases: the pause state must persist in a database shared with the worker process; use PostgreSQL")
	}
	if err := dbConfig.Validate(); err != nil {
		return errxtrace.Wrap("database configuration error", err)
	}
	workerType := strings.TrimSpace(cfg.Type)
	if workerType == "" {
		return errors.New("--type is required")
	}
	if _, ok := models.ParseWorkerType(workerType); !ok {
		return fmt.Errorf("unknown worker type %q; known types: %s", workerType, knownWorkerTypes())
	}
	reason := strings.TrimSpace(cfg.Reason)

	adminService, err := admin.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	control, err := adminService.PauseWorker(c.Cmd().Context(), workerType, reason)
	if err != nil {
		return errxtrace.Wrap("failed to pause worker", err)
	}

	pausedAt := "unknown"
	if control.PausedAt != nil {
		pausedAt = control.PausedAt.Format("2006-01-02 15:04:05 MST")
	}
	fmt.Fprintf(out, "⏸️  Paused worker %q (paused_at: %s).\n", control.WorkerType, pausedAt)
	if reason != "" {
		fmt.Fprintf(out, "   reason: %s\n", reason)
	}
	return nil
}

// knownWorkerTypes renders the canonical worker-type set as a
// comma-separated list for help text and error messages.
func knownWorkerTypes() string {
	all := models.AllWorkerTypes()
	names := make([]string, len(all))
	for i, t := range all {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}
