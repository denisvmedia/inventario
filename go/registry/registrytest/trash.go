package registrytest

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// TrashEnv is what RunTrashContract needs from a backend: registries scoped
// to one empty group, a service file registry that reads every group, the
// context that carries the group scope, and two areas items can be placed
// in.
type TrashEnv struct {
	Ctx          context.Context
	Commodities  registry.CommodityRegistry
	Files        registry.FileRegistry
	ServiceFiles registry.FileRegistry
	AreaID       string
	OtherAreaID  string
	// LocationID is the location of AreaID and OtherAreaID;
	// ElsewhereAreaID is an area of a second location, ElsewhereLocationID.
	LocationID          string
	ElsewhereLocationID string
	ElsewhereAreaID     string
}

// RunTrashContract checks how a backend's FileRegistry splits trashed files
// between the two reads the Trash sweeps of services.TrashService use:
// ListTrashedBefore returns the files deleted on their own, and
// ListStrandedTrashed the files that went to Trash with an item that is no
// longer in Trash. newEnv is called once per subtest and must return an empty
// group.
func RunTrashContract(t *testing.T, newEnv func(t *testing.T) TrashEnv) {
	t.Run("stranded files", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		ctx := env.Ctx
		base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

		// An item that is back while its file is still in Trash: an
		// interrupted restore, or a delete that raced the restore.
		back := trashContractItem(c, env, "Drill")
		backFile := trashContractFile(c, env, "drill-manual.pdf", back.ID)
		// An item that was purged before its file was settled.
		purged := trashContractItem(c, env, "Saw")
		purgedFile := trashContractFile(c, env, "saw-receipt.pdf", purged.ID)
		// An item still in Trash holds its file: nothing to settle.
		held := trashContractItem(c, env, "Sander")
		heldFile := trashContractFile(c, env, "sander-manual.pdf", held.ID)
		// A file deleted on its own is a Trash entry of its own.
		loose := trashContractFile(c, env, "loose.pdf")

		for i, it := range []struct{ item, file string }{
			{back.ID, backFile.ID},
			{purged.ID, purgedFile.ID},
			{held.ID, heldFile.ID},
		} {
			at := base.Add(time.Duration(i) * time.Hour)
			_, err := env.Commodities.Trash(ctx, it.item, "registrytest", at)
			c.Assert(err, qt.IsNil)
			c.Assert(env.Files.Trash(ctx, it.file, "registrytest", it.item, at), qt.IsNil)
		}
		c.Assert(env.Files.Trash(ctx, loose.ID, "registrytest", "", base.Add(3*time.Hour)), qt.IsNil)

		stranded, err := env.Files.ListStrandedTrashed(ctx, 10)
		c.Assert(err, qt.IsNil)
		c.Assert(stranded, qt.HasLen, 0, qt.Commentf("every item is still in Trash"))

		_, err = env.Commodities.Restore(ctx, back.ID)
		c.Assert(err, qt.IsNil)
		deleted, err := env.Commodities.PurgeTrashed(ctx, purged.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(deleted, qt.IsTrue)

		stranded, err = env.Files.ListStrandedTrashed(ctx, 10)
		c.Assert(err, qt.IsNil)
		c.Assert(fileIDs(stranded), qt.DeepEquals, []string{backFile.ID, purgedFile.ID}, qt.Commentf("oldest first"))
		c.Assert(stranded[0].Links, qt.HasLen, 1, qt.Commentf("links are loaded"))

		first, err := env.Files.ListStrandedTrashed(ctx, 1)
		c.Assert(err, qt.IsNil)
		c.Assert(fileIDs(first), qt.DeepEquals, []string{backFile.ID})

		serviceStranded, err := env.ServiceFiles.ListStrandedTrashed(ctx, 1000)
		c.Assert(err, qt.IsNil)
		c.Assert(fileIDs(serviceStranded), qt.Contains, backFile.ID)
		c.Assert(fileIDs(serviceStranded), qt.Contains, purgedFile.ID)
		c.Assert(fileIDs(serviceStranded), qt.Not(qt.Contains), heldFile.ID)
		c.Assert(fileIDs(serviceStranded), qt.Not(qt.Contains), loose.ID)

		// ListTrashedBefore takes only the files deleted on their own, and
		// only those deleted before the cutoff.
		expired, err := env.ServiceFiles.ListTrashedBefore(ctx, base.Add(24*time.Hour), 1000)
		c.Assert(err, qt.IsNil)
		c.Assert(fileIDs(expired), qt.Contains, loose.ID)
		for _, id := range []string{backFile.ID, purgedFile.ID, heldFile.ID} {
			c.Assert(fileIDs(expired), qt.Not(qt.Contains), id)
		}
		notYet, err := env.ServiceFiles.ListTrashedBefore(ctx, base.Add(2*time.Hour), 1000)
		c.Assert(err, qt.IsNil)
		c.Assert(fileIDs(notYet), qt.Not(qt.Contains), loose.ID)
	})
}

func trashContractItem(c *qt.C, env TrashEnv, name string) *models.Commodity {
	c.Helper()
	item, err := env.Commodities.Create(env.Ctx, models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeOther,
		AreaID:    new(env.AreaID),
		Count:     1,
		Status:    models.CommodityStatusInUse,
	})
	c.Assert(err, qt.IsNil)
	return item
}

func trashContractFile(c *qt.C, env TrashEnv, name string, itemIDs ...string) *models.FileEntity {
	c.Helper()
	links := make([]models.FileLink, 0, len(itemIDs))
	for _, id := range itemIDs {
		links = append(links, models.FileLink{EntityType: models.FileLinkCommodity, EntityID: id, Meta: models.FileLinkMetaDocuments})
	}
	file, err := env.Files.Create(env.Ctx, models.FileEntity{
		Title:    name,
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Links:    links,
		File: &models.File{
			Path:         name,
			OriginalPath: "files/" + name,
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	})
	c.Assert(err, qt.IsNil)
	return file
}

func fileIDs(files []*models.FileEntity) []string {
	ids := make([]string, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.ID)
	}
	return ids
}
