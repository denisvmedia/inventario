package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/appctx"
	_ "go.5x5.cz/inventario/internal/fileblob" // Register file driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

type fileLinkFixture struct {
	ctx            context.Context
	factorySet     *registry.FactorySet
	set            *registry.Set
	uploadLocation string
	areaID         string
}

func newFileLinkFixture(c *qt.C) fileLinkFixture {
	c.Helper()
	factorySet := memory.NewFactorySet()
	ctx := appctx.WithGroup(newTestContext(factorySet), &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "test-tenant-id",
		},
	})
	set, err := factorySet.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)

	loc, err := set.LocationRegistry.Create(ctx, models.Location{Name: "Home"})
	c.Assert(err, qt.IsNil)
	area, err := set.AreaRegistry.Create(ctx, models.Area{Name: "Office", LocationID: loc.ID})
	c.Assert(err, qt.IsNil)

	return fileLinkFixture{
		ctx:            ctx,
		factorySet:     factorySet,
		set:            set,
		uploadLocation: uploadLocationForTempDir(c.TempDir()),
		areaID:         area.ID,
	}
}

func (fx fileLinkFixture) commodity(c *qt.C, name string) *models.Commodity {
	c.Helper()
	com, err := fx.set.CommodityRegistry.Create(fx.ctx, models.Commodity{Name: name, AreaID: new(fx.areaID)})
	c.Assert(err, qt.IsNil)
	return com
}

// storedFile creates a file row and writes its blob, so a delete path that
// removes the blob is observable.
func (fx fileLinkFixture) storedFile(c *qt.C, name, mime string, category models.FileCategory, links ...models.FileLink) *models.FileEntity {
	c.Helper()
	key := "files/" + name
	b, err := blob.OpenBucket(fx.ctx, fx.uploadLocation)
	c.Assert(err, qt.IsNil)
	c.Assert(b.WriteAll(fx.ctx, key, []byte("content of "+name), nil), qt.IsNil)
	c.Assert(b.Close(), qt.IsNil)

	now := time.Now()
	file, err := fx.set.FileRegistry.Create(fx.ctx, models.FileEntity{
		Title:     name,
		Type:      models.FileTypeFromMIME(mime),
		Category:  category,
		CreatedAt: now,
		UpdatedAt: now,
		Links:     links,
		File: &models.File{
			Path:         name,
			OriginalPath: key,
			Ext:          ".bin",
			MIMEType:     mime,
			SizeBytes:    int64(len("content of " + name)),
			SHA256:       "sha-" + name,
		},
	})
	c.Assert(err, qt.IsNil)
	return file
}

func (fx fileLinkFixture) blobExists(c *qt.C, key string) bool {
	c.Helper()
	b, err := blob.OpenBucket(fx.ctx, fx.uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	ok, err := b.Exists(fx.ctx, key)
	c.Assert(err, qt.IsNil)
	return ok
}

func linkedIDs(file *models.FileEntity) []string {
	ids := make([]string, 0, len(file.Links))
	for _, l := range file.Links {
		ids = append(ids, l.EntityType+"/"+l.EntityID)
	}
	return ids
}

// TestFileLinks_OneReceiptThreeItems is the acceptance scenario of #2806: one
// receipt attached to three items; removing it from one item and deleting
// another item leaves the third item with the same file and the same bytes.
func TestFileLinks_OneReceiptThreeItems(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	links := services.NewFileLinkService(fx.factorySet)
	entities := services.NewEntityService(fx.factorySet, fx.uploadLocation)

	tv := fx.commodity(c, "TV")
	soundbar := fx.commodity(c, "Soundbar")
	mount := fx.commodity(c, "Wall mount")
	receipt := fx.storedFile(c, "receipt.pdf", "application/pdf", models.FileCategoryDocuments)

	for _, com := range []*models.Commodity{tv, soundbar, mount} {
		_, created, err := links.Attach(fx.ctx, receipt.ID, models.FileLinkCommodity, com.ID, "")
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
	}

	got, err := fx.set.FileRegistry.Get(fx.ctx, receipt.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Links, qt.HasLen, 3)
	for _, l := range got.Links {
		c.Assert(l.Meta, qt.Equals, models.FileLinkMetaDocuments)
	}
	all, err := fx.set.FileRegistry.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1, qt.Commentf("attaching must not create file rows"))

	// Remove the attachment from the soundbar.
	soundbarLink := got.LinkTo(models.FileLinkCommodity, soundbar.ID)
	c.Assert(soundbarLink, qt.IsNotNil)
	got, err = links.Detach(fx.ctx, receipt.ID, soundbarLink.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.LinkTo(models.FileLinkCommodity, soundbar.ID), qt.IsNil)
	c.Assert(got.Links, qt.HasLen, 2)

	// Delete the TV.
	c.Assert(entities.DeleteCommodityRecursive(fx.ctx, tv.ID), qt.IsNil)

	// The wall mount still opens the same file with the same bytes.
	mountFiles, err := fx.set.FileRegistry.ListByLinkedEntity(fx.ctx, models.FileLinkCommodity, mount.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(mountFiles, qt.HasLen, 1)
	c.Assert(mountFiles[0].ID, qt.Equals, receipt.ID)
	c.Assert(linkedIDs(mountFiles[0]), qt.DeepEquals, []string{"commodity/" + mount.ID})
	c.Assert(fx.blobExists(c, receipt.OriginalPath), qt.IsTrue)

	soundbarFiles, err := fx.set.FileRegistry.ListByLinkedEntity(fx.ctx, models.FileLinkCommodity, soundbar.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(soundbarFiles, qt.HasLen, 0)
}

func TestFileLinkService_Attach(t *testing.T) {
	t.Run("is idempotent", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkFixture(c)
		svc := services.NewFileLinkService(fx.factorySet)
		com := fx.commodity(c, "Drill")
		file := fx.storedFile(c, "manual.pdf", "application/pdf", models.FileCategoryDocuments)

		first, created, err := svc.Attach(fx.ctx, file.ID, models.FileLinkCommodity, com.ID, models.FileLinkMetaDocuments)
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
		second, created, err := svc.Attach(fx.ctx, file.ID, models.FileLinkCommodity, com.ID, models.FileLinkMetaDocuments)
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsFalse)
		c.Assert(second.Links, qt.HasLen, 1)
		c.Assert(second.Links[0].ID, qt.Equals, first.Links[0].ID)
	})

	t.Run("picks the bucket from the file category", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkFixture(c)
		svc := services.NewFileLinkService(fx.factorySet)
		com := fx.commodity(c, "Lamp")
		photo := fx.storedFile(c, "lamp.jpg", "image/jpeg", models.FileCategoryImages)

		got, _, err := svc.Attach(fx.ctx, photo.ID, models.FileLinkCommodity, com.ID, "")
		c.Assert(err, qt.IsNil)
		c.Assert(got.Links[0].Meta, qt.Equals, models.FileLinkMetaImages)

		got, _, err = svc.Attach(fx.ctx, photo.ID, models.FileLinkArea, fx.areaID, "")
		c.Assert(err, qt.IsNil)
		c.Assert(got.LinkTo(models.FileLinkArea, fx.areaID).Meta, qt.Equals, models.FileLinkMetaImages)
	})

	t.Run("first placement sets the category from the bucket", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkFixture(c)
		svc := services.NewFileLinkService(fx.factorySet)
		com := fx.commodity(c, "Fridge")
		scan := fx.storedFile(c, "receipt-scan.jpg", "image/jpeg", models.FileCategoryImages)

		got, _, err := svc.Attach(fx.ctx, scan.ID, models.FileLinkCommodity, com.ID, models.FileLinkMetaDocuments)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Category, qt.Equals, models.FileCategoryDocuments)

		// A later attachment does not move it again.
		other := fx.commodity(c, "Freezer")
		got, _, err = svc.Attach(fx.ctx, scan.ID, models.FileLinkCommodity, other.ID, models.FileLinkMetaImages)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Category, qt.Equals, models.FileCategoryDocuments)
	})

	t.Run("rejects", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkFixture(c)
		svc := services.NewFileLinkService(fx.factorySet)
		com := fx.commodity(c, "Chair")
		file := fx.storedFile(c, "chair.pdf", "application/pdf", models.FileCategoryDocuments)
		export := fx.storedFile(c, "backup.inb", "application/octet-stream", models.FileCategoryOther,
			models.FileLink{EntityType: models.FileLinkExport, EntityID: "export-1", Meta: "inb-2.0"})

		_, _, err := svc.Attach(fx.ctx, file.ID, models.FileLinkCommodity, "no-such-item", "")
		c.Assert(err, qt.ErrorIs, services.ErrFileLinkTargetNotFound)

		_, _, err = svc.Attach(fx.ctx, file.ID, models.FileLinkExport, "export-1", "")
		c.Assert(err, qt.ErrorIs, services.ErrFileLinkInvalid)

		_, _, err = svc.Attach(fx.ctx, file.ID, models.FileLinkCommodity, com.ID, models.FileLinkMetaFiles)
		c.Assert(err, qt.ErrorIs, services.ErrFileLinkInvalid)

		_, _, err = svc.Attach(fx.ctx, export.ID, models.FileLinkCommodity, com.ID, "")
		c.Assert(err, qt.ErrorIs, services.ErrExportFileNotAttachable)

		_, _, err = svc.Attach(fx.ctx, "no-such-file", models.FileLinkCommodity, com.ID, "")
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("does not reach another group's entity", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkFixture(c)
		svc := services.NewFileLinkService(fx.factorySet)
		file := fx.storedFile(c, "doc.pdf", "application/pdf", models.FileCategoryDocuments)

		otherCtx := appctx.WithGroup(fx.ctx, &models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "other-group"},
				TenantID: "test-tenant-id",
			},
		})
		otherSet, err := fx.factorySet.CreateUserRegistrySet(otherCtx)
		c.Assert(err, qt.IsNil)
		loc, err := otherSet.LocationRegistry.Create(otherCtx, models.Location{Name: "Elsewhere"})
		c.Assert(err, qt.IsNil)

		_, _, err = svc.Attach(fx.ctx, file.ID, models.FileLinkLocation, loc.ID, "")
		c.Assert(err, qt.ErrorIs, services.ErrFileLinkTargetNotFound)
	})
}

func TestFileLinkService_Detach(t *testing.T) {
	t.Run("last attachment leaves a standalone file", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkFixture(c)
		svc := services.NewFileLinkService(fx.factorySet)
		com := fx.commodity(c, "Bike")
		file := fx.storedFile(c, "bike.pdf", "application/pdf", models.FileCategoryDocuments,
			models.FileLink{EntityType: models.FileLinkCommodity, EntityID: com.ID, Meta: models.FileLinkMetaDocuments})

		got, err := svc.Detach(fx.ctx, file.ID, file.Links[0].ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Links, qt.HasLen, 0)
		c.Assert(fx.blobExists(c, file.OriginalPath), qt.IsTrue)

		stored, err := fx.set.FileRegistry.Get(fx.ctx, file.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(stored.Links, qt.HasLen, 0)
	})

	t.Run("clears only the cover of the item it leaves", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkFixture(c)
		svc := services.NewFileLinkService(fx.factorySet)
		a := fx.commodity(c, "Speaker left")
		b := fx.commodity(c, "Speaker right")
		photo := fx.storedFile(c, "speakers.jpg", "image/jpeg", models.FileCategoryImages,
			models.FileLink{EntityType: models.FileLinkCommodity, EntityID: a.ID, Meta: models.FileLinkMetaImages},
			models.FileLink{EntityType: models.FileLinkCommodity, EntityID: b.ID, Meta: models.FileLinkMetaImages})

		for _, com := range []*models.Commodity{a, b} {
			com.CoverFileID = new(photo.ID)
			_, err := fx.set.CommodityRegistry.Update(fx.ctx, *com)
			c.Assert(err, qt.IsNil)
		}

		_, err := svc.Detach(fx.ctx, photo.ID, photo.LinkTo(models.FileLinkCommodity, a.ID).ID)
		c.Assert(err, qt.IsNil)

		gotA, err := fx.set.CommodityRegistry.Get(fx.ctx, a.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(gotA.CoverFileID, qt.IsNil)
		gotB, err := fx.set.CommodityRegistry.Get(fx.ctx, b.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(gotB.CoverFileID, qt.IsNotNil)
		c.Assert(*gotB.CoverFileID, qt.Equals, photo.ID)
	})

	t.Run("refuses an export link and an unknown link", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkFixture(c)
		svc := services.NewFileLinkService(fx.factorySet)
		export := fx.storedFile(c, "backup.inb", "application/octet-stream", models.FileCategoryOther,
			models.FileLink{EntityType: models.FileLinkExport, EntityID: "export-1", Meta: "inb-2.0"})

		_, err := svc.Detach(fx.ctx, export.ID, export.Links[0].ID)
		c.Assert(err, qt.ErrorIs, services.ErrExportLinkNotRemovable)

		_, err = svc.Detach(fx.ctx, export.ID, "no-such-link")
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})
}

func TestFileLinkService_Duplicates(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	svc := services.NewFileLinkService(fx.factorySet)

	original := fx.storedFile(c, "a.pdf", "application/pdf", models.FileCategoryDocuments)
	copyFile := fx.storedFile(c, "b.pdf", "application/pdf", models.FileCategoryDocuments)
	copyFile.SHA256 = original.SHA256
	_, err := fx.set.FileRegistry.Update(fx.ctx, *copyFile)
	c.Assert(err, qt.IsNil)
	other := fx.storedFile(c, "c.pdf", "application/pdf", models.FileCategoryDocuments)

	dups, err := svc.Duplicates(fx.ctx, copyFile.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(dups, qt.HasLen, 1)
	c.Assert(dups[0].ID, qt.Equals, original.ID)

	dups, err = svc.Duplicates(fx.ctx, other.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(dups, qt.HasLen, 0)
}

func TestEntityService_DeleteCommodityRecursive_KeepsSharedFiles(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	entities := services.NewEntityService(fx.factorySet, fx.uploadLocation)

	gone := fx.commodity(c, "Old TV")
	kept := fx.commodity(c, "New TV")
	own := fx.storedFile(c, "old-tv.jpg", "image/jpeg", models.FileCategoryImages,
		models.FileLink{EntityType: models.FileLinkCommodity, EntityID: gone.ID, Meta: models.FileLinkMetaImages})
	shared := fx.storedFile(c, "warranty.pdf", "application/pdf", models.FileCategoryDocuments,
		models.FileLink{EntityType: models.FileLinkCommodity, EntityID: gone.ID, Meta: models.FileLinkMetaDocuments},
		models.FileLink{EntityType: models.FileLinkCommodity, EntityID: kept.ID, Meta: models.FileLinkMetaDocuments})

	c.Assert(entities.DeleteCommodityRecursive(fx.ctx, gone.ID), qt.IsNil)

	_, err := fx.set.FileRegistry.Get(fx.ctx, own.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(fx.blobExists(c, own.OriginalPath), qt.IsFalse)

	got, err := fx.set.FileRegistry.Get(fx.ctx, shared.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(linkedIDs(got), qt.DeepEquals, []string{"commodity/" + kept.ID})
	c.Assert(fx.blobExists(c, shared.OriginalPath), qt.IsTrue)
}
