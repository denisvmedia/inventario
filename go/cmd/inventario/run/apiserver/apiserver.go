// Package apiserver implements the `inventario run apiserver` subcommand,
// which starts only the HTTP API server without any background worker
// goroutines. It is intended for split deployments where a separate
// `inventario run workers` process handles exports, imports, restores,
// thumbnails, refresh token cleanup and email delivery.
package apiserver

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

type Command struct {
	command.Base

	cfg      *bootstrap.Config
	dbConfig *shared.DatabaseConfig
}

// New constructs the `run apiserver` subcommand. cfg and dbConfig are shared
// with the parent `run` command so every subcommand observes the same resolved
// flag values.
func New(cfg *bootstrap.Config, dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{cfg: cfg, dbConfig: dbConfig}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "apiserver",
		Short: "Start the HTTP API server only (no background workers)",
		Long: `Start only the HTTP API server. No background worker goroutines are started.

In this mode the API server uses a registry-backed RestoreStatusQuerier so it
can still enforce the "one active restore at a time" invariant. The email
service is wired in for enqueueing transactional emails, but its delivery
workers are not started here. The matching "inventario run workers" process
(typically on another host) must be running to actually process exports,
imports, restores, thumbnails, token cleanup and email delivery.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run()
		},
	})

	return c
}

// run starts the HTTP API server without any background worker goroutines. It
// uses a registry-backed RestoreStatusQuerier so the API can still enforce the
// "one active restore at a time" invariant in deployments where the
// RestoreWorker runs in a separate process.
//
// The email service is constructed (and exposed on apiserver.Params so handlers
// can enqueue transactional emails), but its delivery lifecycle is not started
// here: async providers spin up worker goroutines that pop messages off the
// queue, which in a split deployment must only run in `inventario run workers`
// — otherwise both processes would race on the same queue and cause duplicate
// delivery.
func (c *Command) run() error {
	rs, err := bootstrap.Build(c.cfg, c.dbConfig, bootstrap.ModeAPIServer)
	if err != nil {
		return err
	}
	defer rs.CloseReadinessRedisPinger()

	restoreStatus := restore.NewRegistryStatusQuerier(rs.FactorySet.CreateServiceRegistrySet())
	srv, errCh := bootstrap.StartAPIServer(c.cfg, rs, restoreStatus)
	return bootstrap.WaitForShutdown(srv, errCh)
}
