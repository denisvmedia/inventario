package models_test

import (
	"encoding/json"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
)

func TestCommodityDraftValuesRule_AcceptsTheFormTypes(t *testing.T) {
	c := qt.New(t)
	values := models.CommodityDraftValues{
		"name":                 "Drill",
		"short_name":           "",
		"seller_id":            "",
		"count":                "2",
		"original_price":       float64(19.9),
		"current_price":        json.Number("12"),
		"tags":                 []any{"tools", "garage"},
		"urls":                 []any{},
		"draft":                true,
		"from_intake":          map[string]any{"any": "shape"},
		"unknown_number_field": float64(3),
	}
	c.Assert(validation.Validate(values, models.CommodityDraftValuesRule), qt.IsNil)
}

func TestCommodityDraftValuesRule_NamesTheFieldOfTheWrongType(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
		code  string
	}{
		{name: "text field holding null", key: "short_name", value: nil, code: "validation_draft_value_text"},
		{name: "text field holding a number", key: "area_id", value: float64(1), code: "validation_draft_value_text"},
		{name: "number field holding a list", key: "count", value: []any{"1"}, code: "validation_draft_value_number"},
		{name: "number field holding a flag", key: "original_price", value: true, code: "validation_draft_value_number"},
		{name: "list field holding a string", key: "tags", value: "tools", code: "validation_draft_value_list"},
		{name: "list field holding a number", key: "part_numbers", value: []any{"a", float64(2)}, code: "validation_draft_value_list"},
		{name: "flag field holding a string", key: "draft", value: "true", code: "validation_draft_value_flag"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := validation.Validate(models.CommodityDraftValues{"name": "Drill", tc.key: tc.value}, models.CommodityDraftValuesRule)
			c.Assert(err, qt.IsNotNil)
			var errs validation.Errors
			c.Assert(err, qt.ErrorAs, &errs)
			c.Assert(errs, qt.HasLen, 1)
			var fieldErr validation.Error
			c.Assert(errs[tc.key], qt.ErrorAs, &fieldErr)
			c.Assert(fieldErr.Code(), qt.Equals, tc.code)
		})
	}
}

func TestValidateCommodityDraftValues_ReportsUnderValues(t *testing.T) {
	c := qt.New(t)
	c.Assert(models.ValidateCommodityDraftValues(models.CommodityDraftValues{"name": "Drill"}), qt.IsNil)

	err := models.ValidateCommodityDraftValues(models.CommodityDraftValues{"tags": "tools"})
	var errs validation.Errors
	c.Assert(err, qt.ErrorAs, &errs)
	var inner validation.Errors
	c.Assert(errs["values"], qt.ErrorAs, &inner)
	c.Assert(inner["tags"], qt.IsNotNil)
}

func TestPromotionKeyRule(t *testing.T) {
	tests := []struct {
		name string
		key  string
		ok   bool
	}{
		{name: "uuid", key: "5f0c8a7e-3f43-4f1e-9a43-6a1fbb8f2c11", ok: true},
		{name: "longest key", key: strings.Repeat("k", models.CommodityDraftPromotionKeyMaxLength), ok: true},
		{name: "blank", key: ""},
		{name: "too long", key: strings.Repeat("k", models.CommodityDraftPromotionKeyMaxLength+1)},
		{name: "space", key: "a b"},
		{name: "control character", key: "a\tb"},
		{name: "non-ASCII", key: "ключ"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := validation.Validate(tc.key, models.PromotionKeyRule)
			if tc.ok {
				c.Assert(err, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNotNil)
			}
		})
	}
}

func TestMapDraftCustomFieldReference(t *testing.T) {
	mapID := func(id string) string {
		return map[string]string{"old": "new"}[id]
	}
	tests := []struct {
		name  string
		entry any
		want  any
	}{
		{name: "an id", entry: "old", want: "new"},
		{name: "an id it does not know", entry: "other", want: nil},
		{name: "no id", entry: "", want: ""},
		{name: "a form entry", entry: map[string]any{"value": "old", "currency": ""}, want: map[string]any{"value": "new", "currency": ""}},
		{name: "a form entry it does not know", entry: map[string]any{"value": "other"}, want: nil},
		{name: "an empty form entry", entry: map[string]any{"value": ""}, want: map[string]any{"value": ""}},
		{name: "neither shape", entry: float64(3), want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(models.MapDraftCustomFieldReference(tt.entry, mapID), qt.DeepEquals, tt.want)
		})
	}

	c := qt.New(t)
	entry := map[string]any{"value": "old"}
	models.MapDraftCustomFieldReference(entry, mapID)
	c.Assert(entry["value"], qt.Equals, "old", qt.Commentf("the entry given is left as it was"))
}
