package commodityquery_test

import (
	"errors"
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services/commodityquery"
)

func TestParseSaved_AcceptsTheListFilters(t *testing.T) {
	c := qt.New(t)
	raw := "q=drill&type=tools&status=in_use&area_id=area-1&seller_id=s1&seller_id=s2&include_inactive=false" +
		"&warranty_status=expiring&lent_out=1&tags=Fragile,red&tag_mode=any&tags_exclude=garage&sort=-purchase_date"
	q, err := commodityquery.ParseSaved(raw)
	c.Assert(err, qt.IsNil)
	c.Assert(q.Text, qt.Equals, "drill")
	c.Assert(q.Tags.Include, qt.DeepEquals, []string{"fragile", "red"})
	c.Assert(*q.LentOut, qt.IsTrue)

	// The canonical form sorts the parameters and normalizes the values, so
	// it reads back to the same query.
	encoded := q.Encode()
	c.Assert(encoded, qt.Equals, "area_id=area-1&include_inactive=false&lent_out=true&q=drill&seller_id=s1&seller_id=s2"+
		"&sort=-purchase_date&status=in_use&tag_mode=any&tags=fragile%2Cred&tags_exclude=garage&type=tools"+
		"&warranty_status=expiring")
	again, err := commodityquery.ParseSaved(encoded)
	c.Assert(err, qt.IsNil)
	c.Assert(again, qt.DeepEquals, q)
}

// Two queries that select the same items encode to the same string, however
// their list values were ordered.
func TestQuery_EncodeSortsListValues(t *testing.T) {
	c := qt.New(t)
	a, err := commodityquery.ParseSaved("type=tools&type=other&status=sold&status=in_use&seller_id=s2&seller_id=s1" +
		"&warranty_status=expiring&warranty_status=active&tags=red,fragile&tags_exclude=shed,garage")
	c.Assert(err, qt.IsNil)
	b, err := commodityquery.ParseSaved("type=other&type=tools&status=in_use&status=sold&seller_id=s1&seller_id=s2" +
		"&warranty_status=active&warranty_status=expiring&tags=fragile,red&tags_exclude=garage,shed")
	c.Assert(err, qt.IsNil)
	c.Assert(a.Encode(), qt.Equals, b.Encode())
	c.Assert(a.Encode(), qt.Equals, "seller_id=s1&seller_id=s2&status=in_use&status=sold&tags=fragile%2Cred"+
		"&tags_exclude=garage%2Cshed&type=other&type=tools&warranty_status=active&warranty_status=expiring")
	// Encoding leaves the query as it was.
	c.Assert(a.Tags.Include, qt.DeepEquals, []string{"red", "fragile"})
}

// A saved view keeps custom field conditions and a custom field sort. The
// conditions are a set, and so are the options of an "in" condition, so
// their order does not change the canonical form.
func TestParseSaved_KeepsCustomFieldConditions(t *testing.T) {
	c := qt.New(t)
	a, err := commodityquery.ParseSaved("cf=f2:in:o2,o1,o2&cf=f1:gte:10&cf=f3:set&sort=-cf.f1")
	c.Assert(err, qt.IsNil)
	b, err := commodityquery.ParseSaved("sort=-cf.f1&cf=f3:set&cf=f1:gte:10&cf=f2:in:o1,o2")
	c.Assert(err, qt.IsNil)
	c.Assert(a.Encode(), qt.Equals, b.Encode())
	c.Assert(a.Encode(), qt.Equals, "cf=f1%3Agte%3A10&cf=f2%3Ain%3Ao1%2Co2&cf=f3%3Aset&sort=-cf.f1")
	again, err := commodityquery.ParseSaved(a.Encode())
	c.Assert(err, qt.IsNil)
	c.Assert(again.Encode(), qt.Equals, a.Encode())
}

func TestParseSaved_EmptyIsTheDefaultQuery(t *testing.T) {
	c := qt.New(t)
	q, err := commodityquery.ParseSaved("")
	c.Assert(err, qt.IsNil)
	c.Assert(q, qt.DeepEquals, commodityquery.Default())
	c.Assert(q.Encode(), qt.Equals, "")
}

func TestParseSaved_Refuses(t *testing.T) {
	cases := []struct {
		name, raw, param string
	}{
		{"an absolute warranty date", "warranty_expires_before=2026-12-31", "warranty_expires_before"},
		{"items that are not lent out", "lent_out=false", "lent_out"},
		{"the items in Trash", "trashed=true", "trashed"},
		{"a parameter of the page", "page=2", "page"},
		{"an unknown parameter", "colour=red", "colour"},
		{"an unknown status", "status=borrowed", "status"},
		{"a contradicting tag filter", "tags=a&tags_exclude=a", "tags_exclude"},
		{"a malformed escape", "q=%zz", "q"},
		{"a custom field condition without a comparison", "cf=f1", "cf"},
		{"an in condition without options", "cf=f1:in:,", "cf"},
		{"a custom field value with a NUL", "cf=f1:contains:a%00b", "cf"},
		{"a range over year zero", "cf=f1:lte:0000-12-31", "cf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			_, err := commodityquery.ParseSaved(tc.raw)
			c.Assert(err, qt.IsNotNil)
			qErr, ok := errors.AsType[*commodityquery.Error](err)
			c.Assert(ok, qt.IsTrue)
			c.Assert(qErr.Param, qt.Equals, tc.param)
		})
	}
}

func TestSavedParams_AreFilterParams(t *testing.T) {
	c := qt.New(t)
	filters := commodityquery.FilterParams()
	for _, p := range commodityquery.SavedParams() {
		c.Assert(filters, qt.Contains, p)
	}
}

// Every filter the list understands is either kept by a saved view or left
// out on purpose. A new filter fails here until someone decides which: added
// to savedParams (and to the saved view code of the frontend, which maps the
// API name to the page URL and names the condition), or listed below with the
// reason.
func TestFilterParams_AreSavedOrLeftOutOnPurpose(t *testing.T) {
	c := qt.New(t)
	leftOut := map[string]string{
		commodityquery.ParamWarrantyExpiresBefore: "an absolute date; a view uses warranty_status, computed on every run",
		commodityquery.ParamTrashed:               "Trash is its own page, not a list a view opens",
	}
	saved := commodityquery.SavedParams()
	for _, p := range commodityquery.FilterParams() {
		_, out := leftOut[p]
		c.Check(slices.Contains(saved, p) || out, qt.IsTrue,
			qt.Commentf("filter parameter %q is neither saved nor left out on purpose", p))
		c.Check(slices.Contains(saved, p) && out, qt.IsFalse,
			qt.Commentf("filter parameter %q is both saved and left out", p))
	}
}

func TestQuery_RenameTag(t *testing.T) {
	parse := func(c *qt.C, raw string) commodityquery.Query {
		q, err := commodityquery.ParseSaved(raw)
		c.Assert(err, qt.IsNil)
		return q
	}
	cases := []struct {
		name, raw, from, to, want string
		changed                   bool
	}{
		{"included tag", "tags=fragile,red", "fragile", "delicate", "tags=delicate%2Cred", true},
		{"excluded tag", "tags_exclude=fragile", "fragile", "delicate", "tags_exclude=delicate", true},
		{"merges into a tag already named", "tags=fragile,delicate", "fragile", "delicate", "tags=delicate", true},
		{"not named", "tags=red", "fragile", "delicate", "tags=red", false},
		{"no tag filter", "status=in_use", "fragile", "delicate", "status=in_use", false},
		// delicate below names a deleted tag, which is on no item.
		{
			"an excluded deleted tag excluded nothing", "tags=fragile&tags_exclude=delicate,red", "fragile", "delicate",
			"tags=delicate&tags_exclude=red", true,
		},
		{
			"an included deleted tag added nothing under any", "tag_mode=any&tags=delicate,red&tags_exclude=fragile", "fragile", "delicate",
			"tag_mode=any&tags=red&tags_exclude=delicate", true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got, changed := parse(c, tc.raw).RenameTag(tc.from, tc.to)
			c.Assert(changed, qt.Equals, tc.changed)
			c.Assert(got.Encode(), qt.Equals, tc.want)
			_, err := commodityquery.ParseSaved(got.Encode())
			c.Assert(err, qt.IsNil)
		})
	}

	// A view that required a deleted tag selected nothing, and no query the
	// list accepts selects nothing: the rewrite stores one Parse refuses,
	// so the view shows as broken rather than selecting the renamed tag's
	// items, which it excluded.
	for _, raw := range []string{
		"tags=delicate&tags_exclude=fragile",
		"tags=delicate,red&tags_exclude=fragile",
		"tag_mode=any&tags=delicate&tags_exclude=fragile",
	} {
		t.Run("an included deleted tag selected nothing: "+raw, func(t *testing.T) {
			c := qt.New(t)
			got, changed := parse(c, raw).RenameTag("fragile", "delicate")
			c.Assert(changed, qt.IsTrue)
			c.Assert(got.Tags.Include, qt.Contains, "delicate")
			c.Assert(got.Tags.Exclude, qt.DeepEquals, []string{"delicate"})
			_, err := commodityquery.ParseSaved(got.Encode())
			c.Assert(err, qt.ErrorMatches, `.*both required and excluded.*`)
		})
	}
}
