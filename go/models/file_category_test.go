package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestFileCategoryFromMIME(t *testing.T) {
	cases := []struct {
		mime string
		want models.FileCategory
	}{
		{"image/jpeg", models.FileCategoryPhotos},
		{"image/png", models.FileCategoryPhotos},
		{"image/heic", models.FileCategoryPhotos},
		{"application/pdf", models.FileCategoryDocuments},
		{"text/plain", models.FileCategoryDocuments},
		{"text/csv", models.FileCategoryDocuments},
		{"application/msword", models.FileCategoryDocuments},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", models.FileCategoryDocuments},
		{"application/vnd.ms-excel", models.FileCategoryDocuments},
		{"application/json", models.FileCategoryDocuments},
		{"video/mp4", models.FileCategoryOther},
		{"audio/mpeg", models.FileCategoryOther},
		{"application/zip", models.FileCategoryOther},
		{"application/octet-stream", models.FileCategoryOther},
		{"", models.FileCategoryOther},
	}

	for _, tc := range cases {
		t.Run(tc.mime, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(models.FileCategoryFromMIME(tc.mime), qt.Equals, tc.want)
		})
	}
}

func TestFileCategoryFromContext(t *testing.T) {
	cases := []struct {
		name             string
		linkedEntityType string
		linkedEntityMeta string
		mime             string
		want             models.FileCategory
	}{
		// Legacy commodity bucket names take precedence over MIME type so the
		// "manuals" bucket lands in Documents even for image scans.
		{"commodity/images PDF still photos by bucket name",
			"commodity", "images", "application/pdf", models.FileCategoryPhotos},
		{"commodity/manuals image still documents by bucket name",
			"commodity", "manuals", "image/jpeg", models.FileCategoryDocuments},
		{"commodity/invoices image still invoices by bucket name",
			"commodity", "invoices", "image/jpeg", models.FileCategoryInvoices},
		{"location/images uses photos bucket",
			"location", "images", "image/png", models.FileCategoryPhotos},
		// Location/files has no bucket-name hint; falls through to MIME.
		{"location/files PDF falls through to documents via MIME",
			"location", "files", "application/pdf", models.FileCategoryDocuments},
		{"location/files unknown MIME falls through to other",
			"location", "files", "video/mp4", models.FileCategoryOther},
		// Standalone (no linked entity) and export both fall through to MIME.
		{"standalone image",
			"", "", "image/jpeg", models.FileCategoryPhotos},
		{"standalone PDF",
			"", "", "application/pdf", models.FileCategoryDocuments},
		{"export bundles fall through to MIME",
			"export", "xml-1.0", "application/xml", models.FileCategoryOther},
		// Unknown commodity meta falls through to MIME (defensive).
		{"unknown commodity meta falls through to MIME",
			"commodity", "weird", "image/jpeg", models.FileCategoryPhotos},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got := models.FileCategoryFromContext(tc.linkedEntityType, tc.linkedEntityMeta, tc.mime)
			c.Assert(got, qt.Equals, tc.want)
		})
	}
}

func TestFileEntity_ValidateWithContext_Category(t *testing.T) {
	t.Run("rejects empty category", func(t *testing.T) {
		c := qt.New(t)
		fe := buildValidFileEntity()
		fe.Category = ""
		err := fe.ValidateWithContext(context.Background())
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Contains, "category")
	})

	t.Run("rejects unknown category", func(t *testing.T) {
		c := qt.New(t)
		fe := buildValidFileEntity()
		fe.Category = models.FileCategory("warranty")
		err := fe.ValidateWithContext(context.Background())
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Contains, "category")
	})

	t.Run("accepts each of the four valid categories", func(t *testing.T) {
		c := qt.New(t)
		for _, cat := range models.ValidFileCategories {
			fe := buildValidFileEntity()
			fe.Category = cat
			err := fe.ValidateWithContext(context.Background())
			c.Assert(err, qt.IsNil, qt.Commentf("category %q should be accepted", cat))
		}
	})
}

func buildValidFileEntity() models.FileEntity {
	return models.FileEntity{
		Title:    "doc",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		File: &models.File{
			Path:         "doc",
			OriginalPath: "doc.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
}
