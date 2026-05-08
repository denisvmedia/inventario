package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/registry"
)

// BackfillFileSizes walks every file row whose size_bytes is still 0
// (legacy rows that pre-date the column added in #1388) and updates
// them with the byte size reported by the upload bucket. The function
// is idempotent: a second invocation is a no-op once every row has
// been measured.
//
// Errors against individual blobs are logged and swallowed — a missing
// or unreadable object must not block the rest of the catalogue from
// being measured. The function is safe to call from a goroutine at
// boot; it stops early if ctx is cancelled.
func BackfillFileSizes(ctx context.Context, factorySet *registry.FactorySet, uploadLocation string) {
	if factorySet == nil || uploadLocation == "" {
		return
	}

	bucket, err := blob.OpenBucket(ctx, uploadLocation)
	if err != nil {
		slog.Warn("storage backfill skipped: bucket unavailable", "error", err.Error())
		return
	}
	defer func() {
		if cerr := bucket.Close(); cerr != nil {
			slog.Warn("storage backfill: failed to close bucket", "error", cerr.Error())
		}
	}()

	fileReg := factorySet.FileRegistryFactory.CreateServiceRegistry()
	files, err := fileReg.List(ctx)
	if err != nil {
		slog.Warn("storage backfill skipped: failed to list files", "error", err.Error())
		return
	}

	start := time.Now()
	updated, skipped, failed := 0, 0, 0
	for _, file := range files {
		if ctx.Err() != nil {
			slog.Info("storage backfill cancelled",
				"updated", updated, "skipped", skipped, "failed", failed)
			return
		}
		if file == nil || file.File == nil {
			continue
		}
		if file.SizeBytes != 0 {
			skipped++
			continue
		}

		attrs, attrErr := bucket.Attributes(ctx, file.OriginalPath)
		if attrErr != nil {
			// File may have been moved or deleted from the bucket; leave
			// the row at 0 and continue. Don't fail the whole pass.
			if !errors.Is(attrErr, context.Canceled) {
				slog.Debug("storage backfill: missing blob",
					"file_id", file.ID, "path", file.OriginalPath, "error", attrErr.Error())
			}
			failed++
			continue
		}

		file.SizeBytes = attrs.Size
		if _, updErr := fileReg.Update(ctx, *file); updErr != nil {
			slog.Debug("storage backfill: update failed",
				"file_id", file.ID, "error", updErr.Error())
			failed++
			continue
		}
		updated++
	}

	slog.Info("storage backfill complete",
		"updated", updated, "skipped", skipped, "failed", failed,
		"duration_ms", time.Since(start).Milliseconds())
}
