package registrytest

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

// RunTagRewriteContract checks that a tag rename or delete through a
// backend's TagRegistry reaches the items and files in Trash, so a restored
// row carries no slug that no longer names a tag. newEnv is called once per
// subtest and must return an empty group.
func RunTagRewriteContract(t *testing.T, newEnv func(t *testing.T) TrashEnv) {
	t.Run("a rename reaches items and files in Trash", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		itemTag, fileTag := tagRewriteTags(c, env)
		item, file := trashedTaggedRows(c, env)

		_, _, err := env.Tags.RenameAtomic(env.Ctx, itemTag.ID, "", "workshop", "", nil)
		c.Assert(err, qt.IsNil)
		_, _, err = env.Tags.RenameAtomic(env.Ctx, fileTag.ID, "", "paperwork", "", nil)
		c.Assert(err, qt.IsNil)

		trashedItem, err := env.Commodities.GetTrashed(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(trashedItem.Tags, qt.DeepEquals, models.ValuerSlice[string]{"workshop"})
		trashedFile, err := env.Files.GetTrashed(env.Ctx, file.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(trashedFile.Tags, qt.DeepEquals, models.StringSlice{"paperwork"})
	})

	t.Run("a delete reaches items and files in Trash", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		itemTag, fileTag := tagRewriteTags(c, env)
		item, file := trashedTaggedRows(c, env)

		// No live row uses either tag, so both delete without force.
		_, err := env.Tags.DeleteAtomic(env.Ctx, itemTag.ID, false)
		c.Assert(err, qt.IsNil)
		_, err = env.Tags.DeleteAtomic(env.Ctx, fileTag.ID, false)
		c.Assert(err, qt.IsNil)

		trashedItem, err := env.Commodities.GetTrashed(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(trashedItem.Tags, qt.HasLen, 0)
		trashedFile, err := env.Files.GetTrashed(env.Ctx, file.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(trashedFile.Tags, qt.HasLen, 0)
	})
}

// tagRewriteTags creates the item tag "tools" and the file tag "receipts".
func tagRewriteTags(c *qt.C, env TrashEnv) (itemTag, fileTag *models.Tag) {
	c.Helper()
	itemTag, err := env.Tags.Create(env.Ctx, models.Tag{Kind: models.TagKindCommodity, Slug: "tools", Label: "Tools"})
	c.Assert(err, qt.IsNil)
	fileTag, err = env.Tags.Create(env.Ctx, models.Tag{Kind: models.TagKindFile, Slug: "receipts", Label: "Receipts"})
	c.Assert(err, qt.IsNil)
	return itemTag, fileTag
}

// trashedTaggedRows creates an item tagged "tools" and a file of its own
// tagged "receipts", and moves both to Trash.
func trashedTaggedRows(c *qt.C, env TrashEnv) (*models.Commodity, *models.FileEntity) {
	c.Helper()
	item, err := env.Commodities.Create(env.Ctx, models.Commodity{
		Name:      "Drill",
		ShortName: "Drill",
		Type:      models.CommodityTypeOther,
		AreaID:    new(env.AreaID),
		Count:     1,
		Status:    models.CommodityStatusInUse,
		Tags:      models.ValuerSlice[string]{"tools"},
	})
	c.Assert(err, qt.IsNil)
	file, err := env.Files.Create(env.Ctx, models.FileEntity{
		Title:    "drill-receipt.pdf",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Tags:     models.StringSlice{"receipts"},
		File: &models.File{
			Path:         "drill-receipt.pdf",
			OriginalPath: "files/drill-receipt.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	})
	c.Assert(err, qt.IsNil)

	now := time.Now()
	_, err = env.Commodities.Trash(env.Ctx, item.ID, "registrytest", now)
	c.Assert(err, qt.IsNil)
	c.Assert(env.Files.Trash(env.Ctx, file.ID, "registrytest", "", now), qt.IsNil)
	return item, file
}
