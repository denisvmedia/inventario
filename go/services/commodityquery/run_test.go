package commodityquery_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services/commodityquery"
)

func newMemorySet(c *qt.C) (context.Context, *registry.Set) {
	c.Helper()
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
		Slug:          "group-1",
		GroupCurrency: "USD",
	})
	set, err := memory.NewFactorySet().CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	return ctx, set
}

func seedItem(c *qt.C, ctx context.Context, set *registry.Set, name string, tags ...string) *models.Commodity {
	c.Helper()
	created, err := set.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                  name,
		ShortName:             name,
		Type:                  models.CommodityTypeOther,
		Count:                 1,
		OriginalPrice:         new(decimal.NewFromInt(1)),
		OriginalPriceCurrency: "USD",
		CurrentPrice:          new(decimal.NewFromInt(1)),
		Status:                models.CommodityStatusInUse,
		Tags:                  models.ValuerSlice[string](tags),
		PurchaseDate:          models.ToPDate("2024-01-01"),
		RegisteredDate:        models.ToPDate("2024-01-01"),
		LastModifiedDate:      models.ToPDate("2024-01-01"),
	})
	c.Assert(err, qt.IsNil)
	return created
}

func listNames(page commodityquery.Page) []string {
	out := make([]string, 0, len(page.Commodities))
	for _, cm := range page.Commodities {
		out = append(out, cm.Name)
	}
	return out
}

func TestList_FindsByTagLabelAndExplains(t *testing.T) {
	c := qt.New(t)
	ctx, set := newMemorySet(c)

	_, err := set.TagRegistry.Create(ctx, models.Tag{
		Kind: models.TagKindCommodity, Slug: "pt", Label: "Power Tools", Color: models.TagColorBlue,
	})
	c.Assert(err, qt.IsNil)
	// The same label in the file namespace must not leak into item search.
	_, err = set.TagRegistry.Create(ctx, models.Tag{
		Kind: models.TagKindFile, Slug: "manual", Label: "Power tools manual", Color: models.TagColorBlue,
	})
	c.Assert(err, qt.IsNil)

	seedItem(c, ctx, set, "Drill", "pt")
	seedItem(c, ctx, set, "Lamp", "manual")
	seedItem(c, ctx, set, "Saw")

	page, err := commodityquery.List(ctx, set, commodityquery.Query{Text: `"power tools"`, IncludeInactive: true}, 0, 10)
	c.Assert(err, qt.IsNil)
	c.Check(listNames(page), qt.DeepEquals, []string{"Drill"})
	c.Check(page.Total, qt.Equals, 1)
	c.Check(page.Matches[page.Commodities[0].ID], qt.DeepEquals, []registry.CommoditySearchMatch{
		{Field: registry.CommoditySearchFieldTags, Value: "pt"},
	})
}

func TestList_NoTextMeansNoMatches(t *testing.T) {
	c := qt.New(t)
	ctx, set := newMemorySet(c)
	seedItem(c, ctx, set, "Drill", "red")
	seedItem(c, ctx, set, "Saw")

	page, err := commodityquery.List(ctx, set, commodityquery.Query{
		IncludeInactive: true,
		Tags:            registry.CommodityTagFilter{Untagged: true},
	}, 0, 10)
	c.Assert(err, qt.IsNil)
	c.Check(listNames(page), qt.DeepEquals, []string{"Saw"})
	c.Check(page.Matches, qt.IsNil)
}

func TestList_PagesKeepTheTotal(t *testing.T) {
	c := qt.New(t)
	ctx, set := newMemorySet(c)
	for _, name := range []string{"Red A", "Red B", "Red C", "Blue"} {
		seedItem(c, ctx, set, name)
	}

	page, err := commodityquery.List(ctx, set, commodityquery.Query{Text: "red", IncludeInactive: true}, 1, 1)
	c.Assert(err, qt.IsNil)
	c.Check(listNames(page), qt.DeepEquals, []string{"Red B"})
	c.Check(page.Total, qt.Equals, 3)
	c.Check(page.Matches, qt.HasLen, 1)
}

// CountEach gives, per query, the total List reports for it, with no query,
// with the lent-out filter the memory backend resolves to item IDs, and with
// an item in Trash, which both leave out.
func TestCountEach_MatchesTheListTotals(t *testing.T) {
	c := qt.New(t)
	ctx, set := newMemorySet(c)
	drill := seedItem(c, ctx, set, "Red drill", "red", "tools")
	seedItem(c, ctx, set, "Red lamp", "red")
	seedItem(c, ctx, set, "Saw", "tools")
	_, err := set.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID: drill.ID, BorrowerName: "Neighbor", LentAt: models.Date("2026-01-01"),
	})
	c.Assert(err, qt.IsNil)

	qs := []commodityquery.Query{
		{IncludeInactive: true, Tags: registry.CommodityTagFilter{Include: []string{"red"}}},
		{IncludeInactive: true, Text: "saw"},
		{IncludeInactive: true, LentOut: new(true)},
		{IncludeInactive: true, Tags: registry.CommodityTagFilter{Include: []string{"none"}}},
		{IncludeInactive: true, Trashed: true},
	}
	// An item in Trash is left out, as the list leaves it out.
	trashed := seedItem(c, ctx, set, "Red trashed", "red")
	_, err = set.CommodityRegistry.Trash(ctx, trashed.ID, "user-1", time.Now())
	c.Assert(err, qt.IsNil)

	counts, err := commodityquery.CountEach(ctx, set, qs)
	c.Assert(err, qt.IsNil)
	c.Assert(counts, qt.DeepEquals, []int{2, 1, 1, 0, 1})
	for i, q := range qs {
		page, err := commodityquery.List(ctx, set, q, 0, 1)
		c.Assert(err, qt.IsNil)
		c.Check(counts[i], qt.Equals, page.Total, qt.Commentf("query %d", i))
	}

	none, err := commodityquery.CountEach(ctx, set, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(none, qt.HasLen, 0)
}

func TestTagCounts_SetsTheTagFilterAside(t *testing.T) {
	c := qt.New(t)
	ctx, set := newMemorySet(c)
	seedItem(c, ctx, set, "Red drill", "red", "tools")
	seedItem(c, ctx, set, "Red lamp", "red")
	seedItem(c, ctx, set, "Red rag")
	seedItem(c, ctx, set, "Blue saw", "tools")

	q := commodityquery.Query{
		Text:            "red",
		IncludeInactive: true,
		Tags:            registry.CommodityTagFilter{Include: []string{"tools"}},
	}
	counts, err := commodityquery.TagCounts(ctx, set, q)
	c.Assert(err, qt.IsNil)
	// The search applies; the tag filter does not, so "red" counts both
	// red items although the query asks for "tools".
	c.Check(counts, qt.DeepEquals, registry.CommodityTagCounts{
		Tags:     map[string]int{"red": 2, "tools": 1},
		Untagged: 1,
	})

	// Each count is what the list shows with that tag alone selected.
	q.Tags = registry.CommodityTagFilter{Include: []string{"red"}}
	page, err := commodityquery.List(ctx, set, q, 0, 10)
	c.Assert(err, qt.IsNil)
	c.Check(page.Total, qt.Equals, counts.Tags["red"])
}

func TestResolve_LentOutOnTheMemoryBackend(t *testing.T) {
	c := qt.New(t)
	ctx, set := newMemorySet(c)
	lent := seedItem(c, ctx, set, "Ladder")
	seedItem(c, ctx, set, "Bucket")
	_, err := set.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  lent.ID,
		BorrowerName: "Neighbor",
		LentAt:       models.Date("2026-01-01"),
	})
	c.Assert(err, qt.IsNil)

	opts, err := commodityquery.Resolve(ctx, set, commodityquery.Query{IncludeInactive: true, LentOut: new(true)})
	c.Assert(err, qt.IsNil)
	c.Check(opts.OpenLoanCommodityIDs, qt.DeepEquals, []string{lent.ID})

	page, err := commodityquery.List(ctx, set, commodityquery.Query{IncludeInactive: true, LentOut: new(false)}, 0, 10)
	c.Assert(err, qt.IsNil)
	c.Check(listNames(page), qt.DeepEquals, []string{"Bucket"})
}

func TestExplain_SkipsItemsTheRuleDoesNotMatch(t *testing.T) {
	c := qt.New(t)
	red := &models.Commodity{Name: "Red drill"}
	red.ID = "1"
	blue := &models.Commodity{Name: "Blue saw"}
	blue.ID = "2"
	items := []*models.Commodity{red, blue, nil}
	got := commodityquery.Explain(items, registry.CommodityListOptions{Search: "red"})
	c.Assert(got, qt.DeepEquals, map[string][]registry.CommoditySearchMatch{
		"1": {{Field: registry.CommoditySearchFieldName, Value: "Red drill"}},
	})
	c.Assert(commodityquery.Explain(items, registry.CommodityListOptions{}), qt.IsNil)
}
