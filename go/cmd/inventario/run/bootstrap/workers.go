package bootstrap

import (
	"context"

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
