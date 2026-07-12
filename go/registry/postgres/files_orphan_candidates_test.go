package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// TestFileRegistry_Postgres_ListOrphanCandidates exercises the SQL anti-join
// that backs the orphan-file GC (#2237) against a real PostgreSQL schema.
//
// This worker DELETES USER DATA, so the assertions that matter are the ones
// about what must NOT come back:
//
//   - a STANDALONE file (linked_entity_type = ”) — first-class since #2235;
//     "no link" must never mean "orphan";
//   - an 'export'-linked file — the backup subsystem owns its lifecycle, and
//     because the exports table is never probed, the `deleted_at IS NULL`
//     soft-delete trap on ExportRegistry.Get is structurally unreachable;
//   - an unknown/future link type — the registries do NOT enforce
//     models.FileEntity.ValidateWithContext, so the DB is a superset of the
//     validator's enumeration and the allowlist has to fail closed;
//   - a file whose link target is ALIVE IN ANOTHER GROUP — PUT /files/{id}
//     never validates the target's group, so this row is reachable in
//     production, and it is the one a group-scoped (RLS) query would wrongly
//     report as an orphan. The anti-join therefore has to run in SERVICE mode.
//
// Self-skips when POSTGRES_TEST_DSN is unset (via setupTestRegistrySet).
func TestFileRegistry_Postgres_ListOrphanCandidates(t *testing.T) {
	c := qt.New(t)

	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := postgres.NewFactorySet(dbx)

	ctx := context.Background()

	// Two tenants, each with a group — the cross-group / cross-tenant cases are
	// the ones a group-scoped query would get catastrophically wrong.
	tenantA := mustCreateTenant(c, ctx, fs, "orphan-gc-tenant-a")
	userA := mustCreateUser(c, ctx, fs, tenantA, "a@orphan-gc.example")
	groupA := mustCreateActiveGroup(c, ctx, fs, tenantA, userA.ID)

	tenantB := mustCreateTenant(c, ctx, fs, "orphan-gc-tenant-b")
	userB := mustCreateUser(c, ctx, fs, tenantB, "b@orphan-gc.example")
	groupB := mustCreateActiveGroup(c, ctx, fs, tenantB, userB.ID)

	// A second group inside tenant A, holding the LIVE commodity that a file in
	// group A will legitimately point at.
	groupA2 := mustCreateActiveGroup(c, ctx, fs, tenantA, userA.ID)
	liveCommodityID := mustCreateLiveCommodity(c, ctx, fs, userA, groupA2)

	old := time.Now().Add(-30 * 24 * time.Hour)
	fresh := time.Now().Add(-time.Hour)

	seed := func(tenantID, groupID, userID, linkType, linkID string, createdAt, updatedAt time.Time) string {
		c.Helper()
		row, err := fs.FileRegistryFactory.CreateServiceRegistry().Create(ctx, models.FileEntity{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID: tenantID, GroupID: groupID, CreatedByUserID: userID,
			},
			Title:            "seeded",
			Type:             models.FileTypeImage,
			Category:         models.FileCategoryImages,
			LinkedEntityType: linkType,
			LinkedEntityID:   linkID,
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
			File: &models.File{
				Path: "seeded", OriginalPath: "seeded.jpg", Ext: ".jpg", MIMEType: "image/jpeg",
			},
		})
		c.Assert(err, qt.IsNil)
		return row.ID
	}

	orphanCommodity := seed(tenantA, groupA, userA.ID, "commodity", "no-such-commodity", old, old)
	orphanArea := seed(tenantA, groupA, userA.ID, "area", "no-such-area", old, old)
	orphanLocation := seed(tenantA, groupA, userA.ID, "location", "no-such-location", old, old)

	standalone := seed(tenantA, groupA, userA.ID, "", "", old, old)
	exportLinked := seed(tenantA, groupA, userA.ID, "export", "no-such-export", old, old)
	unknownType := seed(tenantA, groupA, userA.ID, "widget", "no-such-widget", old, old)
	emptyLinkID := seed(tenantA, groupA, userA.ID, "commodity", "", old, old)
	freshUpdated := seed(tenantA, groupA, userA.ID, "commodity", "no-such-commodity", old, fresh)
	freshCreated := seed(tenantA, groupA, userA.ID, "commodity", "no-such-commodity", fresh, old)
	crossGroupLive := seed(tenantA, groupA, userA.ID, "commodity", liveCommodityID, old, old)
	otherTenantOrphan := seed(tenantB, groupB, userB.ID, "commodity", "no-such-commodity", old, old)

	// The service registry is what the worker uses — it must see across every
	// tenant and group.
	svc := fs.FileRegistryFactory.CreateServiceRegistry()
	got, err := svc.ListOrphanCandidates(ctx, time.Now().Add(-72*time.Hour), registry.OrphanCandidateCursor{}, 100)
	c.Assert(err, qt.IsNil)

	ids := make(map[string]bool, len(got))
	for _, f := range got {
		ids[f.ID] = true
	}

	// Positive: the genuine crash-window residues, in EVERY tenant.
	c.Assert(ids[orphanCommodity], qt.IsTrue)
	c.Assert(ids[orphanArea], qt.IsTrue)
	c.Assert(ids[orphanLocation], qt.IsTrue)
	c.Assert(ids[otherTenantOrphan], qt.IsTrue,
		qt.Commentf("the anti-join must run RLS-free: a service-mode scan sees every tenant"))

	// Negative: everything that must never be handed to a destructive worker.
	c.Assert(ids[standalone], qt.IsFalse, qt.Commentf("#2235 standalone file entered the candidate set"))
	c.Assert(ids[exportLinked], qt.IsFalse, qt.Commentf("export-linked file entered the candidate set"))
	c.Assert(ids[unknownType], qt.IsFalse, qt.Commentf("unknown link type must fail closed"))
	c.Assert(ids[emptyLinkID], qt.IsFalse, qt.Commentf("malformed link (empty id) is data, not garbage"))
	c.Assert(ids[freshUpdated], qt.IsFalse, qt.Commentf("updated_at inside the age window (concurrent attach)"))
	c.Assert(ids[freshCreated], qt.IsFalse, qt.Commentf("created_at inside the age window"))
	c.Assert(ids[crossGroupLive], qt.IsFalse,
		qt.Commentf("a file linked ACROSS GROUPS to a LIVE commodity was reported as an orphan"))

	t.Run("ordering is oldest-first and the limit is honoured", func(t *testing.T) {
		c := qt.New(t)
		limited, err := svc.ListOrphanCandidates(ctx, time.Now().Add(-72*time.Hour), registry.OrphanCandidateCursor{}, 2)
		c.Assert(err, qt.IsNil)
		c.Assert(limited, qt.HasLen, 2)
		c.Assert(limited[0].CreatedAt.After(limited[1].CreatedAt), qt.IsFalse)
	})

	t.Run("a non-positive limit returns nothing", func(t *testing.T) {
		c := qt.New(t)
		none, err := svc.ListOrphanCandidates(ctx, time.Now(), registry.OrphanCandidateCursor{}, 0)
		c.Assert(err, qt.IsNil)
		c.Assert(none, qt.HasLen, 0)
	})

	// The keyset must resume STRICTLY after the cursor, tie-breaking on id —
	// every orphan seeded above shares one created_at, so a cursor that only
	// compared timestamps would re-serve the same row forever (the head-of-line
	// livelock the cursor exists to prevent) or skip its peers.
	t.Run("the scan resumes strictly after the (created_at, id) cursor", func(t *testing.T) {
		c := qt.New(t)
		cutoff := time.Now().Add(-72 * time.Hour)

		first, err := svc.ListOrphanCandidates(ctx, cutoff, registry.OrphanCandidateCursor{}, 1)
		c.Assert(err, qt.IsNil)
		c.Assert(first, qt.HasLen, 1)

		cursor := registry.OrphanCandidateCursor{CreatedAt: first[0].CreatedAt, ID: first[0].ID}
		rest, err := svc.ListOrphanCandidates(ctx, cutoff, cursor, 100)
		c.Assert(err, qt.IsNil)

		seen := map[string]bool{first[0].ID: true}
		for _, f := range rest {
			c.Assert(f.ID, qt.Not(qt.Equals), first[0].ID,
				qt.Commentf("the cursor re-served the row it was built from — the scan cannot make progress"))
			seen[f.ID] = true
		}

		// Paging with the cursor still covers the whole candidate set.
		for _, want := range []string{orphanCommodity, orphanArea, orphanLocation, otherTenantOrphan} {
			c.Assert(seen[want], qt.IsTrue, qt.Commentf("a candidate was skipped by the keyset paging"))
		}
	})
}

// TestFileRegistry_Postgres_CountByOriginalPath pins the guard that stops the
// orphan-file GC from destroying a LIVE file's bytes.
//
// files.original_path has NO unique index, and an upload key is
// `t/<tenant>/files/<sanitized-name>-<unix SECONDS><ext>` — no group segment, no
// row segment, no randomness. Two rows in one tenant can therefore legitimately
// share one blob key, while the blob delete inside DeleteFileWithPhysical is
// key-scoped: deleting the orphan would take the live row's bytes with it,
// irreversibly (`files` has no soft-delete).
func TestFileRegistry_Postgres_CountByOriginalPath(t *testing.T) {
	c := qt.New(t)

	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := postgres.NewFactorySet(dbx)

	ctx := context.Background()

	tenant := mustCreateTenant(c, ctx, fs, "count-by-path")
	user := mustCreateUser(c, ctx, fs, tenant, "a@count.example")
	group1 := mustCreateActiveGroup(c, ctx, fs, tenant, user.ID)
	group2 := mustCreateActiveGroup(c, ctx, fs, tenant, user.ID)

	svc := fs.FileRegistryFactory.CreateServiceRegistry()
	mk := func(groupID, originalPath string) {
		c.Helper()
		_, err := svc.Create(ctx, models.FileEntity{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID: tenant, GroupID: groupID, CreatedByUserID: user.ID,
			},
			Title: "seeded", Type: models.FileTypeImage, Category: models.FileCategoryImages,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			File: &models.File{Path: "s", OriginalPath: originalPath, Ext: ".jpg", MIMEType: "image/jpeg"},
		})
		c.Assert(err, qt.IsNil)
	}

	// Two members of two different groups upload `receipt.jpg` in the same
	// second: two rows, one key. Nothing in the system rejects this today.
	const shared = "t/count-by-path/files/receipt-1783824560.jpg"
	const sole = "t/count-by-path/files/sole-1783824560.jpg"
	mk(group1, shared)
	mk(group2, shared)
	mk(group1, sole)

	n, err := svc.CountByOriginalPath(ctx, shared)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 2, qt.Commentf("a key shared by two rows must not look sole-owned to the GC"))

	n, err = svc.CountByOriginalPath(ctx, sole)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)

	t.Run("an empty path never reads as unreferenced", func(t *testing.T) {
		c := qt.New(t)
		n, err := svc.CountByOriginalPath(ctx, "")
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 0)
	})
}

// TestFileRegistry_Postgres_ListIDsByTenant checks the membership set that
// backs the thumbnail sweep: ids for the requested tenant only, across all its
// groups.
func TestFileRegistry_Postgres_ListIDsByTenant(t *testing.T) {
	c := qt.New(t)

	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := postgres.NewFactorySet(dbx)

	ctx := context.Background()

	tenantA := mustCreateTenant(c, ctx, fs, "ids-by-tenant-a")
	userA := mustCreateUser(c, ctx, fs, tenantA, "a@ids.example")
	groupA1 := mustCreateActiveGroup(c, ctx, fs, tenantA, userA.ID)
	groupA2 := mustCreateActiveGroup(c, ctx, fs, tenantA, userA.ID)

	tenantB := mustCreateTenant(c, ctx, fs, "ids-by-tenant-b")
	userB := mustCreateUser(c, ctx, fs, tenantB, "b@ids.example")
	groupB := mustCreateActiveGroup(c, ctx, fs, tenantB, userB.ID)

	svc := fs.FileRegistryFactory.CreateServiceRegistry()
	mk := func(tenantID, groupID, userID string) string {
		c.Helper()
		f, err := svc.Create(ctx, models.FileEntity{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID: tenantID, GroupID: groupID, CreatedByUserID: userID,
			},
			Title: "seeded", Type: models.FileTypeImage, Category: models.FileCategoryImages,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			File: &models.File{Path: "s", OriginalPath: "s.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
		})
		c.Assert(err, qt.IsNil)
		return f.ID
	}

	a1 := mk(tenantA, groupA1, userA.ID)
	a2 := mk(tenantA, groupA2, userA.ID)
	b1 := mk(tenantB, groupB, userB.ID)

	ids, err := svc.ListIDsByTenant(ctx, tenantA)
	c.Assert(err, qt.IsNil)

	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	c.Assert(set[a1], qt.IsTrue)
	c.Assert(set[a2], qt.IsTrue, qt.Commentf("the set must span every group in the tenant"))
	c.Assert(set[b1], qt.IsFalse, qt.Commentf("another tenant's file leaked into the membership set"))

	t.Run("an empty tenant id is rejected", func(t *testing.T) {
		c := qt.New(t)
		_, err := svc.ListIDsByTenant(ctx, "")
		c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	})
}

// mustCreateLiveCommodity builds a real location → area → commodity chain in
// the given group and returns the commodity id.
func mustCreateLiveCommodity(c *qt.C, ctx context.Context, fs *registry.FactorySet, user *models.User, groupID string) string {
	c.Helper()

	group, err := fs.LocationGroupRegistry.Get(ctx, groupID)
	c.Assert(err, qt.IsNil)
	uctx := appctx.WithGroup(appctx.WithUser(ctx, user), group)

	loc, err := fs.LocationRegistryFactory.MustCreateUserRegistry(uctx).Create(uctx, models.Location{
		Name: "Live Location", Address: "1 Live St",
	})
	c.Assert(err, qt.IsNil)

	area, err := fs.AreaRegistryFactory.MustCreateUserRegistry(uctx).Create(uctx, models.Area{
		Name: "Live Area", LocationID: loc.ID,
	})
	c.Assert(err, qt.IsNil)

	com, err := fs.CommodityRegistryFactory.MustCreateUserRegistry(uctx).Create(uctx, models.Commodity{
		Name:                   "Live Commodity",
		ShortName:              "LC",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 new(area.ID),
		Count:                  1,
		OriginalPrice:          decimal.NewFromInt(10),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.Zero,
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	return com.ID
}
