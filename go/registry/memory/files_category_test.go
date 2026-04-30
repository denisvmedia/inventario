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
		got, total, err := reg.ListPaginated(ctx, 0, 50, nil, &cat)
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
		got, total, err := reg.ListPaginated(ctx, 0, 50, nil, &cat)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].Category, qt.Equals, models.FileCategoryInvoices)
	})

	t.Run("Search combines tags + category", func(t *testing.T) {
		c := qt.New(t)
		cat := models.FileCategoryDocuments
		got, err := reg.Search(ctx, "", nil, &cat, []string{"manual"})
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
