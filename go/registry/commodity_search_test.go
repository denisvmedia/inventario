package registry_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

func TestSplitSearchTerms(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "blank", in: "  \t ", want: nil},
		{name: "one term, trimmed", in: "  drill ", want: []string{"drill"}},
		{name: "white space separates", in: "red\tdrill  bosch", want: []string{"red", "drill", "bosch"}},
		{name: "quotes keep a phrase", in: `"power tools" red`, want: []string{"power tools", "red"}},
		{name: "a quote also ends a term", in: `a"b c"d`, want: []string{"a", "b c", "d"}},
		{name: "an unterminated quote runs to the end", in: `red "power tools`, want: []string{"red", "power tools"}},
		{name: "empty quotes give nothing", in: `"" "  "`, want: nil},
		{name: "duplicates collapse ignoring case", in: "Red red RED drill", want: []string{"Red", "drill"}},
		{name: "control characters separate, even inside quotes", in: "\"a\x1fb\" c\x00d", want: []string{"a", "b", "c", "d"}},
		{name: "non-ASCII is kept", in: "Küche  ドリル", want: []string{"Küche", "ドリル"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(registry.SplitSearchTerms(tc.in), qt.DeepEquals, tc.want)
		})
	}
}

func searchItem() *models.Commodity {
	return &models.Commodity{
		Name:               "Cordless Drill",
		ShortName:          "Drill",
		SerialNumber:       "SN-ABC-123",
		ExtraSerialNumbers: models.ValuerSlice[string]{"XS-1", "XS-2"},
		PartNumbers:        models.ValuerSlice[string]{"PN-77"},
		Tags:               models.ValuerSlice[string]{"power-tools", "red"},
		Comments:           "Bought at the hardware store",
		WarrantyNotes:      "Two years, receipt in the drawer",
		StatusNote:         "",
		AssetNumber:        new(42),
	}
}

func TestMatchCommoditySearch(t *testing.T) {
	m := func(f registry.CommoditySearchField, v string) registry.CommoditySearchMatch {
		return registry.CommoditySearchMatch{Field: f, Value: v}
	}
	for _, tc := range []struct {
		name   string
		terms  []string
		labels map[string][]string
		want   []registry.CommoditySearchMatch
		ok     bool
	}{
		{name: "no terms match everything and explain nothing", terms: nil, ok: true},
		{
			name: "serial number, any case", terms: []string{"abc"}, ok: true,
			want: []registry.CommoditySearchMatch{m(registry.CommoditySearchFieldSerialNumber, "SN-ABC-123")},
		},
		{
			name: "every matching list element is reported", terms: []string{"xs-"}, ok: true,
			want: []registry.CommoditySearchMatch{
				m(registry.CommoditySearchFieldExtraSerialNumbers, "XS-1"),
				m(registry.CommoditySearchFieldExtraSerialNumbers, "XS-2"),
			},
		},
		{
			name: "one term in several fields, reported in field order", terms: []string{"drill"}, ok: true,
			want: []registry.CommoditySearchMatch{
				m(registry.CommoditySearchFieldName, "Cordless Drill"),
				m(registry.CommoditySearchFieldShortName, "Drill"),
			},
		},
		{
			name: "a tag by slug", terms: []string{"power"}, ok: true,
			want: []registry.CommoditySearchMatch{m(registry.CommoditySearchFieldTags, "power-tools")},
		},
		{
			name: "a tag by a label resolved for the term", terms: []string{"crimson"},
			labels: map[string][]string{"crimson": {"red", "not-on-this-item"}}, ok: true,
			want: []registry.CommoditySearchMatch{m(registry.CommoditySearchFieldTags, "red")},
		},
		{
			name: "asset ID comes first", terms: []string{"A-0042", "drill"}, ok: true,
			want: []registry.CommoditySearchMatch{
				m(registry.CommoditySearchFieldAssetID, "A-0042"),
				m(registry.CommoditySearchFieldName, "Cordless Drill"),
				m(registry.CommoditySearchFieldShortName, "Drill"),
			},
		},
		{name: "every term has to match", terms: []string{"drill", "makita"}, ok: false},
		{name: "a wildcard is literal", terms: []string{"dr%l"}, ok: false},
		{name: "another asset number does not match", terms: []string{"A-0041"}, ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got, ok := registry.MatchCommoditySearch(searchItem(), tc.terms, tc.labels)
			c.Assert(ok, qt.Equals, tc.ok)
			c.Assert(got, qt.DeepEquals, tc.want)
		})
	}
}

func TestMatchCommoditySearch_NoteExcerpt(t *testing.T) {
	c := qt.New(t)
	note := strings.Repeat("lorem ipsum ", 10) + "the RECEIPT is in the blue folder " + strings.Repeat("dolor sit ", 10)
	item := &models.Commodity{Name: "Desk", Comments: note}

	got, ok := registry.MatchCommoditySearch(item, []string{"receipt"}, nil)
	c.Assert(ok, qt.IsTrue)
	c.Assert(got, qt.HasLen, 1)
	c.Check(got[0].Field, qt.Equals, registry.CommoditySearchFieldComments)
	c.Check(got[0].Value, qt.Contains, "RECEIPT is in the blue folder")
	c.Check(strings.HasPrefix(got[0].Value, "…"), qt.IsTrue)
	c.Check(strings.HasSuffix(got[0].Value, "…"), qt.IsTrue)
	c.Check(len([]rune(got[0].Value)) < len([]rune(note)), qt.IsTrue)
}

func TestCommodityTagFilter_Matches(t *testing.T) {
	tagged := []string{"red", "kitchen"}
	for _, tc := range []struct {
		name     string
		filter   registry.CommodityTagFilter
		tagged   bool
		untagged bool
	}{
		{name: "zero filter", filter: registry.CommodityTagFilter{}, tagged: true, untagged: true},
		{name: "all: every tag present", filter: registry.CommodityTagFilter{Include: []string{"red", "kitchen"}}, tagged: true},
		{name: "all: one tag missing", filter: registry.CommodityTagFilter{Include: []string{"red", "garage"}}},
		{
			name:   "any: one tag present",
			filter: registry.CommodityTagFilter{Include: []string{"red", "garage"}, Mode: registry.TagMatchAny}, tagged: true,
		},
		{name: "any: none present", filter: registry.CommodityTagFilter{Include: []string{"garage"}, Mode: registry.TagMatchAny}},
		{name: "exclude hits", filter: registry.CommodityTagFilter{Exclude: []string{"kitchen"}}, untagged: true},
		{name: "exclude misses", filter: registry.CommodityTagFilter{Exclude: []string{"garage"}}, tagged: true, untagged: true},
		{name: "untagged", filter: registry.CommodityTagFilter{Untagged: true}, untagged: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Check(tc.filter.Matches(tagged), qt.Equals, tc.tagged)
			c.Check(tc.filter.Matches(nil), qt.Equals, tc.untagged)
			c.Check(tc.filter.IsZero(), qt.Equals, tc.name == "zero filter")
		})
	}
}

// The memory backend matches the fields in CommoditySearchFields, postgres
// the generated search_text column. A field added to one and not the other
// makes the backends disagree, so the annotation is read back here.
func TestCommoditySearchFields_MatchTheGeneratedColumn(t *testing.T) {
	c := qt.New(t)
	src, err := os.ReadFile("../models/commodity.go")
	c.Assert(err, qt.IsNil)
	m := regexp.MustCompile(`generated="commodity_search_text\(([^)]*)\)"`).FindSubmatch(src)
	c.Assert(m, qt.HasLen, 2, qt.Commentf("search_text annotation not found"))

	var args []registry.CommoditySearchField
	for arg := range strings.SplitSeq(string(m[1]), ",") {
		args = append(args, registry.CommoditySearchField(strings.TrimSpace(arg)))
	}
	c.Assert(args, qt.DeepEquals, registry.CommoditySearchFields)
}
