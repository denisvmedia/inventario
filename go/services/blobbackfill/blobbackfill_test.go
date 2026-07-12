package blobbackfill_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/blobkeys"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services/blobbackfill"
)

// seedRow creates a tenant + user + group + one FileEntity row whose
// OriginalPath is a *legacy* flat blob key. The blob bytes are written
// to the bucket so the backfill has something to copy. Returns the
// created file id and the tenant id.
func seedRow(c *qt.C, ctx context.Context, factorySet *registry.FactorySet, uploadLocation, legacyKey, mime string) (fileID, tenantID string) {
	c.Helper()

	tenant := must.Must(factorySet.TenantRegistry.Create(ctx, models.Tenant{
		Name: "t1", Slug: "t1-" + legacyKey, Status: models.TenantStatusActive,
	}))

	user := must.Must(factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "u-" + legacyKey + "@example.com",
		Name:                "u",
		IsActive:            true,
	}))

	group := must.Must(factorySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Name:                "g1",
		Slug:                "g1-" + legacyKey,
		CreatedBy:           user.ID,
	}))

	// The service-mode file registry skips RLS, which is what the
	// backfill itself uses — keeps the seed and the system-under-test
	// using the same path.
	fileReg := factorySet.FileRegistryFactory.CreateServiceRegistry()
	created := must.Must(fileReg.Create(ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        tenant.ID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Title:    "legacy",
		Type:     models.FileTypeFromMIME(mime),
		Category: models.FileCategoryFromContext("", "", mime),
		Tags:     models.StringSlice{},
		File: &models.File{
			Path:         "legacy",
			OriginalPath: legacyKey,
			Ext:          ".jpg",
			MIMEType:     mime,
		},
	}))

	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()
	c.Assert(b.WriteAll(ctx, legacyKey, []byte{0xff, 0xd8, 0xff, 0xe0}, nil), qt.IsNil)
	return created.ID, tenant.ID
}

func TestBackfill_RewritesLegacyFileKeys(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	factorySet := memory.NewFactorySet()
	ctx := context.Background()

	// Seed: a single FileEntity with a flat legacy OriginalPath and the
	// corresponding bytes in the bucket.
	fileID, tenantID := seedRow(c, ctx, factorySet, uploadLocation, "legacy-photo.jpg", "image/jpeg")

	// Write legacy thumbnails too so we can assert the thumbnail
	// migration step.
	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	c.Assert(b.WriteAll(ctx, "thumbnails/"+fileID+"_small.jpg", []byte("legacy-small"), nil), qt.IsNil)
	c.Assert(b.WriteAll(ctx, "thumbnails/"+fileID+"_medium.jpg", []byte("legacy-medium"), nil), qt.IsNil)
	b.Close()

	svc := blobbackfill.New(factorySet, uploadLocation)
	stats, err := svc.Run(ctx, blobbackfill.Options{})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.RowsMoved, qt.Equals, 1)
	c.Assert(stats.RowsAlreadyMoved, qt.Equals, 0)
	c.Assert(stats.RowsErrored, qt.Equals, 0)
	c.Assert(stats.ThumbsMoved, qt.Equals, 2)

	// Row was updated.
	fileReg := factorySet.FileRegistryFactory.CreateServiceRegistry()
	updated := must.Must(fileReg.Get(ctx, fileID))
	c.Assert(blobkeys.HasTenantPrefix(updated.OriginalPath), qt.IsTrue,
		qt.Commentf("got OriginalPath %q after backfill", updated.OriginalPath))

	// Bucket state: new key present, legacy key gone.
	b2 := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b2.Close()
	newExists := must.Must(b2.Exists(ctx, updated.OriginalPath))
	c.Assert(newExists, qt.IsTrue)
	legacyExists := must.Must(b2.Exists(ctx, "legacy-photo.jpg"))
	c.Assert(legacyExists, qt.IsFalse)

	// Thumbnails at the canonical keys.
	canonicalSmall := blobkeys.BuildThumbnailBlobKey(tenantID, fileID, "small")
	canonicalMedium := blobkeys.BuildThumbnailBlobKey(tenantID, fileID, "medium")
	c.Assert(must.Must(b2.Exists(ctx, canonicalSmall)), qt.IsTrue)
	c.Assert(must.Must(b2.Exists(ctx, canonicalMedium)), qt.IsTrue)
}

func TestBackfill_Idempotent(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	factorySet := memory.NewFactorySet()
	ctx := context.Background()

	_, _ = seedRow(c, ctx, factorySet, uploadLocation, "legacy-doc.pdf", "application/pdf")

	svc := blobbackfill.New(factorySet, uploadLocation)

	// First pass: moves the row.
	stats1, err := svc.Run(ctx, blobbackfill.Options{})
	c.Assert(err, qt.IsNil)
	c.Assert(stats1.RowsMoved, qt.Equals, 1)

	// Second pass: row is already prefixed, nothing to do.
	stats2, err := svc.Run(ctx, blobbackfill.Options{})
	c.Assert(err, qt.IsNil)
	c.Assert(stats2.RowsMoved, qt.Equals, 0)
	c.Assert(stats2.RowsAlreadyMoved, qt.Equals, 1)
	c.Assert(stats2.RowsErrored, qt.Equals, 0)
}

func TestBackfill_DryRunDoesNotMutate(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	factorySet := memory.NewFactorySet()
	ctx := context.Background()

	fileID, _ := seedRow(c, ctx, factorySet, uploadLocation, "legacy-photo.jpg", "image/jpeg")

	svc := blobbackfill.New(factorySet, uploadLocation)
	stats, err := svc.Run(ctx, blobbackfill.Options{DryRun: true})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.RowsMoved, qt.Equals, 1)

	// Row unchanged.
	fileReg := factorySet.FileRegistryFactory.CreateServiceRegistry()
	row := must.Must(fileReg.Get(ctx, fileID))
	c.Assert(row.OriginalPath, qt.Equals, "legacy-photo.jpg")

	// Legacy blob still present.
	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()
	c.Assert(must.Must(b.Exists(ctx, "legacy-photo.jpg")), qt.IsTrue)
}

// seedRowInTenant is seedRow with the tenant named explicitly, so two rows in
// two DIFFERENT tenants can be given the SAME legacy key — the collision that
// #1793 (flat, un-prefixed keys) makes possible and that #2250 is about.
func seedRowInTenant(c *qt.C, ctx context.Context, factorySet *registry.FactorySet, uploadLocation, tenantSlug, legacyKey string) (fileID, tenantID string) {
	c.Helper()

	tenant := must.Must(factorySet.TenantRegistry.Create(ctx, models.Tenant{
		Name: tenantSlug, Slug: tenantSlug, Status: models.TenantStatusActive,
	}))
	user := must.Must(factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "u@" + tenantSlug + ".example.com",
		Name:                "u",
		IsActive:            true,
	}))
	group := must.Must(factorySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Name:                "g", Slug: "g-" + tenantSlug, CreatedBy: user.ID,
	}))

	fileReg := factorySet.FileRegistryFactory.CreateServiceRegistry()
	created := must.Must(fileReg.Create(ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: tenant.ID, GroupID: group.ID, CreatedByUserID: user.ID,
		},
		Title: "legacy", Type: models.FileTypeImage, Category: models.FileCategoryImages,
		Tags: models.StringSlice{},
		File: &models.File{
			Path: "legacy", OriginalPath: legacyKey, Ext: ".jpg", MIMEType: "image/jpeg",
		},
	}))

	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()
	c.Assert(b.WriteAll(ctx, legacyKey, []byte{0xff, 0xd8, 0xff, 0xe0}, nil), qt.IsNil)
	return created.ID, tenant.ID
}

// TWO TENANTS, ONE LEGACY KEY (#2250). A flat key has no tenant prefix — that is
// exactly what #1793 was fixing — so rows in different tenants can reference one.
//
// Their destinations differ (`t/<T1>/files/K` vs `t/<T2>/files/K`), so each row
// needs its own copy of the bytes. The old backfill deleted the source right
// after ITS copy, which left the second row's copy with nothing to read — and
// then re-pointed that row anyway, at a key it had never written. The row ended
// up claiming a location that has never existed, its bytes stranded under the
// first tenant's namespace with nothing linking them, and the run reported it as
// successfully moved. Silent, permanent, invisible in the stats.
//
// Whoever backfills LAST is the one who reclaims the legacy blob.
func TestBackfill_SharedLegacyKeyAcrossTenants(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	factorySet := memory.NewFactorySet()
	ctx := context.Background()

	const shared = "receipt-1783824560.jpg"
	idA, tenantA := seedRowInTenant(c, ctx, factorySet, uploadLocation, "tenant-a", shared)
	idB, tenantB := seedRowInTenant(c, ctx, factorySet, uploadLocation, "tenant-b", shared)

	svc := blobbackfill.New(factorySet, uploadLocation)
	stats, err := svc.Run(ctx, blobbackfill.Options{})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.RowsErrored, qt.Equals, 0,
		qt.Commentf("a row lost its bytes to the other tenant's backfill"))
	c.Assert(stats.RowsMoved, qt.Equals, 2)

	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()
	fileReg := factorySet.FileRegistryFactory.CreateServiceRegistry()

	// BOTH rows must point at their own tenant-namespaced key, and both keys must
	// actually hold bytes. A row pointing at a key that was never written is the
	// failure this test exists for.
	for _, tc := range []struct{ id, tenant string }{{idA, tenantA}, {idB, tenantB}} {
		row := must.Must(fileReg.Get(ctx, tc.id))
		want := blobkeys.RewriteForTenant(shared, tc.tenant)
		c.Assert(row.OriginalPath, qt.Equals, want)
		c.Assert(must.Must(b.Exists(ctx, want)), qt.IsTrue,
			qt.Commentf("row %s points at %q, which holds no bytes", tc.id, want))
	}

	// The legacy key is reclaimed — but only once nobody references it any more.
	c.Assert(must.Must(b.Exists(ctx, shared)), qt.IsFalse,
		qt.Commentf("the last owner did not reclaim the legacy blob"))
}

// A row claims a blob the bucket does not have. The backfill must NOT re-point
// it (#2250).
//
// The old contract re-pointed the row anyway and counted it as moved, on the
// theory that "future reads land at the canonical location". They don't — there
// is nothing there to read, and there never was. What the rewrite actually
// destroys is the legacy key, which is the ONLY record of where those bytes were
// supposed to live: an operator restoring a pruned bucket from a snapshot can
// put `missing-photo.jpg` back, but cannot know that the row now expects
// `t/<tenant>/files/missing-photo.jpg`. And because the run reported RowsMoved=1
// and RowsErrored=0, nobody would ever look.
//
// So: leave the row alone, count it as errored, and name it in the log.
func TestBackfill_LeavesRowAloneWhenItsBlobIsMissing(t *testing.T) {
	c := qt.New(t)

	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	factorySet := memory.NewFactorySet()
	ctx := context.Background()

	fileID, _ := seedRow(c, ctx, factorySet, uploadLocation, "missing-photo.jpg", "image/jpeg")

	// Delete the seeded blob to simulate an admin-pruned bucket.
	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	c.Assert(b.Delete(ctx, "missing-photo.jpg"), qt.IsNil)
	b.Close()

	svc := blobbackfill.New(factorySet, uploadLocation)
	stats, err := svc.Run(ctx, blobbackfill.Options{})
	c.Assert(err, qt.IsNil) // a bad row must never abort the sweep
	c.Assert(stats.RowsErrored, qt.Equals, 1)
	c.Assert(stats.RowsMoved, qt.Equals, 0)
	c.Assert(stats.BlobsCopied, qt.Equals, 0)

	fileReg := factorySet.FileRegistryFactory.CreateServiceRegistry()
	row := must.Must(fileReg.Get(ctx, fileID))
	c.Assert(row.OriginalPath, qt.Equals, "missing-photo.jpg",
		qt.Commentf("the row was re-pointed at a key that was never written, erasing where its bytes belonged"))
}
