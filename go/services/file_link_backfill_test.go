package services_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

func TestMoveLegacyFileLinks(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	com := fx.commodity(c, "Radio")

	legacy := models.FileEntity{
		Title:    "manual",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		LegacyFileLink: models.LegacyFileLink{
			LegacyLinkedEntityType: models.FileLinkCommodity,
			LegacyLinkedEntityID:   com.ID,
			LegacyLinkedEntityMeta: models.FileLinkMetaDocuments,
		},
		File: &models.File{Path: "manual", OriginalPath: "files/manual", Ext: ".pdf", MIMEType: "application/pdf"},
	}
	// Only a row an older release wrote carries legacy values; FileRegistry.Create
	// drops them, so the rows go through the base registry.
	var ids []string
	for range 3 {
		f, err := fx.set.FileRegistry.(*memory.FileRegistry).Registry.Create(fx.ctx, legacy)
		c.Assert(err, qt.IsNil)
		ids = append(ids, f.ID)
	}

	moved, err := services.MoveLegacyFileLinks(fx.ctx, fx.factorySet)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 3)

	for _, id := range ids {
		got, err := fx.set.FileRegistry.Get(fx.ctx, id)
		c.Assert(err, qt.IsNil)
		c.Assert(linkedIDs(got), qt.DeepEquals, []string{"commodity/" + com.ID})
		c.Assert(got.LegacyFileLink, qt.Equals, models.LegacyFileLink{})
	}

	moved, err = services.MoveLegacyFileLinks(fx.ctx, fx.factorySet)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 0)
}

func TestBackfillFileChecksums(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)

	known := fx.storedFile(c, "known.jpg", "image/jpeg", models.FileCategoryImages)
	withBlob := fx.storedFile(c, "photo.jpg", "image/jpeg", models.FileCategoryImages)
	withBlob = recreateWithoutChecksum(c, fx, withBlob)
	svc := fx.factorySet.FileRegistryFactory.CreateServiceRegistry()

	missing, err := fx.set.FileRegistry.Create(fx.ctx, models.FileEntity{
		Title:     "missing",
		Type:      models.FileTypeDocument,
		Category:  models.FileCategoryDocuments,
		CreatedAt: time.Now(),
		File:      &models.File{Path: "missing", OriginalPath: "files/missing", Ext: ".pdf", MIMEType: "application/pdf"},
	})
	c.Assert(err, qt.IsNil)

	services.BackfillFileChecksums(fx.ctx, fx.factorySet, fx.uploadLocation)

	sum := sha256.Sum256([]byte("content of photo.jpg"))
	got, err := fx.set.FileRegistry.Get(fx.ctx, withBlob.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.SHA256, qt.Equals, hex.EncodeToString(sum[:]))

	got, err = fx.set.FileRegistry.Get(fx.ctx, known.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.SHA256, qt.Equals, "sha-known.jpg")

	got, err = fx.set.FileRegistry.Get(fx.ctx, missing.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.SHA256, qt.Equals, "")

	pending, err := svc.ListPendingChecksum(fx.ctx, "", 10)
	c.Assert(err, qt.IsNil)
	c.Assert(pending, qt.HasLen, 1)
	c.Assert(pending[0].ID, qt.Equals, missing.ID)
}

// recreateWithoutChecksum replaces a file row with a copy that has no
// checksum, the shape of a file stored before checksums existed. Update keeps
// a stored checksum when the incoming one is empty, so the row is recreated.
func recreateWithoutChecksum(c *qt.C, fx fileLinkFixture, file *models.FileEntity) *models.FileEntity {
	c.Helper()
	c.Assert(fx.set.FileRegistry.Delete(fx.ctx, file.ID), qt.IsNil)
	next := *file
	next.ID = ""
	next.UUID = ""
	next.Links = nil
	f := *file.File
	f.SHA256 = ""
	next.File = &f
	created, err := fx.set.FileRegistry.Create(fx.ctx, next)
	c.Assert(err, qt.IsNil)
	return created
}
