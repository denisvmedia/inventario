package registrytest

import (
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// RunLocationFilterContract checks the LocationID filter of the item list
// (#2897): it keeps the items in any area of the location, applies together
// with AreaID, leaves out the items without an area and matches nothing for
// a location the group does not have. ListIDs, CountEach and CountTags select
// the same items as ListPaginated. env.AreaID and env.OtherAreaID are areas
// of env.LocationID, env.ElsewhereAreaID an area of env.ElsewhereLocationID.
// newEnv is called once per subtest and must return an empty group.
func RunLocationFilterContract(t *testing.T, newEnv func(t *testing.T) TrashEnv) {
	t.Run("items in the areas of a location", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		ctx := env.Ctx

		locationItem(c, env, "Drill", env.AreaID, "tools")
		locationItem(c, env, "Lamp", env.OtherAreaID, "")
		locationItem(c, env, "Kayak", env.ElsewhereAreaID, "tools")
		locationItem(c, env, "Loose", "", "tools")

		cases := []struct {
			name string
			opts registry.CommodityListOptions
			want []string
		}{
			{"every area of the location", registry.CommodityListOptions{LocationID: env.LocationID}, []string{"Drill", "Lamp"}},
			{"another location", registry.CommodityListOptions{LocationID: env.ElsewhereLocationID}, []string{"Kayak"}},
			{"an area of the location", registry.CommodityListOptions{LocationID: env.LocationID, AreaID: env.AreaID}, []string{"Drill"}},
			{"an area of another location", registry.CommodityListOptions{LocationID: env.LocationID, AreaID: env.ElsewhereAreaID}, []string{}},
			{"a location the group does not have", registry.CommodityListOptions{LocationID: "no-such-location"}, []string{}},
		}
		batch := make([]registry.CommodityListOptions, 0, len(cases))
		for _, tc := range cases {
			opts := tc.opts
			opts.IncludeInactive = true
			batch = append(batch, opts)

			items, total, err := env.Commodities.ListPaginated(ctx, 0, 100, opts)
			c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
			c.Check(commodityNames(items), qt.DeepEquals, tc.want, qt.Commentf(tc.name))
			c.Check(total, qt.Equals, len(tc.want), qt.Commentf(tc.name))

			ids, err := env.Commodities.ListIDs(ctx, opts, 100)
			c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
			c.Check(ids, qt.HasLen, len(tc.want), qt.Commentf(tc.name))
		}

		counts, err := env.Commodities.CountEach(ctx, batch)
		c.Assert(err, qt.IsNil)
		for i, tc := range cases {
			c.Check(counts[i], qt.Equals, len(tc.want), qt.Commentf(tc.name))
		}

		tags, err := env.Commodities.CountTags(ctx, registry.CommodityListOptions{IncludeInactive: true, LocationID: env.LocationID})
		c.Assert(err, qt.IsNil)
		c.Check(tags.Tags, qt.DeepEquals, map[string]int{"tools": 1})
		c.Check(tags.Untagged, qt.Equals, 1)
	})
}

// locationItem creates an in-use item in areaID, or without an area when
// areaID is empty, carrying tag when it is not empty.
func locationItem(c *qt.C, env TrashEnv, name, areaID, tag string) {
	c.Helper()
	item := models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeOther,
		Count:     1,
		Status:    models.CommodityStatusInUse,
	}
	if areaID != "" {
		item.AreaID = new(areaID)
	}
	if tag != "" {
		item.Tags = []string{tag}
	}
	_, err := env.Commodities.Create(env.Ctx, item)
	c.Assert(err, qt.IsNil)
}

// commodityNames returns the names of items, sorted.
func commodityNames(items []*models.Commodity) []string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	slices.Sort(names)
	return names
}
