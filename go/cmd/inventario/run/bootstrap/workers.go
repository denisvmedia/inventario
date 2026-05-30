package bootstrap

import (
	"context"
	"log/slog"
	"strings"

	"github.com/denisvmedia/inventario/backup/export"
	importpkg "github.com/denisvmedia/inventario/backup/import"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/internal/metrics"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
	"github.com/denisvmedia/inventario/services/notifications"
)

// currencyMigrationOp is a local alias for *models.CurrencyMigration so
// the adapter signatures below stay readable.
type currencyMigrationOp = models.CurrencyMigration

// StartPauseController starts the background-worker soft-pause controller
// (#1308) and returns its stop function. When rs.PauseController is nil
// (ModeAPIServer, where no worker run loop exists to gate) it is a no-op.
// It must be started BEFORE the workers so the first worker tick observes
// the correct pause state, and stopped AFTER them.
func StartPauseController(ctx context.Context, rs *RuntimeSetup) func() {
	if rs.PauseController == nil {
		return func() {}
	}
	rs.PauseController.Start(ctx)
	return rs.PauseController.Stop
}

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
	opts := []export.WorkerOption{
		export.WithPollInterval(rs.WorkerDurations.ExportPollInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, export.WithPauseController(rs.PauseController))
	}
	worker := export.NewExportWorker(service, rs.FactorySet, cfg.MaxConcurrentExports, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartImportWorker wires and starts the import worker and returns its stop
// function.
func StartImportWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	service := importpkg.NewImportService(rs.FactorySet, rs.Params.UploadLocation)
	opts := []importpkg.WorkerOption{
		importpkg.WithPollInterval(rs.WorkerDurations.ImportPollInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, importpkg.WithPauseController(rs.PauseController))
	}
	worker := importpkg.NewImportWorker(service, rs.FactorySet, cfg.MaxConcurrentImports, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartRestoreWorker wires and starts the restore worker, returning both the
// worker (needed by the API server to satisfy the RestoreStatusQuerier
// interface) and its stop function.
func StartRestoreWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) (*restore.RestoreWorker, func()) {
	service := restore.NewRestoreService(rs.FactorySet, rs.Params.EntityService, rs.Params.UploadLocation)
	opts := []restore.WorkerOption{
		restore.WithPollInterval(rs.WorkerDurations.RestorePollInterval),
		restore.WithMaxConcurrent(cfg.MaxConcurrentRestores),
	}
	if rs.PauseController != nil {
		opts = append(opts, restore.WithPauseController(rs.PauseController))
	}
	worker := restore.NewRestoreWorker(
		service, rs.FactorySet.CreateServiceRegistrySet(), rs.Params.UploadLocation, opts...,
	)
	worker.Start(ctx)
	return worker, worker.Stop
}

// StartThumbnailWorker wires and starts the thumbnail generation worker and
// returns its stop function.
func StartThumbnailWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	opts := []services.ThumbnailWorkerOption{
		services.WithThumbnailPollInterval(rs.WorkerDurations.ThumbnailPollInterval),
		services.WithThumbnailBatchSize(cfg.ThumbnailBatchSize),
		services.WithThumbnailCleanupInterval(rs.WorkerDurations.ThumbnailCleanupInterval),
		services.WithThumbnailJobRetentionPeriod(rs.WorkerDurations.ThumbnailJobRetentionPeriod),
		services.WithThumbnailJobBatchTimeout(rs.WorkerDurations.ThumbnailJobBatchTimeout),
		services.WithDetachedThumbnailJobTimeout(rs.WorkerDurations.DetachedThumbnailJobTimeout),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithThumbnailPauseController(rs.PauseController))
	}
	worker := services.NewThumbnailGenerationWorker(
		rs.FactorySet, rs.Params.UploadLocation, rs.Params.ThumbnailConfig, opts...,
	)
	worker.Start(ctx)
	return worker.Stop
}

// StartRefreshTokenCleanupWorker wires and starts the refresh token cleanup
// worker (which deletes expired tokens on the configured interval) and returns
// its stop function.
func StartRefreshTokenCleanupWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	opts := []services.RefreshTokenCleanupOption{
		services.WithRefreshTokenCleanupInterval(rs.WorkerDurations.RefreshTokenCleanupInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithRefreshTokenCleanupPauseController(rs.PauseController))
	}
	worker := services.NewRefreshTokenCleanupWorker(rs.FactorySet.RefreshTokenRegistry, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartLoginEventRetentionWorker wires and starts the login_events
// retention worker (#1379). The retention window and sweep interval
// are not currently surfaced as flags — the defaults (90d retention,
// 24h sweep) match the design doc and are conservative enough that
// adding a knob is YAGNI for the v1 surface.
func StartLoginEventRetentionWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	var opts []services.LoginEventRetentionOption
	if rs.PauseController != nil {
		opts = append(opts, services.WithLoginEventRetentionPauseController(rs.PauseController))
	}
	worker := services.NewLoginEventRetentionWorker(rs.FactorySet.LoginEventRegistry, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartGroupPurgeWorker wires and starts the group purge worker (which hard-
// deletes LocationGroups marked pending_deletion and cleans up expired unused
// invites on the configured interval) and returns its stop function.
func StartGroupPurgeWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	fileService := services.NewFileService(rs.FactorySet, rs.Params.UploadLocation)
	service := services.NewGroupPurgeService(rs.FactorySet, fileService)
	opts := []services.GroupPurgeOption{
		services.WithGroupPurgeInterval(rs.WorkerDurations.GroupPurgeInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithGroupPurgePauseController(rs.PauseController))
	}
	worker := services.NewGroupPurgeWorker(service, opts...)
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
	opts := []services.WarrantyReminderOption{
		services.WithWarrantyReminderInterval(rs.WorkerDurations.WarrantyReminderInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithWarrantyReminderPauseController(rs.PauseController))
	}
	worker := services.NewWarrantyReminderWorker(service, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartLoanReminderWorker wires and starts the loan reminder worker
// (#1509). Mirrors the warranty / storage-quota wiring: takes the
// configured interval from rs.WorkerDurations and pulls the public URL
// from cfg for the deep-link block in the email template. The async
// email service comes from rs.EmailLifecycle (already started by
// StartEmailLifecycle in the housekeeping group). Per-user notification
// preferences gate the per-recipient send via notifications.IsEnabled
// against notifications.CategoryLoanReminder.
func StartLoanReminderWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	urlBuilder := buildCommodityURLBuilder(cfg.PublicURL)
	prefs := notifications.NewService(rs.FactorySet.SettingsRegistryFactory)
	prefs.SetGroupPrefs(rs.FactorySet.GroupNotificationPrefRegistry)
	service := services.NewLoanReminderService(rs.FactorySet, rs.EmailLifecycle.Service, urlBuilder).
		WithPreferences(prefs).
		WithDueSoonDays(cfg.LoanReminderDueSoonDays)
	opts := []services.LoanReminderOption{
		services.WithLoanReminderInterval(rs.WorkerDurations.LoanReminderInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithLoanReminderPauseController(rs.PauseController))
	}
	worker := services.NewLoanReminderWorker(service, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartStorageQuotaReminderWorker wires and starts the storage quota
// warning worker (#1585). Uses the configured interval from
// rs.WorkerDurations and pulls the public URL from cfg for the two
// deep-link blocks in the email template. The async email service
// comes from rs.EmailLifecycle (already started by
// StartEmailLifecycle in the housekeeping group).
func StartStorageQuotaReminderWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	filesURL, settingsURL := buildStorageQuotaURLBuilders(cfg.PublicURL)
	service := services.NewStorageQuotaReminderService(rs.FactorySet, rs.EmailLifecycle.Service, filesURL, settingsURL)
	opts := []services.StorageQuotaReminderOption{
		services.WithStorageQuotaReminderInterval(rs.WorkerDurations.StorageQuotaReminderInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithStorageQuotaReminderPauseController(rs.PauseController))
	}
	worker := services.NewStorageQuotaReminderWorker(service, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartBusinessMetricsWorker wires and starts the installation-wide
// business-metrics collector (#843). It mirrors the reminder-worker
// wiring: pulls the configured interval from rs.WorkerDurations and owns
// a single goroutine with a graceful stop.
//
// The collector reads through rs.FactorySet.SystemStats, which bypasses
// tenant scoping (the postgres source runs under the background-worker
// role), so this MUST run in exactly one producer — the housekeeping
// worker group / `run all` — never the apiserver path, otherwise split
// deployments would publish duplicate series for the same gauges.
//
// Returns a no-op stop when no SystemStats source is configured (e.g. a
// backend that left the field nil); the collector itself also no-ops on
// a nil source, but short-circuiting here keeps the log honest.
func StartBusinessMetricsWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	if rs.FactorySet.SystemStats == nil {
		slog.Info("Business metrics collector disabled; no SystemStats source configured")
		return func() {}
	}

	// Adapt registry.SystemStats → metrics.BusinessStats field-by-field.
	// The explicit copy (rather than a struct cast) keeps the metrics
	// package free of any registry import and fails at compile time here
	// if either struct drifts.
	source := func(ctx context.Context) (metrics.BusinessStats, error) {
		s, err := rs.FactorySet.SystemStats(ctx)
		if err != nil {
			return metrics.BusinessStats{}, err
		}
		return systemStatsToBusinessStats(s), nil
	}

	coll := metrics.NewBusinessCollector(source, rs.WorkerDurations.BusinessMetricsInterval)
	coll.Start(ctx)
	return coll.Stop
}

// systemStatsToBusinessStats copies the registry-layer SystemStats
// snapshot into the metrics-layer BusinessStats the collector consumes.
// Kept as an explicit, separately-testable function so the field mapping
// is asserted in a unit test without needing a database.
func systemStatsToBusinessStats(s registry.SystemStats) metrics.BusinessStats {
	return metrics.BusinessStats{
		Tenants:        s.Tenants,
		Users:          s.Users,
		LocationGroups: s.LocationGroups,
		Locations:      s.Locations,
		Areas:          s.Areas,
		Commodities:    s.Commodities,
		Files:          s.Files,

		StorageImages:    s.StorageImages,
		StorageDocuments: s.StorageDocuments,
		StorageOther:     s.StorageOther,
		StorageExports:   s.StorageExports,
	}
}

// StartMaintenanceReminderWorker wires and starts the maintenance
// reminder worker (#1368). Mirrors StartWarrantyReminderWorker — wires
// the same notifications.Service so per-user / per-group opt-out
// toggles are honoured.
func StartMaintenanceReminderWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	urlBuilder := buildCommodityURLBuilder(cfg.PublicURL)
	prefs := notifications.NewService(rs.FactorySet.SettingsRegistryFactory)
	prefs.SetGroupPrefs(rs.FactorySet.GroupNotificationPrefRegistry)
	service := services.NewMaintenanceReminderService(rs.FactorySet, rs.EmailLifecycle.Service, urlBuilder).WithPreferences(prefs)
	opts := []services.MaintenanceReminderOption{
		services.WithMaintenanceReminderInterval(rs.WorkerDurations.MaintenanceReminderInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithMaintenanceReminderPauseController(rs.PauseController))
	}
	worker := services.NewMaintenanceReminderWorker(service, opts...)
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
	opts := []services.CurrencyMigrationWorkerOption{
		services.WithCurrencyMigrationActiveInterval(rs.WorkerDurations.CurrencyMigrationInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithCurrencyMigrationPauseController(rs.PauseController))
	}
	worker := services.NewCurrencyMigrationWorker(
		rs.FactorySet.CurrencyMigrationRegistryFactory.CreateServiceRegistry(),
		adapter,
		opts...,
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

// buildStorageQuotaURLBuilders returns the per-group files URL +
// settings URL builders passed to StorageQuotaReminderService.
// Returns (nil, nil) when no PublicURL is configured — the email
// template suppresses each link block in that case rather than
// printing a relative URL.
func buildStorageQuotaURLBuilders(publicURL string) (filesURLBuilder, settingsURLBuilder func(string) string) {
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if publicURL == "" {
		return nil, nil
	}
	filesURLBuilder = func(groupSlug string) string {
		if groupSlug == "" {
			return ""
		}
		return publicURL + "/g/" + groupSlug + "/files"
	}
	settingsURLBuilder = func(groupSlug string) string {
		if groupSlug == "" {
			return ""
		}
		return publicURL + "/g/" + groupSlug + "/settings"
	}
	return filesURLBuilder, settingsURLBuilder
}
