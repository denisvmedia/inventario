package commodityquery

import (
	"slices"
	"strings"

	"go.5x5.cz/inventario/registry"
)

// savedParams lists the parameters a saved view may keep: the filters and the
// sort the items list shows and edits. A saved view opens as that list, so a
// condition the list cannot show would either be dropped on the way, which
// broadens the view, or stay invisible to the person looking at it.
var savedParams = []string{
	ParamText, ParamType, ParamStatus, ParamAreaID, ParamLocationID, ParamUnassigned,
	ParamSellerID, ParamIncludeInactive, ParamWarrantyStatus, ParamLentOut,
	ParamTags, ParamTagMode, ParamTagsExclude, ParamNoTags,
	ParamNoPhoto, ParamNoPurchasePrice, ParamNoPurchaseDate, ParamCustomField,
	ParamSort,
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

// Encode returns the canonical query string of q: Values with every list
// sorted, encoded with sorted keys. The lists are sets (an item matches any
// of the types or sellers, carries all or any of the tags, meets every
// custom field condition), so two queries that select the same items with
// the same sort encode the same, whatever order their parameters and list
// entries came in. The option IDs of a custom field "in" condition are
// sorted too.
func (q Query) Encode() string {
	canonical := q
	canonical.Types = slices.Sorted(slices.Values(q.Types))
	canonical.Statuses = slices.Sorted(slices.Values(q.Statuses))
	canonical.SellerIDs = slices.Sorted(slices.Values(q.SellerIDs))
	canonical.WarrantyStatuses = slices.Sorted(slices.Values(q.WarrantyStatuses))
	canonical.Tags.Include = slices.Sorted(slices.Values(q.Tags.Include))
	canonical.Tags.Exclude = slices.Sorted(slices.Values(q.Tags.Exclude))
	canonical.CustomFields = canonicalCustomFields(q.CustomFields)
	return canonical.Values().Encode()
}

// canonicalCustomFields returns conds sorted by their cf value, with the
// option IDs of an "in" condition sorted and without repeats, and without
// the conditions that repeat another once that is done.
func canonicalCustomFields(conds []CustomFieldCondition) []CustomFieldCondition {
	if len(conds) == 0 {
		return nil
	}
	out := make([]CustomFieldCondition, 0, len(conds))
	for _, cond := range conds {
		if cond.Op == registry.CustomFieldOpIn {
			var ids []string
			for id := range strings.SplitSeq(cond.Operand, ",") {
				if id = strings.TrimSpace(id); id != "" {
					ids = append(ids, id)
				}
			}
			slices.Sort(ids)
			cond.Operand = strings.Join(slices.Compact(ids), ",")
		}
		if !slices.Contains(out, cond) {
			out = append(out, cond)
		}
	}
	slices.SortFunc(out, func(a, b CustomFieldCondition) int { return strings.Compare(a.String(), b.String()) })
	return out
}

// RenameTag replaces the tag slug from with to wherever q names it and
// reports whether q changed. It keeps a renamed tag in the filters that name
// it.
//
// A rename can only take a slug no live tag holds, so a q that already names
// to names a deleted tag, and a deleted tag is on no item. When the rename
// puts to in both lists, the copy that was there before selected nothing and
// is dropped where that keeps what q selects: from the excluded list, or from
// the included list under tag_mode=any when other included tags remain.
// Otherwise q required a tag no item carried and selected nothing. No query
// Parse accepts selects nothing, so to stays in both lists: Parse refuses the
// result, and the view shows as broken instead of selecting items it did not
// select before.
func (q Query) RenameTag(from, to string) (Query, bool) {
	if from == "" || to == "" || from == to {
		return q, false
	}
	include, inChanged := renameSlug(q.Tags.Include, from, to)
	exclude, exChanged := renameSlug(q.Tags.Exclude, from, to)
	if !inChanged && !exChanged {
		return q, false
	}
	if slices.Contains(include, to) && slices.Contains(exclude, to) {
		isTo := func(s string) bool { return s == to }
		switch {
		case inChanged:
			exclude = slices.DeleteFunc(slices.Clone(exclude), isTo)
		case q.Tags.Mode == registry.TagMatchAny && len(include) > 1:
			include = slices.DeleteFunc(slices.Clone(include), isTo)
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
