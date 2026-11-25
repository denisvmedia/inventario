package commodityquery

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// ParamCustomField is the custom field filter (#2803), repeatable, one
// condition per value: "<field ID>:<op>" for set and unset, otherwise
// "<field ID>:<op>:<operand>". The conditions are AND-ed. See
// registry.CustomFieldFilterOp for the comparisons and CustomFieldOps for
// which a field type takes.
const ParamCustomField = "cf"

// CustomFieldSortPrefix starts the sort value of a custom field:
// sort=cf.<field ID>, or -cf.<field ID> descending.
const CustomFieldSortPrefix = "cf."

// MaxCustomFieldFilters bounds the conditions of one query.
const MaxCustomFieldFilters = 20

// CustomFieldCondition is one cf condition as the request spelled it; the
// field it names is resolved against the group by Resolve.
type CustomFieldCondition struct {
	FieldID string
	Op      registry.CustomFieldFilterOp
	// Operand is the compared value, empty for set and unset: text, a
	// number, a date as YYYY-MM-DD, true or false, comma-separated option
	// IDs, or an item ID.
	Operand string
}

// String is the condition's cf value.
func (c CustomFieldCondition) String() string {
	if !c.Op.TakesOperand() {
		return c.FieldID + ":" + string(c.Op)
	}
	return c.FieldID + ":" + string(c.Op) + ":" + c.Operand
}

// customFields reads the cf conditions. It checks what it can without the
// group's fields: the shape, the comparison, and that a range operand is a
// number or a date. Resolve checks the rest against the field.
func (p parser) customFields(q *Query) error {
	for _, raw := range p.list(ParamCustomField) {
		cond, err := parseCustomFieldCondition(raw)
		if err != nil {
			return err
		}
		if slices.Contains(q.CustomFields, cond) {
			continue
		}
		q.CustomFields = append(q.CustomFields, cond)
		if len(q.CustomFields) > MaxCustomFieldFilters {
			return paramError(ParamCustomField, "", fmt.Sprintf("must list at most %d conditions", MaxCustomFieldFilters))
		}
	}
	return nil
}

func parseCustomFieldCondition(raw string) (CustomFieldCondition, error) {
	parts := strings.SplitN(raw, ":", 3)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		return CustomFieldCondition{}, paramError(ParamCustomField, raw, "must be <field id>:<op>[:<value>]")
	}
	cond := CustomFieldCondition{FieldID: strings.TrimSpace(parts[0]), Op: registry.CustomFieldFilterOp(strings.TrimSpace(parts[1]))}
	if !cond.Op.IsValid() {
		return CustomFieldCondition{}, paramError(ParamCustomField, raw, "the comparison must be set, unset, eq, contains, gte, lte or in")
	}
	if len(parts) == 3 {
		cond.Operand = strings.TrimSpace(parts[2])
	}
	switch {
	case !cond.Op.TakesOperand() && cond.Operand != "":
		return CustomFieldCondition{}, paramError(ParamCustomField, raw, string(cond.Op)+" takes no value")
	case cond.Op.TakesOperand() && cond.Operand == "":
		return CustomFieldCondition{}, paramError(ParamCustomField, raw, string(cond.Op)+" needs a value")
	case utf8.RuneCountInString(cond.Operand) > MaxTextRunes:
		return CustomFieldCondition{}, paramError(ParamCustomField, "", fmt.Sprintf("a value must be at most %d characters", MaxTextRunes))
	}
	if cond.Op == registry.CustomFieldOpIn && strings.Trim(cond.Operand, ", ") == "" {
		return CustomFieldCondition{}, paramError(ParamCustomField, raw, "in needs one or more option IDs")
	}
	if cond.Op == registry.CustomFieldOpGte || cond.Op == registry.CustomFieldOpLte {
		_, numErr := decimal.NewFromString(cond.Operand)
		_, dateErr := time.Parse(time.DateOnly, cond.Operand)
		if numErr != nil && dateErr != nil {
			return CustomFieldCondition{}, paramError(ParamCustomField, raw, "a range value must be a number or a date as YYYY-MM-DD")
		}
	}
	return cond, nil
}

// customFieldSortID returns the field ID of a custom field sort, or "".
func customFieldSortID(field registry.CommoditySortField) string {
	if id, ok := strings.CutPrefix(string(field), CustomFieldSortPrefix); ok {
		return id
	}
	return ""
}

// needsCustomFields reports whether resolving q reads the group's custom
// fields.
func (q Query) needsCustomFields() bool {
	return len(q.CustomFields) > 0 || customFieldSortID(q.SortField) != "" || q.Text != ""
}

// resolveCustomFields fills the custom field parts of opts from the group's
// fields: the filters, the sort, and the values the search reads.
//
// A condition on a field the group does not have keeps no item, the way a
// saved view that names a deleted field lists nothing rather than
// everything; a sort by such a field, or by a field that does not sort,
// falls back to the name like any unknown sort. A comparison the field's
// type does not take, or an operand it cannot read, is an *Error.
func resolveCustomFields(ctx context.Context, set *registry.Set, q Query, opts *registry.CommodityListOptions) error {
	if set.CustomFieldRegistry == nil || !q.needsCustomFields() {
		return nil
	}
	defs, err := set.CustomFieldRegistry.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list custom fields", err)
	}
	byID := make(map[string]*models.CustomFieldDefinition, len(defs))
	for _, d := range defs {
		byID[d.ID] = d
	}

	for _, cond := range q.CustomFields {
		f, err := customFieldFilter(cond, byID[cond.FieldID])
		if err != nil {
			return err
		}
		opts.CustomFields = append(opts.CustomFields, f)
	}

	if id := customFieldSortID(q.SortField); id != "" {
		if d := byID[id]; d != nil && registry.CustomFieldSortable(d.Type) {
			opts.CustomFieldSort = &registry.CustomFieldSort{FieldID: d.ID, Type: d.Type, OptionOrder: d.OptionIDs()}
		}
	}

	opts.SearchCustomFields = customFieldSearch(defs, registry.SplitSearchTerms(q.Text))
	return nil
}

// CheckCustomFields checks the cf conditions of q against defs, the fields
// of the group, as Resolve does: a comparison the field's type does not
// take, or an operand it cannot read, is an *Error. A condition on a field
// defs lacks passes, since it keeps no item rather than failing. A saved
// view runs it before it stores a query, so a stored view never fails to
// resolve: a field's type cannot change.
func CheckCustomFields(q Query, defs []*models.CustomFieldDefinition) error {
	for _, cond := range q.CustomFields {
		var def *models.CustomFieldDefinition
		for _, d := range defs {
			if d.ID == cond.FieldID {
				def = d
				break
			}
		}
		if _, err := customFieldFilter(cond, def); err != nil {
			return err
		}
	}
	return nil
}

func customFieldFilter(cond CustomFieldCondition, d *models.CustomFieldDefinition) (registry.CustomFieldFilter, error) {
	f := registry.CustomFieldFilter{FieldID: cond.FieldID, Op: cond.Op}
	if d == nil {
		return f, nil
	}
	f.Type = d.Type
	if !slices.Contains(registry.CustomFieldOps(d.Type), cond.Op) {
		return f, paramError(ParamCustomField, cond.String(), fmt.Sprintf("a %s field does not take %s", d.Type, cond.Op))
	}
	if !cond.Op.TakesOperand() {
		return f, nil
	}
	invalid := func(reason string) error { return paramError(ParamCustomField, cond.String(), reason) }
	switch d.Type {
	case models.CustomFieldTypeText, models.CustomFieldTypeLongText, models.CustomFieldTypeURL, models.CustomFieldTypeItemLink:
		f.Text = cond.Operand
	case models.CustomFieldTypeNumber, models.CustomFieldTypeMoney:
		n, err := decimal.NewFromString(cond.Operand)
		if err != nil {
			return f, invalid("must be a number")
		}
		f.Number = n
	case models.CustomFieldTypeDate:
		if _, err := time.Parse(time.DateOnly, cond.Operand); err != nil {
			return f, invalid("must be a date as YYYY-MM-DD")
		}
		f.Text = cond.Operand
	case models.CustomFieldTypeBoolean:
		switch strings.ToLower(cond.Operand) {
		case "1", "true":
			f.Bool = true
		case "0", "false":
			f.Bool = false
		default:
			return f, invalid("must be true or false")
		}
	case models.CustomFieldTypeSelect:
		// An option the field no longer has matches no item, as a deleted
		// tag does.
		for id := range strings.SplitSeq(cond.Operand, ",") {
			if id = strings.TrimSpace(id); id != "" && !slices.Contains(f.OptionIDs, id) {
				f.OptionIDs = append(f.OptionIDs, id)
			}
		}
		if len(f.OptionIDs) == 0 || (cond.Op == registry.CustomFieldOpEq && len(f.OptionIDs) > 1) {
			return f, invalid("eq takes one option, in one or more")
		}
	}
	return f, nil
}

// customFieldSearch lists the text fields the search reads and, per term,
// the select options whose label contains it.
func customFieldSearch(defs []*models.CustomFieldDefinition, terms []string) registry.CustomFieldSearch {
	var s registry.CustomFieldSearch
	if len(terms) == 0 {
		return s
	}
	for _, d := range defs {
		switch d.Type {
		case models.CustomFieldTypeText, models.CustomFieldTypeLongText, models.CustomFieldTypeURL:
			if s.TextFields == nil {
				s.TextFields = map[string]bool{}
			}
			s.TextFields[d.ID] = true
		case models.CustomFieldTypeSelect:
			for _, term := range terms {
				for _, o := range d.Options {
					if strings.Contains(strings.ToLower(o.Label), strings.ToLower(term)) {
						if s.Options == nil {
							s.Options = map[string][]registry.CustomFieldOptionRef{}
						}
						s.Options[term] = append(s.Options[term], registry.CustomFieldOptionRef{FieldID: d.ID, OptionID: o.ID, Label: o.Label})
					}
				}
			}
		}
	}
	return s
}
