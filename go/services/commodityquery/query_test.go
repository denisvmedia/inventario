package commodityquery_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/commodityquery"
)

func TestParse_Defaults(t *testing.T) {
	c := qt.New(t)
	q, err := commodityquery.Parse(url.Values{})
	c.Assert(err, qt.IsNil)
	c.Assert(q, qt.DeepEquals, commodityquery.Default())
	c.Assert(q.IncludeInactive, qt.IsTrue)
}

func TestParse_EveryParameter(t *testing.T) {
	c := qt.New(t)
	values, err := url.ParseQuery(strings.Join([]string{
		"q=+drill++red+",
		"type=equipment", "type=tools", "type=equipment",
		"status=in_use", "status=sold",
		"area_id=area-1", "unassigned=1",
		"seller_id=s-1", "seller_id=+s-2+", "seller_id=s-1",
		"include_inactive=false",
		"warranty_status=active", "warranty_status=expired",
		"warranty_expires_before=2026-12-31",
		"lent_out=TRUE",
		"tags=Fragile,power+tools", "tags=red",
		"tag_mode=any",
		"tags_exclude=garage",
		"no_photo=1", "no_purchase_price=true", "no_purchase_date=True",
		"cf=f-1:set", "cf=f-2:contains:+a:b+", "cf=f-1:set",
		"sort=-purchase_date",
	}, "&"))
	c.Assert(err, qt.IsNil)

	q, err := commodityquery.Parse(values)
	c.Assert(err, qt.IsNil)
	c.Assert(q, qt.DeepEquals, commodityquery.Query{
		Text:                  "drill  red",
		Types:                 []models.CommodityType{"equipment", "tools"},
		Statuses:              []models.CommodityStatus{"in_use", "sold"},
		AreaID:                "area-1",
		Unassigned:            true,
		SellerIDs:             []string{"s-1", "s-2"},
		IncludeInactive:       false,
		WarrantyStatuses:      []registry.WarrantyStatusFilter{"active", "expired"},
		WarrantyExpiresBefore: "2026-12-31",
		LentOut:               new(true),
		Tags: registry.CommodityTagFilter{
			Include: []string{"fragile", "power-tools", "red"},
			Mode:    registry.TagMatchAny,
			Exclude: []string{"garage"},
		},
		NoPhoto:         true,
		NoPurchasePrice: true,
		NoPurchaseDate:  true,
		CustomFields: []commodityquery.CustomFieldCondition{
			{FieldID: "f-1", Op: registry.CustomFieldOpSet},
			{FieldID: "f-2", Op: registry.CustomFieldOpContains, Operand: "a:b"},
		},
		SortField: registry.CommoditySortPurchaseDate,
		SortDesc:  true,
	})
}

// The location filter keeps the items in any area of one location (#2897).
// It goes with an area filter, both applying, and round-trips through Values.
func TestParse_Location(t *testing.T) {
	c := qt.New(t)
	values, err := url.ParseQuery("location_id=+loc-1+&area_id=area-1&unassigned=false")
	c.Assert(err, qt.IsNil)

	q, err := commodityquery.Parse(values)
	c.Assert(err, qt.IsNil)
	c.Assert(q.LocationID, qt.Equals, "loc-1")
	c.Assert(q.AreaID, qt.Equals, "area-1")
	c.Assert(q.ListOptions().LocationID, qt.Equals, "loc-1")
	c.Assert(q.Values().Encode(), qt.Equals, "area_id=area-1&location_id=loc-1")

	again, err := commodityquery.Parse(q.Values())
	c.Assert(err, qt.IsNil)
	c.Assert(again, qt.DeepEquals, q)
}

func TestParse_Refuses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		param string
	}{
		{name: "an unknown parameter", query: "colour=red", param: "colour"},
		{name: "the old tag operator", query: "tags=a&operator=AND", param: "operator"},
		{name: "an unknown status", query: "status=broken", param: "status"},
		{name: "an unknown warranty status", query: "warranty_status=soon", param: "warranty_status"},
		{name: "a malformed date", query: "warranty_expires_before=31.12.2026", param: "warranty_expires_before"},
		{name: "a boolean that is neither", query: "include_inactive=maybe", param: "include_inactive"},
		{name: "lent_out that is neither", query: "lent_out=yes", param: "lent_out"},
		{name: "a repeated single-value parameter", query: "q=a&q=b", param: "q"},
		{name: "a repeated location", query: "location_id=l-1&location_id=l-2", param: "location_id"},
		{name: "a location with no area", query: "location_id=l-1&unassigned=true", param: "location_id"},
		{name: "an unknown tag mode", query: "tags=a&tag_mode=some", param: "tag_mode"},
		{name: "a tag with no usable characters", query: "tags=%23%23%23", param: "tags"},
		{name: "a tag both required and excluded", query: "tags=red&tags_exclude=Red", param: "tags_exclude"},
		{name: "no_tags with tags", query: "no_tags=true&tags=red", param: "no_tags"},
		{name: "no_tags with an exclusion", query: "no_tags=1&tags_exclude=red", param: "no_tags"},
		{name: "a search text over the length limit", query: "q=" + strings.Repeat("a", commodityquery.MaxTextRunes+1), param: "q"},
		{name: "a search text over the term limit", query: "q=a+b+c+d+e+f+g+h+i+j+k", param: "q"},
		{name: "too many tags", query: "tags=" + manyTags(commodityquery.MaxTags+1), param: "tags"},
		{name: "too many sellers", query: manySellers(commodityquery.MaxSellers + 1), param: "seller_id"},
		{name: "trashed that is neither", query: "trashed=deleted", param: "trashed"},
		{name: "lent_out in the trash", query: "trashed=true&lent_out=false", param: "lent_out"},
		{name: "no_photo that is neither", query: "no_photo=maybe", param: "no_photo"},
		{name: "no_purchase_price that is neither", query: "no_purchase_price=unknown", param: "no_purchase_price"},
		{name: "no_purchase_date given twice", query: "no_purchase_date=1&no_purchase_date=0", param: "no_purchase_date"},
		{name: "no_photo in the trash", query: "trashed=true&no_photo=true", param: "no_photo"},
		{name: "a custom field condition without a comparison", query: "cf=f-1", param: "cf"},
		{name: "a custom field condition without a field", query: "cf=:set", param: "cf"},
		{name: "an unknown custom field comparison", query: "cf=f-1:like:x", param: "cf"},
		{name: "set with a value", query: "cf=f-1:set:x", param: "cf"},
		{name: "eq without a value", query: "cf=f-1:eq", param: "cf"},
		{name: "a range over a word", query: "cf=f-1:gte:ten", param: "cf"},
		{name: "a range over year zero", query: "cf=f-1:gte:0000-01-01", param: "cf"},
		{name: "a custom field value with a NUL", query: "cf=f-1:contains:a%00b", param: "cf"},
		{name: "a custom field value over the length limit", query: "cf=f-1:eq:" + strings.Repeat("a", commodityquery.MaxTextRunes+1), param: "cf"},
		{name: "too many custom field conditions", query: manyConditions(commodityquery.MaxCustomFieldFilters + 1), param: "cf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			values, err := url.ParseQuery(tc.query)
			c.Assert(err, qt.IsNil)

			_, err = commodityquery.Parse(values)
			var qErr *commodityquery.Error
			c.Assert(err, qt.ErrorAs, &qErr)
			c.Check(qErr.Param, qt.Equals, tc.param)
			c.Check(qErr.Error(), qt.Contains, tc.param)
		})
	}
}

func TestParse_TagLimitStopsEarly(t *testing.T) {
	c := qt.New(t)
	// Far more distinct tags than allowed: refused, whatever comes after the
	// limit, including an entry that would be an error of its own.
	values := url.Values{"tags": {manyTags(commodityquery.MaxTags+1) + ",###"}}
	_, err := commodityquery.Parse(values)
	var qErr *commodityquery.Error
	c.Assert(err, qt.ErrorAs, &qErr)
	c.Check(qErr.Param, qt.Equals, "tags")
	c.Check(qErr.Reason, qt.Contains, "at most")

	// Duplicates do not count towards the limit.
	dup := strings.TrimSuffix(strings.Repeat("a,", commodityquery.MaxTags*3), ",")
	q, err := commodityquery.Parse(url.Values{"tags": {dup + "," + manyTags(commodityquery.MaxTags-1)}})
	c.Assert(err, qt.IsNil)
	c.Check(q.Tags.Include, qt.HasLen, commodityquery.MaxTags)
}

func TestDecodeQuery(t *testing.T) {
	c := qt.New(t)

	values, err := commodityquery.DecodeQuery("q=power+tools&status=sold&status=lost&tags=a%2Cb")
	c.Assert(err, qt.IsNil)
	c.Check(values, qt.DeepEquals, url.Values{
		"q": {"power tools"}, "status": {"sold", "lost"}, "tags": {"a,b"},
	})

	for _, tc := range []struct {
		name, raw, param string
	}{
		{name: "a malformed escape in a value", raw: "q=drill&status=%ZZ", param: "status"},
		{name: "a malformed escape in a name", raw: "stat%ZZus=sold", param: "stat%ZZus"},
		{name: "a semicolon separator", raw: "status=sold;lent_out=true", param: "status"},
	} {
		c.Run(tc.name, func(c *qt.C) {
			_, err := commodityquery.DecodeQuery(tc.raw)
			var qErr *commodityquery.Error
			c.Assert(err, qt.ErrorAs, &qErr)
			c.Check(qErr.Param, qt.Equals, tc.param)
		})
	}
}

func manyTags(n int) string {
	tags := make([]string, n)
	for i := range tags {
		tags[i] = "t" + strings.Repeat("x", i)
	}
	return strings.Join(tags, ",")
}

func manySellers(n int) string {
	params := make([]string, n)
	for i := range params {
		params[i] = fmt.Sprintf("seller_id=s%d", i)
	}
	return strings.Join(params, "&")
}

func TestParse_Lenient(t *testing.T) {
	c := qt.New(t)

	// An item type nobody uses is applied, not refused: it matches nothing.
	q, err := commodityquery.Parse(url.Values{"type": {"spaceship"}})
	c.Assert(err, qt.IsNil)
	c.Check(q.Types, qt.DeepEquals, []models.CommodityType{"spaceship"})

	// Sorting is not filtering: an unknown field is passed on and the
	// registry falls back to name.
	q, err = commodityquery.Parse(url.Values{"sort": {"-colour"}})
	c.Assert(err, qt.IsNil)
	c.Check(q.SortField, qt.Equals, registry.CommoditySortField("colour"))
	c.Check(q.SortDesc, qt.IsTrue)

	// tag_mode without tags has nothing to combine and is dropped from
	// the query, so it does not survive a round trip either.
	q, err = commodityquery.Parse(url.Values{"tag_mode": {"any"}})
	c.Assert(err, qt.IsNil)
	c.Check(q.Tags.IsZero(), qt.IsTrue)
	c.Check(q.Values().Has("tag_mode"), qt.IsFalse)
}

func TestParse_Reserved(t *testing.T) {
	c := qt.New(t)
	// On /search, `type` names the entity type and page parameters are the
	// endpoint's own: Parse leaves them alone.
	values := url.Values{"type": {"commodities"}, "limit": {"5"}, "offset": {"10"}, "q": {"drill"}}
	q, err := commodityquery.Parse(values, "type", "limit", "offset")
	c.Assert(err, qt.IsNil)
	c.Check(q.Types, qt.HasLen, 0)
	c.Check(q.Text, qt.Equals, "drill")

	_, err = commodityquery.Parse(values)
	c.Assert(err, qt.IsNotNil)
}

func TestQuery_ValuesRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		q    commodityquery.Query
	}{
		{name: "default", q: commodityquery.Default()},
		{name: "everything", q: commodityquery.Query{
			Text:                  `"power tools" red`,
			Types:                 []models.CommodityType{"tools", "equipment"},
			Statuses:              []models.CommodityStatus{"sold"},
			AreaID:                "area-1",
			Unassigned:            true,
			SellerIDs:             []string{"s-2", "s-1"},
			IncludeInactive:       false,
			WarrantyStatuses:      []registry.WarrantyStatusFilter{"expiring"},
			WarrantyExpiresBefore: "2027-01-01",
			LentOut:               new(false),
			Tags: registry.CommodityTagFilter{
				Include: []string{"b", "a"}, Mode: registry.TagMatchAny, Exclude: []string{"c"},
			},
			CustomFields: []commodityquery.CustomFieldCondition{
				{FieldID: "f-2", Op: registry.CustomFieldOpIn, Operand: "a,b"},
				{FieldID: "f-1", Op: registry.CustomFieldOpUnset},
			},
			SortField: registry.CommoditySortCurrentPrice,
		}},
		{name: "custom field sort", q: commodityquery.Query{IncludeInactive: true, SortField: "cf.f-1", SortDesc: true}},
		{name: "untagged", q: commodityquery.Query{IncludeInactive: true, Tags: registry.CommodityTagFilter{Untagged: true}}},
		{name: "trashed", q: commodityquery.Query{Text: "drill", IncludeInactive: true, Trashed: true}},
		{name: "missing data", q: commodityquery.Query{NoPhoto: true, NoPurchasePrice: true, NoPurchaseDate: true}},
		{name: "unknown purchase data in the trash", q: commodityquery.Query{
			IncludeInactive: true, Trashed: true, NoPurchasePrice: true, NoPurchaseDate: true,
		}},
		{name: "all is the default mode", q: commodityquery.Query{
			IncludeInactive: true, Tags: registry.CommodityTagFilter{Include: []string{"a"}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			encoded := tc.q.Values().Encode()
			values, err := url.ParseQuery(encoded)
			c.Assert(err, qt.IsNil)
			got, err := commodityquery.Parse(values)
			c.Assert(err, qt.IsNil)
			c.Check(got, qt.DeepEquals, tc.q, qt.Commentf("encoded as %s", encoded))
		})
	}
}

func TestQuery_ListOptions(t *testing.T) {
	c := qt.New(t)
	q := commodityquery.Query{
		Text:            "drill",
		AreaID:          "a",
		SellerIDs:       []string{"s"},
		IncludeInactive: true,
		Tags:            registry.CommodityTagFilter{Exclude: []string{"x"}},
		SortField:       registry.CommoditySortName,
		SortDesc:        true,
	}
	opts := q.ListOptions()
	c.Check(opts.Search, qt.Equals, "drill")
	c.Check(opts.AreaID, qt.Equals, "a")
	c.Check(opts.SellerIDs, qt.DeepEquals, []string{"s"})
	c.Check(opts.IncludeInactive, qt.IsTrue)
	c.Check(opts.Tags, qt.DeepEquals, q.Tags)
	c.Check(opts.SortDesc, qt.IsTrue)
	c.Check(opts.SearchTagLabels, qt.IsNil)
	c.Check(opts.Trashed, qt.IsFalse)

	missing := commodityquery.Query{NoPhoto: true, NoPurchasePrice: true, NoPurchaseDate: true}.ListOptions()
	c.Check(missing.NoPhoto, qt.IsTrue)
	c.Check(missing.NoPurchasePrice, qt.IsTrue)
	c.Check(missing.NoPurchaseDate, qt.IsTrue)
	// Resolve fills the photo IDs; ListOptions leaves them to it.
	c.Check(missing.PhotoCommodityIDs, qt.IsNil)
}

func TestParse_Trashed(t *testing.T) {
	c := qt.New(t)
	q, err := commodityquery.Parse(url.Values{"trashed": {"TRUE"}, "q": {"drill"}})
	c.Assert(err, qt.IsNil)
	c.Check(q.Trashed, qt.IsTrue)
	c.Check(q.ListOptions().Trashed, qt.IsTrue)

	// trashed=false is the default and does not conflict with lent_out.
	q, err = commodityquery.Parse(url.Values{"trashed": {"false"}, "lent_out": {"true"}})
	c.Assert(err, qt.IsNil)
	c.Check(q.Trashed, qt.IsFalse)
	c.Check(q.LentOut, qt.DeepEquals, new(true))
}

func manyConditions(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("cf=f-%d:set", i)
	}
	return strings.Join(parts, "&")
}
