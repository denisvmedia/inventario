package registrytest

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

// RunCoverPhotoContract checks the two batched reads the item list uses to
// resolve covers: FileRegistry.GetMany for the covers items name, and
// FileRegistry.FirstPhotos for the oldest photo of the rest. Both must skip
// files in Trash, and FirstPhotos must skip items in Trash. newEnv is called
// once per subtest and must return an empty group.
func RunCoverPhotoContract(t *testing.T, newEnv func(t *testing.T) TrashEnv) {
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	at := func(minutes int) time.Time { return base.Add(time.Duration(minutes) * time.Minute) }

	t.Run("first photos", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		ctx := env.Ctx

		lamp := trashContractItem(c, env, "Lamp")
		chair := trashContractItem(c, env, "Chair")
		trashed := trashContractItem(c, env, "Desk")
		bare := trashContractItem(c, env, "Shelf")

		// None of these can be the lamp's cover, although each is older
		// than its photos.
		_ = coverContractFile(c, env, "lamp-manual.pdf", models.FileTypeDocument, models.FileCategoryImages, models.FileLinkMetaImages, at(0), lamp.ID)
		_ = coverContractFile(c, env, "lamp-receipt.jpg", models.FileTypeImage, models.FileCategoryDocuments, models.FileLinkMetaImages, at(1), lamp.ID)
		_ = coverContractFile(c, env, "lamp-box.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaDocuments, at(2), lamp.ID)
		gone := coverContractFile(c, env, "lamp-old.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(3), lamp.ID)
		c.Assert(env.Files.Trash(ctx, gone.ID, "registrytest", "", at(30)), qt.IsNil)

		newer := coverContractFile(c, env, "lamp-side.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(20), lamp.ID)
		// One photo shared by two items is the oldest photo of both.
		shared := coverContractFile(c, env, "lamp-and-chair.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(10), lamp.ID, chair.ID)
		_ = coverContractFile(c, env, "chair-back.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(15), chair.ID)

		// An item in Trash has no cover even while its photo is live.
		_ = coverContractFile(c, env, "desk.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(5), trashed.ID)
		_, err := env.Commodities.Trash(ctx, trashed.ID, "registrytest", at(40))
		c.Assert(err, qt.IsNil)

		first, err := env.Files.FirstPhotos(ctx, []string{lamp.ID, chair.ID, trashed.ID, bare.ID, "no-such-item"})
		c.Assert(err, qt.IsNil)
		c.Assert(first, qt.HasLen, 2)
		c.Assert(first[lamp.ID].ID, qt.Equals, shared.ID)
		c.Assert(first[chair.ID].ID, qt.Equals, shared.ID)
		c.Assert(first[lamp.ID].Links, qt.HasLen, 2, qt.Commentf("links are loaded"))

		// Without the shared photo the next oldest takes its place.
		_, err = env.Files.RemoveLink(ctx, shared.ID, linkIDTo(c, env, shared.ID, lamp.ID))
		c.Assert(err, qt.IsNil)
		first, err = env.Files.FirstPhotos(ctx, []string{lamp.ID})
		c.Assert(err, qt.IsNil)
		c.Assert(first[lamp.ID].ID, qt.Equals, newer.ID)

		empty, err := env.Files.FirstPhotos(ctx, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(empty, qt.HasLen, 0)
	})

	t.Run("first photos break a tie by id", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		item := trashContractItem(c, env, "Vase")
		a := coverContractFile(c, env, "vase-a.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(0), item.ID)
		b := coverContractFile(c, env, "vase-b.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(0), item.ID)
		want := min(a.ID, b.ID)

		first, err := env.Files.FirstPhotos(env.Ctx, []string{item.ID})
		c.Assert(err, qt.IsNil)
		c.Assert(first[item.ID].ID, qt.Equals, want)
	})

	t.Run("get many", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		ctx := env.Ctx

		item := trashContractItem(c, env, "Clock")
		one := coverContractFile(c, env, "clock-front.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(0), item.ID)
		two := coverContractFile(c, env, "clock-back.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(1), item.ID)
		gone := coverContractFile(c, env, "clock-old.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, at(2), item.ID)
		c.Assert(env.Files.Trash(ctx, gone.ID, "registrytest", "", at(30)), qt.IsNil)

		files, err := env.Files.GetMany(ctx, []string{two.ID, one.ID, two.ID, gone.ID, "no-such-file"})
		c.Assert(err, qt.IsNil)
		got := fileIDs(files)
		c.Assert(got, qt.HasLen, 2)
		c.Assert(got, qt.Contains, one.ID)
		c.Assert(got, qt.Contains, two.ID)
		for _, f := range files {
			c.Assert(f.Links, qt.HasLen, 1, qt.Commentf("links are loaded"))
		}

		none, err := env.Files.GetMany(ctx, nil)
		c.Assert(err, qt.IsNil)
		c.Assert(none, qt.HasLen, 0)
	})
}

func coverContractFile(c *qt.C, env TrashEnv, name string, fileType models.FileType, category models.FileCategory, meta string, createdAt time.Time, itemIDs ...string) *models.FileEntity {
	c.Helper()
	links := make([]models.FileLink, 0, len(itemIDs))
	for _, id := range itemIDs {
		links = append(links, models.FileLink{EntityType: models.FileLinkCommodity, EntityID: id, Meta: meta})
	}
	file, err := env.Files.Create(env.Ctx, models.FileEntity{
		Title:     name,
		Type:      fileType,
		Category:  category,
		Links:     links,
		CreatedAt: createdAt,
		File: &models.File{
			Path:         name,
			OriginalPath: "files/" + name,
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	})
	c.Assert(err, qt.IsNil)
	return file
}

func linkIDTo(c *qt.C, env TrashEnv, fileID, itemID string) string {
	c.Helper()
	file, err := env.Files.Get(env.Ctx, fileID)
	c.Assert(err, qt.IsNil)
	link := file.LinkTo(models.FileLinkCommodity, itemID)
	c.Assert(link, qt.IsNotNil)
	return link.ID
}
