package registrytest

import (
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// RunBulkPatchContract checks CommodityRegistry.Patch and
// FileRegistry.PatchTags: they hand change the entity as stored, write the
// columns a bulk action edits and no other, write nothing when change
// refuses or changes nothing, refuse a tag the caller did not name, and do
// not reach an entity in Trash. newEnv is called once per subtest and must
// return an empty group.
func RunBulkPatchContract(t *testing.T, newEnv func(t *testing.T) TrashEnv) {
	t.Run("an item patch writes its columns alone", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := patchContractItem(c, env, "Bike")

		var seen models.Commodity
		before, after, err := env.Commodities.Patch(env.Ctx, item.ID, []string{"insured"}, func(next models.Commodity) (models.Commodity, bool, error) {
			seen = next
			next.Tags = append(next.Tags, "insured")
			next.Status = models.CommodityStatusLost
			next.StatusDate = models.ToPDate("2026-03-01")
			next.StatusNote = "Left at the station"
			next.AreaID = new(env.OtherAreaID)
			// Not a column a patch writes.
			next.Name = "Scooter"
			return next, true, nil
		})
		c.Assert(err, qt.IsNil)
		c.Check(seen.Comments, qt.Equals, "Keep the box")
		c.Check([]string(before.Tags), qt.DeepEquals, []string{"garage"})
		c.Check(before.Status, qt.Equals, models.CommodityStatusInUse)

		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		for _, got := range []*models.Commodity{after, stored} {
			c.Check(got.Name, qt.Equals, "Bike")
			c.Check(got.Comments, qt.Equals, "Keep the box")
			c.Check([]string(got.Tags), qt.DeepEquals, []string{"garage", "insured"})
			c.Check(got.Status, qt.Equals, models.CommodityStatusLost)
			c.Check(got.StatusDate, qt.DeepEquals, models.ToPDate("2026-03-01"))
			c.Check(got.StatusNote, qt.Equals, "Left at the station")
			c.Check(got.AreaID, qt.DeepEquals, new(env.OtherAreaID))
			c.Check(got.AssetNumber, qt.DeepEquals, item.AssetNumber)
			c.Check(got.OriginalPrice.Equal(decimal.NewFromInt(10)), qt.IsTrue)
		}
	})

	t.Run("an item patch that changes nothing writes nothing", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := patchContractItem(c, env, "Lamp")

		before, after, err := env.Commodities.Patch(env.Ctx, item.ID, []string{"insured"}, func(next models.Commodity) (models.Commodity, bool, error) {
			next.Tags = append(next.Tags, "insured")
			return next, false, nil
		})
		c.Assert(err, qt.IsNil)
		c.Check(after, qt.IsNil)
		c.Check(before.ID, qt.Equals, item.ID)
		assertItemTags(c, env, item.ID, "garage")
	})

	t.Run("a refused item patch writes nothing", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := patchContractItem(c, env, "Desk")

		refused := errors.New("not allowed")
		_, _, err := env.Commodities.Patch(env.Ctx, item.ID, []string{"insured"}, func(next models.Commodity) (models.Commodity, bool, error) {
			next.Tags = append(next.Tags, "insured")
			return next, true, refused
		})
		c.Assert(err, qt.Equals, refused)
		assertItemTags(c, env, item.ID, "garage")
	})

	t.Run("an item patch refuses a tag it did not name", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := patchContractItem(c, env, "Chair")

		_, _, err := env.Commodities.Patch(env.Ctx, item.ID, nil, func(next models.Commodity) (models.Commodity, bool, error) {
			next.Tags = append(next.Tags, "insured")
			return next, true, nil
		})
		c.Assert(err, qt.ErrorIs, registry.ErrInvalidInput)
		assertItemTags(c, env, item.ID, "garage")
	})

	t.Run("an item patch checks the type and the area", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := patchContractItem(c, env, "Shelf")

		_, _, err := env.Commodities.Patch(env.Ctx, item.ID, nil, func(next models.Commodity) (models.Commodity, bool, error) {
			next.Type = "no-such-type"
			return next, true, nil
		})
		c.Assert(err, qt.ErrorIs, registry.ErrUnknownItemType)
		_, _, err = env.Commodities.Patch(env.Ctx, item.ID, nil, func(next models.Commodity) (models.Commodity, bool, error) {
			next.AreaID = new("no-such-area")
			return next, true, nil
		})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(stored.Type, qt.Equals, models.CommodityTypeOther)
		c.Check(stored.AreaID, qt.DeepEquals, new(env.AreaID))
	})

	t.Run("an item in Trash is not found", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := patchContractItem(c, env, "Sofa")
		_, err := env.Commodities.Trash(env.Ctx, item.ID, "registrytest", time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
		c.Assert(err, qt.IsNil)

		unchanged := func(next models.Commodity) (models.Commodity, bool, error) { return next, false, nil }
		_, _, err = env.Commodities.Patch(env.Ctx, item.ID, nil, unchanged)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		_, _, err = env.Commodities.Patch(env.Ctx, "no-such-item", nil, unchanged)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("a file patch writes the tags alone", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		file := patchContractFile(c, env)

		before, after, err := env.Files.PatchTags(env.Ctx, file.ID, []string{"receipts"}, func(next models.FileEntity) (models.FileEntity, bool, error) {
			next.Tags = append(next.Tags, "receipts")
			// Not a column a patch writes.
			next.Title = "Something else"
			return next, true, nil
		})
		c.Assert(err, qt.IsNil)
		c.Check([]string(before.Tags), qt.DeepEquals, []string{"paperwork"})
		c.Check(before.Title, qt.Equals, "Drill manual")

		stored, err := env.Files.Get(env.Ctx, file.ID)
		c.Assert(err, qt.IsNil)
		for _, got := range []*models.FileEntity{after, stored} {
			c.Check(got.Title, qt.Equals, "Drill manual")
			c.Check([]string(got.Tags), qt.DeepEquals, []string{"paperwork", "receipts"})
			c.Check(got.Links, qt.HasLen, 1)
			c.Check(got.UpdatedAt.Before(before.UpdatedAt), qt.IsFalse)
		}
	})

	t.Run("a file patch that changes nothing, is refused or adds an unnamed tag writes nothing", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		file := patchContractFile(c, env)
		addReceipts := func(changed bool, err error) func(models.FileEntity) (models.FileEntity, bool, error) {
			return func(next models.FileEntity) (models.FileEntity, bool, error) {
				next.Tags = append(next.Tags, "receipts")
				return next, changed, err
			}
		}

		before, after, err := env.Files.PatchTags(env.Ctx, file.ID, []string{"receipts"}, addReceipts(false, nil))
		c.Assert(err, qt.IsNil)
		c.Check(after, qt.IsNil)
		c.Check(before.ID, qt.Equals, file.ID)

		refused := errors.New("not allowed")
		_, _, err = env.Files.PatchTags(env.Ctx, file.ID, []string{"receipts"}, addReceipts(true, refused))
		c.Assert(err, qt.Equals, refused)

		_, _, err = env.Files.PatchTags(env.Ctx, file.ID, nil, addReceipts(true, nil))
		c.Assert(err, qt.ErrorIs, registry.ErrInvalidInput)

		stored, err := env.Files.Get(env.Ctx, file.ID)
		c.Assert(err, qt.IsNil)
		c.Check([]string(stored.Tags), qt.DeepEquals, []string{"paperwork"})
	})

	t.Run("a file in Trash is not found", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		file := patchContractFile(c, env)
		c.Assert(env.Files.Trash(env.Ctx, file.ID, "registrytest", "", time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)), qt.IsNil)

		unchanged := func(next models.FileEntity) (models.FileEntity, bool, error) { return next, false, nil }
		_, _, err := env.Files.PatchTags(env.Ctx, file.ID, nil, unchanged)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		_, _, err = env.Files.PatchTags(env.Ctx, "no-such-file", nil, unchanged)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})
}

// patchContractItem creates an item tagged garage, then stores an edit of
// it, so a patch that started from the first read would undo the edit.
func patchContractItem(c *qt.C, env TrashEnv, name string) *models.Commodity {
	c.Helper()
	item := missingDataItem(c, env, name, new(decimal.NewFromInt(10)), nil, "2024-01-01")
	edited := *item
	edited.Comments = "Keep the box"
	edited.Tags = models.ValuerSlice[string]{"garage"}
	stored, err := env.Commodities.Update(env.Ctx, edited)
	c.Assert(err, qt.IsNil)
	return stored
}

// patchContractFile creates a file linked to an item and tagged paperwork,
// then stores an edit of its title.
func patchContractFile(c *qt.C, env TrashEnv) *models.FileEntity {
	c.Helper()
	item := trashContractItem(c, env, "Drill")
	file := trashContractFile(c, env, "drill-manual.pdf", item.ID)
	edited := *file
	edited.Title = "Drill manual"
	edited.Tags = models.StringSlice{"paperwork"}
	stored, err := env.Files.Update(env.Ctx, edited)
	c.Assert(err, qt.IsNil)
	return stored
}

func assertItemTags(c *qt.C, env TrashEnv, id string, want ...string) {
	c.Helper()
	stored, err := env.Commodities.Get(env.Ctx, id)
	c.Assert(err, qt.IsNil)
	c.Check([]string(stored.Tags), qt.DeepEquals, want)
}
