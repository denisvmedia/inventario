package postgres_test

import (
	"context"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"gocloud.dev/blob"

	_ "go.5x5.cz/inventario/internal/fileblob" // register file:// driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/services"
)

type fileLinkPGFixture struct {
	set        *registry.Set
	factorySet *registry.FactorySet
	dbx        *sqlx.DB
	ctx        context.Context
	user       *models.User
	groupID    string
}

func newFileLinkPGFixture(t *testing.T) fileLinkPGFixture {
	t.Helper()
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	factorySet := postgres.NewFactorySet(dbx)

	user := getTestUser(c, set)
	groups, err := factorySet.CreateServiceRegistrySet().LocationGroupRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(groups, qt.HasLen, 1)

	return fileLinkPGFixture{
		set:        set,
		factorySet: factorySet,
		dbx:        dbx,
		ctx:        tagCtxFor(user, groups[0].ID),
		user:       user,
		groupID:    groups[0].ID,
	}
}

func (fx fileLinkPGFixture) file(c *qt.C, name string, size int64, links ...models.FileLink) *models.FileEntity {
	c.Helper()
	file, err := fx.set.FileRegistry.Create(fx.ctx, models.FileEntity{
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
	})
	c.Assert(err, qt.IsNil)
	return file
}

func pgCommodityLink(id, meta string) models.FileLink {
	return models.FileLink{EntityType: models.FileLinkCommodity, EntityID: id, Meta: meta}
}

func pgLinkKeys(file *models.FileEntity) []string {
	keys := make([]string, 0, len(file.Links))
	for _, l := range file.Links {
		keys = append(keys, l.EntityType+"/"+l.EntityID+"/"+l.Meta)
	}
	return keys
}

func TestFileRegistry_Postgres_Links(t *testing.T) {
	t.Run("create, attach, list", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkPGFixture(t)
		reg := fx.set.FileRegistry

		file := fx.file(c, "receipt", 100, pgCommodityLink("tv", "documents"))
		c.Assert(pgLinkKeys(file), qt.DeepEquals, []string{"commodity/tv/documents"})
		c.Assert(file.Links[0].FileID, qt.Equals, file.ID)
		c.Assert(file.Links[0].GroupID, qt.Equals, fx.groupID)
		c.Assert(file.Links[0].TenantID, qt.Equals, fx.user.TenantID)

		link, created, err := reg.AddLink(fx.ctx, models.FileLink{FileID: file.ID, EntityType: models.FileLinkCommodity, EntityID: "soundbar", Meta: "documents"})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
		again, created, err := reg.AddLink(fx.ctx, models.FileLink{FileID: file.ID, EntityType: models.FileLinkCommodity, EntityID: "soundbar", Meta: "images"})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsFalse)
		c.Assert(again.ID, qt.Equals, link.ID)
		c.Assert(again.Meta, qt.Equals, "documents")

		got, err := reg.Get(fx.ctx, file.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(pgLinkKeys(got), qt.DeepEquals, []string{"commodity/tv/documents", "commodity/soundbar/documents"})

		for _, id := range []string{"tv", "soundbar"} {
			files, err := reg.ListByLinkedEntity(fx.ctx, models.FileLinkCommodity, id)
			c.Assert(err, qt.IsNil)
			c.Assert(files, qt.HasLen, 1)
			c.Assert(files[0].ID, qt.Equals, file.ID)
			c.Assert(files[0].Links, qt.HasLen, 2)
		}

		linkedType, linkedID := models.FileLinkCommodity, "soundbar"
		page, total, err := reg.ListPaginated(fx.ctx, 0, 10, nil, nil, &linkedType, &linkedID)
		c.Assert(err, qt.IsNil)
		c.Assert(total, qt.Equals, 1)
		c.Assert(page[0].ID, qt.Equals, file.ID)

		_, _, err = reg.AddLink(fx.ctx, models.FileLink{FileID: "missing", EntityType: models.FileLinkCommodity, EntityID: "tv", Meta: "documents"})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("remove link and links to an entity", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkPGFixture(t)
		reg := fx.set.FileRegistry

		a := fx.file(c, "a", 1, pgCommodityLink("gone", "images"), pgCommodityLink("kept", "images"))
		b := fx.file(c, "b", 1, pgCommodityLink("gone", "documents"))

		_, err := reg.RemoveLink(fx.ctx, b.ID, a.Links[0].ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		ids, err := reg.RemoveLinksToEntity(fx.ctx, models.FileLinkCommodity, "gone")
		c.Assert(err, qt.IsNil)
		c.Assert(ids, qt.HasLen, 2)

		gotA, err := reg.Get(fx.ctx, a.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(pgLinkKeys(gotA), qt.DeepEquals, []string{"commodity/kept/images"})

		removed, err := reg.RemoveLink(fx.ctx, a.ID, gotA.Links[0].ID)
		c.Assert(err, qt.IsNil)
		c.Assert(removed.EntityID, qt.Equals, "kept")
		gotA, err = reg.Get(fx.ctx, a.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(gotA.Links, qt.HasLen, 0)
	})

	t.Run("delete if unlinked", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkPGFixture(t)
		reg := fx.set.FileRegistry

		shared := fx.file(c, "shared", 1, pgCommodityLink("x", "images"), pgCommodityLink("y", "images"))
		own := fx.file(c, "own", 1, pgCommodityLink("x", "images"))
		standalone := fx.file(c, "standalone", 1)

		deleted, err := reg.DeleteIfUnlinked(fx.ctx, shared.ID, models.FileLinkCommodity, "x")
		c.Assert(err, qt.IsNil)
		c.Assert(deleted, qt.IsFalse)
		_, err = reg.Get(fx.ctx, shared.ID)
		c.Assert(err, qt.IsNil)

		deleted, err = reg.DeleteIfUnlinked(fx.ctx, own.ID, models.FileLinkCommodity, "x")
		c.Assert(err, qt.IsNil)
		c.Assert(deleted, qt.IsTrue)
		_, err = reg.Get(fx.ctx, own.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		deleted, err = reg.DeleteIfUnlinked(fx.ctx, standalone.ID, "", "")
		c.Assert(err, qt.IsNil)
		c.Assert(deleted, qt.IsTrue)

		_, err = reg.DeleteIfUnlinked(fx.ctx, own.ID, "", "")
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		// The link rows of a deleted file go with it.
		var n int
		c.Assert(fx.dbx.QueryRowxContext(fx.ctx, `SELECT count(*) FROM file_links WHERE file_id = $1`, own.ID).Scan(&n), qt.IsNil)
		c.Assert(n, qt.Equals, 0)
	})

	t.Run("update keeps links and checksum", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkPGFixture(t)
		reg := fx.set.FileRegistry

		file := fx.file(c, "photo", 1, pgCommodityLink("lamp", "images"))
		c.Assert(fx.factorySet.FileRegistryFactory.CreateServiceRegistry().SetChecksum(fx.ctx, file.ID, "abc"), qt.IsNil)

		edit := *file
		edit.Title = "Renamed"
		edit.Links = nil
		f := *file.File
		f.SHA256 = ""
		edit.File = &f
		updated, err := reg.Update(fx.ctx, edit)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Title, qt.Equals, "Renamed")
		c.Assert(updated.SHA256, qt.Equals, "abc")
		c.Assert(pgLinkKeys(updated), qt.DeepEquals, []string{"commodity/lamp/images"})
	})

	t.Run("the thumbnail orientation pass lists unmarked JPEG files and an edit keeps the mark", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkPGFixture(t)
		svc := fx.factorySet.FileRegistryFactory.CreateServiceRegistry()

		fx.file(c, "manual", 1)
		jpegFile := func(name string) *models.FileEntity {
			file, err := fx.set.FileRegistry.Create(fx.ctx, models.FileEntity{
				Title: name, Type: models.FileTypeImage, Category: models.FileCategoryImages,
				File: &models.File{Path: name, OriginalPath: "files/" + name, Ext: ".jpg", MIMEType: "image/jpeg"},
			})
			c.Assert(err, qt.IsNil)
			return file
		}
		first, second := jpegFile("first"), jpegFile("second")
		if second.ID < first.ID {
			first, second = second, first
		}

		pending, err := svc.ListPendingThumbnailOrientation(fx.ctx, "", 100)
		c.Assert(err, qt.IsNil)
		c.Assert(pgFileIDs(pending), qt.DeepEquals, []string{first.ID, second.ID})
		pending, err = svc.ListPendingThumbnailOrientation(fx.ctx, first.ID, 100)
		c.Assert(err, qt.IsNil)
		c.Assert(pgFileIDs(pending), qt.DeepEquals, []string{second.ID})

		c.Assert(svc.SetThumbnailsOriented(fx.ctx, first.ID), qt.IsNil)
		c.Assert(svc.SetThumbnailsOriented(fx.ctx, "no-such-file"), qt.IsNil)
		pending, err = svc.ListPendingThumbnailOrientation(fx.ctx, "", 100)
		c.Assert(err, qt.IsNil)
		c.Assert(pgFileIDs(pending), qt.DeepEquals, []string{second.ID})

		edit := *first
		edit.Title = "Renamed"
		updated, err := fx.set.FileRegistry.Update(fx.ctx, edit)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.ThumbnailsOriented, qt.IsTrue)
	})

	t.Run("checksums stay within the group", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkPGFixture(t)
		reg := fx.set.FileRegistry
		svc := fx.factorySet.FileRegistryFactory.CreateServiceRegistry()

		first := fx.file(c, "first", 1)
		second := fx.file(c, "second", 1)

		pending, err := svc.ListPendingChecksum(fx.ctx, "", 100)
		c.Assert(err, qt.IsNil)
		c.Assert(pending, qt.HasLen, 2)

		c.Assert(svc.SetChecksum(fx.ctx, first.ID, "same"), qt.IsNil)
		c.Assert(svc.SetChecksum(fx.ctx, second.ID, "same"), qt.IsNil)
		c.Assert(svc.SetChecksum(fx.ctx, second.ID, "different"), qt.IsNil)

		matches, err := reg.ListByChecksum(fx.ctx, "same")
		c.Assert(err, qt.IsNil)
		c.Assert(matches, qt.HasLen, 2)

		pending, err = svc.ListPendingChecksum(fx.ctx, "", 100)
		c.Assert(err, qt.IsNil)
		c.Assert(pending, qt.HasLen, 0)

		bgCtx := context.Background()
		tenantB := mustCreateTenant(c, bgCtx, fx.factorySet, "tenant-b-checksum")
		userB := mustCreateUser(c, bgCtx, fx.factorySet, tenantB, "user-b@checksum.example")
		groupB := mustCreateActiveGroup(c, bgCtx, fx.factorySet, tenantB, userB.ID)
		setB := postgres.NewRegistrySetWithUserAndGroupID(fx.dbx, userB.ID, tenantB, groupB)
		matches, err = setB.FileRegistry.ListByChecksum(tagCtxFor(userB, groupB), "same")
		c.Assert(err, qt.IsNil)
		c.Assert(matches, qt.HasLen, 0)
	})

	t.Run("storage counts a shared file once", func(t *testing.T) {
		c := qt.New(t)
		fx := newFileLinkPGFixture(t)

		fx.file(c, "receipt", 100, pgCommodityLink("a", "documents"), pgCommodityLink("b", "documents"), pgCommodityLink("c", "documents"))
		fx.file(c, "backup", 1000, models.FileLink{EntityType: models.FileLinkExport, EntityID: "export-1", Meta: "inb-2.0"})

		breakdown, err := fx.set.FileRegistry.SumSizeBreakdown(fx.ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(breakdown.Documents, qt.Equals, int64(100))
		c.Assert(breakdown.Exports, qt.Equals, int64(1000))
	})
}

// TestFileRegistry_Postgres_AttachRacesDelete runs an attach and a
// delete-if-unlinked of the same file at the same time, many times. Whatever
// the interleaving, the outcome is one of the two consistent states: the file
// is gone and the attach failed, or the file is kept with the new link.
func TestFileRegistry_Postgres_AttachRacesDelete(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	reg := fx.set.FileRegistry

	for range 20 {
		file := fx.file(c, "race", 1, pgCommodityLink("deleted-item", "documents"))

		var wg sync.WaitGroup
		var addErr, delErr error
		var deleted bool
		wg.Go(func() {
			_, _, addErr = reg.AddLink(fx.ctx, models.FileLink{FileID: file.ID, EntityType: models.FileLinkCommodity, EntityID: "kept-item", Meta: "documents"})
		})
		wg.Go(func() {
			deleted, delErr = reg.DeleteIfUnlinked(fx.ctx, file.ID, models.FileLinkCommodity, "deleted-item")
		})
		wg.Wait()
		c.Assert(delErr, qt.IsNil)

		got, getErr := reg.Get(fx.ctx, file.ID)
		if deleted {
			c.Assert(getErr, qt.ErrorIs, registry.ErrNotFound)
			c.Assert(addErr, qt.ErrorIs, registry.ErrNotFound, qt.Commentf("attach after delete"))
			continue
		}
		c.Assert(addErr, qt.IsNil)
		c.Assert(getErr, qt.IsNil)
		c.Assert(got.LinkTo(models.FileLinkCommodity, "kept-item"), qt.IsNotNil)
	}
}

func TestFileRegistry_Postgres_MoveLegacyLinks(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	reg := fx.set.FileRegistry
	svc := fx.factorySet.FileRegistryFactory.CreateServiceRegistry()

	setLegacy := func(fileID, typ, id, meta string) {
		_, err := fx.dbx.ExecContext(fx.ctx,
			`UPDATE files SET linked_entity_type = $2, linked_entity_id = $3, linked_entity_meta = $4 WHERE id = $1`,
			fileID, typ, id, meta)
		c.Assert(err, qt.IsNil)
	}
	legacyColumns := func(fileID string) (typ, id, meta string) {
		c.Assert(fx.dbx.QueryRowxContext(fx.ctx,
			`SELECT COALESCE(linked_entity_type, ''), COALESCE(linked_entity_id, ''), COALESCE(linked_entity_meta, '') FROM files WHERE id = $1`,
			fileID).Scan(&typ, &id, &meta), qt.IsNil)
		return typ, id, meta
	}

	a := fx.file(c, "a", 1)
	setLegacy(a.ID, "commodity", "tv", "images")
	b := fx.file(c, "b", 1)
	setLegacy(b.ID, "location", "home", "files")
	// Already linked to the same entity: the move adds no second link.
	dup := fx.file(c, "dup", 1, pgCommodityLink("tv", "documents"))
	setLegacy(dup.ID, "commodity", "tv", "images")
	// An export bundle keeps its export link.
	export := fx.file(c, "export", 1)
	setLegacy(export.ID, "export", "export-1", "inb-2.0")
	// A type with no id carries nothing to move; its columns are emptied.
	broken := fx.file(c, "broken", 1)
	setLegacy(broken.ID, "commodity", "", "images")
	standalone := fx.file(c, "standalone", 1)

	total := 0
	for {
		moved, err := svc.MoveLegacyLinks(fx.ctx, 2)
		c.Assert(err, qt.IsNil)
		c.Assert(moved <= 2, qt.IsTrue)
		if moved == 0 {
			break
		}
		total += moved
	}
	c.Assert(total, qt.Equals, 5)

	expect := map[string][]string{
		a.ID:          {"commodity/tv/images"},
		b.ID:          {"location/home/files"},
		dup.ID:        {"commodity/tv/documents"},
		export.ID:     {"export/export-1/inb-2.0"},
		broken.ID:     {},
		standalone.ID: {},
	}
	for id, want := range expect {
		got, err := reg.Get(fx.ctx, id)
		c.Assert(err, qt.IsNil)
		c.Assert(pgLinkKeys(got), qt.DeepEquals, want, qt.Commentf("file %s", got.Title))
		typ, eid, meta := legacyColumns(id)
		c.Assert(typ+eid+meta, qt.Equals, "", qt.Commentf("legacy columns of %s", got.Title))
	}

	// A file that was moved is not moved again, even when the run repeats.
	moved, err := svc.MoveLegacyLinks(fx.ctx, 100)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 0)
}

// TestFileLinks_Postgres_OneReceiptThreeItems is the acceptance scenario of
// #2806 against PostgreSQL: one receipt attached to three items; removing it
// from one item and deleting another item leaves the third item with the same
// file and the same bytes.
func TestFileLinks_Postgres_OneReceiptThreeItems(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"

	area := createTestArea(c, fx.set, createTestLocation(c, fx.set).ID)
	tv := createTestCommodity(c, fx.set, area.ID)
	soundbar := createTestCommodity(c, fx.set, area.ID)
	mount := createTestCommodity(c, fx.set, area.ID)

	b, err := blob.OpenBucket(fx.ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	c.Assert(b.WriteAll(fx.ctx, "files/receipt", []byte("receipt bytes"), nil), qt.IsNil)
	c.Assert(b.Close(), qt.IsNil)
	receipt := fx.file(c, "receipt", int64(len("receipt bytes")))

	links := services.NewFileLinkService(fx.factorySet)
	entities := services.NewEntityService(fx.factorySet, uploadLocation)

	for _, com := range []*models.Commodity{tv, soundbar, mount} {
		_, created, err := links.Attach(fx.ctx, receipt.ID, models.FileLinkCommodity, com.ID, "")
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
	}
	got, err := fx.set.FileRegistry.Get(fx.ctx, receipt.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Links, qt.HasLen, 3)

	_, err = links.Detach(fx.ctx, receipt.ID, got.LinkTo(models.FileLinkCommodity, soundbar.ID).ID)
	c.Assert(err, qt.IsNil)
	c.Assert(entities.DeleteCommodityRecursive(fx.ctx, tv.ID), qt.IsNil)

	mountFiles, err := fx.set.FileRegistry.ListByLinkedEntity(fx.ctx, models.FileLinkCommodity, mount.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(mountFiles, qt.HasLen, 1)
	c.Assert(mountFiles[0].ID, qt.Equals, receipt.ID)
	c.Assert(pgLinkKeys(mountFiles[0]), qt.DeepEquals, []string{"commodity/" + mount.ID + "/documents"})

	b, err = blob.OpenBucket(fx.ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	data, err := b.ReadAll(fx.ctx, "files/receipt")
	c.Assert(err, qt.IsNil)
	c.Assert(string(data), qt.Equals, "receipt bytes")

	files, err := fx.set.FileRegistry.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 1)
}

func pgFileIDs(files []*models.FileEntity) []string {
	ids := make([]string, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.ID)
	}
	return ids
}
