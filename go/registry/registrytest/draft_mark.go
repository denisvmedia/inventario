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

// RunDraftMarkContract checks CommodityRegistry.SetDraft: it changes the
// draft mark and no other column, hands check the item with the new mark,
// returns the item as it found it and as it stored it, writes nothing when
// check refuses, and does not reach an item in Trash.
// newEnv is called once per subtest and must return an empty group.
func RunDraftMarkContract(t *testing.T, newEnv func(t *testing.T) TrashEnv) {
	t.Run("changes the mark alone", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := missingDataItem(c, env, "Bike", new(decimal.NewFromInt(10)), nil, "2024-01-01")

		// An edit stored after a client read the item stays.
		edited := *item
		edited.Comments = "Keep the box"
		_, err := env.Commodities.Update(env.Ctx, edited)
		c.Assert(err, qt.IsNil)

		var seen models.Commodity
		before, got, err := env.Commodities.SetDraft(env.Ctx, item.ID, true, func(next models.Commodity) error {
			seen = next
			return nil
		})
		c.Assert(err, qt.IsNil)
		c.Check(seen.Draft, qt.IsTrue)
		c.Check(seen.Comments, qt.Equals, "Keep the box")
		c.Check(got.Draft, qt.IsTrue)
		c.Check(got.Comments, qt.Equals, "Keep the box")
		// The item as the write found it: the edit stored after the item
		// was first read is in it, the new mark is not.
		c.Check(before.Draft, qt.IsFalse)
		c.Check(before.Comments, qt.Equals, "Keep the box")

		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(stored.Draft, qt.IsTrue)
		c.Check(stored.Comments, qt.Equals, "Keep the box")
		c.Check(stored.Name, qt.Equals, "Bike")
		c.Check(stored.OriginalPrice.Equal(decimal.NewFromInt(10)), qt.IsTrue)
		c.Check(stored.AssetNumber, qt.DeepEquals, item.AssetNumber)

		_, _, err = env.Commodities.SetDraft(env.Ctx, item.ID, false, nil)
		c.Assert(err, qt.IsNil)
		stored, err = env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(stored.Draft, qt.IsFalse)
	})

	t.Run("a refused check writes nothing", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := missingDataItem(c, env, "Lamp", nil, nil, "")
		_, _, err := env.Commodities.SetDraft(env.Ctx, item.ID, true, nil)
		c.Assert(err, qt.IsNil)

		refused := errors.New("not complete")
		_, _, err = env.Commodities.SetDraft(env.Ctx, item.ID, false, func(models.Commodity) error { return refused })
		c.Assert(err, qt.ErrorIs, refused)

		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(stored.Draft, qt.IsTrue)
	})

	t.Run("an item in Trash is not found", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := missingDataItem(c, env, "Desk", nil, nil, "")
		_, err := env.Commodities.Trash(env.Ctx, item.ID, "registrytest", time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
		c.Assert(err, qt.IsNil)

		_, _, err = env.Commodities.SetDraft(env.Ctx, item.ID, true, nil)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		_, _, err = env.Commodities.SetDraft(env.Ctx, "no-such-item", true, nil)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})
}
