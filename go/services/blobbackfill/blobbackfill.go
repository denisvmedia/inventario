// Package blobbackfill walks every FileEntity row in the database and
// migrates its physical blob (and, for images, its thumbnails) into the
// per-tenant namespace introduced by issue #1793.
//
// The migration is idempotent and copy-first: each row is examined; if
// its OriginalPath is already tenant-prefixed (`t/...`) the row is
// skipped, otherwise the original blob is copied to the new key, the
// row is updated, the legacy blob is deleted, and the thumbnail keys
// are migrated the same way. An interrupted run leaves rows in either
// the legacy or the migrated state — re-running picks up where it
// left off without clobbering the rows it has already touched.
//
// The package deliberately depends only on the service-mode file
// registry: backfilling is a cross-tenant background operation and
// must NOT be filtered by RLS.
package blobbackfill

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"

	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/internal/mimekit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// Stats records the per-run outcome for the operator. Zero values are
// the expected steady state once the migration has run once across
// every row.
type Stats struct {
	RowsScanned      int
	RowsAlreadyMoved int
	RowsMoved        int
	RowsSkippedNoKey int
	RowsErrored      int
	ThumbsMoved      int
	ThumbsMissing    int
	BlobsCopied      int
	BlobsDeleted     int
}

// Options configures a backfill run.
type Options struct {
	// DryRun reports what would change without touching the bucket or
	// any row. Recommended for the first invocation in a new env.
	DryRun bool
	// Logger receives per-row diagnostics. Use slog.Default() to fall
	// through to the process-wide handler.
	Logger *slog.Logger
}

// Service is the entry point for the backfill workflow. Bind it to a
// service-mode FactorySet (the user-mode registry would only see one
// tenant at a time, which is the opposite of what the backfill needs).
type Service struct {
	factorySet     *registry.FactorySet
	uploadLocation string
}

// New constructs a backfill service. The supplied uploadLocation must
// match the running server's upload bucket — the backfill walks blobs
// directly through gocloud.dev/blob.
func New(factorySet *registry.FactorySet, uploadLocation string) *Service {
	return &Service{
		factorySet:     factorySet,
		uploadLocation: uploadLocation,
	}
}

// Run executes the backfill. Returns aggregated Stats and an error if
// the run aborted; per-row failures increment Stats.RowsErrored and
// are logged but do not stop the sweep.
func (s *Service) Run(ctx context.Context, opts Options) (*Stats, error) {
	if s.uploadLocation == "" {
		return nil, errors.New("upload location is required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	bucket, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return nil, errxtrace.Wrap("failed to open upload bucket", err)
	}
	defer bucket.Close()

	fileReg := s.factorySet.FileRegistryFactory.CreateServiceRegistry()

	stats := &Stats{}
	// Stream rows in fixed-size pages so the migration doesn't allocate
	// the full file table at once — production deployments may have
	// millions of rows. Pagination is offset/ID-ordered; mutations
	// during the sweep don't shift ordering because rows are updated
	// (not deleted), and idempotency lets a re-run pick up anything we
	// somehow missed.
	//
	// We stop when the page is short (`len(page) < pageSize`) instead
	// of comparing offset against `total`. ListPaginated executes a
	// COUNT(*) on every call to populate `total`, which would re-count
	// the whole file table on every page for a long-running sweep.
	const pageSize = 500
	offset := 0
	for {
		page, _, err := fileReg.ListPaginated(ctx, offset, pageSize, nil, nil, nil, nil)
		if err != nil {
			return nil, errxtrace.Wrap("failed to list file rows", err)
		}
		if len(page) == 0 {
			break
		}
		for _, file := range page {
			s.processRow(ctx, bucket, fileReg, file, opts, logger, stats)
		}
		offset += len(page)
		if len(page) < pageSize {
			break
		}
	}

	return stats, nil
}

// processRow handles a single FileEntity row. Pulled out of Run so
// the loop body stays inside the project's cognitive-complexity
// budget. Mutates `stats` in place; on per-row failure increments the
// appropriate stat counter and returns silently — failures here must
// never abort the sweep.
func (s *Service) processRow(
	ctx context.Context,
	bucket *blob.Bucket,
	fileReg registry.FileRegistry,
	file *models.FileEntity,
	opts Options,
	logger *slog.Logger,
	stats *Stats,
) {
	stats.RowsScanned++
	if file == nil || file.File == nil || file.OriginalPath == "" {
		stats.RowsSkippedNoKey++
		return
	}
	if file.TenantID == "" {
		logger.Warn("blobbackfill: file row missing tenant id, skipped",
			"file_id", file.ID, "blob_key", file.OriginalPath)
		stats.RowsErrored++
		return
	}

	legacyKey := file.OriginalPath

	// Already-prefixed rows are skipped (the migration is for legacy
	// flat keys only). Thumbnails for already-prefixed rows may still
	// need a sweep though, so we run the thumbnail step regardless.
	if blobkeys.HasTenantPrefix(legacyKey) {
		stats.RowsAlreadyMoved++
		s.maybeMigrateThumbs(ctx, bucket, file, opts.DryRun, logger, stats)
		return
	}

	newKey := blobkeys.RewriteForTenant(legacyKey, file.TenantID)
	if newKey == legacyKey {
		stats.RowsAlreadyMoved++
		return
	}

	if opts.DryRun {
		logger.Info("blobbackfill: would move row", "file_id", file.ID, "from", legacyKey, "to", newKey)
		stats.RowsMoved++
		return
	}

	// Step 1: copy. If destination already exists, copyIfAbsent
	// short-circuits — that's how interrupted runs become idempotent.
	copied, err := copyIfAbsent(ctx, bucket, legacyKey, newKey)
	if err != nil {
		logger.Warn("blobbackfill: copy failed", "file_id", file.ID, "err", err)
		stats.RowsErrored++
		return
	}
	if copied {
		stats.BlobsCopied++
	}

	// Step 2: update the row. The user-aware registry would do
	// validation against the tenant on context; we go through the
	// service registry to keep cross-tenant iteration cheap.
	file.OriginalPath = newKey
	if _, err := fileReg.Update(ctx, *file); err != nil {
		logger.Warn("blobbackfill: row update failed", "file_id", file.ID, "err", err)
		stats.RowsErrored++
		return
	}

	// Step 3: delete the legacy blob. Failure here is non-fatal — the
	// row already points at the new key, so the legacy blob is
	// unreferenced; a later sweep can clean it up. NotFound is the
	// expected state on a re-run (we already deleted it last time) or
	// on a partially-pruned bucket where copyIfAbsent took the
	// short-circuit path, so suppress it to keep the log quiet.
	switch err := bucket.Delete(ctx, legacyKey); {
	case err == nil:
		stats.BlobsDeleted++
	case gcerrors.Code(err) == gcerrors.NotFound:
		// Legacy blob already gone — nothing to do.
	default:
		logger.Warn("blobbackfill: legacy blob delete failed",
			"file_id", file.ID, "legacy_key", legacyKey, "err", err)
	}

	// Step 4: thumbnails.
	s.maybeMigrateThumbs(ctx, bucket, file, opts.DryRun, logger, stats)
	stats.RowsMoved++
}

// maybeMigrateThumbs is a thin wrapper around migrateThumbnailsForRow
// that no-ops for non-image rows and folds the result into the stats.
func (s *Service) maybeMigrateThumbs(
	ctx context.Context,
	bucket *blob.Bucket,
	file *models.FileEntity,
	dryRun bool,
	logger *slog.Logger,
	stats *Stats,
) {
	if !mimekit.IsImage(file.MIMEType) {
		return
	}
	moved, missing, err := s.migrateThumbnailsForRow(ctx, bucket, thumbMigrateParams{
		TenantID: file.TenantID,
		FileID:   file.ID,
		DryRun:   dryRun,
	})
	stats.ThumbsMoved += moved
	stats.ThumbsMissing += missing
	if err != nil {
		logger.Warn("blobbackfill: thumbnail migration error", "file_id", file.ID, "err", err)
		stats.RowsErrored++
	}
}

// thumbMigrateParams bundles the per-row thumbnail-migration arguments
// so migrateThumbnailsForRow doesn't take a flag boolean as a
// positional parameter (revive flag-parameter).
type thumbMigrateParams struct {
	TenantID string
	FileID   string
	DryRun   bool
}

// migrateThumbnailsForRow walks the two thumbnail sizes and migrates
// each from the legacy flat key to the canonical tenant-prefixed key.
// Returns (moved, missing, err). "Missing" counts each size that had
// no blob at either the legacy or the canonical location — common for
// rows that were uploaded but never had their thumbnails generated.
func (s *Service) migrateThumbnailsForRow(
	ctx context.Context,
	bucket *blob.Bucket,
	p thumbMigrateParams,
) (moved, missing int, err error) {
	sizes := []string{"small", "medium"}
	for _, size := range sizes {
		legacy := fmt.Sprintf("thumbnails/%s_%s.jpg", p.FileID, size)
		canonical := blobkeys.BuildThumbnailBlobKey(p.TenantID, p.FileID, size)

		didMove, didMiss, err := migrateOneThumb(ctx, bucket, thumbKeys{Legacy: legacy, Canonical: canonical, DryRun: p.DryRun})
		if err != nil {
			return moved, missing, err
		}
		moved += didMove
		missing += didMiss
	}
	return moved, missing, nil
}

// thumbKeys bundles migrateOneThumb's per-size inputs so the DryRun
// boolean isn't a positional control flag (revive flag-parameter).
type thumbKeys struct {
	Legacy    string
	Canonical string
	DryRun    bool
}

// migrateOneThumb performs the legacy → canonical migration for a
// single thumbnail key. Extracted so migrateThumbnailsForRow stays
// inside the project's cognitive-complexity budget.
func migrateOneThumb(ctx context.Context, bucket *blob.Bucket, k thumbKeys) (moved, missing int, err error) {
	canonicalExists, err := bucket.Exists(ctx, k.Canonical)
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to probe canonical thumbnail", err)
	}
	if canonicalExists {
		// Already migrated; still try to clean up any leftover legacy blob.
		if legacyExists, lErr := bucket.Exists(ctx, k.Legacy); lErr == nil && legacyExists && !k.DryRun {
			_ = bucket.Delete(ctx, k.Legacy)
		}
		return 0, 0, nil
	}

	legacyExists, err := bucket.Exists(ctx, k.Legacy)
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to probe legacy thumbnail", err)
	}
	if !legacyExists {
		return 0, 1, nil
	}
	if k.DryRun {
		return 1, 0, nil
	}

	if _, err := copyIfAbsent(ctx, bucket, k.Legacy, k.Canonical); err != nil {
		return 0, 0, errxtrace.Wrap("failed to copy legacy thumbnail", err)
	}
	_ = bucket.Delete(ctx, k.Legacy)
	return 1, 0, nil
}

// copyIfAbsent copies from src to dst unless dst already exists.
// Returns true if a copy happened, false otherwise. The bucket-level
// Copy operation isn't always available on every gocloud backend so we
// use a streaming reader→writer to stay portable.
func copyIfAbsent(ctx context.Context, bucket *blob.Bucket, src, dst string) (bool, error) {
	dstExists, err := bucket.Exists(ctx, dst)
	if err != nil {
		return false, errxtrace.Wrap("failed to probe destination", err)
	}
	if dstExists {
		return false, nil
	}

	srcExists, err := bucket.Exists(ctx, src)
	if err != nil {
		return false, errxtrace.Wrap("failed to probe source", err)
	}
	if !srcExists {
		// The row claims a blob lives here but the bucket disagrees —
		// not an error per se (an admin may have manually pruned the
		// bucket), just nothing to copy. The row update step still
		// runs so the row eventually points at the canonical location.
		return false, nil
	}

	reader, err := bucket.NewReader(ctx, src, nil)
	if err != nil {
		return false, errxtrace.Wrap("failed to open source reader", err)
	}
	defer reader.Close()

	writer, err := bucket.NewWriter(ctx, dst, nil)
	if err != nil {
		return false, errxtrace.Wrap("failed to open destination writer", err)
	}
	if _, err := io.Copy(writer, reader); err != nil {
		_ = writer.Close()
		return false, errxtrace.Wrap("failed to copy bytes", err)
	}
	if err := writer.Close(); err != nil {
		return false, errxtrace.Wrap("failed to close destination writer", err)
	}
	return true, nil
}
