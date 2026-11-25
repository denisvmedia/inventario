package commodityquery_test

import (
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services/commodityquery"
)

func TestParseSaved_AcceptsTheListFilters(t *testing.T) {
	c := qt.New(t)
	raw := "q=drill&type=tools&status=in_use&area_id=area-1&include_inactive=false" +
		"&warranty_status=expiring&lent_out=1&tags=Fragile,red&tag_mode=any&tags_exclude=garage&sort=-purchase_date"
	q, err := commodityquery.ParseSaved(raw)
	c.Assert(err, qt.IsNil)
	c.Assert(q.Text, qt.Equals, "drill")
	c.Assert(q.Tags.Include, qt.DeepEquals, []string{"fragile", "red"})
	c.Assert(*q.LentOut, qt.IsTrue)

	// The canonical form sorts the parameters and normalizes the values, so
	// it reads back to the same query.
	encoded := q.Encode()
	c.Assert(encoded, qt.Equals, "area_id=area-1&include_inactive=false&lent_out=true&q=drill&sort=-purchase_date"+
		"&status=in_use&tag_mode=any&tags=fragile%2Cred&tags_exclude=garage&type=tools&warranty_status=expiring")
	again, err := commodityquery.ParseSaved(encoded)
	c.Assert(err, qt.IsNil)
	c.Assert(again, qt.DeepEquals, q)
}

// Two queries that select the same items encode to the same string, however
// their list values were ordered.
func TestQuery_EncodeSortsListValues(t *testing.T) {
	c := qt.New(t)
	a, err := commodityquery.ParseSaved("type=tools&type=other&status=sold&status=in_use" +
		"&warranty_status=expiring&warranty_status=active&tags=red,fragile&tags_exclude=shed,garage")
	c.Assert(err, qt.IsNil)
	b, err := commodityquery.ParseSaved("type=other&type=tools&status=in_use&status=sold" +
		"&warranty_status=active&warranty_status=expiring&tags=fragile,red&tags_exclude=garage,shed")
	c.Assert(err, qt.IsNil)
	c.Assert(a.Encode(), qt.Equals, b.Encode())
	c.Assert(a.Encode(), qt.Equals, "status=in_use&status=sold&tags=fragile%2Cred&tags_exclude=garage%2Cshed"+
		"&type=other&type=tools&warranty_status=active&warranty_status=expiring")
	// Encoding leaves the query as it was.
	c.Assert(a.Tags.Include, qt.DeepEquals, []string{"red", "fragile"})
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
		{"would contradict", "tags=fragile&tags_exclude=delicate", "fragile", "delicate", "tags=fragile&tags_exclude=delicate", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got, changed := parse(c, tc.raw).RenameTag(tc.from, tc.to)
			c.Assert(changed, qt.Equals, tc.changed)
			c.Assert(got.Encode(), qt.Equals, tc.want)
		})
	}
}
