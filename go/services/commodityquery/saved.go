package commodityquery

import (
	"slices"
)

// savedParams lists the parameters a saved view may keep: the filters and the
// sort the items list shows and edits. A saved view opens as that list, so a
// condition the list cannot show would either be dropped on the way, which
// broadens the view, or stay invisible to the person looking at it.
var savedParams = []string{
	ParamText, ParamType, ParamStatus, ParamAreaID, ParamUnassigned,
	ParamIncludeInactive, ParamWarrantyStatus, ParamLentOut, ParamTags,
	ParamTagMode, ParamTagsExclude, ParamNoTags, ParamSort,
}

// SavedParams returns the name of every parameter ParseSaved accepts.
func SavedParams() []string {
	return slices.Clone(savedParams)
}

// ParseSaved reads the query string of a saved view (without the leading
// "?"). It is Parse limited to SavedParams: warranty_expires_before is an
// absolute date, which a view that is meant to stay current must not keep
// (warranty_status is computed on every run instead), and lent_out=false has
// no control on the list. Both are an Error, as is anything Parse refuses.
func ParseSaved(rawQuery string) (Query, error) {
	values, err := DecodeQuery(rawQuery)
	if err != nil {
		return Query{}, err
	}
	for name := range values {
		if slices.Contains(filterParams, name) && !slices.Contains(savedParams, name) {
			return Query{}, paramError(name, "", "is not available in a saved view")
		}
	}
	q, err := Parse(values)
	if err != nil {
		return Query{}, err
	}
	if q.LentOut != nil && !*q.LentOut {
		return Query{}, paramError(ParamLentOut, "false", "only lent_out=true is available in a saved view")
	}
	return q, nil
}

// Encode returns the canonical query string of q: Values encoded with sorted
// keys. Two queries that select the same items with the same sort encode the
// same, whatever order their parameters came in.
func (q Query) Encode() string {
	return q.Values().Encode()
}

// RenameTag replaces the tag slug from with to wherever q names it and
// reports whether q changed. It keeps a renamed tag in the filters that name
// it. A rename that would put the same slug in both the included and the
// excluded list leaves q unchanged, since that query selects nothing and
// Parse refuses it.
func (q Query) RenameTag(from, to string) (Query, bool) {
	if from == "" || to == "" || from == to {
		return q, false
	}
	include, inChanged := renameSlug(q.Tags.Include, from, to)
	exclude, exChanged := renameSlug(q.Tags.Exclude, from, to)
	if !inChanged && !exChanged {
		return q, false
	}
	for _, s := range include {
		if slices.Contains(exclude, s) {
			return q, false
		}
	}
	q.Tags.Include = include
	q.Tags.Exclude = exclude
	return q, true
}

func renameSlug(slugs []string, from, to string) ([]string, bool) {
	i := slices.Index(slugs, from)
	if i < 0 {
		return slugs, false
	}
	out := make([]string, 0, len(slugs))
	for j, s := range slugs {
		if j == i {
			s = to
		}
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out, true
}
