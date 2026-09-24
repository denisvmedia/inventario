package postgres_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
)

// TestFileRegistry_Postgres_CategoryFilter exercises the category WHERE-clause
// path on the postgres FileRegistry — the actual path GET /files?category= will
// take in production. CountByCategory is verified end-to-end against a real
// SQL GROUP BY so we catch ordering / scan / RLS regressions that the memory
// fast-path would miss.
func TestFileRegistry_Postgres_CategoryFilter(t *testing.T) {
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	user := getTestUser(c, registrySet)
	ctx := appctx.WithUser(c.Context(), user)
	// setupTestRegistrySet wires the user-aware registry to a specific group
	// already; the FileRegistry resolves the group from its closure, not the
	// context. Loading a LocationGroup into ctx here would only matter for
	// services that read GroupIDFromContext directly — none of which we hit
	// from this test.

	for _, fe := range categoryPostgresSeed() {
		fe.TenantID = user.TenantID
		fe.CreatedByUserID = user.ID
		_, err := registrySet.FileRegistry.Create(ctx, fe)
		c.Assert(err, qt.IsNil)
	}

	t.Run("ListPaginated by category=images", func(t *testing.T) {
		c := qt.New(t)
		cat := models.FileCategoryImages
		got, total, err := registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, &cat, nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 2)
		c.Assert(got, qt.HasLen, 2)
		for _, f := range got {
			c.Assert(f.Category, qt.Equals, models.FileCategoryImages)
		}
	})

	t.Run("Search by category + tag", func(t *testing.T) {
		c := qt.New(t)
		cat := models.FileCategoryDocuments
		got, err := registrySet.FileRegistry.Search(ctx, "", nil, &cat, []string{"manual"}, nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].Category, qt.Equals, models.FileCategoryDocuments)
	})

	t.Run("CountByCategory returns all three buckets, even empty ones", func(t *testing.T) {
		c := qt.New(t)
		counts, bytes, err := registrySet.FileRegistry.CountByCategory(ctx, "", nil, nil, nil, nil)
		c.Assert(err, qt.IsNil)
		// #1622: three buckets — invoices collapsed into documents.
		c.Assert(counts, qt.HasLen, 3)
		c.Assert(counts[models.FileCategoryImages], qt.Equals, 2)
		c.Assert(counts[models.FileCategoryDocuments], qt.Equals, 2)
		c.Assert(counts[models.FileCategoryOther], qt.Equals, 1)
		// SUM(size_bytes) is COALESCEd to 0 — buckets that match no rows
		// don't appear in GROUP BY output, so the three-bucket guarantee is
		// the registry method, not SQL.
		c.Assert(bytes, qt.HasLen, 3)
	})

	t.Run("CountByCategory respects search filter", func(t *testing.T) {
		c := qt.New(t)
		counts, _, err := registrySet.FileRegistry.CountByCategory(ctx, "manual", nil, nil, nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(counts[models.FileCategoryDocuments], qt.Equals, 1)
		c.Assert(counts[models.FileCategoryImages], qt.Equals, 0)
	})

	t.Run("CountByCategory filters by tag (#1622)", func(t *testing.T) {
		c := qt.New(t)
		counts, _, err := registrySet.FileRegistry.CountByCategory(ctx, "", nil, []string{models.FileTagInvoice}, nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(counts[models.FileCategoryDocuments], qt.Equals, 1)
		c.Assert(counts[models.FileCategoryImages], qt.Equals, 0)
		c.Assert(counts[models.FileCategoryOther], qt.Equals, 0)
	})
}

// TestFileRegistry_Postgres_LinkedEntityFilter exercises the
// linked_entity_type / linked_entity_id WHERE-clause path on the
// postgres FileRegistry — what GET /files?linked_entity_type=... will
// hit in production for the commodity / location detail Files panels.
// Cross-entity isolation is the load-bearing assertion.
func TestFileRegistry_Postgres_LinkedEntityFilter(t *testing.T) {
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	user := getTestUser(c, registrySet)
	ctx := appctx.WithUser(c.Context(), user)

	for _, fe := range linkedEntityPostgresSeed() {
		fe.TenantID = user.TenantID
		fe.CreatedByUserID = user.ID
		_, err := registrySet.FileRegistry.Create(ctx, fe)
		c.Assert(err, qt.IsNil)
	}

	commodityType := "commodity"
	commodityA := "com-A"
	commodityB := "com-B"
	locationType := "location"
	locationA := "loc-A"

	t.Run("ListPaginated narrows to one commodity", func(t *testing.T) {
		c := qt.New(t)
		got, total, err := registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, nil, &commodityType, &commodityA)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 3)
		c.Assert(got, qt.HasLen, 3)
		for _, f := range got {
			c.Assert(f.LinkedEntityType, qt.Equals, "commodity")
			c.Assert(f.LinkedEntityID, qt.Equals, "com-A")
		}
	})

	t.Run("ListPaginated narrows to one location", func(t *testing.T) {
		c := qt.New(t)
		got, total, err := registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, nil, &locationType, &locationA)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].LinkedEntityID, qt.Equals, "loc-A")
	})

	t.Run("ListPaginated combines linked entity + category", func(t *testing.T) {
		c := qt.New(t)
		cat := models.FileCategoryImages
		got, total, err := registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, &cat, &commodityType, &commodityA)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].Category, qt.Equals, models.FileCategoryImages)
		c.Assert(got[0].LinkedEntityID, qt.Equals, "com-A")
	})

	t.Run("Search applies linked-entity filter together with text query", func(t *testing.T) {
		c := qt.New(t)
		got, err := registrySet.FileRegistry.Search(ctx, "manual", nil, nil, nil, &commodityType, &commodityA)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].Title, qt.Equals, "manual-A")
	})

	t.Run("ListPaginated commodityA does not leak rows from commodityB", func(t *testing.T) {
		c := qt.New(t)
		got, _, err := registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, nil, &commodityType, &commodityA)
		c.Assert(err, qt.IsNil)
		for _, f := range got {
			c.Assert(f.LinkedEntityID, qt.Not(qt.Equals), "com-B")
		}
		got, _, err = registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, nil, &commodityType, &commodityB)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].LinkedEntityID, qt.Equals, "com-B")
	})

	t.Run("only-type or only-id is treated as no filter", func(t *testing.T) {
		c := qt.New(t)
		got, total, err := registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, nil, &commodityType, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 5)
		c.Assert(got, qt.HasLen, 5)
		got, total, err = registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, nil, nil, &commodityA)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 5)
		c.Assert(got, qt.HasLen, 5)
	})
}

func linkedEntityPostgresSeed() []models.FileEntity {
	now := time.Now()
	mk := func(name, mime, ext string, cat models.FileCategory, linkedType, linkedID, linkedMeta string) models.FileEntity {
		return models.FileEntity{
			Title:            name,
			Type:             models.FileTypeFromMIME(mime),
			Category:         cat,
			LinkedEntityType: linkedType,
			LinkedEntityID:   linkedID,
			LinkedEntityMeta: linkedMeta,
			CreatedAt:        now,
			UpdatedAt:        now,
			File: &models.File{
				Path:         name,
				OriginalPath: name + ext,
				Ext:          ext,
				MIMEType:     mime,
			},
		}
	}
	return []models.FileEntity{
		mk("photo-A", "image/jpeg", ".jpg", models.FileCategoryImages, "commodity", "com-A", "images"),
		// #1622: legacy `invoices` bucket folds into documents; the row
		// still lives under linked_entity_meta="invoices".
		mk("invoice-A", "application/pdf", ".pdf", models.FileCategoryDocuments, "commodity", "com-A", "invoices"),
		mk("manual-A", "application/pdf", ".pdf", models.FileCategoryDocuments, "commodity", "com-A", "manuals"),
		mk("photo-B", "image/png", ".png", models.FileCategoryImages, "commodity", "com-B", "images"),
		mk("loc-photo-A", "image/jpeg", ".jpg", models.FileCategoryImages, "location", "loc-A", "images"),
	}
}

func categoryPostgresSeed() []models.FileEntity {
	now := time.Now()
	mk := func(name, mime, ext string, cat models.FileCategory, tags ...string) models.FileEntity {
		return models.FileEntity{
			Title:     name,
			Type:      models.FileTypeFromMIME(mime),
			Category:  cat,
			Tags:      tags,
			CreatedAt: now,
			UpdatedAt: now,
			File: &models.File{
				Path:         name,
				OriginalPath: name + ext,
				Ext:          ext,
				MIMEType:     mime,
			},
		}
	}
	return []models.FileEntity{
		mk("photo-1", "image/jpeg", ".jpg", models.FileCategoryImages, "lounge"),
		mk("photo-2", "image/png", ".png", models.FileCategoryImages),
		// #1622: invoice-1 lands in `documents` and carries the conventional
		// `invoice` tag — the seed mirrors what AutoTagsForContext does in
		// production.
		mk("invoice-1", "application/pdf", ".pdf", models.FileCategoryDocuments, "tax", models.FileTagInvoice),
		mk("manual-1", "application/pdf", ".pdf", models.FileCategoryDocuments, "manual"),
		mk("clip-1", "video/mp4", ".mp4", models.FileCategoryOther),
	}
}

// TestFileRegistry_Postgres_CountByCategory_LinkedEntity is the SQL half of
// the scope an entity's Files tab needs. The memory registry filters in Go;
// this proves the same predicate reaches the GROUP BY, where a missing WHERE
// clause would silently count the group instead.
func TestFileRegistry_Postgres_CountByCategory_LinkedEntity(t *testing.T) {
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	user := getTestUser(c, registrySet)
	ctx := appctx.WithUser(c.Context(), user)

	now := time.Now()
	mk := func(name, mime, ext string, cat models.FileCategory, commodityID string) models.FileEntity {
		return models.FileEntity{
			Title:            name,
			Type:             models.FileTypeFromMIME(mime),
			Category:         cat,
			LinkedEntityType: "commodity",
			LinkedEntityID:   commodityID,
			CreatedAt:        now,
			UpdatedAt:        now,
			File: &models.File{
				Path:         name,
				OriginalPath: name + ext,
				Ext:          ext,
				MIMEType:     mime,
			},
		}
	}
	for _, fe := range []models.FileEntity{
		mk("a-photo", "image/jpeg", ".jpg", models.FileCategoryImages, "com-a"),
		mk("a-manual", "application/pdf", ".pdf", models.FileCategoryDocuments, "com-a"),
		mk("b-photo-1", "image/jpeg", ".jpg", models.FileCategoryImages, "com-b"),
		mk("b-photo-2", "image/png", ".png", models.FileCategoryImages, "com-b"),
		mk("b-clip", "video/mp4", ".mp4", models.FileCategoryOther, "com-b"),
	} {
		fe.TenantID = user.TenantID
		fe.CreatedByUserID = user.ID
		_, err := registrySet.FileRegistry.Create(ctx, fe)
		c.Assert(err, qt.IsNil)
	}

	linkedType := "commodity"
	comA, comB := "com-a", "com-b"

	countsA, _, err := registrySet.FileRegistry.CountByCategory(ctx, "", nil, nil, &linkedType, &comA)
	c.Assert(err, qt.IsNil)
	c.Check(countsA[models.FileCategoryImages], qt.Equals, 1)
	c.Check(countsA[models.FileCategoryDocuments], qt.Equals, 1)
	c.Check(countsA[models.FileCategoryOther], qt.Equals, 0)

	countsB, _, err := registrySet.FileRegistry.CountByCategory(ctx, "", nil, nil, &linkedType, &comB)
	c.Assert(err, qt.IsNil)
	c.Check(countsB[models.FileCategoryImages], qt.Equals, 2)
	c.Check(countsB[models.FileCategoryDocuments], qt.Equals, 0)
	c.Check(countsB[models.FileCategoryOther], qt.Equals, 1)

	all, _, err := registrySet.FileRegistry.CountByCategory(ctx, "", nil, nil, nil, nil)
	c.Assert(err, qt.IsNil)
	c.Check(all[models.FileCategoryImages], qt.Equals, 3)
	c.Check(all[models.FileCategoryDocuments], qt.Equals, 1)
	c.Check(all[models.FileCategoryOther], qt.Equals, 1)
}
