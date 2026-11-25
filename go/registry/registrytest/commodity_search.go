// Package registrytest holds behavior tests that every registry backend has
// to pass. A backend test calls the Run function with its own registries, so
// the memory and postgres backends are held to one table of expectations
// instead of two that can drift apart.
package registrytest

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// CommoditySearchEnv is what RunCommoditySearchContract needs from a
// backend: registries scoped to one empty group, the context that carries
// that scope, and an area items can be placed in.
type CommoditySearchEnv struct {
	Ctx         context.Context
	Commodities registry.CommodityRegistry
	Tags        registry.TagRegistry
	AreaID      string
}

type searchSeed struct {
	name, shortName, serial string
	brand, model            string
	extraSerials, parts     []string
	tags                    []string
	comments, warrantyNotes string
	statusNote              string
	status                  models.CommodityStatus
}

// searchSeeds is the group every case runs against. Each item exists to make
// some case tell a right answer from a wrong one; the comments say which.
var searchSeeds = []searchSeed{
	{
		name: "Cordless Drill", shortName: "Drill", serial: "SN-ABC-123",
		brand: "Bosch", model: "GSR 12V-15",
		extraSerials: []string{"XS-999"}, parts: []string{"PN-77"},
		tags:          []string{"power-tools", "red"},
		comments:      "Bought at the hardware store",
		warrantyNotes: "Extended warranty via retailer",
	},
	{
		// "50% off" catches an unescaped % (the pattern 5%f would match).
		name: "Kitchen Fridge", shortName: "Fridge", serial: "FR-2024",
		brand: "Liebherr", model: "CNsfd 5023",
		tags:     []string{"kitchen"},
		comments: "Came with a 50% off sticker",
	},
	{
		// GH_10 and "Hose" catch an unescaped _ (the pattern hos_ would
		// match "Hose"). An empty tag list.
		name: "Garden Hose", shortName: "Hose", parts: []string{"GH_10"},
		brand:      "Gardena",
		tags:       []string{},
		statusNote: "Leaks at the tap",
	},
	{
		// Sold, so the default view hides it. Its brand differs from the
		// drill's only in case, so the brand list keeps the two apart.
		name: "Old Phone", shortName: "Phone", brand: "bosch",
		tags:       []string{"red"},
		statusNote: "Sold to a neighbor",
		status:     models.CommodityStatusSold,
	},
	{
		// "red" in the name only; no tag list at all (NULL in postgres).
		name: "Red Toolbox", shortName: "Toolbox", brand: "Bosch",
	},
}

// RunCommoditySearchContract checks that a backend's CommodityRegistry
// applies the item search and the tag filter exactly as
// registry.MatchCommoditySearch and registry.CommodityTagFilter describe,
// that it sorts unknown dates and prices last, that its
// TagRegistry.SlugsByLabel keeps to its own tag kind, and that its tag search
// treats % and _ as plain characters. newEnv is called once per subtest and
// must return an empty group.
func RunCommoditySearchContract(t *testing.T, newEnv func(t *testing.T) CommoditySearchEnv) {
	t.Run("search and tag filter", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		hose := seedSearchGroup(c, env)

		for _, tc := range searchCases(hose) {
			c.Run(tc.name, func(c *qt.C) {
				opts := tc.opts
				opts.IncludeInactive = !tc.activeOnly
				opts.SearchTagLabels = resolveLabels(c, env, opts.Search)

				got, total, err := env.Commodities.ListPaginated(env.Ctx, 0, 50, opts)
				c.Assert(err, qt.IsNil)
				c.Check(names(got), qt.DeepEquals, tc.want)
				c.Check(total, qt.Equals, len(tc.want))

				// Every row the backend returns must be explainable by the
				// shared rule, or the match hints would come back empty.
				terms := registry.SplitSearchTerms(opts.Search)
				for _, cm := range got {
					matches, ok := registry.MatchCommoditySearch(cm, terms, opts.SearchTagLabels)
					c.Check(ok, qt.IsTrue, qt.Commentf("%s is not explained by the search rule", cm.Name))
					if tc.match != nil && len(got) == 1 {
						c.Check(matches, qt.Contains, *tc.match, qt.Commentf("matches of %s", cm.Name))
					}
				}
			})
		}
	})

	t.Run("tag counts", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		seedSearchGroup(c, env)

		for _, tc := range []struct {
			name string
			opts registry.CommodityListOptions
			want registry.CommodityTagCounts
		}{
			{
				name: "every item",
				opts: registry.CommodityListOptions{IncludeInactive: true},
				want: registry.CommodityTagCounts{
					Tags:     map[string]int{"power-tools": 1, "red": 2, "kitchen": 1},
					Untagged: 2,
				},
			},
			{
				// The sold phone drops out of "red".
				name: "default view",
				opts: registry.CommodityListOptions{},
				want: registry.CommodityTagCounts{
					Tags:     map[string]int{"power-tools": 1, "red": 1, "kitchen": 1},
					Untagged: 2,
				},
			},
			{
				name: "with a search",
				opts: registry.CommodityListOptions{IncludeInactive: true, Search: "red"},
				want: registry.CommodityTagCounts{
					Tags:     map[string]int{"power-tools": 1, "red": 2},
					Untagged: 1,
				},
			},
			{
				name: "with a tag filter",
				opts: registry.CommodityListOptions{
					IncludeInactive: true,
					Tags:            registry.CommodityTagFilter{Exclude: []string{"red"}},
				},
				want: registry.CommodityTagCounts{Tags: map[string]int{"kitchen": 1}, Untagged: 2},
			},
			{
				name: "nothing selected",
				opts: registry.CommodityListOptions{Search: "nonexistent"},
				want: registry.CommodityTagCounts{Tags: map[string]int{}},
			},
		} {
			c.Run(tc.name, func(c *qt.C) {
				got, err := env.Commodities.CountTags(env.Ctx, tc.opts)
				c.Assert(err, qt.IsNil)
				c.Check(got, qt.DeepEquals, tc.want)

				// The counts describe the items the list returns.
				items, _, err := env.Commodities.ListPaginated(env.Ctx, 0, 50, tc.opts)
				c.Assert(err, qt.IsNil)
				c.Check(registry.CountCommodityTags(items), qt.DeepEquals, got)
			})
		}
	})

	t.Run("unknown dates and prices sort last", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		seedSortGroup(c, env)

		for _, tc := range []struct {
			field registry.CommoditySortField
			desc  bool
			want  []string
		}{
			// Bravo's current value is a known zero: it sorts as a value.
			{field: registry.CommoditySortCurrentPrice, want: []string{"Bravo", "Alpha", "Charlie"}},
			{field: registry.CommoditySortCurrentPrice, desc: true, want: []string{"Alpha", "Bravo", "Charlie"}},
			{field: registry.CommoditySortOriginalPrice, want: []string{"Alpha", "Bravo", "Charlie"}},
			{field: registry.CommoditySortOriginalPrice, desc: true, want: []string{"Bravo", "Alpha", "Charlie"}},
			{field: registry.CommoditySortPurchaseDate, want: []string{"Bravo", "Alpha", "Charlie"}},
			{field: registry.CommoditySortPurchaseDate, desc: true, want: []string{"Alpha", "Bravo", "Charlie"}},
		} {
			name := string(tc.field)
			if tc.desc {
				name = "-" + name
			}
			c.Run(name, func(c *qt.C) {
				got, _, err := env.Commodities.ListPaginated(env.Ctx, 0, 50, registry.CommodityListOptions{
					SortField: tc.field, SortDesc: tc.desc,
				})
				c.Assert(err, qt.IsNil)
				c.Check(names(got), qt.DeepEquals, tc.want)
			})
		}
	})

	t.Run("brands", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		seedSearchGroup(c, env)

		for _, tc := range []struct {
			name  string
			query string
			limit int
			want  []registry.CommodityBrandCount
		}{
			{
				// The sold phone counts; "bosch" and "Bosch" stay apart.
				name: "every brand", limit: 10,
				want: []registry.CommodityBrandCount{
					{Brand: "Bosch", Count: 2},
					{Brand: "bosch", Count: 1},
					{Brand: "Gardena", Count: 1},
					{Brand: "Liebherr", Count: 1},
				},
			},
			{name: "substring, any case", query: "EBH", limit: 10, want: []registry.CommodityBrandCount{{Brand: "Liebherr", Count: 1}}},
			{name: "cut to the limit", query: "bos", limit: 1, want: []registry.CommodityBrandCount{{Brand: "Bosch", Count: 2}}},
			{name: "% is literal", query: "%", limit: 10, want: []registry.CommodityBrandCount{}},
			{name: "no limit, no brands", limit: 0, want: []registry.CommodityBrandCount{}},
		} {
			c.Run(tc.name, func(c *qt.C) {
				got, err := env.Commodities.ListBrands(env.Ctx, tc.query, tc.limit)
				c.Assert(err, qt.IsNil)
				c.Check(got, qt.DeepEquals, tc.want)

				all, err := env.Commodities.List(env.Ctx)
				c.Assert(err, qt.IsNil)
				c.Check(registry.CountCommodityBrands(all, tc.query, tc.limit), qt.DeepEquals, got)
			})
		}
	})

	t.Run("trashed items", func(t *testing.T) { runTrashedItemsCase(t, newEnv) })

	t.Run("tag labels stay within their kind", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		for _, tag := range []models.Tag{
			{Kind: models.TagKindCommodity, Slug: "power-tools", Label: "Power Tools", Color: models.TagColorBlue},
			{Kind: models.TagKindCommodity, Slug: "tools-misc", Label: "Misc tools", Color: models.TagColorMuted},
			{Kind: models.TagKindCommodity, Slug: "pct", Label: "100% done", Color: models.TagColorGreen},
			{Kind: models.TagKindFile, Slug: "tool-manuals", Label: "Tool manuals", Color: models.TagColorRed},
		} {
			_, err := env.Tags.Create(env.Ctx, tag)
			c.Assert(err, qt.IsNil)
		}

		slugs, err := env.Tags.SlugsByLabel(env.Ctx, models.TagKindCommodity, "TOOL")
		c.Assert(err, qt.IsNil)
		c.Check(slugs, qt.DeepEquals, []string{"power-tools", "tools-misc"})

		slugs, err = env.Tags.SlugsByLabel(env.Ctx, models.TagKindFile, "tool")
		c.Assert(err, qt.IsNil)
		c.Check(slugs, qt.DeepEquals, []string{"tool-manuals"})

		// % is literal: "0%" occurs only in "100% done".
		slugs, err = env.Tags.SlugsByLabel(env.Ctx, models.TagKindCommodity, "0%")
		c.Assert(err, qt.IsNil)
		c.Check(slugs, qt.DeepEquals, []string{"pct"})

		slugs, err = env.Tags.SlugsByLabel(env.Ctx, models.TagKindCommodity, "")
		c.Assert(err, qt.IsNil)
		c.Check(slugs, qt.HasLen, 0)
	})

	t.Run("tag search matches % and _ literally", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		for _, tag := range []models.Tag{
			{Kind: models.TagKindCommodity, Slug: "pct", Label: "100% done", Color: models.TagColorGreen},
			// An unescaped "0%" (0, then anything) would match it.
			{Kind: models.TagKindCommodity, Slug: "model-2010", Label: "Model 2010", Color: models.TagColorBlue},
			{Kind: models.TagKindFile, Slug: "snake-case", Label: "Snake_case", Color: models.TagColorRed},
			// An unescaped "e_c" (e, any one character, c) would match
			// its label and its slug.
			{Kind: models.TagKindFile, Slug: "blue-cap", Label: "Blue cap", Color: models.TagColorMuted},
		} {
			_, err := env.Tags.Create(env.Ctx, tag)
			c.Assert(err, qt.IsNil)
		}

		for _, tc := range []struct {
			search string
			kind   models.TagKind
			want   []string
		}{
			{search: "0%", kind: models.TagKindAny, want: []string{"pct"}},
			{search: "e_c", kind: models.TagKindAny, want: []string{"snake-case"}},
			{search: "E_C", kind: models.TagKindFile, want: []string{"snake-case"}},
			{search: "%", kind: models.TagKindAny, want: []string{"pct"}},
			{search: "_", kind: models.TagKindAny, want: []string{"snake-case"}},
		} {
			c.Run(tc.search, func(c *qt.C) {
				got, total, err := env.Tags.ListPaginated(env.Ctx, 0, 50, registry.TagListOptions{
					Search: tc.search, Kind: tc.kind,
				})
				c.Assert(err, qt.IsNil)
				c.Check(tagSlugs(got), qt.DeepEquals, tc.want)
				c.Check(total, qt.Equals, len(tc.want))

				// The tag autocomplete filters by the same predicate.
				got, err = env.Tags.Search(env.Ctx, tc.search, 50, tc.kind)
				c.Assert(err, qt.IsNil)
				c.Check(tagSlugs(got), qt.DeepEquals, tc.want)
			})
		}
	})
}

type searchCase struct {
	name       string
	opts       registry.CommodityListOptions
	activeOnly bool
	want       []string
	// match, when set and the result is a single item, must be among the
	// reasons that item matched.
	match *registry.CommoditySearchMatch
}

func searchCases(hose *models.Commodity) []searchCase {
	match := func(field registry.CommoditySearchField, value string) *registry.CommoditySearchMatch {
		return &registry.CommoditySearchMatch{Field: field, Value: value}
	}
	search := func(q string) registry.CommodityListOptions {
		return registry.CommodityListOptions{Search: q}
	}
	tags := func(f registry.CommodityTagFilter) registry.CommodityListOptions {
		return registry.CommodityListOptions{Tags: f}
	}
	return []searchCase{
		{name: "serial number, any case", opts: search("abc-123"), want: []string{"Cordless Drill"},
			match: match(registry.CommoditySearchFieldSerialNumber, "SN-ABC-123")},
		{name: "extra serial number", opts: search("xs-999"), want: []string{"Cordless Drill"},
			match: match(registry.CommoditySearchFieldExtraSerialNumbers, "XS-999")},
		{name: "part number", opts: search("PN-77"), want: []string{"Cordless Drill"},
			match: match(registry.CommoditySearchFieldPartNumbers, "PN-77")},
		{name: "tag slug", opts: search("power-tools"), want: []string{"Cordless Drill"},
			match: match(registry.CommoditySearchFieldTags, "power-tools")},
		{name: "tag label as a quoted phrase", opts: search(`"power tools"`), want: []string{"Cordless Drill"},
			match: match(registry.CommoditySearchFieldTags, "power-tools")},
		{name: "comments phrase", opts: search(`"hardware store"`), want: []string{"Cordless Drill"},
			match: match(registry.CommoditySearchFieldComments, "Bought at the hardware store")},
		{name: "warranty notes", opts: search("retailer"), want: []string{"Cordless Drill"},
			match: match(registry.CommoditySearchFieldWarrantyNotes, "Extended warranty via retailer")},
		{name: "status note", opts: search("neighbor"), want: []string{"Old Phone"},
			match: match(registry.CommoditySearchFieldStatusNote, "Sold to a neighbor")},
		{name: "short name", opts: search("toolbox"), want: []string{"Red Toolbox"}},
		{name: "brand, any case", opts: search("BOSCH"), want: []string{"Cordless Drill", "Old Phone", "Red Toolbox"}},
		{name: "brand in the default view", opts: search("liebherr"), activeOnly: true, want: []string{"Kitchen Fridge"},
			match: match(registry.CommoditySearchFieldBrand, "Liebherr")},
		{name: "model", opts: search("12v-15"), want: []string{"Cordless Drill"},
			match: match(registry.CommoditySearchFieldModel, "GSR 12V-15")},
		{name: "brand and model AND-ed with another field", opts: search("gardena hose"), want: []string{"Garden Hose"}},
		{name: "asset ID", opts: search(models.FormatAssetID(*hose.AssetNumber)), want: []string{"Garden Hose"},
			match: match(registry.CommoditySearchFieldAssetID, models.FormatAssetID(*hose.AssetNumber))},
		{name: "one term, several fields across items", opts: search("red"),
			want: []string{"Cordless Drill", "Old Phone", "Red Toolbox"}},
		{name: "terms are AND-ed", opts: search("drill red"), want: []string{"Cordless Drill"}},
		{name: "terms are AND-ed across fields of one item", opts: search("hardware retailer"),
			want: []string{"Cordless Drill"}},
		{name: "a term that matches nothing empties the result", opts: search("red nonexistent"), want: []string{}},
		{name: "% is literal", opts: search("5%f"), want: []string{}},
		{name: "% matches itself", opts: search("50%"), want: []string{"Kitchen Fridge"}},
		{name: "_ is literal", opts: search("hos_"), want: []string{}},
		{name: "_ matches itself", opts: search("gh_1"), want: []string{"Garden Hose"}},
		{name: "default view hides sold items", opts: search("red"), activeOnly: true,
			want: []string{"Cordless Drill", "Red Toolbox"}},

		{name: "tags: all", opts: tags(registry.CommodityTagFilter{Include: []string{"red", "power-tools"}}),
			want: []string{"Cordless Drill"}},
		{name: "tags: any", opts: tags(registry.CommodityTagFilter{Include: []string{"kitchen", "power-tools"}, Mode: registry.TagMatchAny}),
			want: []string{"Cordless Drill", "Kitchen Fridge"}},
		{name: "tags: exclude keeps untagged items", opts: tags(registry.CommodityTagFilter{Exclude: []string{"red"}}),
			want: []string{"Garden Hose", "Kitchen Fridge", "Red Toolbox"}},
		{name: "tags: untagged, NULL and empty alike", opts: tags(registry.CommodityTagFilter{Untagged: true}),
			want: []string{"Garden Hose", "Red Toolbox"}},
		{name: "tags: unknown slug matches nothing", opts: tags(registry.CommodityTagFilter{Include: []string{"nope"}}),
			want: []string{}},
		{name: "tags and search combine", opts: registry.CommodityListOptions{
			Search: "red", Tags: registry.CommodityTagFilter{Exclude: []string{"red"}},
		}, want: []string{"Red Toolbox"}},
	}
}

// seedSearchGroup creates the commodity tags and searchSeeds and returns the
// Garden Hose as stored, for its asset number.
func seedSearchGroup(c *qt.C, env CommoditySearchEnv) *models.Commodity {
	c.Helper()
	for _, tag := range []models.Tag{
		{Kind: models.TagKindCommodity, Slug: "power-tools", Label: "Power Tools", Color: models.TagColorBlue},
		{Kind: models.TagKindCommodity, Slug: "red", Label: "Red", Color: models.TagColorRed},
		{Kind: models.TagKindCommodity, Slug: "kitchen", Label: "Kitchen", Color: models.TagColorAmber},
		// A file tag whose label contains "tools": it must not make the
		// label search find anything.
		{Kind: models.TagKindFile, Slug: "red", Label: "Power tools manual", Color: models.TagColorMuted},
	} {
		_, err := env.Tags.Create(env.Ctx, tag)
		c.Assert(err, qt.IsNil)
	}

	var hose *models.Commodity
	for _, s := range searchSeeds {
		status := s.status
		if status == "" {
			status = models.CommodityStatusInUse
		}
		var tags models.ValuerSlice[string]
		if s.tags != nil {
			tags = models.ValuerSlice[string](s.tags)
		}
		created, err := env.Commodities.Create(env.Ctx, models.Commodity{
			Name:                  s.name,
			ShortName:             s.shortName,
			Brand:                 s.brand,
			Model:                 s.model,
			Type:                  models.CommodityTypeOther,
			AreaID:                new(env.AreaID),
			Count:                 1,
			OriginalPrice:         new(decimal.NewFromInt(10)),
			OriginalPriceCurrency: "USD",
			CurrentPrice:          new(decimal.NewFromInt(10)),
			Status:                status,
			StatusNote:            s.statusNote,
			SerialNumber:          s.serial,
			ExtraSerialNumbers:    models.ValuerSlice[string](s.extraSerials),
			PartNumbers:           models.ValuerSlice[string](s.parts),
			Tags:                  tags,
			Comments:              s.comments,
			WarrantyNotes:         s.warrantyNotes,
			PurchaseDate:          models.ToPDate("2024-01-01"),
			RegisteredDate:        models.ToPDate("2024-01-02"),
			LastModifiedDate:      models.ToPDate("2024-01-03"),
		})
		c.Assert(err, qt.IsNil, qt.Commentf("seeding %s", s.name))
		if s.name == "Garden Hose" {
			hose = created
		}
	}
	c.Assert(hose.AssetNumber, qt.IsNotNil)
	return hose
}

// seedSortGroup creates three items: Alpha and Bravo with known prices and
// purchase dates (Bravo's current value a known zero), and Charlie with
// neither.
func seedSortGroup(c *qt.C, env CommoditySearchEnv) {
	c.Helper()
	for _, seed := range []struct {
		name              string
		original, current *decimal.Decimal
		purchased         models.PDate
	}{
		{name: "Alpha", original: new(decimal.NewFromInt(5)), current: new(decimal.NewFromInt(5)), purchased: models.ToPDate("2024-01-01")},
		{name: "Bravo", original: new(decimal.NewFromInt(20)), current: new(decimal.Zero), purchased: models.ToPDate("2023-01-01")},
		{name: "Charlie"},
	} {
		_, err := env.Commodities.Create(env.Ctx, models.Commodity{
			Name:                  seed.name,
			ShortName:             seed.name,
			Type:                  models.CommodityTypeOther,
			AreaID:                new(env.AreaID),
			Count:                 1,
			OriginalPrice:         seed.original,
			OriginalPriceCurrency: "USD",
			CurrentPrice:          seed.current,
			Status:                models.CommodityStatusInUse,
			PurchaseDate:          seed.purchased,
		})
		c.Assert(err, qt.IsNil, qt.Commentf("seeding %s", seed.name))
	}
}

// resolveLabels does what services/commodityquery.Resolve does for the
// search text: it maps every term to the item tags whose label contains it.
func resolveLabels(c *qt.C, env CommoditySearchEnv, q string) map[string][]string {
	c.Helper()
	labels := make(map[string][]string)
	for _, term := range registry.SplitSearchTerms(q) {
		slugs, err := env.Tags.SlugsByLabel(env.Ctx, models.TagKindCommodity, term)
		c.Assert(err, qt.IsNil)
		if len(slugs) > 0 {
			labels[term] = slugs
		}
	}
	return labels
}

func tagSlugs(tags []*models.Tag) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		out = append(out, tag.Slug)
	}
	return out
}

func names(cs []*models.Commodity) []string {
	out := make([]string, 0, len(cs))
	for _, cm := range cs {
		out = append(out, cm.Name)
	}
	return out
}

// runTrashedItemsCase checks that the list and the tag counts read live items
// by default and the items in Trash with CommodityListOptions.Trashed, with
// the search and the tag filter applied the same way to both.
func runTrashedItemsCase(t *testing.T, newEnv func(t *testing.T) CommoditySearchEnv) {
	c := qt.New(t)
	env := newEnv(t)
	seedSearchGroup(c, env)
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	live, _, err := env.Commodities.ListPaginated(env.Ctx, 0, 50, registry.CommodityListOptions{IncludeInactive: true})
	c.Assert(err, qt.IsNil)
	for _, cm := range live {
		if cm.Name == "Red Toolbox" || cm.Name == "Kitchen Fridge" {
			_, err := env.Commodities.Trash(env.Ctx, cm.ID, "registrytest", at)
			c.Assert(err, qt.IsNil, qt.Commentf("trashing %s", cm.Name))
		}
	}

	for _, tc := range []struct {
		name   string
		opts   registry.CommodityListOptions
		want   []string
		counts registry.CommodityTagCounts
	}{
		{name: "live by default", opts: registry.CommodityListOptions{},
			want: []string{"Cordless Drill", "Garden Hose", "Old Phone"},
			counts: registry.CommodityTagCounts{
				Tags: map[string]int{"power-tools": 1, "red": 2}, Untagged: 1,
			}},
		{name: "live search", opts: registry.CommodityListOptions{Search: "red"},
			want:   []string{"Cordless Drill", "Old Phone"},
			counts: registry.CommodityTagCounts{Tags: map[string]int{"power-tools": 1, "red": 2}}},
		{name: "trash only", opts: registry.CommodityListOptions{Trashed: true},
			want: []string{"Kitchen Fridge", "Red Toolbox"},
			counts: registry.CommodityTagCounts{
				Tags: map[string]int{"kitchen": 1}, Untagged: 1,
			}},
		{name: "search in the trash", opts: registry.CommodityListOptions{Trashed: true, Search: "red"},
			want:   []string{"Red Toolbox"},
			counts: registry.CommodityTagCounts{Tags: map[string]int{}, Untagged: 1}},
		{name: "tag filter in the trash", opts: registry.CommodityListOptions{
			Trashed: true, Tags: registry.CommodityTagFilter{Include: []string{"kitchen"}},
		}, want: []string{"Kitchen Fridge"},
			counts: registry.CommodityTagCounts{Tags: map[string]int{"kitchen": 1}}},
	} {
		c.Run(tc.name, func(c *qt.C) {
			opts := tc.opts
			opts.IncludeInactive = true
			got, total, err := env.Commodities.ListPaginated(env.Ctx, 0, 50, opts)
			c.Assert(err, qt.IsNil)
			c.Check(names(got), qt.DeepEquals, tc.want)
			c.Check(total, qt.Equals, len(tc.want))
			for _, cm := range got {
				c.Check(cm.IsTrashed(), qt.Equals, opts.Trashed, qt.Commentf("%s", cm.Name))
			}

			counts, err := env.Commodities.CountTags(env.Ctx, opts)
			c.Assert(err, qt.IsNil)
			c.Check(counts, qt.DeepEquals, tc.counts)
		})
	}

	// The brand suggestions count live items only: the fridge's brand is
	// gone and the toolbox no longer adds to Bosch.
	brands, err := env.Commodities.ListBrands(env.Ctx, "", 10)
	c.Assert(err, qt.IsNil)
	c.Check(brands, qt.DeepEquals, []registry.CommodityBrandCount{
		{Brand: "Bosch", Count: 1},
		{Brand: "bosch", Count: 1},
		{Brand: "Gardena", Count: 1},
	})
}
