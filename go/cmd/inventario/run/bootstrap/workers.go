package bootstrap

import (
	"context"
	"strings"

	"github.com/denisvmedia/inventario/backup/export"
	importpkg "github.com/denisvmedia/inventario/backup/import"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/services"
)

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
	service := services.NewWarrantyReminderService(rs.FactorySet, rs.EmailLifecycle.Service, urlBuilder)
	worker := services.NewWarrantyReminderWorker(
		service,
		services.WithWarrantyReminderInterval(rs.WorkerDurations.WarrantyReminderInterval),
	)
	worker.Start(ctx)
	return worker.Stop
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
