package differtypes_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
)

func TestSchemaDiff_HasChanges(t *testing.T) {
	tests := []struct {
		name     string
		diff     *differtypes.SchemaDiff
		expected bool
	}{
		{
			name:     "no changes",
			diff:     &differtypes.SchemaDiff{},
			expected: false,
		},
		{
			name: "tables added",
			diff: &differtypes.SchemaDiff{
				TablesAdded: []string{"users"},
			},
			expected: true,
		},
		{
			name: "tables removed",
			diff: &differtypes.SchemaDiff{
				TablesRemoved: []string{"old_table"},
			},
			expected: true,
		},
		{
			name: "tables modified",
			diff: &differtypes.SchemaDiff{
				TablesModified: []differtypes.TableDiff{
					{TableName: "users", ColumnsAdded: []string{"email"}},
				},
			},
			expected: true,
		},
		{
			name: "enums added",
			diff: &differtypes.SchemaDiff{
				EnumsAdded: []string{"status_enum"},
			},
			expected: true,
		},
		{
			name: "enums removed",
			diff: &differtypes.SchemaDiff{
				EnumsRemoved: []string{"old_enum"},
			},
			expected: true,
		},
		{
			name: "enums modified",
			diff: &differtypes.SchemaDiff{
				EnumsModified: []differtypes.EnumDiff{
					{EnumName: "status", ValuesAdded: []string{"pending"}},
				},
			},
			expected: true,
		},
		{
			name: "indexes added",
			diff: &differtypes.SchemaDiff{
				IndexesAdded: []string{"idx_user_email"},
			},
			expected: true,
		},
		{
			name: "indexes removed",
			diff: &differtypes.SchemaDiff{
				IndexesRemoved: []string{"old_index"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := tt.diff.HasChanges()
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}
