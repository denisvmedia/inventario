package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// storageBackfillBatchSize bounds how many rows the backfill pulls per
// chunk. Picked low enough that even a multi-million-row install
// doesn't materialize the whole catalogue in memory at boot, high
// enough that each round-trip carries useful work.
const storageBackfillBatchSize = 200

// BackfillFileSizes walks every file row whose size_bytes is still 0
// (legacy rows that pre-date the column added in #1388) and updates
// them with the byte size reported by the upload bucket. The function
// is idempotent: a second invocation is a no-op once every row has
// been measured.
//
// Pulls rows in chunks via FileRegistry.ListPendingSizeBackfill so the
// query is bounded by SQL — the previous implementation called List()
// and filtered in memory, which scaled with the entire catalogue at
// every boot.
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

	start := time.Now()
	updated, failed := 0, 0
	for {
		if ctx.Err() != nil {
			slog.Info("storage backfill cancelled",
				"updated", updated, "failed", failed)
			return
		}

		batch, listErr := fileReg.ListPendingSizeBackfill(ctx, storageBackfillBatchSize)
		if listErr != nil {
			slog.Warn("storage backfill skipped: failed to list pending files",
				"error", listErr.Error(), "updated", updated, "failed", failed)
			return
		}
		if len(batch) == 0 {
			break
		}

		// Track how many rows actually advanced this round so we don't
		// loop forever on rows that legitimately can't be re-stat'd
		// (missing blobs leave size_bytes at 0). When every row in a
		// batch failed, the next ListPending call would return the
		// same set — bail out instead.
		batchUpdated, batchFailed, cancelled := backfillBatch(ctx, fileReg, bucket, batch)
		updated += batchUpdated
		failed += batchFailed
		if cancelled {
			slog.Info("storage backfill cancelled",
				"updated", updated, "failed", failed)
			return
		}

		// All rows in the batch failed — every subsequent batch would
		// surface the same un-stat'able rows. Stop instead of spinning.
		if batchUpdated == 0 {
			break
		}
	}

	slog.Info("storage backfill complete",
		"updated", updated, "failed", failed,
		"duration_ms", time.Since(start).Milliseconds())
}

// backfillBatch processes one chunk of pending rows. Split out of the
// outer loop so the per-batch error handling doesn't pile cyclomatic
// complexity on BackfillFileSizes itself. Returns (updated, failed,
// cancelled) where `cancelled` signals the outer loop to bail out.
func backfillBatch(ctx context.Context, fileReg backfillFileRegistry, bucket *blob.Bucket, batch []*models.FileEntity) (updated, failed int, cancelled bool) {
	for _, file := range batch {
		if ctx.Err() != nil {
			return updated, failed, true
		}
		if file == nil || file.File == nil {
			continue
		}

		attrs, attrErr := bucket.Attributes(ctx, file.OriginalPath)
		if attrErr != nil {
			// File may have been moved or deleted from the bucket;
			// leave the row at 0 and continue. Don't fail the whole
			// pass.
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
	return updated, failed, false
}

// backfillFileRegistry narrows the registry surface backfillBatch
// touches — only Update on a service-mode FileRegistry. Stated as a
// local interface so the helper is trivially mockable in tests
// without dragging in the whole registry.FileRegistry contract.
type backfillFileRegistry interface {
	Update(ctx context.Context, file models.FileEntity) (*models.FileEntity, error)
}
