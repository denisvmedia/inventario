package seeddata

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// reconcileSeedBlobs repairs the blob bytes for bundled fixture file rows
// whose row exists in the database but whose object is missing from the
// upload bucket.
//
// Why this exists: SeedData is idempotent via a location-count gate that
// short-circuits the WHOLE seed once data tables are populated — including
// the blob-upload step. That gate keys solely on database state, but the
// fixture bytes live in a separate store (e.g. the demo MinIO bucket)
// with an independent lifecycle. Two situations leave the DB carrying
// fixture file rows that point at blobs which do not exist:
//
//   - A database seeded metadata-only BEFORE blob uploads were enabled
//     for its tenant. Blob writes for non-`test-org` tenants only became
//     possible with the AllowBlobUploads opt-in (#1931); any demo/preview
//     env first seeded before that shipped has rows with no bytes and a
//     SizeBytes of 0, and the location-count gate means a later re-seed
//     never backfills them.
//   - A bucket that lost its contents independently of the DB (the demo
//     MinIO runs on an emptyDir, so a MinIO-only pod replacement wipes the
//     objects while Postgres keeps the rows).
//
// Both leave the Files page showing entries whose download/thumbnail
// 404s. reconcileSeedBlobs converges the bucket back to the rows: it runs
// on every seed call (including the already-seeded fast path), so the
// next sync self-heals instead of carrying permanently-dangling rows.
//
// It is deliberately conservative — it only ever WRITES fixture bytes at
// keys that already belong to recognised seed rows, never deletes, and
// never touches a row a user has renamed. It honours the same tenant gate
// as the fresh-seed upload path so it cannot be used to spam an arbitrary
// tenant's bucket through the public /api/v1/seed surface.
func reconcileSeedBlobs(ctx context.Context, set *registry.Set, tenant *models.Tenant, opts SeedOptions) error {
	// Same gate as the fresh-seed upload step in SeedData: fixture bytes
	// are only written for the well-known `test-org` tenant or when a
	// caller explicitly opts in (env-gated, server-side only). For any
	// other tenant there is intentionally nothing to reconcile.
	if tenant.Slug != "test-org" && !opts.AllowBlobUploads {
		return nil
	}
	if opts.UploadLocation == "" {
		// No bucket attached (in-memory unit tests, or a deployment that
		// deliberately keeps metadata-only rows). Nothing to repair.
		return nil
	}
	if err := ensureFixturesPresent(); err != nil {
		return err
	}

	uploader, err := newBlobUploader(ctx, opts.UploadLocation)
	if err != nil {
		return err
	}
	defer uploader.close()

	files, err := set.FileRegistry.List(ctx)
	if err != nil {
		return fmt.Errorf("list files for blob reconcile: %w", err)
	}

	var repaired, resized int
	for _, f := range files {
		fixture, ok := seedFixtureForRow(f)
		if !ok {
			continue
		}
		size, wrote, eerr := uploader.ensure(ctx, f.OriginalPath, fixture)
		if eerr != nil {
			return fmt.Errorf("reconcile blob for file %s (%s): %w", f.ID, f.OriginalPath, eerr)
		}
		if wrote {
			repaired++
		}
		// Heal a drifted SizeBytes whether we just wrote the blob (the
		// metadata-only row recorded 0) or the row simply disagreed with
		// the blob already in the bucket. The per-group storage-usage
		// aggregation (#1388) reads SizeBytes, so leaving it at 0 would
		// undercount even after the bytes are restored.
		if f.File != nil && f.SizeBytes != size {
			f.SizeBytes = size
			if _, uerr := set.FileRegistry.Update(ctx, *f); uerr != nil {
				return fmt.Errorf("update size for reconciled file %s: %w", f.ID, uerr)
			}
			resized++
		}
	}
	if repaired > 0 || resized > 0 {
		slog.Info("Reconciled seed blobs",
			"tenant", tenant.Slug,
			"bytes_rewritten", repaired,
			"sizes_healed", resized,
		)
	}
	return nil
}

// seedFixtureForRow returns the bundled fixture a seeded file row was
// created from, or ok=false when the row is not a reconcilable seed
// fixture. Identification requires BOTH:
//
//   - the OriginalPath basename carries the seed prefix
//     (`seed-<uuid><ext>`), so user-uploaded files are never matched even
//     if a user happened to name one "invoice"; and
//   - the row's Path still maps to a known fixture basename, so a seed
//     file a user has since renamed is left untouched.
//
// The fixture's extension is checked against the row's so a row can never
// be repaired with bytes of the wrong type (e.g. PDF bytes onto an image
// row).
func seedFixtureForRow(f *models.FileEntity) (fixtureKind, bool) {
	if f == nil || f.File == nil {
		return "", false
	}
	if !strings.HasPrefix(path.Base(f.OriginalPath), seedBasenamePrefix) {
		return "", false
	}
	fixture, ok := fixtureByPathBasename[f.Path]
	if !ok {
		return "", false
	}
	if fixtureExt(fixture) != f.Ext {
		return "", false
	}
	return fixture, true
}
