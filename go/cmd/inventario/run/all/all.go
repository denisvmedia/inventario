// Package all implements the `inventario run all` subcommand, which starts
// the HTTP API server together with every background worker in a single
// process. It is the default single-process mode and is also invoked by bare
// `inventario run` for backward compatibility.
package all

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/services"
)

type Command struct {
	command.Base

	cfg      *bootstrap.Config
	dbConfig *shared.DatabaseConfig
}

// New constructs the `run all` subcommand. cfg and dbConfig are shared with the
// parent `run` command so every subcommand observes the same resolved flag
// values.
func New(cfg *bootstrap.Config, dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{cfg: cfg, dbConfig: dbConfig}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "all",
		Short: "Start the API server and every background worker",
		Long: `Start the HTTP API server together with every background worker.

This is equivalent to invoking "inventario run" without a subcommand and is the
default, single-process mode.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run()
		},
	})

	return c
}

// Run exposes the same startup sequence used by `run all` so the parent
// `inventario run` (with no subcommand) can delegate to it without going
// through cobra a second time.
func Run(cfg *bootstrap.Config, dbConfig *shared.DatabaseConfig) error {
	return (&Command{cfg: cfg, dbConfig: dbConfig}).run()
}

// run wires the API server, every background worker and the email lifecycle
// together as a composition of small, independently startable primitives.
func (c *Command) run() error {
	rs, err := bootstrap.Build(c.cfg, c.dbConfig, bootstrap.ModeAll)
	if err != nil {
		return err
	}
	defer rs.CloseReadinessRedisPinger()
	// Flush buffered Sentry events on shutdown (#844); no-op when disabled.
	defer rs.SentryFlush(2 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the soft-pause controller (#1308) before any worker so the
	// first worker tick observes the correct pause state, and defer its
	// stop FIRST so it shuts down LAST (after every worker has stopped).
	stopPause := bootstrap.StartPauseController(ctx, rs)
	defer stopPause()

	stopEmail := bootstrap.StartEmailLifecycle(ctx, rs, c.cfg)
	defer stopEmail()

	stopExport := bootstrap.StartExportWorker(ctx, rs, c.cfg)
	defer stopExport()

	restoreWorker, stopRestore := bootstrap.StartRestoreWorker(ctx, rs, c.cfg)
	defer stopRestore()

	stopImport := bootstrap.StartImportWorker(ctx, rs, c.cfg)
	defer stopImport()

	stopThumbnail := bootstrap.StartThumbnailWorker(ctx, rs, c.cfg)
	defer stopThumbnail()

	stopRefreshTokenCleanup := bootstrap.StartRefreshTokenCleanupWorker(ctx, rs, c.cfg)
	defer stopRefreshTokenCleanup()

	stopEmailVerificationCleanup := bootstrap.StartEmailVerificationCleanupWorker(ctx, rs, c.cfg)
	defer stopEmailVerificationCleanup()

	stopMagicLinkTokenCleanup := bootstrap.StartMagicLinkTokenCleanupWorker(ctx, rs, c.cfg)
	defer stopMagicLinkTokenCleanup()

	stopOperationSlotCleanup := bootstrap.StartOperationSlotCleanupWorker(ctx, rs, c.cfg)
	defer stopOperationSlotCleanup()

	stopLoginEventRetention := bootstrap.StartLoginEventRetentionWorker(ctx, rs, c.cfg)
	defer stopLoginEventRetention()

	stopGroupPurge := bootstrap.StartGroupPurgeWorker(ctx, rs, c.cfg)
	defer stopGroupPurge()

	stopOrphanFileGC := bootstrap.StartOrphanFileGCWorker(ctx, rs, c.cfg)
	defer stopOrphanFileGC()

	stopWarrantyReminder := bootstrap.StartWarrantyReminderWorker(ctx, rs, c.cfg)
	defer stopWarrantyReminder()

	stopStorageQuotaReminder := bootstrap.StartStorageQuotaReminderWorker(ctx, rs, c.cfg)
	defer stopStorageQuotaReminder()

	stopLoanReminder := bootstrap.StartLoanReminderWorker(ctx, rs, c.cfg)
	defer stopLoanReminder()
	stopMaintenanceReminder := bootstrap.StartMaintenanceReminderWorker(ctx, rs, c.cfg)
	defer stopMaintenanceReminder()

	stopCurrencyMigration := bootstrap.StartCurrencyMigrationWorker(ctx, rs, c.cfg)
	defer stopCurrencyMigration()

	stopBusinessMetrics := bootstrap.StartBusinessMetricsWorker(ctx, rs, c.cfg)
	defer stopBusinessMetrics()

	// One-shot backfill of files.size_bytes for rows that pre-date #1388.
	// Runs in a goroutine so a slow bucket walk can't delay readiness;
	// errors are swallowed (logged at debug) so a partial blob outage
	// can't block startup.
	go services.BackfillFileSizes(ctx, rs.FactorySet, rs.Params.UploadLocation)

	srv, errCh := bootstrap.StartAPIServer(c.cfg, rs, restoreWorker)
	return bootstrap.WaitForShutdown(srv, errCh)
}
