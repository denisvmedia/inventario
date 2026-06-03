package aivision_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/aivision"
)

// TestResponseSchema_TypeEnumWarrantyShortNameMultiItem locks the parts of
// the structured-output contract that steer prefill quality: a closed
// commodity-type enum, the short_name length cap, the warranty field, and
// the multiple_items warning code.
func TestResponseSchema_TypeEnumWarrantyShortNameMultiItem(t *testing.T) {
	c := qt.New(t)
	schema := aivision.ResponseSchema()

	props := schema["properties"].(map[string]any)
	fields := props["fields"].(map[string]any)["properties"].(map[string]any)

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

	// "warranty_expires_at" is part of the extracted set.
	_, hasWarranty := fields[aivision.FieldNameWarrantyExpiresAt]
	c.Assert(hasWarranty, qt.IsTrue)

	// "multiple_items" is an allowed warning code.
	codeEnum := props["warnings"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["code"].(map[string]any)["enum"].([]string)
	c.Assert(codeEnum, qt.Contains, "multiple_items")

	// items[] is an array whose entries carry the same `fields` object.
	items := props["items"].(map[string]any)
	c.Assert(items["type"], qt.Equals, "array")
	itemEntry := items["items"].(map[string]any)
	_, hasItemFields := itemEntry["properties"].(map[string]any)["fields"]
	c.Assert(hasItemFields, qt.IsTrue)
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

func TestToScanResult_ItemsOnly_MirrorsPrimary(t *testing.T) {
	c := qt.New(t)
	// Model left `fields` empty but populated `items` — Fields mirrors the
	// first item so single-item consumers still see a primary extraction.
	res, err := aivision.ToScanResult([]byte(`{"fields":{},"items":[{"fields":{"name":{"value":"A","confidence":0.9}}}]}`))
	c.Assert(err, qt.IsNil)
	c.Assert(res.Fields["name"].Value, qt.Equals, "A")
	c.Assert(res.Items, qt.HasLen, 1)
}
