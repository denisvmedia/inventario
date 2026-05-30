// Package resume implements `inventario workers resume`.
package resume

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

// Config carries the resume command's flags.
type Config struct {
	Type string `yaml:"type" env:"TYPE"`
}

// Command is the `workers resume` cobra wrapper.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("workers.resume", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "resume",
		Short: "Resume a soft-paused background worker",
		Long: `Resume the background worker named by --type.

Clears the soft-pause so the worker starts claiming jobs again on its
next tick. The operation is idempotent: resuming a worker that is not
paused is a no-op.

Known worker types: ` + knownWorkerTypes() + `

Examples:
  inventario workers resume --type export`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	c.Cmd().Flags().StringVar(&c.config.Type, "type", c.config.Type, "Worker type to resume (required)")
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	if err := dbConfig.Validate(); err != nil {
		return errxtrace.Wrap("database configuration error", err)
	}
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return errors.New("worker pause commands are not supported for memory databases: the pause state must persist in a database shared with the worker process; use PostgreSQL")
	}
	workerType := strings.TrimSpace(cfg.Type)
	if workerType == "" {
		return errors.New("--type is required")
	}
	if _, ok := models.ParseWorkerType(workerType); !ok {
		return fmt.Errorf("unknown worker type %q; known types: %s", workerType, knownWorkerTypes())
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

	control, err := adminService.ResumeWorker(c.Cmd().Context(), workerType)
	if err != nil {
		return errxtrace.Wrap("failed to resume worker", err)
	}

	fmt.Fprintf(out, "▶️  Resumed worker %q.\n", control.WorkerType)
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
