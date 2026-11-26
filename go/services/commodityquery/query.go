// Package commodityquery is the request half of the item query contract: the
// one representation of an item filter that the items list
// (GET /commodities), the item search (GET /search?type=commodities) and every
// count built on them share, so the same query string selects the same items
// on each surface.
//
// Parse validates a query string into a Query and refuses what it does not
// support; Query.Values encodes it back, so a URL round-trips without losing
// a condition. Resolve and List run a Query against a registry set. The
// matching rules themselves (what a search term or a tag filter matches) are
// the registry half, in registry/commodity_search.go, and both registry
// backends implement them. devdocs/item-query-contract.md describes the
// whole contract.
package commodityquery

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// Query parameter names. They are the wire format of the contract: the
// frontend writes them, saved URLs keep them.
const (
	ParamText                  = "q"
	ParamType                  = "type"
	ParamStatus                = "status"
	ParamAreaID                = "area_id"
	ParamUnassigned            = "unassigned"
	ParamSellerID              = "seller_id"
	ParamIncludeInactive       = "include_inactive"
	ParamWarrantyStatus        = "warranty_status"
	ParamWarrantyExpiresBefore = "warranty_expires_before"
	ParamLentOut               = "lent_out"
	ParamTags                  = "tags"
	ParamTagMode               = "tag_mode"
	ParamTagsExclude           = "tags_exclude"
	ParamNoTags                = "no_tags"
	ParamNoPhoto               = "no_photo"
	ParamNoPurchasePrice       = "no_purchase_price"
	ParamNoPurchaseDate        = "no_purchase_date"
	ParamTrashed               = "trashed"
	ParamSort                  = "sort"
)

// Limits on a query. A request over one fails with an Error rather than
// being cut down silently.
const (
	MaxTextRunes = 200
	MaxTerms     = 10
	MaxTags      = 50
	MaxSellers   = 50
)

// Query is a validated item filter plus its sort. The zero value is not the
// default query: Default is, because IncludeInactive defaults to true.
type Query struct {
	// Text is the free-text search, trimmed. See registry.MatchCommoditySearch.
	Text string
	// Types and Statuses keep items whose type or status is in the list
	// (OR within the list). A status must be a known one; a type may be
	// any value, see parser.types.
	Types    []models.CommodityType
	Statuses []models.CommodityStatus
	// AreaID keeps items in one area; Unassigned keeps items with no area
	// and is ignored when AreaID is set.
	AreaID     string
	Unassigned bool
	// SellerIDs keeps items bought from one of the sellers (OR within the
	// list). An ID no seller has matches no item.
	SellerIDs []string
	// IncludeInactive, when false, hides drafts and, unless Statuses is
	// set, items that are not in use. Default true.
	IncludeInactive bool
	// WarrantyStatuses keeps items whose computed warranty status is in the
	// list; WarrantyExpiresBefore (YYYY-MM-DD) keeps items whose warranty
	// ends strictly before that day.
	WarrantyStatuses      []registry.WarrantyStatusFilter
	WarrantyExpiresBefore string
	// LentOut, when set, keeps items that are (true) or are not (false)
	// on an open loan.
	LentOut *bool
	// Tags narrows by tag slugs; see registry.CommodityTagFilter.
	Tags registry.CommodityTagFilter
	// NoPhoto, NoPurchasePrice and NoPurchaseDate keep the items that lack
	// a photo, a purchase price or a purchase date. An unknown value is
	// missing whether or not the item was reviewed with it: the data keeps
	// no "unknown on purpose" mark (#2813). A known zero price is a price.
	// The photos of an item in Trash are in Trash too, so NoPhoto cannot be
	// combined with Trashed.
	NoPhoto         bool
	NoPurchasePrice bool
	NoPurchaseDate  bool
	// Trashed selects the items in Trash instead of the live ones. The
	// loans of a trashed item are hidden, so LentOut cannot be combined
	// with it.
	Trashed bool
	// SortField and SortDesc order the result. Sorting is not filtering:
	// an unknown field falls back to name, see registry.CommoditySortField.
	SortField registry.CommoditySortField
	SortDesc  bool
}

// Default returns the query of a request without parameters: every item,
// drafts and inactive ones included, sorted by name.
func Default() Query {
	return Query{IncludeInactive: true}
}

// Error reports a query parameter the contract refuses: an unknown name, a
// value that does not parse, or a combination that contradicts itself. The
// API answers it with 400 and the parameter name.
type Error struct {
	Param  string
	Value  string
	Reason string
}

func (e *Error) Error() string {
	if e.Value == "" {
		return fmt.Sprintf("query parameter %q: %s", e.Param, e.Reason)
	}
	return fmt.Sprintf("query parameter %q (%q): %s", e.Param, e.Value, e.Reason)
}

func paramError(param, value, reason string) *Error {
	return &Error{Param: param, Value: value, Reason: reason}
}

// filterParams lists every parameter Parse understands.
var filterParams = []string{
	ParamText, ParamType, ParamStatus, ParamAreaID, ParamUnassigned,
	ParamSellerID, ParamIncludeInactive, ParamWarrantyStatus, ParamWarrantyExpiresBefore,
	ParamLentOut, ParamTags, ParamTagMode, ParamTagsExclude, ParamNoTags,
	ParamNoPhoto, ParamNoPurchasePrice, ParamNoPurchaseDate,
	ParamTrashed, ParamSort,
}

// FilterParams returns the name of every parameter Parse understands.
func FilterParams() []string {
	return slices.Clone(filterParams)
}

// Parse reads a Query from a query string. reserved names the parameters the
// endpoint handles itself (pagination, the entity type of /search); Parse
// skips them, and a reserved name that is also a filter parameter is not
// available as a filter on that endpoint. Any other parameter Parse does not
// know is an Error, as is a value it cannot read: a filter is never dropped
// silently, because a list that ignores a condition looks like a list that
// applied it.
func Parse(values url.Values, reserved ...string) (Query, error) {
	q := Default()
	for name := range values {
		if slices.Contains(reserved, name) {
			continue
		}
		if !slices.Contains(filterParams, name) {
			return Query{}, paramError(name, "", "unsupported parameter")
		}
	}
	p := parser{values: values, reserved: reserved}

	steps := []func(*Query) error{
		p.text, p.types, p.statuses, p.area, p.sellers, p.flags, p.missing, p.warranty, p.tags, p.sort,
	}
	for _, step := range steps {
		if err := step(&q); err != nil {
			return Query{}, err
		}
	}
	return q, nil
}

// DecodeQuery decodes a raw query string for Parse. url.URL.Query drops a
// pair it cannot decode, which would run the query without that condition,
// so a pair with a malformed escape or a semicolon separator is an Error
// naming its parameter.
func DecodeQuery(rawQuery string) (url.Values, error) {
	values, err := url.ParseQuery(rawQuery)
	if err == nil {
		return values, nil
	}
	for pair := range strings.SplitSeq(rawQuery, "&") {
		rawKey, rawValue, _ := strings.Cut(pair, "=")
		if strings.Contains(pair, ";") {
			name, _, _ := strings.Cut(rawKey, ";")
			return nil, paramError(name, "", "contains a semicolon; separate parameters with &")
		}
		key, keyErr := url.QueryUnescape(rawKey)
		if keyErr != nil {
			return nil, paramError(rawKey, "", "is not a valid URL-encoded name")
		}
		if _, valueErr := url.QueryUnescape(rawValue); valueErr != nil {
			return nil, paramError(key, rawValue, "is not a valid URL-encoded value")
		}
	}
	// url.ParseQuery found an error the scan above does not know.
	return nil, paramError("", "", err.Error())
}

type parser struct {
	values   url.Values
	reserved []string
}

// list returns the non-empty, trimmed values of a repeatable parameter.
func (p parser) list(name string) []string {
	if slices.Contains(p.reserved, name) {
		return nil
	}
	var out []string
	for _, v := range p.values[name] {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// commaList is list for a parameter whose values may also be
// comma-separated, so tags=a,b and tags=a&tags=b read the same.
func (p parser) commaList(name string) []string {
	var out []string
	for _, raw := range p.list(name) {
		for v := range strings.SplitSeq(raw, ",") {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

// single returns the trimmed value of a parameter that may be given once.
func (p parser) single(name string) (string, error) {
	if slices.Contains(p.reserved, name) {
		return "", nil
	}
	vals := p.values[name]
	if len(vals) > 1 {
		return "", paramError(name, "", "must be given once")
	}
	if len(vals) == 0 {
		return "", nil
	}
	return strings.TrimSpace(vals[0]), nil
}

// flag reads a boolean parameter: 1/true or 0/false, any case. Absent or
// empty is nil.
func (p parser) flag(name string) (*bool, error) {
	v, err := p.single(name)
	if err != nil || v == "" {
		return nil, err
	}
	switch strings.ToLower(v) {
	case "1", "true":
		return new(true), nil
	case "0", "false":
		return new(false), nil
	}
	return nil, paramError(name, v, "must be true or false")
}

func (p parser) text(q *Query) error {
	v, err := p.single(ParamText)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(v) > MaxTextRunes {
		return paramError(ParamText, "", fmt.Sprintf("must be at most %d characters", MaxTextRunes))
	}
	if len(registry.SplitSearchTerms(v)) > MaxTerms {
		return paramError(ParamText, v, fmt.Sprintf("must have at most %d terms", MaxTerms))
	}
	q.Text = v
	return nil
}

// types accepts any value: an item type slug nobody uses matches no item,
// which is an applied condition rather than a dropped one. Item types are
// per group (#2802), so the parser, which knows no group, does not check
// the slugs against a list.
func (p parser) types(q *Query) error {
	for _, v := range p.list(ParamType) {
		t := models.CommodityType(v)
		if !slices.Contains(q.Types, t) {
			q.Types = append(q.Types, t)
		}
	}
	return nil
}

func (p parser) statuses(q *Query) error {
	for _, v := range p.list(ParamStatus) {
		s := models.CommodityStatus(v)
		if !s.IsValid() {
			return paramError(ParamStatus, v, "unknown status")
		}
		if !slices.Contains(q.Statuses, s) {
			q.Statuses = append(q.Statuses, s)
		}
	}
	return nil
}

func (p parser) area(q *Query) error {
	v, err := p.single(ParamAreaID)
	if err != nil {
		return err
	}
	q.AreaID = v
	unassigned, err := p.flag(ParamUnassigned)
	if err != nil {
		return err
	}
	q.Unassigned = unassigned != nil && *unassigned
	return nil
}

// sellers reads the seller filter. Like types, it does not check that a
// seller with the ID exists: an unknown ID matches no item, which is an
// applied condition.
func (p parser) sellers(q *Query) error {
	for _, v := range p.list(ParamSellerID) {
		if slices.Contains(q.SellerIDs, v) {
			continue
		}
		q.SellerIDs = append(q.SellerIDs, v)
		if len(q.SellerIDs) > MaxSellers {
			return paramError(ParamSellerID, "", fmt.Sprintf("must list at most %d sellers", MaxSellers))
		}
	}
	return nil
}

func (p parser) flags(q *Query) error {
	inactive, err := p.flag(ParamIncludeInactive)
	if err != nil {
		return err
	}
	if inactive != nil {
		q.IncludeInactive = *inactive
	}
	q.LentOut, err = p.flag(ParamLentOut)
	if err != nil {
		return err
	}
	trashed, err := p.flag(ParamTrashed)
	if err != nil {
		return err
	}
	q.Trashed = trashed != nil && *trashed
	if q.Trashed && q.LentOut != nil {
		return paramError(ParamLentOut, "", "cannot be combined with trashed=true: the loans of an item in Trash are hidden")
	}
	return nil
}

// missing reads the missing-data flags. It runs after flags, which reads
// trashed.
func (p parser) missing(q *Query) error {
	for _, f := range []struct {
		name string
		dst  *bool
	}{
		{ParamNoPhoto, &q.NoPhoto},
		{ParamNoPurchasePrice, &q.NoPurchasePrice},
		{ParamNoPurchaseDate, &q.NoPurchaseDate},
	} {
		v, err := p.flag(f.name)
		if err != nil {
			return err
		}
		*f.dst = v != nil && *v
	}
	if q.Trashed && q.NoPhoto {
		return paramError(ParamNoPhoto, "", "cannot be combined with trashed=true: the photos of an item in Trash are in Trash too")
	}
	return nil
}

func (p parser) warranty(q *Query) error {
	for _, v := range p.list(ParamWarrantyStatus) {
		s := registry.WarrantyStatusFilter(v)
		if !s.IsValid() {
			return paramError(ParamWarrantyStatus, v, "must be active, expiring, expired or none")
		}
		if !slices.Contains(q.WarrantyStatuses, s) {
			q.WarrantyStatuses = append(q.WarrantyStatuses, s)
		}
	}
	v, err := p.single(ParamWarrantyExpiresBefore)
	if err != nil {
		return err
	}
	if v != "" {
		if _, err := time.Parse(time.DateOnly, v); err != nil {
			return paramError(ParamWarrantyExpiresBefore, v, "must be a date as YYYY-MM-DD")
		}
	}
	q.WarrantyExpiresBefore = v
	return nil
}

// slugs reads a tag list (comma-separated, repeatable) and normalizes every
// entry the way a tag slug is normalized on write, so a hand-typed "Fragile"
// finds the tag "fragile".
func (p parser) slugs(name string) ([]string, error) {
	var out []string
	for _, v := range p.commaList(name) {
		slug := models.NormalizeTagSlug(v)
		if slug == "" {
			return nil, paramError(name, v, "is not a tag slug")
		}
		if slices.Contains(out, slug) {
			continue
		}
		out = append(out, slug)
		// Checked per new slug, so a request with thousands of tags is
		// refused after MaxTags+1 of them instead of after deduplicating all.
		if len(out) > MaxTags {
			return nil, paramError(name, "", fmt.Sprintf("must list at most %d tags", MaxTags))
		}
	}
	return out, nil
}

func (p parser) tags(q *Query) error {
	include, err := p.slugs(ParamTags)
	if err != nil {
		return err
	}
	exclude, err := p.slugs(ParamTagsExclude)
	if err != nil {
		return err
	}
	for _, s := range include {
		if slices.Contains(exclude, s) {
			return paramError(ParamTagsExclude, s, "a tag cannot be both required and excluded")
		}
	}
	mode, err := p.single(ParamTagMode)
	if err != nil {
		return err
	}
	if mode != "" && !registry.TagMatchMode(mode).IsValid() {
		return paramError(ParamTagMode, mode, "must be all or any")
	}
	untagged, err := p.flag(ParamNoTags)
	if err != nil {
		return err
	}
	q.Tags = registry.CommodityTagFilter{
		Include:  include,
		Exclude:  exclude,
		Untagged: untagged != nil && *untagged,
	}
	if len(include) > 0 && mode == string(registry.TagMatchAny) {
		q.Tags.Mode = registry.TagMatchAny
	}
	if q.Tags.Untagged && (len(include) > 0 || len(exclude) > 0) {
		return paramError(ParamNoTags, "", "cannot be combined with tags or tags_exclude")
	}
	return nil
}

func (p parser) sort(q *Query) error {
	v, err := p.single(ParamSort)
	if err != nil || v == "" {
		return err
	}
	q.SortDesc = strings.HasPrefix(v, "-")
	q.SortField = registry.CommoditySortField(strings.TrimPrefix(v, "-"))
	return nil
}

// Values encodes q as a query string that Parse reads back to the same Query.
// Defaults are left out, lists keep their order, and tags are comma-joined.
func (q Query) Values() url.Values {
	v := url.Values{}
	if q.Text != "" {
		v.Set(ParamText, q.Text)
	}
	for _, t := range q.Types {
		v.Add(ParamType, string(t))
	}
	for _, s := range q.Statuses {
		v.Add(ParamStatus, string(s))
	}
	if q.AreaID != "" {
		v.Set(ParamAreaID, q.AreaID)
	}
	for _, id := range q.SellerIDs {
		v.Add(ParamSellerID, id)
	}
	if !q.IncludeInactive {
		v.Set(ParamIncludeInactive, "false")
	}
	for _, s := range q.WarrantyStatuses {
		v.Add(ParamWarrantyStatus, string(s))
	}
	if q.WarrantyExpiresBefore != "" {
		v.Set(ParamWarrantyExpiresBefore, q.WarrantyExpiresBefore)
	}
	if q.LentOut != nil {
		v.Set(ParamLentOut, fmt.Sprint(*q.LentOut))
	}
	if len(q.Tags.Include) > 0 {
		v.Set(ParamTags, strings.Join(q.Tags.Include, ","))
		if q.Tags.Mode == registry.TagMatchAny {
			v.Set(ParamTagMode, string(registry.TagMatchAny))
		}
	}
	if len(q.Tags.Exclude) > 0 {
		v.Set(ParamTagsExclude, strings.Join(q.Tags.Exclude, ","))
	}
	for _, flag := range []struct {
		param string
		on    bool
	}{
		{ParamUnassigned, q.Unassigned},
		{ParamNoTags, q.Tags.Untagged},
		{ParamNoPhoto, q.NoPhoto},
		{ParamNoPurchasePrice, q.NoPurchasePrice},
		{ParamNoPurchaseDate, q.NoPurchaseDate},
		{ParamTrashed, q.Trashed},
	} {
		if flag.on {
			v.Set(flag.param, "true")
		}
	}
	if q.SortField != "" {
		sort := string(q.SortField)
		if q.SortDesc {
			sort = "-" + sort
		}
		v.Set(ParamSort, sort)
	}
	return v
}

// ListOptions maps q onto the registry options without the resolved parts
// (tag labels, open loans); Resolve adds those.
func (q Query) ListOptions() registry.CommodityListOptions {
	return registry.CommodityListOptions{
		Types:                 q.Types,
		Statuses:              q.Statuses,
		AreaID:                q.AreaID,
		Unassigned:            q.Unassigned,
		SellerIDs:             q.SellerIDs,
		Search:                q.Text,
		Tags:                  q.Tags,
		IncludeInactive:       q.IncludeInactive,
		SortField:             q.SortField,
		SortDesc:              q.SortDesc,
		WarrantyStatuses:      q.WarrantyStatuses,
		WarrantyExpiresBefore: q.WarrantyExpiresBefore,
		LentOut:               q.LentOut,
		NoPhoto:               q.NoPhoto,
		NoPurchasePrice:       q.NoPurchasePrice,
		NoPurchaseDate:        q.NoPurchaseDate,
		Trashed:               q.Trashed,
	}
}
