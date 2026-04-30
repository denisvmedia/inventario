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
		got, total, err := registrySet.FileRegistry.ListPaginated(ctx, 0, 50, nil, &cat)
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
		got, err := registrySet.FileRegistry.Search(ctx, "", nil, &cat, []string{"manual"})
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
