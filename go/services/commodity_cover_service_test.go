package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
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

// makeImage builds an image file entity linked to the commodity with
// `meta=images`, with the supplied id and creation time. Tests use
// distinct timestamps to disambiguate the "earliest by created_at" pick.
func makeImage(id, commodityID string, createdAt time.Time) models.FileEntity {
	return models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: id},
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		Type:      models.FileTypeImage,
		Category:  models.FileCategoryImages,
		Links:     []models.FileLink{{EntityType: "commodity", EntityID: commodityID, Meta: "images"}},
		CreatedAt: createdAt,
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

	cov, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRef("commodity-A"), "user-1", "")
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

	_, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRef("commodity-empty"), "user-1", "")
	c.Assert(ok, qt.IsFalse)

	resolved := coverSvc.ResolveMany(ctx, fileReg, commodityRefs("commodity-empty"), "user-1", "")
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

	resolved := coverSvc.ResolveMany(ctx, fileReg, commodityRefs("commodity-A", "commodity-B", "commodity-C"), "user-1", "")
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
	resolved := coverSvc.ResolveMany(ctx, fileReg, commodityRefs("commodity-A"), "", "")
	c.Assert(resolved, qt.HasLen, 0)
	_, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRef("commodity-A"), "", "")
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

	_, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRef("commodity-A"), "user-1", "")
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
	cov, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRefWithCover("commodity-A", picked.ID), "user-1", "")
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
	cov, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRefWithCover("commodity-A", "no-such-file"), "user-1", "")
	c.Assert(ok, qt.IsTrue)
	c.Assert(cov.FileID, qt.Equals, older.ID)
	c.Assert(cov.Source, qt.Equals, services.CoverSourceFirstPhoto)
}

// An image attached to the item outside its photos is not its cover: the No
// photo filter does not count it either.
func TestCommodityCoverService_OverrideOutsidePhotosFallsBack(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	photo, err := fileReg.Create(ctx, makeImage("photo", "commodity-A", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	c.Assert(err, qt.IsNil)
	filed := makeImage("filed", "commodity-A", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	filed.Links[0].Meta = models.FileLinkMetaDocuments
	other, err := fileReg.Create(ctx, filed)
	c.Assert(err, qt.IsNil)

	signing := services.NewFileSigningService(signingKey32, time.Hour)
	coverSvc := services.NewCommodityCoverService(signing)

	cov, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRefWithCover("commodity-A", other.ID), "user-1", "")
	c.Assert(ok, qt.IsTrue)
	c.Assert(cov.FileID, qt.Equals, photo.ID)
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

	_, ok := coverSvc.ResolveOne(ctx, fileReg, commodityRefWithCover("commodity-B", other.ID), "user-1", "")
	c.Assert(ok, qt.IsFalse)
}

// coverReadCounter counts the reads the cover resolver makes.
type coverReadCounter struct {
	registry.FileRegistry
	reads map[string]int
}

func (r *coverReadCounter) Get(ctx context.Context, id string) (*models.FileEntity, error) {
	r.reads["Get"]++
	return r.FileRegistry.Get(ctx, id)
}

func (r *coverReadCounter) ListByLinkedEntityAndMeta(ctx context.Context, entityType, entityID, meta string) ([]*models.FileEntity, error) {
	r.reads["ListByLinkedEntityAndMeta"]++
	return r.FileRegistry.ListByLinkedEntityAndMeta(ctx, entityType, entityID, meta)
}

func (r *coverReadCounter) GetMany(ctx context.Context, ids []string) ([]*models.FileEntity, error) {
	r.reads["GetMany"]++
	return r.FileRegistry.GetMany(ctx, ids)
}

func (r *coverReadCounter) FirstPhotos(ctx context.Context, commodityIDs []string) (map[string]*models.FileEntity, error) {
	r.reads["FirstPhotos"]++
	return r.FileRegistry.FirstPhotos(ctx, commodityIDs)
}

// A page of items costs two reads however many items it holds: one for the
// covers items name, one for the first photos of the rest. A stale override
// joins the first-photo read.
func TestCommodityCoverService_ResolveMany_ReadsOncePerKind(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	t1 := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	aFirst, err := fileReg.Create(ctx, makeImage("a-first", "commodity-A", t1))
	c.Assert(err, qt.IsNil)
	aPicked, err := fileReg.Create(ctx, makeImage("a-picked", "commodity-A", t2))
	c.Assert(err, qt.IsNil)
	bFirst, err := fileReg.Create(ctx, makeImage("b-first", "commodity-B", t1))
	c.Assert(err, qt.IsNil)
	cFirst, err := fileReg.Create(ctx, makeImage("c-first", "commodity-C", t1))
	c.Assert(err, qt.IsNil)

	counting := &coverReadCounter{FileRegistry: fileReg, reads: map[string]int{}}
	coverSvc := services.NewCommodityCoverService(services.NewFileSigningService(signingKey32, time.Hour))

	items := []*models.Commodity{
		commodityRefWithCover("commodity-A", aPicked.ID),
		commodityRefWithCover("commodity-B", "no-such-file"),
		commodityRef("commodity-C"),
		commodityRef("commodity-D"),
	}
	resolved := coverSvc.ResolveMany(ctx, counting, items, "user-1", "")

	c.Assert(counting.reads, qt.DeepEquals, map[string]int{"GetMany": 1, "FirstPhotos": 1})
	c.Assert(resolved, qt.HasLen, 3)
	c.Assert(resolved["commodity-A"].FileID, qt.Equals, aPicked.ID)
	c.Assert(resolved["commodity-A"].Source, qt.Equals, services.CoverSourceExplicit)
	c.Assert(resolved["commodity-B"].FileID, qt.Equals, bFirst.ID)
	c.Assert(resolved["commodity-B"].Source, qt.Equals, services.CoverSourceFirstPhoto)
	c.Assert(resolved["commodity-C"].FileID, qt.Equals, cFirst.ID)
	c.Assert(aFirst.ID, qt.Not(qt.Equals), resolved["commodity-A"].FileID)
}

// An item in Trash has no cover, and its photos are not read.
func TestCommodityCoverService_SkipsTrashedItems(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)

	_, err := fileReg.Create(ctx, makeImage("photo", "commodity-A", time.Now()))
	c.Assert(err, qt.IsNil)

	trashed := commodityRef("commodity-A")
	trashed.DeletedAt = new(time.Now())
	counting := &coverReadCounter{FileRegistry: fileReg, reads: map[string]int{}}
	coverSvc := services.NewCommodityCoverService(services.NewFileSigningService(signingKey32, time.Hour))

	resolved := coverSvc.ResolveMany(ctx, counting, []*models.Commodity{trashed}, "user-1", "")
	c.Assert(resolved, qt.HasLen, 0)
	c.Assert(counting.reads, qt.HasLen, 0)
}

// coverReadFailure fails the batched cover reads it is given an error for.
type coverReadFailure struct {
	registry.FileRegistry
	getMany, firstPhotos error
}

func (r *coverReadFailure) GetMany(ctx context.Context, ids []string) ([]*models.FileEntity, error) {
	if r.getMany != nil {
		return nil, r.getMany
	}
	return r.FileRegistry.GetMany(ctx, ids)
}

func (r *coverReadFailure) FirstPhotos(ctx context.Context, commodityIDs []string) (map[string]*models.FileEntity, error) {
	if r.firstPhotos != nil {
		return nil, r.firstPhotos
	}
	return r.FileRegistry.FirstPhotos(ctx, commodityIDs)
}

// A failed cover read leaves the items it was for without that cover, and
// the log says which read failed, for which group, and why (#2900).
func TestCommodityCoverService_ResolveMany_LogsAFailedRead(t *testing.T) {
	t1 := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		failing func(registry.FileRegistry) registry.FileRegistry
		read    string
		// The covers that still resolve, by item.
		want map[string]string
	}{
		{
			name: "first photos",
			failing: func(reg registry.FileRegistry) registry.FileRegistry {
				return &coverReadFailure{FileRegistry: reg, firstPhotos: errors.New("connection reset")}
			},
			read: "first photos",
			want: map[string]string{"commodity-A": "a-pinned"},
		},
		{
			name: "named covers",
			failing: func(reg registry.FileRegistry) registry.FileRegistry {
				return &coverReadFailure{FileRegistry: reg, getMany: errors.New("connection reset")}
			},
			read: "named covers",
			// The item that names a cover shows none rather than its first
			// photo, which the user did not pick (#3049).
			want: map[string]string{"commodity-B": "b-first"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			ctx, fileReg := newCoverTestContext(c)
			// The registry gives each file its own ID; the test names them.
			names := map[string]string{}
			for _, f := range []struct {
				name, item string
				at         time.Time
			}{{"a-first", "commodity-A", t1}, {"a-pinned", "commodity-A", t2}, {"b-first", "commodity-B", t1}} {
				created, err := fileReg.Create(ctx, makeImage(f.name, f.item, f.at))
				c.Assert(err, qt.IsNil)
				names[created.ID] = f.name
			}
			pinned := ""
			for id, name := range names {
				if name == "a-pinned" {
					pinned = id
				}
			}
			coverSvc := services.NewCommodityCoverService(services.NewFileSigningService(signingKey32, time.Hour))
			items := []*models.Commodity{commodityRefWithCover("commodity-A", pinned), commodityRef("commodity-B")}

			var resolved map[string]services.ResolvedCover
			logs := captureLogs(c, func() {
				resolved = coverSvc.ResolveMany(ctx, tt.failing(fileReg), items, "user-1", "")
			})

			got := make(map[string]string, len(resolved))
			for id, cover := range resolved {
				got[id] = names[cover.FileID]
			}
			c.Assert(got, qt.DeepEquals, tt.want)
			c.Assert(logs, qt.Contains, `"msg":"cover lookup failed; the items are listed without these covers"`)
			c.Assert(logs, qt.Contains, `"read":"`+tt.read+`"`)
			c.Assert(logs, qt.Contains, `"group_id":"group-1"`)
			c.Assert(logs, qt.Contains, `"error":"connection reset"`)
		})
	}
}

// A client that went away is not a failed cover read.
func TestCommodityCoverService_ResolveMany_DoesNotLogACanceledRead(t *testing.T) {
	c := qt.New(t)
	ctx, fileReg := newCoverTestContext(c)
	_, err := fileReg.Create(ctx, makeImage("b-first", "commodity-B", time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)))
	c.Assert(err, qt.IsNil)
	coverSvc := services.NewCommodityCoverService(services.NewFileSigningService(signingKey32, time.Hour))
	failing := &coverReadFailure{FileRegistry: fileReg, firstPhotos: context.Canceled}

	logs := captureLogs(c, func() {
		c.Assert(coverSvc.ResolveMany(ctx, failing, []*models.Commodity{commodityRef("commodity-B")}, "user-1", ""), qt.HasLen, 0)
	})
	c.Assert(logs, qt.Equals, "")
}
