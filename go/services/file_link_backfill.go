package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/registry"
)

// fileLinkMoveBatchSize bounds how many files one MoveLegacyLinks statement
// moves, so a large catalogue is moved in short transactions.
const fileLinkMoveBatchSize = 500

// fileChecksumBatchSize bounds how many files the checksum backfill lists per
// round trip.
const fileChecksumBatchSize = 100

// MoveLegacyFileLinks moves every file association still stored in the legacy
// files.linked_entity_* columns into file_links. It returns when nothing is
// left to move, and reports the number of files it moved.
//
// Every batch moves its rows and empties their legacy columns in one
// statement, so a crash, a retry or several replicas running this at once can
// neither lose an association nor create a second link for it.
func MoveLegacyFileLinks(ctx context.Context, factorySet *registry.FactorySet) (int, error) {
	if factorySet == nil || factorySet.FileRegistryFactory == nil {
		return 0, nil
	}
	reg := factorySet.FileRegistryFactory.CreateServiceRegistry()
	total := 0
	for {
		moved, err := reg.MoveLegacyLinks(ctx, fileLinkMoveBatchSize)
		total += moved
		if err != nil {
			return total, errxtrace.Wrap("failed to move legacy file links", err)
		}
		if moved == 0 {
			return total, nil
		}
	}
}

// RunLegacyFileLinkMove runs MoveLegacyFileLinks and retries it every
// retryInterval until a pass succeeds or ctx ends.
func RunLegacyFileLinkMove(ctx context.Context, factorySet *registry.FactorySet, retryInterval time.Duration) {
	for {
		start := time.Now()
		moved, err := MoveLegacyFileLinks(ctx, factorySet)
		if err == nil {
			if moved > 0 {
				slog.Info("legacy file links moved",
					"files", moved, "duration_ms", time.Since(start).Milliseconds())
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		slog.Warn("legacy file link move failed; will retry",
			"error", err.Error(), "files", moved, "retry_in", retryInterval.String())
		if !sleepCtx(ctx, retryInterval) {
			return
		}
	}
}

// BackfillFileChecksums computes the SHA-256 of every file that has none yet
// (files stored before checksums existed, seeded files) by reading its blob.
// New uploads and restored files get their checksum while their bytes stream,
// so this pass only ever sees the backlog.
//
// A blob that cannot be read is skipped and logged; the pass moves on by id,
// so one missing blob cannot stall the rest. The pass is idempotent: SetChecksum
// writes only an empty checksum, and a second run finds nothing to do.
func BackfillFileChecksums(ctx context.Context, factorySet *registry.FactorySet, uploadLocation string) {
	if factorySet == nil || factorySet.FileRegistryFactory == nil || uploadLocation == "" {
		return
	}
	bucket, err := blob.OpenBucket(ctx, uploadLocation)
	if err != nil {
		slog.Warn("checksum backfill skipped: bucket unavailable", "error", err.Error())
		return
	}
	defer func() {
		if cerr := bucket.Close(); cerr != nil {
			slog.Warn("checksum backfill: failed to close bucket", "error", cerr.Error())
		}
	}()

	reg := factorySet.FileRegistryFactory.CreateServiceRegistry()
	start := time.Now()
	updated, failed := 0, 0
	afterID := ""
	for {
		if ctx.Err() != nil {
			return
		}
		batch, err := reg.ListPendingChecksum(ctx, afterID, fileChecksumBatchSize)
		if err != nil {
			slog.Warn("checksum backfill stopped: failed to list pending files",
				"error", err.Error(), "updated", updated, "failed", failed)
			return
		}
		if len(batch) == 0 {
			break
		}
		for _, file := range batch {
			afterID = file.ID
			sum, err := blobChecksum(ctx, bucket, file.OriginalPath)
			if err == nil {
				err = reg.SetChecksum(ctx, file.ID, sum)
			}
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				failed++
				slog.Debug("checksum backfill: file skipped", "file_id", file.ID, "error", err.Error())
				continue
			}
			updated++
		}
	}
	if updated > 0 || failed > 0 {
		slog.Info("checksum backfill complete",
			"updated", updated, "failed", failed, "duration_ms", time.Since(start).Milliseconds())
	}
}

// blobChecksum returns the hex SHA-256 of the blob stored under key.
func blobChecksum(ctx context.Context, bucket *blob.Bucket, key string) (string, error) {
	r, err := bucket.NewReader(ctx, key, nil)
	if err != nil {
		return "", errxtrace.Wrap("failed to open blob", err)
	}
	defer r.Close()
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", errxtrace.Wrap("failed to read blob", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// sleepCtx waits for d and reports false when ctx ended first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
