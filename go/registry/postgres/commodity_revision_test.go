package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// A bulk change runs with the group's row-level security: an item of
// another group is not found, and nothing is written to it.
func TestCommodityPatch_Postgres_AnotherGroupsItemIsNotFound(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	itemB := seedTagCommodity(c, fx.groupBSet, fx.ctxB, fx.areaBID, "Drill", "garage")

	_, _, err := fx.groupASet.CommodityRegistry.Patch(fx.ctxA, itemB, []string{"insured"}, func(next models.Commodity) (models.Commodity, bool, error) {
		next.Tags = append(next.Tags, "insured")
		return next, true, nil
	})
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	got, err := fx.groupBSet.CommodityRegistry.Get(fx.ctxB, itemB)
	c.Assert(err, qt.IsNil)
	c.Check([]string(got.Tags), qt.DeepEquals, []string{"garage"})
	c.Check(got.Revision, qt.Equals, int64(1))
}

// A bulk move names the target area by id; an area of another group is not
// found, and the item stays where it is.
func TestCommodityPatch_Postgres_MoveIntoAnotherGroupsAreaIsNotFound(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	itemA := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "Drill")

	_, _, err := fx.groupASet.CommodityRegistry.Patch(fx.ctxA, itemA, nil, func(next models.Commodity) (models.Commodity, bool, error) {
		next.AreaID = new(fx.areaBID)
		return next, true, nil
	})
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	got, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, itemA)
	c.Assert(err, qt.IsNil)
	c.Check(got.AreaID, qt.DeepEquals, new(fx.areaAID))
	c.Check(got.Revision, qt.Equals, int64(1))
}

// A tag rename and a tag delete rewrite the tags of the items that carry the
// tag, so an edit form opened before either is refused: it would put the old
// tag back.
func TestTagRewrite_Postgres_MovesTheItemRevisionOn(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	items := fx.groupASet.CommodityRegistry
	itemID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "Drill", "garage", "spare")
	form, err := items.Get(fx.ctxA, itemID)
	c.Assert(err, qt.IsNil)

	garage, err := fx.groupASet.TagRegistry.GetBySlug(fx.ctxA, models.TagKindCommodity, "garage")
	c.Assert(err, qt.IsNil)
	_, _, err = fx.groupASet.TagRegistry.RenameAtomic(fx.ctxA, garage.ID, "Workshop", "workshop", "", nil)
	c.Assert(err, qt.IsNil)
	spare, err := fx.groupASet.TagRegistry.GetBySlug(fx.ctxA, models.TagKindCommodity, "spare")
	c.Assert(err, qt.IsNil)
	_, err = fx.groupASet.TagRegistry.DeleteAtomic(fx.ctxA, spare.ID, true)
	c.Assert(err, qt.IsNil)

	got, err := items.Get(fx.ctxA, itemID)
	c.Assert(err, qt.IsNil)
	c.Check([]string(got.Tags), qt.DeepEquals, []string{"workshop"})
	c.Check(got.Revision, qt.Equals, form.Revision+2)

	edit := *form
	edit.Name = "Cordless drill"
	_, err = items.Update(registry.WithExpectedRevision(fx.ctxA, form.Revision), edit)
	c.Assert(err, qt.ErrorIs, registry.ErrRevisionConflict)
}
