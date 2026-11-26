package registry

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
)

// This file holds the custom field half of the item query contract (#2803):
// what a custom field filter keeps, how a sort by a custom field orders, and
// which custom field values the free-text search reads. The memory backend
// calls these functions; the postgres backend states the same rules in SQL
// against commodity_custom_field_values.

// CustomFieldFilterOp is the comparison of a custom field filter.
type CustomFieldFilterOp string

const (
	// CustomFieldOpSet keeps items that have a value for the field, of any
	// kind: zero, false and empty text are values.
	CustomFieldOpSet CustomFieldFilterOp = "set"
	// CustomFieldOpUnset keeps items without a value for the field.
	CustomFieldOpUnset CustomFieldFilterOp = "unset"
	// CustomFieldOpEq keeps items whose value equals the operand: text
	// without regard to case, a number or amount by value, a date, a yes/no
	// value, a select option, a linked item.
	CustomFieldOpEq CustomFieldFilterOp = "eq"
	// CustomFieldOpContains keeps items whose text contains the operand,
	// without regard to case.
	CustomFieldOpContains CustomFieldFilterOp = "contains"
	// CustomFieldOpGte and CustomFieldOpLte keep items whose number, amount
	// or date is at least or at most the operand. An amount is compared
	// without regard to its currency.
	CustomFieldOpGte CustomFieldFilterOp = "gte"
	CustomFieldOpLte CustomFieldFilterOp = "lte"
	// CustomFieldOpIn keeps items whose select value is one of the listed
	// options.
	CustomFieldOpIn CustomFieldFilterOp = "in"
)

// IsValid reports whether op is a known comparison.
func (op CustomFieldFilterOp) IsValid() bool {
	switch op {
	case CustomFieldOpSet, CustomFieldOpUnset, CustomFieldOpEq, CustomFieldOpContains,
		CustomFieldOpGte, CustomFieldOpLte, CustomFieldOpIn:
		return true
	}
	return false
}

// TakesOperand reports whether op compares against a value.
func (op CustomFieldFilterOp) TakesOperand() bool {
	return op != CustomFieldOpSet && op != CustomFieldOpUnset
}

// CustomFieldOps returns the comparisons a field of type t supports.
func CustomFieldOps(t models.CustomFieldType) []CustomFieldFilterOp {
	base := []CustomFieldFilterOp{CustomFieldOpSet, CustomFieldOpUnset}
	switch t {
	case models.CustomFieldTypeText, models.CustomFieldTypeLongText, models.CustomFieldTypeURL:
		return append(base, CustomFieldOpEq, CustomFieldOpContains)
	case models.CustomFieldTypeNumber, models.CustomFieldTypeMoney, models.CustomFieldTypeDate:
		return append(base, CustomFieldOpEq, CustomFieldOpGte, CustomFieldOpLte)
	case models.CustomFieldTypeBoolean, models.CustomFieldTypeItemLink:
		return append(base, CustomFieldOpEq)
	case models.CustomFieldTypeSelect:
		return append(base, CustomFieldOpEq, CustomFieldOpIn)
	}
	return base
}

// CustomFieldFilter keeps the items whose value for one custom field passes
// Op. services/commodityquery builds it from the cf parameter and fills in
// Type from the group's field.
type CustomFieldFilter struct {
	FieldID string
	// Type is the field's type, or empty when the group has no field with
	// FieldID. A filter on a field that does not exist keeps no item, so a
	// saved view naming a deleted field lists nothing instead of
	// everything.
	Type models.CustomFieldType
	Op   CustomFieldFilterOp
	// Text is the operand of a text, URL, date (YYYY-MM-DD) or item link
	// comparison.
	Text string
	// Number is the operand of a number or money comparison.
	Number decimal.Decimal
	// Bool is the operand of a yes/no comparison.
	Bool bool
	// OptionIDs are the options of a select comparison: one for eq, one or
	// more for in.
	OptionIDs []string
}

// Matches reports whether an item with values passes the filter. It is the
// reference statement of the rule the postgres backend writes in SQL.
func (f CustomFieldFilter) Matches(values models.CustomFieldValues) bool {
	if f.Type == "" {
		return false
	}
	raw, has := values[f.FieldID]
	if has && models.IsNullJSON(raw) {
		has = false
	}
	switch f.Op {
	case CustomFieldOpSet:
		return has
	case CustomFieldOpUnset:
		return !has
	}
	return has && f.matchesValue(raw)
}

// matchesValue compares a value against the operand of a comparison that
// takes one.
func (f CustomFieldFilter) matchesValue(raw json.RawMessage) bool {
	switch f.Type {
	case models.CustomFieldTypeText, models.CustomFieldTypeLongText, models.CustomFieldTypeURL:
		s, ok := rawString(raw)
		if !ok {
			return false
		}
		if f.Op == CustomFieldOpContains {
			return containsFold(s, f.Text)
		}
		return f.Op == CustomFieldOpEq && strings.EqualFold(s, f.Text)
	case models.CustomFieldTypeNumber, models.CustomFieldTypeMoney:
		n, ok := rawDecimal(f.Type, raw)
		return ok && compareOp(f.Op, n.Cmp(f.Number))
	case models.CustomFieldTypeDate:
		s, ok := rawString(raw)
		return ok && compareOp(f.Op, strings.Compare(s, f.Text))
	case models.CustomFieldTypeBoolean:
		var b bool
		return json.Unmarshal(raw, &b) == nil && f.Op == CustomFieldOpEq && b == f.Bool
	case models.CustomFieldTypeSelect:
		s, ok := rawString(raw)
		return ok && (f.Op == CustomFieldOpEq || f.Op == CustomFieldOpIn) && slices.Contains(f.OptionIDs, s)
	case models.CustomFieldTypeItemLink:
		s, ok := rawString(raw)
		return ok && f.Op == CustomFieldOpEq && s == f.Text
	}
	return false
}

func compareOp(op CustomFieldFilterOp, cmp int) bool {
	switch op {
	case CustomFieldOpEq:
		return cmp == 0
	case CustomFieldOpGte:
		return cmp >= 0
	case CustomFieldOpLte:
		return cmp <= 0
	}
	return false
}

func rawString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// rawDecimal reads a number value, or the amount of a money value.
func rawDecimal(t models.CustomFieldType, raw json.RawMessage) (decimal.Decimal, bool) {
	if t == models.CustomFieldTypeMoney {
		var m models.CustomFieldMoney
		if err := json.Unmarshal(raw, &m); err != nil {
			return decimal.Zero, false
		}
		raw = json.RawMessage(m.Amount)
	}
	n, err := decimal.NewFromString(strings.Trim(strings.TrimSpace(string(raw)), `"`))
	return n, err == nil
}

// CustomFieldSort orders items by their value for one custom field. Items
// without a value come after every item with one, in both directions, as an
// unknown price does. Text and URL values compare without regard to case,
// select values by the option's position in the field, a yes/no value false
// before true, and an amount by its value whatever its currency. A
// long text and a link to an item do not sort: services/commodityquery
// falls back to the name for them.
type CustomFieldSort struct {
	FieldID string
	Type    models.CustomFieldType
	// OptionOrder are the option IDs of a select field in the field's
	// order.
	OptionOrder []string
}

// CustomFieldSortable reports whether items can be sorted by a field of
// type t.
func CustomFieldSortable(t models.CustomFieldType) bool {
	switch t {
	case models.CustomFieldTypeText, models.CustomFieldTypeURL, models.CustomFieldTypeNumber,
		models.CustomFieldTypeMoney, models.CustomFieldTypeDate, models.CustomFieldTypeBoolean,
		models.CustomFieldTypeSelect:
		return true
	}
	return false
}

// Compare orders two items' value maps by the sort's field, ascending.
// aUnset and bUnset report an item without a value, which the caller puts
// last; cmp is then 0.
func (s CustomFieldSort) Compare(a, b models.CustomFieldValues) (cmp int, aUnset, bUnset bool) {
	ka, aok := s.key(a)
	kb, bok := s.key(b)
	if !aok || !bok {
		return 0, !aok, !bok
	}
	switch {
	case ka.num != nil:
		return ka.num.Cmp(*kb.num), false, false
	default:
		return strings.Compare(ka.text, kb.text), false, false
	}
}

type customFieldSortKey struct {
	text string
	num  *decimal.Decimal
}

func (s CustomFieldSort) key(values models.CustomFieldValues) (customFieldSortKey, bool) {
	raw, ok := values[s.FieldID]
	if !ok || models.IsNullJSON(raw) {
		return customFieldSortKey{}, false
	}
	switch s.Type {
	case models.CustomFieldTypeText, models.CustomFieldTypeURL:
		v, ok := rawString(raw)
		return customFieldSortKey{text: strings.ToLower(v)}, ok
	case models.CustomFieldTypeDate:
		v, ok := rawString(raw)
		return customFieldSortKey{text: v}, ok
	case models.CustomFieldTypeNumber, models.CustomFieldTypeMoney:
		n, ok := rawDecimal(s.Type, raw)
		return customFieldSortKey{num: &n}, ok
	case models.CustomFieldTypeBoolean:
		var b bool
		if json.Unmarshal(raw, &b) != nil {
			return customFieldSortKey{}, false
		}
		n := decimal.Zero
		if b {
			n = decimal.NewFromInt(1)
		}
		return customFieldSortKey{num: &n}, true
	case models.CustomFieldTypeSelect:
		v, ok := rawString(raw)
		if !ok {
			return customFieldSortKey{}, false
		}
		// An option the field no longer has sorts after every known one.
		pos := slices.Index(s.OptionOrder, v)
		if pos < 0 {
			pos = len(s.OptionOrder)
		}
		n := decimal.NewFromInt(int64(pos))
		return customFieldSortKey{num: &n}, true
	}
	return customFieldSortKey{}, false
}

// CustomFieldSearch is what the free-text search reads of custom field
// values. services/commodityquery fills it from the group's fields.
type CustomFieldSearch struct {
	// TextFields are the IDs of the group's text, long text and URL
	// fields. A term matches such a value when it is a substring of it,
	// without regard to case.
	TextFields map[string]bool
	// Options maps a term to the select options whose label contains it,
	// so a term finds the items with that option.
	Options map[string][]CustomFieldOptionRef
}

// CustomFieldOptionRef names one option of one select field.
type CustomFieldOptionRef struct {
	FieldID  string
	OptionID string
	Label    string
}

// IsZero reports whether the search reads no custom field value.
func (s CustomFieldSearch) IsZero() bool {
	return len(s.TextFields) == 0 && len(s.Options) == 0
}

// match returns the custom field values of c that term matches.
func (s CustomFieldSearch) match(c *models.Commodity, term string) []CommoditySearchMatch {
	if s.IsZero() || len(c.CustomFields) == 0 {
		return nil
	}
	var out []CommoditySearchMatch
	for _, id := range slices.Sorted(maps.Keys(c.CustomFields)) {
		if !s.TextFields[id] {
			continue
		}
		v, ok := rawString(c.CustomFields[id])
		if ok && containsFold(v, term) {
			out = append(out, CommoditySearchMatch{Field: CommoditySearchFieldCustomFields, Value: searchExcerpt(v, term), CustomFieldID: id})
		}
	}
	for _, ref := range s.Options[term] {
		v, ok := rawString(c.CustomFields[ref.FieldID])
		if ok && v == ref.OptionID {
			out = append(out, CommoditySearchMatch{Field: CommoditySearchFieldCustomFields, Value: ref.Label, CustomFieldID: ref.FieldID})
		}
	}
	return out
}
