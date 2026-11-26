package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.5x5.cz/inventario/backup/export"
	importpkg "go.5x5.cz/inventario/backup/import"
	"go.5x5.cz/inventario/backup/restore"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/notifications"
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
	service := export.NewExportService(rs.FactorySet, rs.Params.UploadLocation, rs.Params.BackupSigner)
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
	service := importpkg.NewImportService(rs.FactorySet, rs.Params.UploadLocation, rs.Params.BackupSigner)
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
	service := restore.NewRestoreService(rs.FactorySet, rs.Params.EntityService, rs.Params.UploadLocation, rs.Params.BackupSigner)
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

// StartIntakeWorker wires and starts the worker that scans the files of batch
// intake sessions (#2810) and returns its stop function. Without an AI scan
// provider the worker does not start.
func StartIntakeWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	var opts []services.IntakeWorkerOption
	if rs.PauseController != nil {
		opts = append(opts, services.WithIntakePauseController(rs.PauseController))
	}
	worker := services.NewIntakeWorker(rs.FactorySet, rs.Params.CommodityScanService, rs.Params.UploadLocation, opts...)
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

// StartEmailVerificationCleanupWorker wires and starts the email verification
// cleanup worker (which deletes expired verification tokens on the configured
// interval) and returns its stop function.
func StartEmailVerificationCleanupWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	opts := []services.EmailVerificationCleanupOption{
		services.WithEmailVerificationCleanupInterval(rs.WorkerDurations.EmailVerificationCleanupInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithEmailVerificationCleanupPauseController(rs.PauseController))
	}
	worker := services.NewEmailVerificationCleanupWorker(rs.FactorySet.EmailVerificationRegistry, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartMagicLinkTokenCleanupWorker wires and starts the magic-link token
// cleanup worker (which deletes expired sign-in tokens on the configured
// interval) and returns its stop function.
func StartMagicLinkTokenCleanupWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	opts := []services.MagicLinkTokenCleanupOption{
		services.WithMagicLinkTokenCleanupInterval(rs.WorkerDurations.MagicLinkTokenCleanupInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithMagicLinkTokenCleanupPauseController(rs.PauseController))
	}
	worker := services.NewMagicLinkTokenCleanupWorker(rs.FactorySet.MagicLinkTokenRegistry, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartOperationSlotCleanupWorker wires and starts the operation-slot cleanup
// worker (#2122 F4), which deletes expired operation-slot rows on the
// configured interval, and returns its stop function. It uses the service-mode
// registry because the sweep spans all tenants/users (no request context).
func StartOperationSlotCleanupWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	opts := []services.OperationSlotCleanupOption{
		services.WithOperationSlotCleanupInterval(rs.WorkerDurations.OperationSlotCleanupInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithOperationSlotCleanupPauseController(rs.PauseController))
	}
	worker := services.NewOperationSlotCleanupWorker(
		rs.FactorySet.OperationSlotRegistryFactory.CreateServiceRegistry(), opts...,
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

// StartTrashPurgeWorker wires and starts the worker that deletes Trash
// entries older than --trash-retention and returns its stop function.
func StartTrashPurgeWorker(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	trash := services.NewTrashService(rs.FactorySet, rs.Params.UploadLocation)
	opts := []services.TrashPurgeOption{
		services.WithTrashRetention(rs.WorkerDurations.TrashRetention),
		services.WithTrashPurgeInterval(rs.WorkerDurations.TrashPurgeInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithTrashPurgePauseController(rs.PauseController))
	}
	worker := services.NewTrashPurgeWorker(trash, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// StartOrphanFileGCWorker wires and starts the orphan-file GC sweeper (#2237)
// and returns its stop function.
//
// This is the only DESTRUCTIVE periodic worker in the tree. Two things about
// the wiring are load-bearing:
//
//  1. Every registry it reads is a SERVICE (RLS-bypassing) registry, and every
//     one of them is read-only. In particular the linked-entity existence
//     probes MUST be service-mode and match BY ID ONLY: PUT /files/{id} does
//     not validate the link target's group, so a file legitimately linked to an
//     entity in ANOTHER group exists in production, and a group-scoped probe
//     would read that LIVE entity as missing and hand the GC a live file.
//  2. Deletion goes exclusively through services.FileService.DeleteFileWithPhysical,
//     which internally creates a USER-scoped registry. The worker impersonates
//     the file's own creator + group, so the DELETE statement executes under RLS
//     bound to the file's own (tenant, group): discovery is broad, destruction
//     is narrow.
//
// It ships in report mode (see Config.OrphanFileGCMode) and therefore deletes
// nothing until an operator explicitly opts in.
func StartOrphanFileGCWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	fs := rs.FactorySet
	fileService := services.NewFileService(fs, rs.Params.UploadLocation)

	// Service-mode (RLS-bypassing), read-only. Held for the process lifetime,
	// matching StartOperationSlotCleanupWorker.
	commodityReg := fs.CommodityRegistryFactory.CreateServiceRegistry()
	areaReg := fs.AreaRegistryFactory.CreateServiceRegistry()
	locationReg := fs.LocationRegistryFactory.CreateServiceRegistry()

	deps := services.OrphanFileGCDeps{
		Files: fs.FileRegistryFactory.CreateServiceRegistry(),
		Probes: services.OrphanFileGCProbes{
			// A trashed item still exists: it can be restored with its
			// files, so a link to it is not dangling.
			Commodity: func(ctx context.Context, id string) error {
				_, err := commodityReg.Get(ctx, id)
				if errors.Is(err, registry.ErrNotFound) {
					_, err = commodityReg.GetTrashed(ctx, id)
				}
				return err
			},
			Area: func(ctx context.Context, id string) error {
				_, err := areaReg.Get(ctx, id)
				return err
			},
			Location: func(ctx context.Context, id string) error {
				_, err := locationReg.Get(ctx, id)
				return err
			},
		},
		Exports:        fs.ExportRegistryFactory.CreateServiceRegistry(),
		Restores:       fs.RestoreOperationRegistryFactory.CreateServiceRegistry(),
		Tenants:        fs.TenantRegistry,
		Groups:         fs.LocationGroupRegistry,
		Users:          fs.UserRegistry,
		Deleter:        fileService,
		UploadLocation: rs.Params.UploadLocation,
	}

	// Both values were already validated in ParseWorkerDurations (positive
	// interval, min-age at or above the hard floor, known mode), so the
	// options here cannot silently fall back.
	mode, _ := services.ParseOrphanFileGCMode(cfg.OrphanFileGCMode)
	opts := []services.OrphanFileGCOption{
		services.WithOrphanFileGCInterval(rs.WorkerDurations.OrphanFileGCInterval),
		services.WithOrphanFileGCMinAge(rs.WorkerDurations.OrphanFileGCMinAge),
		services.WithOrphanFileGCMode(mode),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithOrphanFileGCPauseController(rs.PauseController))
	}

	worker := services.NewOrphanFileGCWorker(deps, opts...)
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

// StartWeeklyDigestWorker wires and starts the weekly digest worker (#1391).
//
// Unlike the reminder workers this one has a schedule of its own: it ticks
// hourly and sends only on Monday at the configured UTC hour, so the digest
// lands on the calendar rather than on whatever moment the process started.
// Per-group notification preferences gate each group's contribution against
// notifications.CategoryWeeklyDigest.
func StartWeeklyDigestWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	appURL, settingsURL := buildWeeklyDigestURLBuilders(cfg.PublicURL)
	prefs := notifications.NewService(rs.FactorySet.SettingsRegistryFactory)
	prefs.SetGroupPrefs(rs.FactorySet.GroupNotificationPrefRegistry)
	service := services.NewWeeklyDigestService(
		rs.FactorySet,
		rs.EmailLifecycle.Service,
		appURL,
		settingsURL,
		buildCommodityURLBuilder(cfg.PublicURL),
	).WithPreferences(prefs)

	opts := []services.WeeklyDigestOption{
		services.WithWeeklyDigestInterval(rs.WorkerDurations.WeeklyDigestInterval),
		services.WithWeeklyDigestSendHour(cfg.WeeklyDigestSendHourUTC),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithWeeklyDigestPauseController(rs.PauseController))
	}
	worker := services.NewWeeklyDigestWorker(service, opts...)
	worker.Start(ctx)
	return worker.Stop
}

// buildWeeklyDigestURLBuilders returns the "open the app" and "notification
// settings" link builders for the digest email. Both return empty when no
// public URL is configured, and the template then drops the link.
func buildWeeklyDigestURLBuilders(publicURL string) (appURLBuilder, settingsURLBuilder func(string) string) {
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if publicURL == "" {
		return nil, nil
	}
	appURLBuilder = func(groupSlug string) string {
		if groupSlug == "" {
			return ""
		}
		return publicURL + "/g/" + groupSlug
	}
	settingsURLBuilder = func(groupSlug string) string {
		if groupSlug == "" {
			return ""
		}
		return publicURL + "/g/" + groupSlug + "/settings/notifications"
	}
	return appURLBuilder, settingsURLBuilder
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
		slog.InfoContext(ctx, "Business metrics collector disabled; no SystemStats source configured")
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

// StartItemReminderWorker wires and starts the item reminder worker
// (#2805), with the same notifications.Service as the other reminder
// workers so per-user and per-group toggles apply.
func StartItemReminderWorker(ctx context.Context, rs *RuntimeSetup, cfg *Config) func() {
	urlBuilder := buildCommodityURLBuilder(cfg.PublicURL)
	prefs := notifications.NewService(rs.FactorySet.SettingsRegistryFactory)
	prefs.SetGroupPrefs(rs.FactorySet.GroupNotificationPrefRegistry)
	service := services.NewItemReminderService(rs.FactorySet, rs.EmailLifecycle.Service, urlBuilder).WithPreferences(prefs)
	opts := []services.ItemReminderOption{
		services.WithItemReminderInterval(rs.WorkerDurations.ItemReminderInterval),
	}
	if rs.PauseController != nil {
		opts = append(opts, services.WithItemReminderPauseController(rs.PauseController))
	}
	worker := services.NewItemReminderWorker(service, opts...)
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
		// Name the variable an operator can ACTUALLY set: shared.ReadSection
		// wraps the run section with an INVENTARIO_RUN_ env-prefix, so the bare
		// `env:"FEATURE_CURRENCY_MIGRATION"` tag is never read on its own (#1618).
		slog.InfoContext(ctx, "Currency migration worker disabled; set INVENTARIO_RUN_FEATURE_CURRENCY_MIGRATION=true (or feature_currency_migration in config.yaml) to enable")
		return func() {}
	}
	pgFactory, ok := rs.FactorySet.CurrencyMigrationRegistryFactory.(*postgres.CurrencyMigrationRegistryFactory)
	if !ok {
		// Memory backend or any future stub. The conversion service +
		// audit + advisory lock surface lives in the postgres package,
		// so a non-postgres deployment cannot run TX2 — log loudly so
		// operators don't think the worker silently succeeded.
		slog.WarnContext(ctx, "Currency migration worker: skipping startup, backend is not postgres")
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

// assetNumberBackfillRetryInterval spaces the retries of a failed asset
// number backfill pass.
const assetNumberBackfillRetryInterval = time.Minute

// StartAssetNumberBackfill numbers, in the background, the commodities that
// have no asset number yet (see services.BackfillAssetNumbers) and returns a
// stop function that cancels the pass and waits for it to return. It runs in
// a goroutine so a large catalogue cannot delay readiness.
func StartAssetNumberBackfill(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		services.BackfillAssetNumbers(ctx, rs.FactorySet, assetNumberBackfillRetryInterval)
	})
	return func() {
		cancel()
		wg.Wait()
	}
}

// itemTypeBackfillRetryInterval spaces the retries of a failed item type
// backfill pass.
const itemTypeBackfillRetryInterval = time.Minute

// StartItemTypeBackfill seeds, in the background, the built-in item types of
// the groups that have none (see services.BackfillItemTypes). The returned
// stop function cancels the pass and waits for it to return.
func StartItemTypeBackfill(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		services.BackfillItemTypes(ctx, rs.FactorySet, itemTypeBackfillRetryInterval)
	})
	return func() {
		cancel()
		wg.Wait()
	}
}

// fileLinkMoveRetryInterval spaces the retries of a failed legacy file link
// move.
const fileLinkMoveRetryInterval = time.Minute

// fileLinkMoveSyncTimeout bounds the synchronous first pass of the legacy file
// link move, so a slow database cannot hold startup indefinitely.
const fileLinkMoveSyncTimeout = 2 * time.Minute

// StartFileLinkMove moves the file associations still stored in the legacy
// files.linked_entity_* columns into file_links (see
// services.MoveLegacyFileLinks). The first pass runs before it returns, so the
// API does not serve item Files tabs that miss their attachments; when that
// pass fails or times out, it keeps retrying in the background. The returned
// stop function cancels the retries and waits for them to return.
func StartFileLinkMove(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	syncCtx, cancelSync := context.WithTimeout(ctx, fileLinkMoveSyncTimeout)
	moved, err := services.MoveLegacyFileLinks(syncCtx, rs.FactorySet)
	cancelSync()
	if err == nil {
		if moved > 0 {
			slog.InfoContext(ctx, "legacy file links moved", "files", moved)
		}
		return func() {}
	}
	slog.WarnContext(ctx, "legacy file link move failed at startup; retrying in the background",
		"error", err.Error(), "files", moved)

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		services.RunLegacyFileLinkMove(ctx, rs.FactorySet, fileLinkMoveRetryInterval)
	})
	return func() {
		cancel()
		wg.Wait()
	}
}

// legacyPriceBackfillRetryInterval spaces the retries of a failed legacy
// price backfill.
const legacyPriceBackfillRetryInterval = time.Minute

// legacyPriceBackfillSyncTimeout bounds the synchronous first pass of the
// legacy price backfill, so a slow database cannot hold startup indefinitely.
const legacyPriceBackfillSyncTimeout = 2 * time.Minute

// StartLegacyPriceBackfill rewrites the commodities stored before a price
// could be unknown (see services.BackfillLegacyPrices). The first pass runs
// before it returns, so the API never serves such a row's "not set" zero as
// a known zero value; when that pass fails or times out, it keeps retrying
// in the background. The returned stop function cancels the retries and
// waits for them to return.
func StartLegacyPriceBackfill(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	syncCtx, cancelSync := context.WithTimeout(ctx, legacyPriceBackfillSyncTimeout)
	rewritten, err := services.BackfillLegacyPrices(syncCtx, rs.FactorySet)
	cancelSync()
	if err == nil {
		if rewritten > 0 {
			slog.InfoContext(ctx, "legacy prices backfilled", "commodities", rewritten)
		}
		return func() {}
	}
	slog.WarnContext(ctx, "legacy price backfill failed at startup; retrying in the background",
		"error", err.Error(), "commodities", rewritten)

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		services.RunLegacyPriceBackfill(ctx, rs.FactorySet, legacyPriceBackfillRetryInterval)
	})
	return func() {
		cancel()
		wg.Wait()
	}
}

// StartFileChecksumBackfill computes, in the background, the SHA-256 of the
// files that have none yet (see services.BackfillFileChecksums) and returns a
// stop function that cancels the pass and waits for it to return.
func StartFileChecksumBackfill(ctx context.Context, rs *RuntimeSetup, _ *Config) func() {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		services.BackfillFileChecksums(ctx, rs.FactorySet, rs.Params.UploadLocation)
	})
	return func() {
		cancel()
		wg.Wait()
	}
}
