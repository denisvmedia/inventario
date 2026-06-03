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
}
