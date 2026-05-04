package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// signingKey32 is a fixed 32-byte key used across the cover-service tests.
// `NewFileSigningService` validates length ≥ 32 — we don't care about the
// crypto strength, just that the constructor doesn't reject it.
var signingKey32 = []byte("0123456789abcdef0123456789abcdef")

func newCoverTestContext(c *qt.C) (context.Context, *memory.FileRegistry) {
	c.Helper()
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	}
	ctx := appctx.WithUser(context.Background(), user)
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
	})

	factorySet := memory.NewFactorySet()
	registrySet, err := factorySet.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	fileReg, ok := registrySet.FileRegistry.(*memory.FileRegistry)
	c.Assert(ok, qt.IsTrue)
	return ctx, fileReg
}

// commodityRef builds a stub commodity with just the id set — enough
// for ResolveOne / ResolveMany since the resolver only reads ID and
// CoverFileID off the row.
func commodityRef(id string) *models.Commodity {
	c := &models.Commodity{}
	c.ID = id
	return c
}

// commodityRefWithCover is `commodityRef` plus an explicit
// `cover_file_id` override (issue #1451 option B).
func commodityRefWithCover(id, coverFileID string) *models.Commodity {
	c := commodityRef(id)
	v := coverFileID
	c.CoverFileID = &v
	return c
}

// commodityRefs maps a list of ids to stub commodities so the
// `ResolveMany` call sites stay readable.
func commodityRefs(ids ...string) []*models.Commodity {
	out := make([]*models.Commodity, len(ids))
	for i, id := range ids {
		out[i] = commodityRef(id)
	}
	return out
}

// makeImage builds a `linked_entity_type=commodity` / `meta=images` file
// entity with the supplied id, commodity and creation time. Tests use
// distinct timestamps to disambiguate the "earliest by created_at" pick.
func makeImage(id, commodityID string, createdAt time.Time) models.FileEntity {
	return models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: id},
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		Type:             models.FileTypeImage,
		Category:         models.FileCategoryPhotos,
		LinkedEntityType: "commodity",
		LinkedEntityID:   commodityID,
		LinkedEntityMeta: "images",
		CreatedAt:        createdAt,
		File: &models.File{
			Path:         id,
			OriginalPath: id + ".jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}
}

func TestCommodityCoverService_PicksEarliestPhoto(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	older := makeImage("file-old", "commodity-A", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	newer := makeImage("file-new", "commodity-A", time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC))
	// Insert newer first so the resulting "earliest" pick is provably
	// `created_at`-derived rather than insertion-order-derived. The
	// memory registry rewrites the caller-supplied ID with a fresh UUID,
	// so we capture the returned id to assert against.
	createdNewer, err := fileReg.Create(ctx, newer)
	c.Assert(err, qt.IsNil)
	createdOlder, err := fileReg.Create(ctx, older)
	c.Assert(err, qt.IsNil)
	c.Assert(createdNewer.ID, qt.Not(qt.Equals), createdOlder.ID)

	signing := services.NewFileSigningService(signingKey32, time.Hour)
	coverSvc := services.NewCommodityCoverService(signing)

	cov, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRef("commodity-A"), "user-1")
	c.Assert(ok, qt.IsTrue)
	c.Assert(cov.FileID, qt.Equals, createdOlder.ID)
	c.Assert(cov.Source, qt.Equals, services.CoverSourceFirstPhoto)
	c.Assert(len(cov.Thumbnails) > 0, qt.IsTrue)
	for size, url := range cov.Thumbnails {
		c.Assert(url, qt.Contains, "/files/download/thumbnails/"+createdOlder.ID+"/"+size,
			qt.Commentf("thumbnail URL for size %q should embed the file id and size, got %q", size, url))
	}
}

func TestCommodityCoverService_AbsentWhenNoPhotos(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	signing := services.NewFileSigningService(signingKey32, time.Hour)
	coverSvc := services.NewCommodityCoverService(signing)

	_, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRef("commodity-empty"), "user-1")
	c.Assert(ok, qt.IsFalse)

	resolved := coverSvc.ResolveMany(ctx, fileReg, commodityRefs("commodity-empty"), "user-1")
	c.Assert(resolved, qt.HasLen, 0)
}

func TestCommodityCoverService_ResolveMany_PerCommodity(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	t1 := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	aCover, err := fileReg.Create(ctx, makeImage("a-cover", "commodity-A", t1))
	c.Assert(err, qt.IsNil)
	_, err = fileReg.Create(ctx, makeImage("a-second", "commodity-A", t2))
	c.Assert(err, qt.IsNil)
	bCover, err := fileReg.Create(ctx, makeImage("b-cover", "commodity-B", t1))
	c.Assert(err, qt.IsNil)
	// commodity-C deliberately has no images.

	signing := services.NewFileSigningService(signingKey32, time.Hour)
	coverSvc := services.NewCommodityCoverService(signing)

	resolved := coverSvc.ResolveMany(ctx, fileReg, commodityRefs("commodity-A", "commodity-B", "commodity-C"), "user-1")
	c.Assert(resolved, qt.HasLen, 2)
	c.Assert(resolved["commodity-A"].FileID, qt.Equals, aCover.ID)
	c.Assert(resolved["commodity-B"].FileID, qt.Equals, bCover.ID)
	_, hasC := resolved["commodity-C"]
	c.Assert(hasC, qt.IsFalse)
}

func TestCommodityCoverService_EmptyUserShortCircuits(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	_, err := fileReg.Create(ctx, makeImage("any", "commodity-A", time.Now()))
	c.Assert(err, qt.IsNil)

	signing := services.NewFileSigningService(signingKey32, time.Hour)
	coverSvc := services.NewCommodityCoverService(signing)

	// Anonymous caller — signing wouldn't produce a verifiable URL anyway.
	resolved := coverSvc.ResolveMany(ctx, fileReg, commodityRefs("commodity-A"), "")
	c.Assert(resolved, qt.HasLen, 0)
	_, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRef("commodity-A"), "")
	c.Assert(ok, qt.IsFalse)
}

func TestCommodityCoverService_SkipsNonImageMetaImages(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	// Defensive guard test: a row mis-classified as `meta=images` but
	// with `type=document` should not be promoted to the cover slot.
	bad := makeImage("bad", "commodity-A", time.Now())
	bad.Type = models.FileTypeDocument
	_, err := fileReg.Create(ctx, bad)
	c.Assert(err, qt.IsNil)

	signing := services.NewFileSigningService(signingKey32, time.Hour)
	coverSvc := services.NewCommodityCoverService(signing)

	_, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRef("commodity-A"), "user-1")
	c.Assert(ok, qt.IsFalse)
}

func TestCommodityCoverService_ExplicitOverridePreferred(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	older, err := fileReg.Create(ctx, makeImage("older", "commodity-A", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	c.Assert(err, qt.IsNil)
	picked, err := fileReg.Create(ctx, makeImage("picked", "commodity-A", time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)))
	c.Assert(err, qt.IsNil)

	signing := services.NewFileSigningService(signingKey32, time.Hour)
	coverSvc := services.NewCommodityCoverService(signing)

	// Explicit cover_file_id wins over the otherwise-earlier "older".
	cov, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRefWithCover("commodity-A", picked.ID), "user-1")
	c.Assert(ok, qt.IsTrue)
	c.Assert(cov.FileID, qt.Equals, picked.ID)
	c.Assert(cov.Source, qt.Equals, services.CoverSourceExplicit)
	c.Assert(older.ID, qt.Not(qt.Equals), cov.FileID)
}

func TestCommodityCoverService_StaleOverrideFallsBack(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	older, err := fileReg.Create(ctx, makeImage("older", "commodity-A", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	c.Assert(err, qt.IsNil)

	signing := services.NewFileSigningService(signingKey32, time.Hour)
	coverSvc := services.NewCommodityCoverService(signing)

	// Stale override — file id doesn't exist. Resolver falls back to
	// the first-photo path so a deleted-image race never blanks out the
	// cover slot until the user re-picks.
	cov, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRefWithCover("commodity-A", "no-such-file"), "user-1")
	c.Assert(ok, qt.IsTrue)
	c.Assert(cov.FileID, qt.Equals, older.ID)
	c.Assert(cov.Source, qt.Equals, services.CoverSourceFirstPhoto)
}

func TestCommodityCoverService_OverrideFromOtherCommodityRejected(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	// commodity-A has its own photo; commodity-B's "cover" points at A's
	// file (as could happen via a hand-crafted PATCH). The resolver
	// must reject it and fall through to commodity-B's first-photo path
	// (none here → ok=false). RLS already blocks cross-tenant reads;
	// this guard catches same-tenant cross-commodity foot-guns.
	other, err := fileReg.Create(ctx, makeImage("other", "commodity-A", time.Now()))
	c.Assert(err, qt.IsNil)

	signing := services.NewFileSigningService(signingKey32, time.Hour)
	coverSvc := services.NewCommodityCoverService(signing)

	_, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRefWithCover("commodity-B", other.ID), "user-1")
	c.Assert(ok, qt.IsFalse)
}
