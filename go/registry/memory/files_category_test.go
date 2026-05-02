package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestFileRegistry_Memory_FilterByCategory(t *testing.T) {
	c := qt.New(t)

	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
		Slug: "g1",
	})

	reg := memory.NewFileRegistryFactory().MustCreateUserRegistry(ctx)
	for _, fe := range categoryTestFiles() {
		_, err := reg.Create(ctx, fe)
		c.Assert(err, qt.IsNil)
	}

	t.Run("ListPaginated by category=photos", func(t *testing.T) {
		c := qt.New(t)
		cat := models.FileCategoryPhotos
		got, total, err := reg.ListPaginated(ctx, 0, 50, nil, &cat, nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 2)
		c.Assert(got, qt.HasLen, 2)
		for _, f := range got {
			c.Assert(f.Category, qt.Equals, models.FileCategoryPhotos)
		}
	})

	t.Run("ListPaginated by category=invoices", func(t *testing.T) {
		c := qt.New(t)
		cat := models.FileCategoryInvoices
		got, total, err := reg.ListPaginated(ctx, 0, 50, nil, &cat, nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].Category, qt.Equals, models.FileCategoryInvoices)
	})

	t.Run("Search combines tags + category", func(t *testing.T) {
		c := qt.New(t)
		cat := models.FileCategoryDocuments
		got, err := reg.Search(ctx, "", nil, &cat, []string{"manual"}, nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].Category, qt.Equals, models.FileCategoryDocuments)
	})

	t.Run("CountByCategory returns all four buckets", func(t *testing.T) {
		c := qt.New(t)
		counts, err := reg.CountByCategory(ctx, "", nil, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(counts, qt.HasLen, 4)
		c.Assert(counts[models.FileCategoryPhotos], qt.Equals, 2)
		c.Assert(counts[models.FileCategoryInvoices], qt.Equals, 1)
		c.Assert(counts[models.FileCategoryDocuments], qt.Equals, 1)
		c.Assert(counts[models.FileCategoryOther], qt.Equals, 1)
	})

	t.Run("CountByCategory respects tag filter", func(t *testing.T) {
		c := qt.New(t)
		counts, err := reg.CountByCategory(ctx, "", nil, []string{"manual"})
		c.Assert(err, qt.IsNil)
		c.Assert(counts[models.FileCategoryPhotos], qt.Equals, 0)
		c.Assert(counts[models.FileCategoryDocuments], qt.Equals, 1)
		c.Assert(counts[models.FileCategoryInvoices], qt.Equals, 0)
		c.Assert(counts[models.FileCategoryOther], qt.Equals, 0)
	})
}

// TestFileRegistry_Memory_FilterByLinkedEntity exercises the
// linked_entity_type + linked_entity_id filter introduced for the
// commodity / location detail Files panel. Both must be supplied
// together; either alone is treated as "no filter".
func TestFileRegistry_Memory_FilterByLinkedEntity(t *testing.T) {
	c := qt.New(t)

	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
		Slug: "g1",
	})

	reg := memory.NewFileRegistryFactory().MustCreateUserRegistry(ctx)
	for _, fe := range linkedEntityTestFiles() {
		_, err := reg.Create(ctx, fe)
		c.Assert(err, qt.IsNil)
	}

	commodityType := "commodity"
	commodityA := "com-A"
	commodityB := "com-B"
	locationType := "location"
	locationA := "loc-A"

	t.Run("ListPaginated narrows to one commodity", func(t *testing.T) {
		c := qt.New(t)
		got, total, err := reg.ListPaginated(ctx, 0, 50, nil, nil, &commodityType, &commodityA)
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
		got, total, err := reg.ListPaginated(ctx, 0, 50, nil, nil, &locationType, &locationA)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].LinkedEntityID, qt.Equals, "loc-A")
	})

	t.Run("ListPaginated combines linked entity + category", func(t *testing.T) {
		c := qt.New(t)
		cat := models.FileCategoryPhotos
		got, total, err := reg.ListPaginated(ctx, 0, 50, nil, &cat, &commodityType, &commodityA)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].Category, qt.Equals, models.FileCategoryPhotos)
		c.Assert(got[0].LinkedEntityID, qt.Equals, "com-A")
	})

	t.Run("Search applies linked-entity filter together with text query", func(t *testing.T) {
		c := qt.New(t)
		got, err := reg.Search(ctx, "manual", nil, nil, nil, &commodityType, &commodityA)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].Title, qt.Equals, "manual-A")
	})

	t.Run("ListPaginated commodityA does not leak rows from commodityB", func(t *testing.T) {
		c := qt.New(t)
		got, _, err := reg.ListPaginated(ctx, 0, 50, nil, nil, &commodityType, &commodityA)
		c.Assert(err, qt.IsNil)
		for _, f := range got {
			c.Assert(f.LinkedEntityID, qt.Not(qt.Equals), "com-B")
		}
		got, _, err = reg.ListPaginated(ctx, 0, 50, nil, nil, &commodityType, &commodityB)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].LinkedEntityID, qt.Equals, "com-B")
	})

	t.Run("only-type or only-id is treated as no filter", func(t *testing.T) {
		c := qt.New(t)
		// Only type → returns everything (filter inactive).
		got, total, err := reg.ListPaginated(ctx, 0, 50, nil, nil, &commodityType, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 5)
		c.Assert(got, qt.HasLen, 5)
		// Only id → same.
		got, total, err = reg.ListPaginated(ctx, 0, 50, nil, nil, nil, &commodityA)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 5)
		c.Assert(got, qt.HasLen, 5)
	})
}

func linkedEntityTestFiles() []models.FileEntity {
	mk := func(name, mime, ext string, cat models.FileCategory, linkedType, linkedID, linkedMeta string) models.FileEntity {
		return models.FileEntity{
			Title:            name,
			Type:             models.FileTypeFromMIME(mime),
			Category:         cat,
			LinkedEntityType: linkedType,
			LinkedEntityID:   linkedID,
			LinkedEntityMeta: linkedMeta,
			File: &models.File{
				Path:         name,
				OriginalPath: name + ext,
				Ext:          ext,
				MIMEType:     mime,
			},
		}
	}
	return []models.FileEntity{
		// commodity A — three files across two categories.
		mk("photo-A", "image/jpeg", ".jpg", models.FileCategoryPhotos, "commodity", "com-A", "images"),
		mk("invoice-A", "application/pdf", ".pdf", models.FileCategoryInvoices, "commodity", "com-A", "invoices"),
		mk("manual-A", "application/pdf", ".pdf", models.FileCategoryDocuments, "commodity", "com-A", "manuals"),
		// commodity B — one file, must not leak into A's filter.
		mk("photo-B", "image/png", ".png", models.FileCategoryPhotos, "commodity", "com-B", "images"),
		// location A — one file, different entity type.
		mk("loc-photo-A", "image/jpeg", ".jpg", models.FileCategoryPhotos, "location", "loc-A", "images"),
	}
}

func categoryTestFiles() []models.FileEntity {
	mk := func(name, mime, ext string, cat models.FileCategory, tags ...string) models.FileEntity {
		return models.FileEntity{
			Title:    name,
			Type:     models.FileTypeFromMIME(mime),
			Category: cat,
			Tags:     tags,
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
