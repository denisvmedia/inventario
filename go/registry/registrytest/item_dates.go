package registrytest

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// RunItemDatesContract checks the dates a backend's CommodityRegistry
// stores. Create dates a new item today, in UTC, unless the caller gives the
// dates. Update keeps the day the item was added when the caller gives none
// and dates the change, whatever date the caller sends; under
// registry.WithKeptModifiedDate it keeps the date the caller gives. Patch and
// SetDraft date the change. newEnv is called once per subtest and must
// return an empty group.
func RunItemDatesContract(t *testing.T, newEnv func(t *testing.T) TrashEnv) {
	t.Run("create dates a new item today", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		days := dayWindow()
		item := datedItem(c, env, "Bike", "", "")

		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		for _, got := range []*models.Commodity{item, stored} {
			assertDatedToday(c, got.RegisteredDate, days())
			assertDatedToday(c, got.LastModifiedDate, days())
		}
	})

	t.Run("create keeps the dates the caller gives", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := datedItem(c, env, "Bike", "2021-03-04", "2022-05-06")

		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		for _, got := range []*models.Commodity{item, stored} {
			c.Check(got.RegisteredDate, qt.DeepEquals, models.ToPDate("2021-03-04"))
			c.Check(got.LastModifiedDate, qt.DeepEquals, models.ToPDate("2022-05-06"))
		}
	})

	t.Run("an edit keeps the day added and dates the change", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := datedItem(c, env, "Bike", "2021-03-04", "2021-03-04")

		// An edit form sends neither date back; a date the caller sends as
		// the last change is not the day of this change.
		days := dayWindow()
		edit := *item
		edit.Comments = "Keep the box"
		edit.RegisteredDate = nil
		edit.LastModifiedDate = models.ToPDate("2000-01-01")
		updated, err := env.Commodities.Update(env.Ctx, edit)
		c.Assert(err, qt.IsNil)

		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		for _, got := range []*models.Commodity{updated, stored} {
			c.Check(got.RegisteredDate, qt.DeepEquals, models.ToPDate("2021-03-04"))
			assertDatedToday(c, got.LastModifiedDate, days())
		}

		// A day added the caller gives is stored.
		edit = *stored
		edit.RegisteredDate = models.ToPDate("2020-01-02")
		_, err = env.Commodities.Update(env.Ctx, edit)
		c.Assert(err, qt.IsNil)
		stored, err = env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(stored.RegisteredDate, qt.DeepEquals, models.ToPDate("2020-01-02"))
	})

	t.Run("a write that keeps the date stores the caller's", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := datedItem(c, env, "Bike", "2021-03-04", "2021-03-04")
		ctx := registry.WithKeptModifiedDate(env.Ctx)

		edit := *item
		edit.LastModifiedDate = models.ToPDate("2022-05-06")
		_, err := env.Commodities.Update(ctx, edit)
		c.Assert(err, qt.IsNil)
		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(stored.LastModifiedDate, qt.DeepEquals, models.ToPDate("2022-05-06"))

		// With no date to keep, the change is dated today.
		days := dayWindow()
		edit = *stored
		edit.LastModifiedDate = nil
		_, err = env.Commodities.Update(ctx, edit)
		c.Assert(err, qt.IsNil)
		stored, err = env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		assertDatedToday(c, stored.LastModifiedDate, days())
		c.Check(stored.RegisteredDate, qt.DeepEquals, models.ToPDate("2021-03-04"))
	})

	t.Run("a bulk edit dates the change", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := datedItem(c, env, "Bike", "2021-03-04", "2021-03-04")

		days := dayWindow()
		_, after, err := env.Commodities.Patch(env.Ctx, item.ID, nil, func(next models.Commodity) (models.Commodity, bool, error) {
			next.AreaID = new(env.OtherAreaID)
			return next, true, nil
		})
		c.Assert(err, qt.IsNil)
		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		for _, got := range []*models.Commodity{after, stored} {
			c.Check(got.RegisteredDate, qt.DeepEquals, models.ToPDate("2021-03-04"))
			assertDatedToday(c, got.LastModifiedDate, days())
		}
	})

	t.Run("the draft mark dates the change", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		item := datedItem(c, env, "Bike", "2021-03-04", "2021-03-04")

		days := dayWindow()
		_, after, err := env.Commodities.SetDraft(env.Ctx, item.ID, true, nil)
		c.Assert(err, qt.IsNil)
		stored, err := env.Commodities.Get(env.Ctx, item.ID)
		c.Assert(err, qt.IsNil)
		for _, got := range []*models.Commodity{after, stored} {
			c.Check(got.RegisteredDate, qt.DeepEquals, models.ToPDate("2021-03-04"))
			assertDatedToday(c, got.LastModifiedDate, days())
		}
	})
}

// datedItem creates an item in env's area with the registered and
// last-modified dates given, "" for none.
func datedItem(c *qt.C, env TrashEnv, name, registered, modified string) *models.Commodity {
	c.Helper()
	created, err := env.Commodities.Create(env.Ctx, models.Commodity{
		Name:                  name,
		ShortName:             name,
		Type:                  models.CommodityTypeOther,
		AreaID:                new(env.AreaID),
		Count:                 1,
		Status:                models.CommodityStatusInUse,
		OriginalPriceCurrency: "USD",
		RegisteredDate:        models.ToPDate(models.Date(registered)),
		LastModifiedDate:      models.ToPDate(models.Date(modified)),
	})
	c.Assert(err, qt.IsNil)
	return created
}

// dayWindow returns a function that lists the UTC days from the call of
// dayWindow to its own call, so a check made across midnight UTC passes.
func dayWindow() func() []models.Date {
	first := models.NewDate(time.Now())
	return func() []models.Date {
		return []models.Date{first, models.NewDate(time.Now())}
	}
}

// assertDatedToday checks that d is one of days.
func assertDatedToday(c *qt.C, d models.PDate, days []models.Date) {
	c.Helper()
	c.Assert(d, qt.IsNotNil)
	c.Assert(days, qt.Contains, *d)
}
