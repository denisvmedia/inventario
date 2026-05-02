package postgres_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
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

	t.Run("ListPaginated by category=photos", func(t *testing.T) {
		c := qt.New(t)
		cat := models.FileCategoryPhotos
		got, total, err := registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, &cat, nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 2)
		c.Assert(got, qt.HasLen, 2)
		for _, f := range got {
			c.Assert(f.Category, qt.Equals, models.FileCategoryPhotos)
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

	t.Run("CountByCategory returns all four buckets, even empty ones", func(t *testing.T) {
		c := qt.New(t)
		counts, err := registrySet.FileRegistry.CountByCategory(ctx, "", nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(counts, qt.HasLen, 4)
		c.Assert(counts[models.FileCategoryPhotos], qt.Equals, 2)
		c.Assert(counts[models.FileCategoryInvoices], qt.Equals, 1)
		c.Assert(counts[models.FileCategoryDocuments], qt.Equals, 1)
		c.Assert(counts[models.FileCategoryOther], qt.Equals, 1)
	})

	t.Run("CountByCategory respects search filter", func(t *testing.T) {
		c := qt.New(t)
		counts, err := registrySet.FileRegistry.CountByCategory(ctx, "manual", nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(counts[models.FileCategoryDocuments], qt.Equals, 1)
		c.Assert(counts[models.FileCategoryPhotos], qt.Equals, 0)
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
		cat := models.FileCategoryPhotos
		got, total, err := registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, &cat, &commodityType, &commodityA)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].Category, qt.Equals, models.FileCategoryPhotos)
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
		mk("photo-A", "image/jpeg", ".jpg", models.FileCategoryPhotos, "commodity", "com-A", "images"),
		mk("invoice-A", "application/pdf", ".pdf", models.FileCategoryInvoices, "commodity", "com-A", "invoices"),
		mk("manual-A", "application/pdf", ".pdf", models.FileCategoryDocuments, "commodity", "com-A", "manuals"),
		mk("photo-B", "image/png", ".png", models.FileCategoryPhotos, "commodity", "com-B", "images"),
		mk("loc-photo-A", "image/jpeg", ".jpg", models.FileCategoryPhotos, "location", "loc-A", "images"),
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
		mk("photo-1", "image/jpeg", ".jpg", models.FileCategoryPhotos, "lounge"),
		mk("photo-2", "image/png", ".png", models.FileCategoryPhotos),
		mk("invoice-1", "application/pdf", ".pdf", models.FileCategoryInvoices, "tax"),
		mk("manual-1", "application/pdf", ".pdf", models.FileCategoryDocuments, "manual"),
		mk("clip-1", "video/mp4", ".mp4", models.FileCategoryOther),
	}
}
