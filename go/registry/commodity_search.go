package registry

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.5x5.cz/inventario/models"
)

// This file holds the matching rules of the item search and the tag filter,
// the half of the item query contract that the registries implement. Both
// CommodityRegistry backends must return exactly the rows these rules
// describe: the memory backend calls the functions below directly, and the
// postgres backend states the same rules in SQL (see commodityWhere). The
// request half — parsing, validation, URL encoding and the resolution of tag
// labels — lives in services/commodityquery. devdocs/item-query-contract.md
// describes both.

// CommoditySearchField names a part of a commodity the free-text search
// reads. The values are part of the API: a search result reports the fields
// that matched with them.
type CommoditySearchField string

const (
	CommoditySearchFieldAssetID            CommoditySearchField = "asset_id"
	CommoditySearchFieldName               CommoditySearchField = "name"
	CommoditySearchFieldShortName          CommoditySearchField = "short_name"
	CommoditySearchFieldSerialNumber       CommoditySearchField = "serial_number"
	CommoditySearchFieldExtraSerialNumbers CommoditySearchField = "extra_serial_numbers"
	CommoditySearchFieldPartNumbers        CommoditySearchField = "part_numbers"
	CommoditySearchFieldTags               CommoditySearchField = "tags"
	CommoditySearchFieldComments           CommoditySearchField = "comments"
	CommoditySearchFieldWarrantyNotes      CommoditySearchField = "warranty_notes"
	CommoditySearchFieldStatusNote         CommoditySearchField = "status_note"
)

// CommoditySearchFields lists the text fields a search term is matched
// against, in the order the generated commodities.search_text column joins
// them. The asset ID is matched separately, by number.
var CommoditySearchFields = []CommoditySearchField{
	CommoditySearchFieldName,
	CommoditySearchFieldShortName,
	CommoditySearchFieldSerialNumber,
	CommoditySearchFieldExtraSerialNumbers,
	CommoditySearchFieldPartNumbers,
	CommoditySearchFieldTags,
	CommoditySearchFieldComments,
	CommoditySearchFieldWarrantyNotes,
	CommoditySearchFieldStatusNote,
}

// CommoditySearchMatch is one reason a commodity matched a search: the field
// and the value in it that a term was found in. For a JSONB list (serial
// numbers, part numbers, tags) Value is the matching element, a tag by its
// slug; for the long notes it is an excerpt around the first match.
type CommoditySearchMatch struct {
	Field CommoditySearchField `json:"field"`
	Value string               `json:"value"`
}

// SplitSearchTerms splits a free-text query into the terms that must all
// match. Terms are separated by white space; a double-quoted run is one term
// with its spaces kept (an unterminated quote runs to the end). Control
// characters count as white space, so no term ever contains one: the
// postgres backend relies on that to keep a term from matching across two
// fields of search_text. Terms that differ only in case are collapsed to the
// first.
func SplitSearchTerms(q string) []string {
	var terms []string
	seen := make(map[string]struct{})
	add := func(term string) {
		term = strings.TrimSpace(term)
		if term == "" {
			return
		}
		key := strings.ToLower(term)
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		terms = append(terms, term)
	}

	var cur strings.Builder
	quoted := false
	for _, r := range q {
		switch {
		case r == '"':
			add(cur.String())
			cur.Reset()
			quoted = !quoted
		case unicode.IsControl(r):
			add(cur.String())
			cur.Reset()
		case unicode.IsSpace(r) && !quoted:
			add(cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	add(cur.String())
	return terms
}

// containsFold reports whether s contains term, ignoring case. term is
// matched literally: unlike a LIKE pattern, % and _ have no meaning.
func containsFold(s, term string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(term))
}

// MatchCommoditySearch applies the free-text search to c. Every term must
// match at least one of these, or the commodity does not match:
//
//   - a text field in CommoditySearchFields, as a case-insensitive literal
//     substring (a tag matches by its slug);
//   - a tag of c whose slug is listed for the term in tagLabelSlugs, that is
//     a commodity tag whose label contains the term (see
//     CommodityListOptions.SearchTagLabels);
//   - the asset number, when the term reads as an asset ID
//     (models.ParseAssetID: "A-0042", "42").
//
// It returns every field a term was found in (asset ID first, then the
// order of CommoditySearchFields, each value once) and whether all terms
// matched. No terms match everything and explain nothing.
func MatchCommoditySearch(c *models.Commodity, terms []string, tagLabelSlugs map[string][]string) ([]CommoditySearchMatch, bool) {
	if len(terms) == 0 {
		return nil, true
	}
	var matches []CommoditySearchMatch
	seen := make(map[CommoditySearchMatch]struct{})
	record := func(m CommoditySearchMatch) {
		if _, dup := seen[m]; dup {
			return
		}
		seen[m] = struct{}{}
		matches = append(matches, m)
	}

	all := true
	for _, term := range terms {
		found := false
		for _, m := range matchTerm(c, term, tagLabelSlugs[term]) {
			found = true
			record(m)
		}
		if !found {
			all = false
		}
	}
	if !all {
		return nil, false
	}
	slices.SortStableFunc(matches, func(a, b CommoditySearchMatch) int {
		return searchFieldRank(a.Field) - searchFieldRank(b.Field)
	})
	return matches, true
}

// searchFieldRank orders matches: the asset ID first, then the text fields
// in CommoditySearchFields order.
func searchFieldRank(f CommoditySearchField) int {
	if f == CommoditySearchFieldAssetID {
		return -1
	}
	return slices.Index(CommoditySearchFields, f)
}

// matchTerm returns every field of c that term matches.
func matchTerm(c *models.Commodity, term string, labelSlugs []string) []CommoditySearchMatch {
	var out []CommoditySearchMatch
	if n, ok := models.ParseAssetID(term); ok && c.AssetNumber != nil && *c.AssetNumber == n {
		out = append(out, CommoditySearchMatch{Field: CommoditySearchFieldAssetID, Value: models.FormatAssetID(n)})
	}
	text := func(field CommoditySearchField, value string) {
		if containsFold(value, term) {
			out = append(out, CommoditySearchMatch{Field: field, Value: value})
		}
	}
	note := func(field CommoditySearchField, value string) {
		if containsFold(value, term) {
			out = append(out, CommoditySearchMatch{Field: field, Value: searchExcerpt(value, term)})
		}
	}
	list := func(field CommoditySearchField, values []string) {
		for _, v := range values {
			if containsFold(v, term) {
				out = append(out, CommoditySearchMatch{Field: field, Value: v})
			}
		}
	}

	text(CommoditySearchFieldName, c.Name)
	text(CommoditySearchFieldShortName, c.ShortName)
	text(CommoditySearchFieldSerialNumber, c.SerialNumber)
	list(CommoditySearchFieldExtraSerialNumbers, c.ExtraSerialNumbers)
	list(CommoditySearchFieldPartNumbers, c.PartNumbers)
	for _, slug := range c.Tags {
		if containsFold(slug, term) || slices.Contains(labelSlugs, slug) {
			out = append(out, CommoditySearchMatch{Field: CommoditySearchFieldTags, Value: slug})
		}
	}
	note(CommoditySearchFieldComments, c.Comments)
	note(CommoditySearchFieldWarrantyNotes, c.WarrantyNotes)
	note(CommoditySearchFieldStatusNote, c.StatusNote)
	return out
}

// searchExcerptRunes is the most a note excerpt shows on either side of the
// match.
const searchExcerptRunes = 40

// searchExcerpt cuts a window of the note around the first occurrence of
// term, with an ellipsis where text was dropped. A short note comes back
// whole.
func searchExcerpt(value, term string) string {
	runes := []rune(value)
	if len(runes) <= 2*searchExcerptRunes+utf8.RuneCountInString(term) {
		return value
	}
	lowered := []rune(strings.ToLower(value))
	needle := []rune(strings.ToLower(term))
	at := runeIndex(lowered, needle)
	if at < 0 || len(lowered) != len(runes) {
		// Lowercasing changed the length (a handful of scripts do), so the
		// offset does not map back. Fall back to the start of the note.
		at = 0
	}
	start := max(at-searchExcerptRunes, 0)
	end := min(at+len(needle)+searchExcerptRunes, len(runes))
	excerpt := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		excerpt = "…" + excerpt
	}
	if end < len(runes) {
		excerpt += "…"
	}
	return excerpt
}

func runeIndex(haystack, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if slices.Equal(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

// TagMatchMode says how CommodityTagFilter.Include combines its tags.
type TagMatchMode string

const (
	// TagMatchAll keeps items that carry every included tag. It is the
	// default: an empty mode means all.
	TagMatchAll TagMatchMode = "all"
	// TagMatchAny keeps items that carry at least one included tag.
	TagMatchAny TagMatchMode = "any"
)

// IsValid reports whether m is a known mode. The empty mode is not valid
// here; callers treat it as TagMatchAll.
func (m TagMatchMode) IsValid() bool {
	return m == TagMatchAll || m == TagMatchAny
}

// CommodityTagFilter narrows a list by the tags items carry, compared by slug
// in the commodity tag namespace. Its parts are AND-ed:
//
//   - Include with Mode all (the default): the item carries every slug;
//     with Mode any: at least one.
//   - Exclude: the item carries none of these slugs.
//   - Untagged: the item carries no tag at all.
//
// The zero value filters nothing. A slug no item carries is valid and simply
// matches nothing (in Include) or everything (in Exclude). Contradictory
// combinations, such as Untagged with Include, are refused by the request
// parser rather than here.
type CommodityTagFilter struct {
	Include  []string
	Mode     TagMatchMode
	Exclude  []string
	Untagged bool
}

// IsZero reports whether the filter narrows nothing.
func (f CommodityTagFilter) IsZero() bool {
	return len(f.Include) == 0 && len(f.Exclude) == 0 && !f.Untagged
}

// Matches reports whether an item carrying tags passes the filter. It is the
// reference statement of the rules the postgres backend writes in SQL.
func (f CommodityTagFilter) Matches(tags []string) bool {
	carries := func(s string) bool { return slices.Contains(tags, s) }
	switch {
	case f.Untagged && len(tags) > 0:
		return false
	case len(f.Include) == 0:
	case f.Mode == TagMatchAny && !slices.ContainsFunc(f.Include, carries):
		return false
	case f.Mode != TagMatchAny && !allOf(f.Include, carries):
		return false
	}
	return !slices.ContainsFunc(f.Exclude, carries)
}

func allOf(values []string, pred func(string) bool) bool {
	for _, v := range values {
		if !pred(v) {
			return false
		}
	}
	return true
}

// CommodityTagCounts is how many of a set of commodities carry each tag.
type CommodityTagCounts struct {
	// Tags maps a tag slug to the number of commodities carrying it. A slug
	// no commodity in the set carries is absent.
	Tags map[string]int `json:"tags"`
	// Untagged counts the commodities without tags (a NULL tags column
	// included).
	Untagged int `json:"untagged"`
}

// CountCommodityTags counts the tags of commodities, the way
// CommodityRegistry.CountTags does over the commodities a query selects.
// A slug repeated on one commodity counts once.
func CountCommodityTags(commodities []*models.Commodity) CommodityTagCounts {
	counts := CommodityTagCounts{Tags: map[string]int{}}
	for _, c := range commodities {
		if c == nil {
			continue
		}
		if len(c.Tags) == 0 {
			counts.Untagged++
			continue
		}
		seen := make(map[string]struct{}, len(c.Tags))
		for _, slug := range c.Tags {
			if _, dup := seen[slug]; dup {
				continue
			}
			seen[slug] = struct{}{}
			counts.Tags[slug]++
		}
	}
	return counts
}
