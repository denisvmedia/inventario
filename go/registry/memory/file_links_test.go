package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

func fileLinkTestContext(c *qt.C, groupID string) context.Context {
	c.Helper()
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	return appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: groupID},
			TenantID: "tenant-1",
		},
	})
}

func newLinkTestFile(name string, size int64, links ...models.FileLink) models.FileEntity {
	return models.FileEntity{
		Title:    name,
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Links:    links,
		File: &models.File{
			Path:         name,
			OriginalPath: "files/" + name,
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
			SizeBytes:    size,
		},
	}
}

func commodityLink(id, meta string) models.FileLink {
	return models.FileLink{EntityType: models.FileLinkCommodity, EntityID: id, Meta: meta}
}

func linkKeys(file *models.FileEntity) []string {
	keys := make([]string, 0, len(file.Links))
	for _, l := range file.Links {
		keys = append(keys, l.EntityType+"/"+l.EntityID+"/"+l.Meta)
	}
	return keys
}

func TestFileRegistry_Memory_Links(t *testing.T) {
	factory := memory.NewFileRegistryFactory()

	t.Run("create, attach, list", func(t *testing.T) {
		c := qt.New(t)
		ctx := fileLinkTestContext(c, "group-1")
		reg := factory.MustCreateUserRegistry(ctx)

		file, err := reg.Create(ctx, newLinkTestFile("receipt", 100, commodityLink("tv", "documents")))
		c.Assert(err, qt.IsNil)
		c.Assert(linkKeys(file), qt.DeepEquals, []string{"commodity/tv/documents"})
		c.Assert(file.Links[0].FileID, qt.Equals, file.ID)
		c.Assert(file.Links[0].GroupID, qt.Equals, "group-1")
		c.Assert(file.Links[0].TenantID, qt.Equals, "tenant-1")

		link, created, err := reg.AddLink(ctx, models.FileLink{FileID: file.ID, EntityType: models.FileLinkCommodity, EntityID: "soundbar", Meta: "documents"})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
		again, created, err := reg.AddLink(ctx, models.FileLink{FileID: file.ID, EntityType: models.FileLinkCommodity, EntityID: "soundbar", Meta: "images"})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsFalse)
		c.Assert(again.ID, qt.Equals, link.ID)
		c.Assert(again.Meta, qt.Equals, "documents")

		got, err := reg.Get(ctx, file.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(linkKeys(got), qt.DeepEquals, []string{"commodity/tv/documents", "commodity/soundbar/documents"})

		for _, id := range []string{"tv", "soundbar"} {
			files, err := reg.ListByLinkedEntity(ctx, models.FileLinkCommodity, id)
			c.Assert(err, qt.IsNil)
			c.Assert(files, qt.HasLen, 1)
			c.Assert(files[0].ID, qt.Equals, file.ID)
		}

		_, _, err = reg.AddLink(ctx, models.FileLink{FileID: "missing", EntityType: models.FileLinkCommodity, EntityID: "tv", Meta: "documents"})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("remove link keeps the file", func(t *testing.T) {
		c := qt.New(t)
		ctx := fileLinkTestContext(c, "group-1")
		reg := factory.MustCreateUserRegistry(ctx)

		file, err := reg.Create(ctx, newLinkTestFile("manual", 10, commodityLink("drill", "documents")))
		c.Assert(err, qt.IsNil)
		other, err := reg.Create(ctx, newLinkTestFile("other", 10))
		c.Assert(err, qt.IsNil)

		_, err = reg.RemoveLink(ctx, other.ID, file.Links[0].ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		removed, err := reg.RemoveLink(ctx, file.ID, file.Links[0].ID)
		c.Assert(err, qt.IsNil)
		c.Assert(removed.EntityID, qt.Equals, "drill")

		got, err := reg.Get(ctx, file.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Links, qt.HasLen, 0)
	})

	t.Run("remove links to an entity", func(t *testing.T) {
		c := qt.New(t)
		ctx := fileLinkTestContext(c, "group-1")
		reg := factory.MustCreateUserRegistry(ctx)

		a, err := reg.Create(ctx, newLinkTestFile("a", 1, commodityLink("gone", "images"), commodityLink("kept", "images")))
		c.Assert(err, qt.IsNil)
		b, err := reg.Create(ctx, newLinkTestFile("b", 1, commodityLink("gone", "documents")))
		c.Assert(err, qt.IsNil)

		ids, err := reg.RemoveLinksToEntity(ctx, models.FileLinkCommodity, "gone")
		c.Assert(err, qt.IsNil)
		c.Assert(ids, qt.HasLen, 2)

		gotA, err := reg.Get(ctx, a.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(linkKeys(gotA), qt.DeepEquals, []string{"commodity/kept/images"})
		gotB, err := reg.Get(ctx, b.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(gotB.Links, qt.HasLen, 0)
	})

	t.Run("delete if unlinked", func(t *testing.T) {
		c := qt.New(t)
		ctx := fileLinkTestContext(c, "group-1")
		reg := factory.MustCreateUserRegistry(ctx)

		shared, err := reg.Create(ctx, newLinkTestFile("shared", 1, commodityLink("x", "images"), commodityLink("y", "images")))
		c.Assert(err, qt.IsNil)
		own, err := reg.Create(ctx, newLinkTestFile("own", 1, commodityLink("x", "images")))
		c.Assert(err, qt.IsNil)
		standalone, err := reg.Create(ctx, newLinkTestFile("standalone", 1))
		c.Assert(err, qt.IsNil)

		deleted, err := reg.DeleteIfUnlinked(ctx, shared.ID, models.FileLinkCommodity, "x")
		c.Assert(err, qt.IsNil)
		c.Assert(deleted, qt.IsFalse)
		_, err = reg.Get(ctx, shared.ID)
		c.Assert(err, qt.IsNil)

		deleted, err = reg.DeleteIfUnlinked(ctx, own.ID, models.FileLinkCommodity, "x")
		c.Assert(err, qt.IsNil)
		c.Assert(deleted, qt.IsTrue)
		_, err = reg.Get(ctx, own.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		deleted, err = reg.DeleteIfUnlinked(ctx, standalone.ID, "", "")
		c.Assert(err, qt.IsNil)
		c.Assert(deleted, qt.IsTrue)

		_, err = reg.DeleteIfUnlinked(ctx, own.ID, "", "")
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("update keeps links and checksum", func(t *testing.T) {
		c := qt.New(t)
		ctx := fileLinkTestContext(c, "group-1")
		reg := factory.MustCreateUserRegistry(ctx)

		fe := newLinkTestFile("photo", 1, commodityLink("lamp", "images"))
		fe.SHA256 = "abc"
		file, err := reg.Create(ctx, fe)
		c.Assert(err, qt.IsNil)

		edit := *file
		edit.Title = "Renamed"
		edit.Links = nil
		f := *file.File
		f.SHA256 = ""
		edit.File = &f
		updated, err := reg.Update(ctx, edit)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Title, qt.Equals, "Renamed")
		c.Assert(updated.SHA256, qt.Equals, "abc")
		c.Assert(linkKeys(updated), qt.DeepEquals, []string{"commodity/lamp/images"})
	})

	t.Run("the thumbnail orientation pass lists unmarked JPEG files and an edit keeps the mark", func(t *testing.T) {
		c := qt.New(t)
		ctx := fileLinkTestContext(c, "group-orientation")
		reg := factory.MustCreateUserRegistry(ctx)
		svc := factory.CreateServiceRegistry()

		_, err := reg.Create(ctx, newLinkTestFile("manual", 1))
		c.Assert(err, qt.IsNil)
		jpegFile := func(name string) *models.FileEntity {
			f := newLinkTestFile(name, 1)
			f.Type, f.Category = models.FileTypeImage, models.FileCategoryImages
			f.Ext, f.MIMEType = ".jpg", "image/jpeg"
			file, err := reg.Create(ctx, f)
			c.Assert(err, qt.IsNil)
			return file
		}
		first, second := jpegFile("first"), jpegFile("second")
		if second.ID < first.ID {
			first, second = second, first
		}

		pending, err := svc.ListPendingThumbnailOrientation(ctx, "", 100)
		c.Assert(err, qt.IsNil)
		c.Assert(fileIDs(pending), qt.DeepEquals, []string{first.ID, second.ID})
		pending, err = svc.ListPendingThumbnailOrientation(ctx, first.ID, 100)
		c.Assert(err, qt.IsNil)
		c.Assert(fileIDs(pending), qt.DeepEquals, []string{second.ID})

		c.Assert(svc.SetThumbnailsOriented(ctx, first.ID), qt.IsNil)
		c.Assert(svc.SetThumbnailsOriented(ctx, "no-such-file"), qt.IsNil)
		pending, err = svc.ListPendingThumbnailOrientation(ctx, "", 100)
		c.Assert(err, qt.IsNil)
		c.Assert(fileIDs(pending), qt.DeepEquals, []string{second.ID})

		edit := *first
		edit.Title = "Renamed"
		updated, err := reg.Update(ctx, edit)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.ThumbnailsOriented, qt.IsTrue)
	})

	t.Run("checksums", func(t *testing.T) {
		c := qt.New(t)
		ctx := fileLinkTestContext(c, "group-checksums")
		reg := factory.MustCreateUserRegistry(ctx)
		svc := factory.CreateServiceRegistry()

		first, err := reg.Create(ctx, newLinkTestFile("first", 1))
		c.Assert(err, qt.IsNil)
		second, err := reg.Create(ctx, newLinkTestFile("second", 1))
		c.Assert(err, qt.IsNil)

		pending, err := svc.ListPendingChecksum(ctx, "", 100)
		c.Assert(err, qt.IsNil)
		ids := map[string]bool{}
		for _, p := range pending {
			ids[p.ID] = true
		}
		c.Assert(ids[first.ID] && ids[second.ID], qt.IsTrue)

		c.Assert(svc.SetChecksum(ctx, first.ID, "same"), qt.IsNil)
		c.Assert(svc.SetChecksum(ctx, second.ID, "same"), qt.IsNil)
		c.Assert(svc.SetChecksum(ctx, second.ID, "different"), qt.IsNil)

		matches, err := reg.ListByChecksum(ctx, "same")
		c.Assert(err, qt.IsNil)
		c.Assert(matches, qt.HasLen, 2)

		otherGroup := factory.MustCreateUserRegistry(fileLinkTestContext(c, "group-other"))
		matches, err = otherGroup.ListByChecksum(ctx, "same")
		c.Assert(err, qt.IsNil)
		c.Assert(matches, qt.HasLen, 0)
	})

	t.Run("storage counts a shared file once", func(t *testing.T) {
		c := qt.New(t)
		ctx := fileLinkTestContext(c, "group-storage")
		reg := factory.MustCreateUserRegistry(ctx)

		_, err := reg.Create(ctx, newLinkTestFile("receipt", 100,
			commodityLink("a", "documents"), commodityLink("b", "documents"), commodityLink("c", "documents")))
		c.Assert(err, qt.IsNil)
		_, err = reg.Create(ctx, newLinkTestFile("backup", 1000,
			models.FileLink{EntityType: models.FileLinkExport, EntityID: "export-1", Meta: "inb-2.0"}))
		c.Assert(err, qt.IsNil)

		breakdown, err := reg.SumSizeBreakdown(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(breakdown.Documents, qt.Equals, int64(100))
		c.Assert(breakdown.Exports, qt.Equals, int64(1000))
	})
}

func TestFileRegistry_Memory_MoveLegacyLinks(t *testing.T) {
	c := qt.New(t)
	ctx := fileLinkTestContext(c, "group-1")
	factory := memory.NewFileRegistryFactory()
	reg := factory.MustCreateUserRegistry(ctx)
	svc := factory.CreateServiceRegistry()

	// Only a row an older release wrote carries legacy values, so they are
	// stored through the base registry, below the file registry's Create.
	legacy := func(name, typ, id, meta string) *models.FileEntity {
		fe := newLinkTestFile(name, 1)
		fe.LegacyFileLink = models.LegacyFileLink{LegacyLinkedEntityType: typ, LegacyLinkedEntityID: id, LegacyLinkedEntityMeta: meta}
		stored, err := reg.(*memory.FileRegistry).Registry.Create(ctx, fe)
		c.Assert(err, qt.IsNil)
		return stored
	}
	a := legacy("a", "commodity", "tv", "images")
	b := legacy("b", "location", "home", "files")
	standalone, err := reg.Create(ctx, newLinkTestFile("standalone", 1))
	c.Assert(err, qt.IsNil)

	// An edit keeps the legacy values for the move.
	edit := *a
	edit.Title = "a edited"
	edit.LegacyFileLink = models.LegacyFileLink{}
	_, err = reg.Update(ctx, edit)
	c.Assert(err, qt.IsNil)

	moved, err := svc.MoveLegacyLinks(ctx, 1)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 1)
	moved, err = svc.MoveLegacyLinks(ctx, 10)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 1)
	moved, err = svc.MoveLegacyLinks(ctx, 10)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 0)

	gotA, err := reg.Get(ctx, a.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(linkKeys(gotA), qt.DeepEquals, []string{"commodity/tv/images"})
	c.Assert(gotA.LegacyFileLink, qt.Equals, models.LegacyFileLink{})
	gotB, err := reg.Get(ctx, b.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(linkKeys(gotB), qt.DeepEquals, []string{"location/home/files"})
	gotS, err := reg.Get(ctx, standalone.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(gotS.Links, qt.HasLen, 0)
}

// Create leaves the legacy link fields out, as the postgres INSERT leaves out
// their columns.
func TestFileRegistry_Memory_CreateDropsLegacyLinkFields(t *testing.T) {
	c := qt.New(t)
	ctx := fileLinkTestContext(c, "group-1")
	reg := memory.NewFileRegistryFactory().MustCreateUserRegistry(ctx)

	fe := newLinkTestFile("a", 1)
	fe.LegacyFileLink = models.LegacyFileLink{LegacyLinkedEntityType: "commodity", LegacyLinkedEntityID: "tv"}
	created, err := reg.Create(ctx, fe)
	c.Assert(err, qt.IsNil)
	c.Assert(created.LegacyFileLink, qt.Equals, models.LegacyFileLink{})

	got, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.LegacyFileLink, qt.Equals, models.LegacyFileLink{})
}

func fileIDs(files []*models.FileEntity) []string {
	ids := make([]string, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.ID)
	}
	return ids
}
