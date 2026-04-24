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
		Long: `Start every background worker group (archive, media, emails, housekeeping)
without opening the application HTTP listener.

This mode is intended for split deployments where the API server runs as a
separate "inventario run apiserver" process. Email delivery workers only run
here, so the API server can safely enqueue messages without producing
duplicate deliveries.

Groups bundle worker families that share an operational profile:
  * archive       - exports, imports, restores (long-running, I/O + DB heavy)
  * emails        - SMTP / provider-rate-limited delivery lifecycle
  * housekeeping  - periodic maintenance (refresh token GC, and follow-ups)
  * media         - CPU/RAM-heavy media processing (thumbnails)

A minimal observability listener is started on --probe-addr exposing /healthz,
/readyz and /metrics so Kubernetes liveness/readiness probes and Prometheus
scrapes work uniformly across "run apiserver" and "run workers" deployments.

Use --workers-only or --workers-exclude to isolate a subset of groups onto a
dedicated pod/host. Valid group identifiers: archive, emails, housekeeping,
media. Legacy per-family names (exports, imports, restores, thumbnails,
token-cleanup) are accepted for one release and mapped to their owning group
with a deprecation warning. "--workers-only=all" is an explicit synonym for
the default (every group). The two flags are mutually exclusive.`,
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
		"Comma-separated list of worker-group identifiers to run exclusively "+
			"(e.g., media,archive). Valid groups: archive, emails, housekeeping, media. "+
			"Legacy family names (exports, imports, restores, thumbnails, token-cleanup) "+
			"are accepted with a deprecation warning. Use \"all\" or leave empty to run "+
			"every group. Mutually exclusive with --workers-exclude.")
	flags.StringVar(&c.cfg.WorkersExclude, "workers-exclude", c.cfg.WorkersExclude,
		"Comma-separated list of worker-group identifiers to skip (e.g., emails). "+
			"Mutually exclusive with --workers-only.")
	flags.StringVar(&c.cfg.ProbeAddr, "probe-addr", c.cfg.ProbeAddr,
		"Bind address for the workers' probe listener that serves /healthz, /readyz and /metrics.")
}

// starter is the shared shape every bootstrap.Start* helper reduces to after
// wrapping. It returns a stop function that the run loop pushes onto a LIFO
// shutdown stack.
type starter func(context.Context, *bootstrap.RuntimeSetup, *bootstrap.Config) func()

// group bundles one or more starters under a single operational group id.
// Members of a group share a process, a DB/Redis connection pool, and a
// Deployment when running in split mode; see the Long description on the
// workers subcommand for the operational profile each group targets.
type group struct {
	id       WorkerID
	starters []starter
}

// run starts the worker groups selected by --workers-only / --workers-exclude
// (all four by default) without opening the HTTP listener. It blocks until
// the process receives SIGINT or SIGTERM.
func (c *Command) run() error {
	selected, deprecated, err := ParseSelector(c.cfg.WorkersOnly, c.cfg.WorkersExclude)
	if err != nil {
		slog.Error("Invalid worker selector", "error", err)
		return err
	}
	for _, d := range deprecated {
		slog.Warn("Worker identifier is deprecated; update --workers-only/--workers-exclude to the group name",
			"alias", d.Alias,
			"canonical", string(d.Canonical),
		)
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

	// Group and starter order matches the historical `run all` / `run workers`
	// sequence so LIFO shutdown is unchanged for the default (all-groups) case.
	groups := []group{
		{WorkerEmails, []starter{bootstrap.StartEmailLifecycle}},
		{WorkerArchive, []starter{
			bootstrap.StartExportWorker,
			func(ctx context.Context, rs *bootstrap.RuntimeSetup, cfg *bootstrap.Config) func() {
				_, stop := bootstrap.StartRestoreWorker(ctx, rs, cfg)
				return stop
			},
			bootstrap.StartImportWorker,
		}},
		{WorkerMedia, []starter{bootstrap.StartThumbnailWorker}},
		{WorkerHousekeeping, []starter{
			bootstrap.StartRefreshTokenCleanupWorker,
			bootstrap.StartGroupPurgeWorker,
		}},
	}

	stops := make([]func(), 0, 8)
	defer func() {
		// LIFO shutdown to mirror the deferred-stop ordering of `run all`.
		for i := len(stops) - 1; i >= 0; i-- {
			stops[i]()
		}
	}()
	for _, g := range groups {
		if !selected.Has(g.id) {
			continue
		}
		for _, start := range g.starters {
			stops = append(stops, start(ctx, rs, c.cfg))
		}
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
