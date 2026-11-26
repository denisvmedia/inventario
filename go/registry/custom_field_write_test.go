package registry_test

import (
	"encoding/json"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

func TestPlanCustomFieldWrite(t *testing.T) {
	c := qt.New(t)
	defs := map[string]*models.CustomFieldDefinition{
		"text": {Type: models.CustomFieldTypeText},
		"num":  {Type: models.CustomFieldTypeNumber},
		"link": {Type: models.CustomFieldTypeItemLink},
	}
	items := map[string]registry.LinkedItemState{
		"live":    {Exists: true},
		"trashed": {Exists: true, Trashed: true},
	}
	lookups := 0
	lookup := func(id string) (registry.LinkedItemState, error) {
		lookups++
		return items[id], nil
	}
	stored := models.CustomFieldValues{"text": json.RawMessage(`"a"`), "num": json.RawMessage(`1`), "link": json.RawMessage(`"trashed"`)}

	plan, err := registry.PlanCustomFieldWrite("self", defs, stored, models.CustomFieldValues{
		"text":    json.RawMessage(`" a "`),
		"num":     json.RawMessage(`null`),
		"link":    json.RawMessage(`"trashed"`),
		"deleted": json.RawMessage(`null`),
	}, lookup)
	c.Assert(err, qt.IsNil)
	c.Check(plan.Deletes, qt.DeepEquals, []string{"num"})
	c.Check(plan.Sets, qt.HasLen, 0, qt.Commentf("unchanged values are skipped, a kept link to a trashed item too"))
	c.Check(lookups, qt.Equals, 0)

	plan, err = registry.PlanCustomFieldWrite("self", defs, stored, models.CustomFieldValues{"link": json.RawMessage(`"live"`)}, lookup)
	c.Assert(err, qt.IsNil)
	c.Check(*plan.Sets["link"].ItemID, qt.Equals, "live")
	out := registry.ApplyCustomFieldWritePlan(stored, plan, defs)
	c.Check(string(out["link"]), qt.Equals, `"live"`)
	c.Check(string(stored["link"]), qt.Equals, `"trashed"`, qt.Commentf("the stored map is not changed"))

	_, err = registry.PlanCustomFieldWrite("self", defs, nil, models.CustomFieldValues{
		"link":  json.RawMessage(`"self"`),
		"num":   json.RawMessage(`"x"`),
		"other": json.RawMessage(`1`),
	}, lookup)
	valueErr, ok := errors.AsType[*registry.CustomFieldValueError](err)
	c.Assert(ok, qt.IsTrue)
	c.Check(valueErr.Fields["link"].Error(), qt.Equals, registry.ErrCustomFieldLinkSelf.Error())
	c.Check(valueErr.Fields["other"].Error(), qt.Equals, registry.ErrCustomFieldUnknown.Error())
	c.Check(valueErr.Fields["num"], qt.IsNotNil)
	c.Check(err, qt.ErrorIs, registry.ErrInvalidCustomFieldValue)

	for target, want := range map[string]error{"trashed": registry.ErrCustomFieldLinkTrash, "gone": registry.ErrCustomFieldLinkGone} {
		_, err := registry.PlanCustomFieldWrite("self", defs, nil, models.CustomFieldValues{"link": json.RawMessage(`"` + target + `"`)}, lookup)
		valueErr, ok := errors.AsType[*registry.CustomFieldValueError](err)
		c.Assert(ok, qt.IsTrue)
		c.Check(valueErr.Fields["link"].Error(), qt.Equals, want.Error(), qt.Commentf("target %s", target))
	}

	boom := errors.New("boom")
	_, err = registry.PlanCustomFieldWrite("self", defs, nil, models.CustomFieldValues{"link": json.RawMessage(`"live"`)},
		func(string) (registry.LinkedItemState, error) { return registry.LinkedItemState{}, boom })
	c.Check(err, qt.Equals, boom)

	c.Check(registry.ApplyCustomFieldWritePlan(stored, registry.CustomFieldWritePlan{Deletes: []string{"text", "num", "link"}}, defs), qt.IsNil)
}

func TestCustomFieldFilterAndSort(t *testing.T) {
	c := qt.New(t)
	values := func(raw string) models.CustomFieldValues {
		return models.CustomFieldValues{"f": json.RawMessage(raw)}
	}
	f := func(typ models.CustomFieldType, op registry.CustomFieldFilterOp) registry.CustomFieldFilter {
		return registry.CustomFieldFilter{FieldID: "f", Type: typ, Op: op}
	}

	money := f(models.CustomFieldTypeMoney, registry.CustomFieldOpGte)
	money.Number = decimal.RequireFromString("10")
	c.Check(money.Matches(values(`{"amount":10.00,"currency":"USD"}`)), qt.IsTrue)
	c.Check(money.Matches(values(`{"amount":9.99,"currency":"EUR"}`)), qt.IsFalse)
	c.Check(f(models.CustomFieldTypeText, registry.CustomFieldOpSet).Matches(values(`""`)), qt.IsTrue)
	c.Check(f(models.CustomFieldTypeText, registry.CustomFieldOpUnset).Matches(nil), qt.IsTrue)
	c.Check(registry.CustomFieldFilter{FieldID: "f", Op: registry.CustomFieldOpUnset}.Matches(nil), qt.IsFalse,
		qt.Commentf("a field that does not exist keeps nothing"))
	link := f(models.CustomFieldTypeItemLink, registry.CustomFieldOpEq)
	link.Text = "item-1"
	c.Check(link.Matches(values(`"item-1"`)), qt.IsTrue)

	c.Check(registry.CustomFieldOps(models.CustomFieldTypeBoolean), qt.DeepEquals,
		[]registry.CustomFieldFilterOp{registry.CustomFieldOpSet, registry.CustomFieldOpUnset, registry.CustomFieldOpEq})
	c.Check(registry.CustomFieldSortable(models.CustomFieldTypeLongText), qt.IsFalse)

	sel := registry.CustomFieldSort{FieldID: "f", Type: models.CustomFieldTypeSelect, OptionOrder: []string{"b", "a"}}
	cmp, aUnset, bUnset := sel.Compare(values(`"b"`), values(`"a"`))
	c.Check(cmp < 0, qt.IsTrue, qt.Commentf("options sort in the field's order"))
	c.Check(aUnset || bUnset, qt.IsFalse)
	_, aUnset, bUnset = sel.Compare(nil, values(`"a"`))
	c.Check(aUnset, qt.IsTrue)
	c.Check(bUnset, qt.IsFalse)
	text := registry.CustomFieldSort{FieldID: "f", Type: models.CustomFieldTypeText}
	cmp, _, _ = text.Compare(values(`"B"`), values(`"a"`))
	c.Check(cmp > 0, qt.IsTrue)
}
