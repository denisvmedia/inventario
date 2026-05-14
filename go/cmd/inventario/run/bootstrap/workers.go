package bootstrap

import (
	"context"
	"log/slog"
	"strings"

	"github.com/denisvmedia/inventario/backup/export"
	importpkg "github.com/denisvmedia/inventario/backup/import"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
	"github.com/denisvmedia/inventario/services/notifications"
)

// currencyMigrationOp is a local alias for *models.CurrencyMigration so
// the adapter signatures below stay readable.
type currencyMigrationOp = models.CurrencyMigration

// StartEmailLifecycle starts the configured email service (if any) and returns
// the matching stop function. cfg is accepted (and ignored) so the signature
// matches the other Start* helpers and lets them be stored in a uniform slice.
func StartEmailLifecycle(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	rs.EmailLifecycle.Start(ctx)
	return rs.EmailLifecycle.Stop
}

// StartExportWorker wires and starts the export worker and returns its stop
// function.
func StartExportWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	service := export.NewExportService(rs.FactorySet, rs.Params.UploadLocation)
	worker := export.NewExportWorker(
		service, rs.FactorySet, cfg.MaxConcurrentExports,
		export.WithPollInterval(rs.WorkerDurations.ExportPollInterval),
	)
	worker.Start(ctx)
	return worker.Stop
}

// StartImportWorker wires and starts the import worker and returns its stop
// function.
func StartImportWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	service := importpkg.NewImportService(rs.FactorySet, rs.Params.UploadLocation)
	worker := importpkg.NewImportWorker(
		service, rs.FactorySet, cfg.MaxConcurrentImports,
		importpkg.WithPollInterval(rs.WorkerDurations.ImportPollInterval),
	)
	worker.Start(ctx)
	return worker.Stop
}

// StartRestoreWorker wires and starts the restore worker, returning both the
// worker (needed by the API server to satisfy the RestoreStatusQuerier
// interface) and its stop function.
func StartRestoreWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) (*restore.RestoreWorker, func()) {
	service := restore.NewRestoreService(rs.FactorySet, rs.Params.EntityService, rs.Params.UploadLocation)
	worker := restore.NewRestoreWorker(
		service, rs.FactorySet.CreateServiceRegistrySet(), rs.Params.UploadLocation,
		restore.WithPollInterval(rs.WorkerDurations.RestorePollInterval),
		restore.WithMaxConcurrent(cfg.MaxConcurrentRestores),
	)
	worker.Start(ctx)
	return worker, worker.Stop
}

// StartThumbnailWorker wires and starts the thumbnail generation worker and
// returns its stop function.
func StartThumbnailWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	worker := services.NewThumbnailGenerationWorker(
		rs.FactorySet, rs.Params.UploadLocation, rs.Params.ThumbnailConfig,
		services.WithThumbnailPollInterval(rs.WorkerDurations.ThumbnailPollInterval),
		services.WithThumbnailBatchSize(cfg.ThumbnailBatchSize),
		services.WithThumbnailCleanupInterval(rs.WorkerDurations.ThumbnailCleanupInterval),
		services.WithThumbnailJobRetentionPeriod(rs.WorkerDurations.ThumbnailJobRetentionPeriod),
		services.WithThumbnailJobBatchTimeout(rs.WorkerDurations.ThumbnailJobBatchTimeout),
		services.WithDetachedThumbnailJobTimeout(rs.WorkerDurations.DetachedThumbnailJobTimeout),
	)
	worker.Start(ctx)
	return worker.Stop
}

// StartRefreshTokenCleanupWorker wires and starts the refresh token cleanup
// worker (which deletes expired tokens on the configured interval) and returns
// its stop function.
func StartRefreshTokenCleanupWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	worker := services.NewRefreshTokenCleanupWorker(
		rs.FactorySet.RefreshTokenRegistry,
		services.WithRefreshTokenCleanupInterval(rs.WorkerDurations.RefreshTokenCleanupInterval),
	)
	worker.Start(ctx)
	return worker.Stop
}

// StartLoginEventRetentionWorker wires and starts the login_events
// retention worker (#1379). The retention window and sweep interval
// are not currently surfaced as flags — the defaults (90d retention,
// 24h sweep) match the design doc and are conservative enough that
// adding a knob is YAGNI for the v1 surface.
func StartLoginEventRetentionWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	worker := services.NewLoginEventRetentionWorker(rs.FactorySet.LoginEventRegistry)
	worker.Start(ctx)
	return worker.Stop
}

// StartGroupPurgeWorker wires and starts the group purge worker (which hard-
// deletes LocationGroups marked pending_deletion and cleans up expired unused
// invites on the configured interval) and returns its stop function.
func StartGroupPurgeWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	fileService := services.NewFileService(rs.FactorySet, rs.Params.UploadLocation)
	service := services.NewGroupPurgeService(rs.FactorySet, fileService)
	worker := services.NewGroupPurgeWorker(
		service,
		services.WithGroupPurgeInterval(rs.WorkerDurations.GroupPurgeInterval),
	)
	worker.Start(ctx)
	return worker.Stop
}

// StartWarrantyReminderWorker wires and starts the warranty reminder
// worker (#1367). Mirrors the group-purge wiring: takes the configured
// interval from rs.WorkerDurations and pulls the public URL from cfg
// for the deep-link block in the email template. The async email
// service comes from rs.EmailLifecycle (already started by
// StartEmailLifecycle in the housekeeping group).
func StartWarrantyReminderWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	urlBuilder := buildCommodityURLBuilder(cfg.PublicURL)
	// Per-user notification preferences gate the per-recipient email
	// fan-out (see notifications.IsEnabled). Wired here so the worker
	// respects the warranty_expiry / channel.email toggles users flip
	// from Settings → Notifications. The per-group override registry
	// (issue #1648) lets the same fan-out additionally honour a
	// user's per-group opt-out without changing the call signature
	// on the warranty side — see Service.IsEnabledForGroup.
	prefs := notifications.NewService(rs.FactorySet.SettingsRegistryFactory)
	prefs.SetGroupPrefs(rs.FactorySet.GroupNotificationPrefRegistry)
	service := services.NewWarrantyReminderService(rs.FactorySet, rs.EmailLifecycle.Service, urlBuilder).WithPreferences(prefs)
	worker := services.NewWarrantyReminderWorker(
		service,
		services.WithWarrantyReminderInterval(rs.WorkerDurations.WarrantyReminderInterval),
	)
	worker.Start(ctx)
	return worker.Stop
}

// StartCurrencyMigrationWorker wires and starts the currency migration
// worker (#1552 / #202 §4.5). Returns a no-op stop function when the
// feature flag is off OR the active backend is not postgres — TX2 of
// the migration lifecycle is postgres-only by design (advisory lock,
// SET LOCAL role, transactional audit_logs insert), so a memory or
// future non-postgres backend simply never schedules work. The
// CurrencyMigrationRegistryFactory always exposes the registry side so
// the apiserver endpoints stay live.
func StartCurrencyMigrationWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	if !cfg.FeatureCurrencyMigration {
		slog.Info("Currency migration worker disabled; FEATURE_CURRENCY_MIGRATION is off")
		return func() {}
	}
	pgFactory, ok := rs.FactorySet.CurrencyMigrationRegistryFactory.(*postgres.CurrencyMigrationRegistryFactory)
	if !ok {
		// Memory backend or any future stub. The conversion service +
		// audit + advisory lock surface lives in the postgres package,
		// so a non-postgres deployment cannot run TX2 — log loudly so
		// operators don't think the worker silently succeeded.
		slog.Warn("Currency migration worker: skipping startup, backend is not postgres")
		return func() {}
	}

	processor := pgFactory.NewProcessor()
	adapter := &currencyMigrationProcessorAdapter{inner: processor}
	worker := services.NewCurrencyMigrationWorker(
		rs.FactorySet.CurrencyMigrationRegistryFactory.CreateServiceRegistry(),
		adapter,
		services.WithCurrencyMigrationActiveInterval(rs.WorkerDurations.CurrencyMigrationInterval),
	)
	worker.Start(ctx)
	return worker.Stop
}

// currencyMigrationProcessorAdapter bridges the concrete postgres
// processor to the services.CurrencyMigrationProcessor interface. The
// summary types are field-compatible so the conversion is a single
// struct cast — kept as an explicit adapter (rather than relying on
// implicit interface satisfaction) so a future drift in either struct
// fails at compile time here, not in a downstream caller.
type currencyMigrationProcessorAdapter struct {
	inner *postgres.CurrencyMigrationProcessor
}

func (a *currencyMigrationProcessorAdapter) ProcessRunningMigration(ctx context.Context, op *currencyMigrationOp) (services.CurrencyMigrationProcessSummary, error) {
	res, err := a.inner.ProcessRunningMigration(ctx, op)
	return services.CurrencyMigrationProcessSummary(res), err
}

func (a *currencyMigrationProcessorAdapter) WriteSweepFailureAuditLog(ctx context.Context, op *currencyMigrationOp) error {
	return a.inner.WriteSweepFailureAuditLog(ctx, op)
}

// buildCommodityURLBuilder returns the per-commodity URL builder
// passed to WarrantyReminderService. Returns nil when no PublicURL is
// configured — the email template suppresses the link block in that
// case rather than printing a relative URL.
func buildCommodityURLBuilder(publicURL string) func(string, string) string {
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if publicURL == "" {
		return nil
	}
	return func(groupSlug, commodityID string) string {
		if groupSlug == "" || commodityID == "" {
			return ""
		}
		return publicURL + "/g/" + groupSlug + "/commodities/" + commodityID
	}
}
