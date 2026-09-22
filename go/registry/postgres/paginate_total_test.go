package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// #1032: the page and its total now arrive in one query, through
// `COUNT(*) OVER()`. The window rides along on a row, so a page with no rows
// carries no total — an offset past the end has to keep reporting the real
// count rather than zero, or a client on page 99 of 3 is told the collection
// is empty.
func TestListPaginated_ReportsTheTotalForAPageBeyondTheEnd(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)
	ctx := appctx.WithUser(context.Background(), user)

	const seeded = 3
	for i := range seeded {
		loc, err := set.LocationRegistry.Create(ctx, models.Location{
			Name: "paginate-location-" + string(rune('a'+i)),
		})
		c.Assert(err, qt.IsNil)

		_, err = set.AreaRegistry.Create(ctx, models.Area{
			Name:       "paginate-area-" + string(rune('a'+i)),
			LocationID: loc.ID,
		})
		c.Assert(err, qt.IsNil)
	}

	t.Run("locations", func(t *testing.T) {
		c := qt.New(t)

		page, total, err := set.LocationRegistry.ListPaginated(ctx, 0, 2)
		c.Assert(err, qt.IsNil)
		c.Check(page, qt.HasLen, 2)
		c.Check(total, qt.Equals, seeded, qt.Commentf("a full page carries the window's total"))

		beyond, total, err := set.LocationRegistry.ListPaginated(ctx, 100, 2)
		c.Assert(err, qt.IsNil)
		c.Check(beyond, qt.HasLen, 0)
		c.Check(total, qt.Equals, seeded,
			qt.Commentf("an empty page has no row to carry the total, so it must be counted"))
	})

	t.Run("areas", func(t *testing.T) {
		c := qt.New(t)

		page, total, err := set.AreaRegistry.ListPaginated(ctx, 0, 2, registry.AreaListOptions{})
		c.Assert(err, qt.IsNil)
		c.Check(page, qt.HasLen, 2)
		c.Check(total, qt.Equals, seeded)

		beyond, total, err := set.AreaRegistry.ListPaginated(ctx, 100, 2, registry.AreaListOptions{})
		c.Assert(err, qt.IsNil)
		c.Check(beyond, qt.HasLen, 0)
		c.Check(total, qt.Equals, seeded)
	})

	t.Run("commodities, where the count has to respect the filter", func(t *testing.T) {
		c := qt.New(t)

		// No commodities were seeded, so both the page and the total are empty
		// — and the total still has to come from a count, not from nothing.
		page, total, err := set.CommodityRegistry.ListPaginated(ctx, 0, 2, registry.CommodityListOptions{})
		c.Assert(err, qt.IsNil)
		c.Check(page, qt.HasLen, 0)
		c.Check(total, qt.Equals, 0)
	})
}
