package errkit_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/errkit"
)

func TestToFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []any
		result errkit.Fields
	}{
		{
			name:   "Empty Fields",
			fields: make([]any, 0),
			result: nil,
		},
		{
			name:   "Single Fields Map",
			fields: []any{errkit.Fields{"key1": "value1"}},
			result: errkit.Fields{"key1": "value1"},
		},
		{
			name:   "Odd Number of Fields",
			fields: []any{"key1", "value1", "key2"},                    // An odd number of elements
			result: errkit.Fields{"key1": "value1", "!BADKEY": "key2"}, // Expecting "!BADKEY" added as key
		},
		{
			name:   "Mixed Types",
			fields: []any{"key1", "value1", 42, "value2"},                    // Mixed types, including an int
			result: errkit.Fields{"key1": "value1", "!BADKEY(42)": "value2"}, // Expecting type conversions and "!BADKEY"
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)

			fs := errkit.ToFields(test.fields)
			c.Assert(fs, qt.DeepEquals, test.result)
		})
	}
}
