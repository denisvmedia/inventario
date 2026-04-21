// Package workers implements the `inventario run workers` subcommand, which
// starts every background worker (or a subset selected via --workers-only /
// --workers-exclude) without opening the HTTP listener. It is intended for
// split deployments where the API server runs as a separate
// `inventario run apiserver` process.
package workers

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

type Command struct {
	command.Base

	cfg      *bootstrap.Config
	dbConfig *shared.DatabaseConfig
}

// New constructs the `run workers` subcommand. cfg and dbConfig are shared
// with the parent `run` command so every subcommand observes the same resolved
// flag values. The --workers-only / --workers-exclude flags are bound on this
// subcommand only, since they are meaningless for `run all` / `run apiserver`.
func New(cfg *bootstrap.Config, dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{cfg: cfg, dbConfig: dbConfig}

	c.Base = command.NewBase(&cobra.Command{
		Use:     "workers",
		Aliases: []string{"worker"},
		Short:   "Start every background worker with an observability-only HTTP listener",
		Long: `Start every background worker (export, import, restore, thumbnail generation,
refresh token cleanup) together with the email delivery lifecycle, without
opening the application HTTP listener.

This mode is intended for split deployments where the API server runs as a
separate "inventario run apiserver" process. Email delivery workers only run
here, so the API server can safely enqueue messages without producing
duplicate deliveries.

A minimal observability listener is started on --probe-addr exposing /healthz,
/readyz and /metrics so Kubernetes liveness/readiness probes and Prometheus
scrapes work uniformly across "run apiserver" and "run workers" deployments.

Use --workers-only or --workers-exclude to isolate a subset of workers onto a
dedicated pod/host. Valid worker identifiers: exports, imports, restores,
thumbnails, emails, token-cleanup. "--workers-only=all" is an explicit synonym
for the default (every worker). The two flags are mutually exclusive.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run()
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	flags := c.Cmd().Flags()
	flags.StringVar(&c.cfg.WorkersOnly, "workers-only", c.cfg.WorkersOnly,
		"Comma-separated list of worker identifiers to run exclusively (e.g., thumbnails,exports). "+
			"Use \"all\" or leave empty to run every worker. Mutually exclusive with --workers-exclude.")
	flags.StringVar(&c.cfg.WorkersExclude, "workers-exclude", c.cfg.WorkersExclude,
		"Comma-separated list of worker identifiers to skip (e.g., emails). "+
			"Mutually exclusive with --workers-only.")
	flags.StringVar(&c.cfg.ProbeAddr, "probe-addr", c.cfg.ProbeAddr,
		"Bind address for the workers' probe listener that serves /healthz, /readyz and /metrics.")
}

// run starts the subset of background workers selected by --workers-only /
// --workers-exclude (all six by default) without opening the HTTP listener. It
// blocks until the process receives SIGINT or SIGTERM.
func (c *Command) run() error {
	selected, err := ParseSelector(c.cfg.WorkersOnly, c.cfg.WorkersExclude)
	if err != nil {
		slog.Error("Invalid worker selector", "error", err)
		return err
	}
	if len(selected) == 0 {
		return errors.New("worker selector resolved to an empty set; nothing to run")
	}

	rs, err := bootstrap.Build(c.cfg, c.dbConfig, bootstrap.ModeWorkers)
	if err != nil {
		return err
	}
	defer rs.CloseReadinessRedisPinger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Worker start order matches the historical `run all` / `run workers`
	// sequence so LIFO shutdown is unchanged for the default (all-workers) case.
	starters := []struct {
		id    WorkerID
		start func(context.Context, *bootstrap.RuntimeSetup, *bootstrap.Config) func()
	}{
		{WorkerEmails, bootstrap.StartEmailLifecycle},
		{WorkerExports, bootstrap.StartExportWorker},
		{WorkerRestores, func(ctx context.Context, rs *bootstrap.RuntimeSetup, cfg *bootstrap.Config) func() {
			_, stop := bootstrap.StartRestoreWorker(ctx, rs, cfg)
			return stop
		}},
		{WorkerImports, bootstrap.StartImportWorker},
		{WorkerThumbnails, bootstrap.StartThumbnailWorker},
		{WorkerTokenCleanup, bootstrap.StartRefreshTokenCleanupWorker},
	}

	stops := make([]func(), 0, len(starters))
	defer func() {
		// LIFO shutdown to mirror the deferred-stop ordering of `run all`.
		for i := len(stops) - 1; i >= 0; i-- {
			stops[i]()
		}
	}()
	for _, s := range starters {
		if !selected.Has(s.id) {
			continue
		}
		stops = append(stops, s.start(ctx, rs, c.cfg))
	}

	active := selected.Sorted()
	activeStrs := make([]string, len(active))
	for i, id := range active {
		activeStrs[i] = string(id)
	}
	slog.Info("Workers started; waiting for shutdown signal",
		"active", strings.Join(activeStrs, ","),
		"count", len(active),
	)

	probeSrv, probeErrCh := bootstrap.StartProbes(c.cfg, rs)
	if err := bootstrap.WaitForWorkersShutdown(probeSrv, probeErrCh); err != nil {
		slog.Error("Shutting down workers after probe listener failure", "error", err)
		return err
	}

	slog.Info("Shutting down workers")
	return nil
}
