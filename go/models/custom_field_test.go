package models_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
)

func TestCustomFieldDefinition_ParseValue(t *testing.T) {
	fuel := &models.CustomFieldDefinition{
		Type:    models.CustomFieldTypeSelect,
		Options: models.ValuerSlice[models.CustomFieldOption]{{ID: "p1", Label: "Petrol"}},
	}
	def := func(t models.CustomFieldType) *models.CustomFieldDefinition {
		return &models.CustomFieldDefinition{Type: t}
	}
	for _, tc := range []struct {
		name string
		def  *models.CustomFieldDefinition
		in   string
		want string
		code string
	}{
		{name: "text is trimmed", def: def(models.CustomFieldTypeText), in: `"  AB 1 "`, want: `"AB 1"`},
		{name: "empty text is a value", def: def(models.CustomFieldTypeText), in: `""`, want: `""`},
		{name: "text over the limit", def: def(models.CustomFieldTypeText), in: `"` + strings.Repeat("a", models.MaxCustomFieldTextRunes+1) + `"`, code: "custom_field_too_long"},
		{name: "text that is a number", def: def(models.CustomFieldTypeText), in: `12`, code: "custom_field_invalid_text"},
		{name: "long text keeps its spaces", def: def(models.CustomFieldTypeLongText), in: `" a\nb "`, want: `" a\nb "`},
		{name: "number", def: def(models.CustomFieldTypeNumber), in: `12.50`, want: `"12.5"`},
		{name: "number as a string", def: def(models.CustomFieldTypeNumber), in: `" 0 "`, want: `"0"`},
		// More digits than a JavaScript number keeps, and a small number
		// a JavaScript number prints with an exponent: both stay decimal
		// strings with every digit.
		{name: "text with a NUL", def: def(models.CustomFieldTypeText), in: `"a\u0000b"`, code: "custom_field_invalid_character"},
		{name: "long text with a NUL", def: def(models.CustomFieldTypeLongText), in: `"\u0000"`, code: "custom_field_invalid_character"},
		{name: "empty text", def: def(models.CustomFieldTypeText), in: `""`, want: `""`},
		{name: "date in year zero", def: def(models.CustomFieldTypeDate), in: `"0000-01-01"`, code: "custom_field_invalid_date"},
		{name: "leap day", def: def(models.CustomFieldTypeDate), in: `"2028-02-29"`, want: `"2028-02-29"`},
		{name: "number with every digit", def: def(models.CustomFieldTypeNumber), in: `"123456789012345.1234567891"`, want: `"123456789012345.1234567891"`},
		{name: "small number", def: def(models.CustomFieldTypeNumber), in: `0.0000005`, want: `"0.0000005"`},
		{name: "number too precise", def: def(models.CustomFieldTypeNumber), in: `0.12345678901`, code: "custom_field_too_precise"},
		{name: "number too large", def: def(models.CustomFieldTypeNumber), in: `1e15`, code: "custom_field_too_large"},
		{name: "number that is a word", def: def(models.CustomFieldTypeNumber), in: `"ten"`, code: "custom_field_invalid_number"},
		{name: "date", def: def(models.CustomFieldTypeDate), in: `"2026-02-28"`, want: `"2026-02-28"`},
		{name: "a date that does not exist", def: def(models.CustomFieldTypeDate), in: `"2026-02-30"`, code: "custom_field_invalid_date"},
		{name: "false is a value", def: def(models.CustomFieldTypeBoolean), in: `false`, want: `false`},
		{name: "boolean as a string", def: def(models.CustomFieldTypeBoolean), in: `"true"`, code: "custom_field_invalid_boolean"},
		{name: "select option", def: fuel, in: `"p1"`, want: `"p1"`},
		{name: "select label instead of an option", def: fuel, in: `"Petrol"`, code: "custom_field_unknown_option"},
		{name: "url", def: def(models.CustomFieldTypeURL), in: `"https://example.com/a"`, want: `"https://example.com/a"`},
		{name: "url without a scheme", def: def(models.CustomFieldTypeURL), in: `"example.com"`, code: "custom_field_invalid_url"},
		{name: "url of another scheme", def: def(models.CustomFieldTypeURL), in: `"javascript:alert(1)"`, code: "custom_field_invalid_url"},
		{name: "money", def: def(models.CustomFieldTypeMoney), in: `{"amount":"10.5","currency":"eur"}`, want: `{"amount":10.50,"currency":"EUR"}`},
		{name: "money without a currency", def: def(models.CustomFieldTypeMoney), in: `{"amount":1}`, code: "custom_field_invalid_money"},
		{name: "money with extra keys", def: def(models.CustomFieldTypeMoney), in: `{"amount":1,"currency":"EUR","x":1}`, code: "custom_field_invalid_money"},
		{name: "item link", def: def(models.CustomFieldTypeItemLink), in: `" item-1 "`, want: `"item-1"`},
		{name: "empty item link", def: def(models.CustomFieldTypeItemLink), in: `""`, code: "custom_field_invalid_item"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			data, err := tc.def.ParseValue(json.RawMessage(tc.in))
			if tc.code != "" {
				verr, ok := errors.AsType[validation.Error](err)
				c.Assert(ok, qt.IsTrue, qt.Commentf("error %v", err))
				c.Check(verr.Code(), qt.Equals, tc.code)
				return
			}
			c.Assert(err, qt.IsNil)
			c.Check(string(data.JSON(tc.def.Type)), qt.Equals, tc.want)
		})
	}
}

func TestCustomFieldDefinition_NormalizeAndValidate(t *testing.T) {
	c := qt.New(t)

	d := models.CustomFieldDefinition{
		Name: "  Fuel   type ", Type: models.CustomFieldTypeSelect,
		Options: models.ValuerSlice[models.CustomFieldOption]{{Label: " Petrol "}, {ID: "keep", Label: "Diesel"}},
	}
	d.Normalize()
	c.Check(d.Name, qt.Equals, "Fuel type")
	c.Check(d.NameKey, qt.Equals, models.CustomFieldNameKey("fuel TYPE"))
	c.Check(d.Options[0].Label, qt.Equals, "Petrol")
	c.Check(d.Options[0].ID, qt.Matches, `[a-z0-9]{8}`)
	c.Check(d.Options[1].ID, qt.Equals, "keep")
	c.Check(d.ValidateFields(t.Context()), qt.IsNil)

	text := models.CustomFieldDefinition{Name: "Plate", Type: models.CustomFieldTypeText, Options: models.ValuerSlice[models.CustomFieldOption]{{Label: "x"}}}
	text.Normalize()
	c.Check(text.Options, qt.HasLen, 0, qt.Commentf("only a select field keeps options"))

	for _, tc := range []struct {
		name  string
		def   models.CustomFieldDefinition
		field string
	}{
		{name: "no name", def: models.CustomFieldDefinition{Type: models.CustomFieldTypeText}, field: "name"},
		{name: "unknown type", def: models.CustomFieldDefinition{Name: "X", Type: "colour"}, field: "type"},
		{name: "select without options", def: models.CustomFieldDefinition{Name: "X", Type: models.CustomFieldTypeSelect}, field: "options"},
		{name: "two options with one label", def: models.CustomFieldDefinition{
			Name: "X", Type: models.CustomFieldTypeSelect,
			Options: models.ValuerSlice[models.CustomFieldOption]{{ID: "a", Label: "Red"}, {ID: "b", Label: "RED"}},
		}, field: "options"},
		{name: "an option id of the wrong shape", def: models.CustomFieldDefinition{
			Name: "X", Type: models.CustomFieldTypeSelect,
			Options: models.ValuerSlice[models.CustomFieldOption]{{ID: "Not OK", Label: "Red"}},
		}, field: "options"},
		{name: "a template without an item type", def: models.CustomFieldDefinition{
			Name: "X", Type: models.CustomFieldTypeText, Templates: []models.CustomFieldTemplate{{}},
		}, field: "templates"},
		{name: "an item type listed twice", def: models.CustomFieldDefinition{
			Name: "X", Type: models.CustomFieldTypeText, Templates: []models.CustomFieldTemplate{{ItemTypeID: "t"}, {ItemTypeID: "t"}},
		}, field: "templates"},
		{name: "a default the field refuses", def: models.CustomFieldDefinition{
			Name: "X", Type: models.CustomFieldTypeNumber, Templates: []models.CustomFieldTemplate{{ItemTypeID: "t", DefaultValue: models.JSONValue(`"ten"`)}},
		}, field: "templates"},
		{name: "a default for an item link", def: models.CustomFieldDefinition{
			Name: "X", Type: models.CustomFieldTypeItemLink, Templates: []models.CustomFieldTemplate{{ItemTypeID: "t", DefaultValue: models.JSONValue(`"item-1"`)}},
		}, field: "templates"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := tc.def.ValidateFields(t.Context())
			errs, ok := errors.AsType[validation.Errors](err)
			c.Assert(ok, qt.IsTrue, qt.Commentf("error %v", err))
			c.Check(errs[tc.field], qt.IsNotNil, qt.Commentf("errors %v", errs))
		})
	}

	withDefault := models.CustomFieldDefinition{
		Name: "Wheels", Type: models.CustomFieldTypeNumber,
		Templates: []models.CustomFieldTemplate{{ItemTypeID: " t ", DefaultValue: models.JSONValue(`"4.0"`)}, {ItemTypeID: "u", DefaultValue: models.JSONValue(`null`)}},
	}
	c.Assert(withDefault.ValidateFields(t.Context()), qt.IsNil)
	c.Check(withDefault.Templates[0].ItemTypeID, qt.Equals, "t")
	c.Check(string(withDefault.Templates[0].DefaultValue), qt.Equals, `"4"`, qt.Commentf("a default is stored in its canonical form"))
	c.Check(withDefault.Templates[1].DefaultValue.IsNull(), qt.IsTrue)
}

func TestJSONValue_RoundTrip(t *testing.T) {
	c := qt.New(t)
	var v models.JSONValue
	c.Assert(json.Unmarshal([]byte(`{"a":1}`), &v), qt.IsNil)
	c.Check(v.IsNull(), qt.IsFalse)
	out, err := json.Marshal(v)
	c.Assert(err, qt.IsNil)
	c.Check(string(out), qt.Equals, `{"a":1}`)

	var empty models.JSONValue
	c.Check(empty.IsNull(), qt.IsTrue)
	out, err = json.Marshal(empty)
	c.Assert(err, qt.IsNil)
	c.Check(string(out), qt.Equals, `null`)
	dbv, err := empty.Value()
	c.Assert(err, qt.IsNil)
	c.Check(dbv, qt.IsNil)

	var scanned models.JSONValue
	c.Assert(scanned.Scan("true"), qt.IsNil)
	c.Check(string(scanned), qt.Equals, `true`)
}
