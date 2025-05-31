package differ

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/differ/internal/normalize"
)

// TestTypeNormalization tests the type comparison logic
func TestTypeNormalization(t *testing.T) {

	tests := []struct {
		input    string
		expected string
	}{
		{"VARCHAR(255)", "varchar"},
		{"TEXT", "text"},
		{"SERIAL", "integer"},
		{"INTEGER", "integer"},
		{"BOOLEAN", "boolean"},
		{"TIMESTAMP", "timestamp"},
		{"DECIMAL(10,2)", "decimal"},
		{"NUMERIC(10,2)", "decimal"},
		{"custom_enum", "custom_enum"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			c := qt.New(t)
			result := differtypes.NormalizeType(tt.input)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}
