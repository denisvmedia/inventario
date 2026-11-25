package aivision_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/aivision"
)

// TestResponseSchema_TypeEnumWarrantyShortNameMultiItem locks the parts of
// the structured-output contract that steer prefill quality: a closed
// commodity-type enum, the short_name length cap, the warranty field, and
// the multiple_items warning code.
func TestResponseSchema_TypeEnumWarrantyShortNameMultiItem(t *testing.T) {
	c := qt.New(t)
	schema := aivision.ResponseSchema(aivision.ScanRequest{})

	props := schema["properties"].(map[string]any)

	// items[] is the REQUIRED primary output — a list of products. Making the
	// model produce a list is what gets every invoice line enumerated.
	items := props["items"].(map[string]any)
	c.Assert(items["type"], qt.Equals, "array")
	c.Assert(items["minItems"], qt.Equals, 1)
	c.Assert(schema["required"], qt.DeepEquals, []string{"items"})

	// Each item IS the field object directly (flat) — where the per-field
	// constraints (type enum, short_name cap, warranty) live.
	fields := items["items"].(map[string]any)["properties"].(map[string]any)

	fieldValue := func(name string) map[string]any {
		return fields[name].(map[string]any)["properties"].(map[string]any)["value"].(map[string]any)
	}

	// "type" is constrained to the commodity-type categories so a valid
	// guess isn't dropped by the FE's isKnownType gate.
	typeEnum, ok := fieldValue(aivision.FieldNameType)["enum"].([]string)
	c.Assert(ok, qt.IsTrue)
	c.Assert(typeEnum, qt.Contains, "electronics")
	c.Assert(typeEnum, qt.Contains, "white_goods")

	// "short_name" carries the 40-char cap that mirrors the form limit.
	c.Assert(fieldValue(aivision.FieldNameShortName)["maxLength"], qt.Equals, 40)

	// "brand" and "model" carry the 100-char caps of the item's fields,
	// "seller" the cap of a seller's name.
	c.Assert(fieldValue(aivision.FieldNameBrand)["maxLength"], qt.Equals, 100)
	c.Assert(fieldValue(aivision.FieldNameModel)["maxLength"], qt.Equals, 100)
	c.Assert(fieldValue(aivision.FieldNameSeller)["maxLength"], qt.Equals, 100)

	// "warranty_expires_at" and "tags" are part of the extracted set.
	_, hasWarranty := fields[aivision.FieldNameWarrantyExpiresAt]
	c.Assert(hasWarranty, qt.IsTrue)
	_, hasTags := fields[aivision.FieldNameTags]
	c.Assert(hasTags, qt.IsTrue)

	// "multiple_items" is an allowed warning code.
	codeEnum := props["warnings"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["code"].(map[string]any)["enum"].([]string)
	c.Assert(codeEnum, qt.Contains, "multiple_items")
}

func TestToScanResult_MultiItem(t *testing.T) {
	c := qt.New(t)
	body := []byte(`{
		"fields": { "name": {"value":"Primary","confidence":0.9} },
		"items": [
			{ "fields": { "name": {"value":"Primary","confidence":0.9}, "original_price": {"value":10.5,"confidence":0.8} } },
			{ "fields": { "name": {"value":"Second","confidence":0.7} } }
		]
	}`)
	res, err := aivision.ToScanResult(body)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Fields["name"].Value, qt.Equals, "Primary")
	c.Assert(res.Items, qt.HasLen, 2)
	c.Assert(res.Items[0].Fields["original_price"].Value, qt.Equals, 10.5)
	c.Assert(res.Items[1].Fields["name"].Value, qt.Equals, "Second")
}

func TestToScanResult_SingleItem_NoItems(t *testing.T) {
	c := qt.New(t)
	res, err := aivision.ToScanResult([]byte(`{"fields":{"name":{"value":"Only","confidence":0.9}}}`))
	c.Assert(err, qt.IsNil)
	c.Assert(res.Items, qt.HasLen, 0)
	c.Assert(res.Fields["name"].Value, qt.Equals, "Only")
}

func TestToScanResult_ItemsOnly_SingleProduct(t *testing.T) {
	c := qt.New(t)
	// New contract: the model returns only `items`. A single product → Fields
	// mirrors items[0] and Items stays empty (no chooser for one product).
	res, err := aivision.ToScanResult([]byte(`{"items":[{"fields":{"name":{"value":"A","confidence":0.9}}}]}`))
	c.Assert(err, qt.IsNil)
	c.Assert(res.Fields["name"].Value, qt.Equals, "A")
	c.Assert(res.Items, qt.HasLen, 0)
}

func TestToScanResult_ItemsOnly_MultiProduct(t *testing.T) {
	c := qt.New(t)
	// New contract, multi-product invoice: only `items`, several entries.
	res, err := aivision.ToScanResult([]byte(`{"items":[
		{"fields":{"name":{"value":"Pampers","confidence":0.9}}},
		{"fields":{"name":{"value":"Calculator","confidence":0.8}}},
		{"fields":{"name":{"value":"Cleaner","confidence":0.7}}}
	]}`))
	c.Assert(err, qt.IsNil)
	c.Assert(res.Items, qt.HasLen, 3)
	c.Assert(res.Fields["name"].Value, qt.Equals, "Pampers")
	c.Assert(res.Items[1].Fields["name"].Value, qt.Equals, "Calculator")
}

func TestToScanResult_FlatItems(t *testing.T) {
	c := qt.New(t)
	// The schema we now request: each item IS the field map (no `fields`
	// wrapper). This is the shape that was blanking the review before the
	// parser learned to accept it.
	res, err := aivision.ToScanResult([]byte(`{"items":[
		{"name":{"value":"Pampers","confidence":0.9},"original_price":{"value":434,"confidence":0.9}},
		{"name":{"value":"Calculator","confidence":0.8}}
	]}`))
	c.Assert(err, qt.IsNil)
	c.Assert(res.Items, qt.HasLen, 2)
	c.Assert(res.Fields["name"].Value, qt.Equals, "Pampers")
	c.Assert(res.Items[0].Fields["original_price"].Value, qt.Equals, float64(434))
	c.Assert(res.Items[1].Fields["name"].Value, qt.Equals, "Calculator")
}

// The schema enum and the prompt follow the group's item types, so a custom
// type can be guessed by meaning and its slug stays the stored value.
func TestResponseSchemaAndPrompt_UseTheGroupsItemTypes(t *testing.T) {
	c := qt.New(t)
	req := aivision.ScanRequest{ItemTypes: []aivision.ItemTypeOption{
		{Slug: "electronics", Name: "Electronics"},
		{Slug: "vehicle", Name: "Vehicle"},
		{Slug: "автомобиль", Name: "Автомобиль"},
	}}

	schema := aivision.ResponseSchema(req)
	fields := schema["properties"].(map[string]any)["items"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	typeEnum := fields[aivision.FieldNameType].(map[string]any)["properties"].(map[string]any)["value"].(map[string]any)["enum"].([]string)
	c.Assert(typeEnum, qt.DeepEquals, []string{"electronics", "vehicle", "автомобиль"})

	header := aivision.UserPromptHeader(req)
	c.Check(header, qt.Contains, "- vehicle (Vehicle)\n")
	c.Check(header, qt.Contains, "- автомобиль (Автомобиль)\n")
	c.Check(header, qt.Not(qt.Contains), "white_goods")

	// Without types the built-in ones apply.
	builtin := aivision.UserPromptHeader(aivision.ScanRequest{})
	c.Check(builtin, qt.Contains, "- white_goods (Appliance)\n")
}

func TestResponseSchemaAndPrompt_OfferTheGroupsCustomFields(t *testing.T) {
	c := qt.New(t)
	req := aivision.ScanRequest{CustomFields: []aivision.CustomFieldOption{
		{Key: "f-vin", Name: "VIN", Type: "text"},
		{Key: "f-fuel", Name: "Fuel", Type: "select", Options: []string{"Petrol", "Diesel"}},
		{Key: "f-paid", Name: "Paid", Type: "money"},
		{Key: "f-charger", Name: "Charger", Type: "item_link"},
		{Key: "f-electric", Name: "Electric", Type: "boolean"},
	}}

	schema := aivision.ResponseSchema(req)
	fields := schema["properties"].(map[string]any)["items"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	custom := fields[aivision.FieldNameCustomFields].(map[string]any)["properties"].(map[string]any)
	c.Check(custom, qt.HasLen, 3, qt.Commentf("money and item links are left out"))
	fuelEnum := custom["f-fuel"].(map[string]any)["properties"].(map[string]any)["value"].(map[string]any)["enum"].([]string)
	c.Check(fuelEnum, qt.DeepEquals, []string{"Petrol", "Diesel"})

	header := aivision.UserPromptHeader(req)
	c.Check(header, qt.Contains, `- f-vin: "VIN" (text)`)
	c.Check(header, qt.Contains, `- f-fuel: "Fuel" (select; options: "Petrol", "Diesel")`)
	c.Check(header, qt.Not(qt.Contains), "f-paid")

	// Without custom fields the schema has no custom_fields member.
	plain := aivision.ResponseSchema(aivision.ScanRequest{})
	plainFields := plain["properties"].(map[string]any)["items"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	_, has := plainFields[aivision.FieldNameCustomFields]
	c.Check(has, qt.IsFalse)
}

func TestToScanResult_CustomFields(t *testing.T) {
	c := qt.New(t)
	res, err := aivision.ToScanResult([]byte(`{"items":[{
		"name":{"value":"Car","confidence":0.9},
		"custom_fields":{
			"f-vin":{"value":"WVW123","confidence":0.8},
			"f-km":{"value":120000,"confidence":0.5},
			"f-electric":{"value":false,"confidence":0.7},
			"f-none":{"value":null,"confidence":0.1},
			"f-list":{"value":["a"],"confidence":0.1}
		}
	}]}`))
	c.Assert(err, qt.IsNil)
	c.Check(res.Fields[aivision.FieldNameName].Value, qt.Equals, "Car")
	c.Check(res.CustomFields, qt.DeepEquals, map[string]aivision.FieldGuess{
		"f-vin":      {Value: "WVW123", Confidence: 0.8},
		"f-km":       {Value: float64(120000), Confidence: 0.5},
		"f-electric": {Value: false, Confidence: 0.7},
	})
}
