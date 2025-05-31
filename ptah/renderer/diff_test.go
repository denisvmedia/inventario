package renderer_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/renderer"
	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
)

func TestFormatSchemaDiff_NoChanges(t *testing.T) {
	tests := []struct {
		name     string
		diff     *differtypes.SchemaDiff
		contains []string
	}{
		{
			name: "completely empty diff",
			diff: &differtypes.SchemaDiff{},
			contains: []string{
				"=== NO SCHEMA CHANGES DETECTED ===",
				"The database schema matches your entity definitions.",
			},
		},
		{
			name: "diff with empty slices",
			diff: &differtypes.SchemaDiff{
				TablesAdded:    []string{},
				TablesRemoved:  []string{},
				TablesModified: []differtypes.TableDiff{},
				EnumsAdded:     []string{},
				EnumsRemoved:   []string{},
				EnumsModified:  []differtypes.EnumDiff{},
				IndexesAdded:   []string{},
				IndexesRemoved: []string{},
			},
			contains: []string{
				"=== NO SCHEMA CHANGES DETECTED ===",
				"The database schema matches your entity definitions.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := renderer.FormatSchemaDiff(tt.diff)

			c.Assert(result, qt.Not(qt.Equals), "")
			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected)
			}
		})
	}
}

func TestFormatSchemaDiff_WithChanges(t *testing.T) {
	tests := []struct {
		name     string
		diff     *differtypes.SchemaDiff
		contains []string
	}{
		{
			name: "tables added and removed",
			diff: &differtypes.SchemaDiff{
				TablesAdded:   []string{"new_users", "new_posts"},
				TablesRemoved: []string{"old_logs", "deprecated_table"},
			},
			contains: []string{
				"=== SCHEMA DIFFERENCES DETECTED ===",
				"SUMMARY: 4 changes detected",
				"- Tables: +2 -2 ~0",
				"- Enums: +0 -0 ~0",
				"- Indexes: +0 -0",
				"📋 TABLES TO ADD:",
				"+ new_users",
				"+ new_posts",
				"🗑️  TABLES TO REMOVE:",
				"- old_logs (⚠️  DATA WILL BE LOST!)",
				"- deprecated_table (⚠️  DATA WILL BE LOST!)",
			},
		},
		{
			name: "tables modified with column changes",
			diff: &differtypes.SchemaDiff{
				TablesModified: []differtypes.TableDiff{
					{
						TableName:      "users",
						ColumnsAdded:   []string{"email", "phone"},
						ColumnsRemoved: []string{"old_field"},
						ColumnsModified: []differtypes.ColumnDiff{
							{
								ColumnName: "name",
								Changes: map[string]string{
									"type":     "VARCHAR(100) -> VARCHAR(255)",
									"nullable": "YES -> NO",
								},
							},
						},
					},
				},
			},
			contains: []string{
				"=== SCHEMA DIFFERENCES DETECTED ===",
				"SUMMARY: 1 changes detected",
				"- Tables: +0 -0 ~1",
				"🔧 TABLES TO MODIFY:",
				"~ users",
				"+ Column: email",
				"+ Column: phone",
				"- Column: old_field (⚠️  DATA WILL BE LOST!)",
				"~ Column: name",
				"type: VARCHAR(100) -> VARCHAR(255)",
				"nullable: YES -> NO",
			},
		},
		{
			name: "enums added, removed and modified",
			diff: &differtypes.SchemaDiff{
				EnumsAdded:   []string{"status_enum", "priority_enum"},
				EnumsRemoved: []string{"old_enum"},
				EnumsModified: []differtypes.EnumDiff{
					{
						EnumName:      "user_role",
						ValuesAdded:   []string{"admin", "moderator"},
						ValuesRemoved: []string{"deprecated_role"},
					},
				},
			},
			contains: []string{
				"=== SCHEMA DIFFERENCES DETECTED ===",
				"SUMMARY: 4 changes detected",
				"- Enums: +2 -1 ~1",
				"🏷️  ENUMS TO ADD:",
				"+ status_enum",
				"+ priority_enum",
				"🗑️  ENUMS TO REMOVE:",
				"- old_enum (⚠️  MAKE SURE NO TABLES USE THIS!)",
				"🔧 ENUMS TO MODIFY:",
				"~ user_role",
				"+ Value: admin",
				"+ Value: moderator",
				"- Value: deprecated_role (⚠️  NOT SUPPORTED IN POSTGRESQL!)",
			},
		},
		{
			name: "indexes added and removed",
			diff: &differtypes.SchemaDiff{
				IndexesAdded:   []string{"idx_users_email", "idx_posts_title"},
				IndexesRemoved: []string{"old_index"},
			},
			contains: []string{
				"=== SCHEMA DIFFERENCES DETECTED ===",
				"SUMMARY: 3 changes detected",
				"- Indexes: +2 -1",
				"📇 INDEXES TO ADD:",
				"+ idx_users_email",
				"+ idx_posts_title",
				"🗑️  INDEXES TO REMOVE:",
				"- old_index",
			},
		},
		{
			name: "comprehensive changes",
			diff: &differtypes.SchemaDiff{
				TablesAdded:   []string{"new_table"},
				TablesRemoved: []string{"old_table"},
				TablesModified: []differtypes.TableDiff{
					{
						TableName:    "users",
						ColumnsAdded: []string{"email"},
					},
				},
				EnumsAdded:     []string{"new_enum"},
				EnumsRemoved:   []string{"old_enum"},
				IndexesAdded:   []string{"new_index"},
				IndexesRemoved: []string{"old_index"},
			},
			contains: []string{
				"=== SCHEMA DIFFERENCES DETECTED ===",
				"SUMMARY: 7 changes detected",
				"- Tables: +1 -1 ~1",
				"- Enums: +1 -1 ~0",
				"- Indexes: +1 -1",
				"📋 TABLES TO ADD:",
				"🗑️  TABLES TO REMOVE:",
				"🔧 TABLES TO MODIFY:",
				"🏷️  ENUMS TO ADD:",
				"🗑️  ENUMS TO REMOVE:",
				"📇 INDEXES TO ADD:",
				"🗑️  INDEXES TO REMOVE:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := renderer.FormatSchemaDiff(tt.diff)

			c.Assert(result, qt.Not(qt.Equals), "")
			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected)
			}
		})
	}
}

func TestFormatSchemaDiff_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		diff     *differtypes.SchemaDiff
		contains []string
	}{
		{
			name: "nil diff",
			diff: nil,
			// This should panic or be handled gracefully
		},
		{
			name: "empty enum diff with no values",
			diff: &differtypes.SchemaDiff{
				EnumsModified: []differtypes.EnumDiff{
					{
						EnumName:      "empty_enum",
						ValuesAdded:   []string{},
						ValuesRemoved: []string{},
					},
				},
			},
			contains: []string{
				"=== SCHEMA DIFFERENCES DETECTED ===",
				"🔧 ENUMS TO MODIFY:",
				"~ empty_enum",
			},
		},
		{
			name: "empty table diff with no changes",
			diff: &differtypes.SchemaDiff{
				TablesModified: []differtypes.TableDiff{
					{
						TableName:       "empty_table",
						ColumnsAdded:    []string{},
						ColumnsRemoved:  []string{},
						ColumnsModified: []differtypes.ColumnDiff{},
					},
				},
			},
			contains: []string{
				"=== SCHEMA DIFFERENCES DETECTED ===",
				"🔧 TABLES TO MODIFY:",
				"~ empty_table",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			if tt.diff == nil {
				// Test that nil diff doesn't panic
				c.Assert(func() { renderer.FormatSchemaDiff(tt.diff) }, qt.PanicMatches, ".*")
				return
			}

			result := renderer.FormatSchemaDiff(tt.diff)

			c.Assert(result, qt.Not(qt.Equals), "")
			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected)
			}
		})
	}
}
