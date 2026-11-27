package registry_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

func TestCustomFieldFilter_Matches_TextFoldsCase(t *testing.T) {
	for _, tc := range []struct {
		name  string
		op    registry.CustomFieldFilterOp
		value string
		text  string
		want  bool
	}{
		{name: "eq, final sigma", op: registry.CustomFieldOpEq, value: "ΟΔΟΣ", text: "οδος", want: true},
		{name: "eq, sharp s and ss", op: registry.CustomFieldOpEq, value: "Straße", text: "STRASSE", want: true},
		{name: "eq, ASCII", op: registry.CustomFieldOpEq, value: "Blue", text: "BLUE", want: true},
		{name: "eq is not contains", op: registry.CustomFieldOpEq, value: "ΟΔΟΣ 12", text: "οδος", want: false},
		{name: "contains, final sigma", op: registry.CustomFieldOpContains, value: "ΟΔΟΣ 12", text: "οδος", want: true},
		{name: "contains, sharp s and ss", op: registry.CustomFieldOpContains, value: "Hauptstraße 5", text: "STRASSE", want: true},
		{name: "contains, no match", op: registry.CustomFieldOpContains, value: "ΟΔΟΣ 12", text: "οδοι", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			raw, err := json.Marshal(tc.value)
			c.Assert(err, qt.IsNil)
			filter := registry.CustomFieldFilter{FieldID: "f", Type: models.CustomFieldTypeText, Op: tc.op, Text: tc.text}
			c.Assert(filter.Matches(models.CustomFieldValues{"f": raw}), qt.Equals, tc.want)
		})
	}
}
