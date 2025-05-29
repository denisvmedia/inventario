package builder_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

func TestProcessEmbeddedFields_InlineMode(t *testing.T) {
	tests := []struct {
		name           string
		embeddedFields []meta.EmbeddedField
		allFields      []meta.SchemaField
		structName     string
		expected       []meta.SchemaField
	}{
		{
			name: "inline without prefix",
			embeddedFields: []meta.EmbeddedField{
				{
					StructName:       "User",
					Mode:             "inline",
					EmbeddedTypeName: "Timestamps",
				},
			},
			allFields: []meta.SchemaField{
				{
					StructName: "Timestamps",
					FieldName:  "CreatedAt",
					Name:       "created_at",
					Type:       "TIMESTAMP",
				},
				{
					StructName: "Timestamps",
					FieldName:  "UpdatedAt",
					Name:       "updated_at",
					Type:       "TIMESTAMP",
				},
			},
			structName: "User",
			expected: []meta.SchemaField{
				{
					StructName: "User",
					FieldName:  "CreatedAt",
					Name:       "created_at",
					Type:       "TIMESTAMP",
				},
				{
					StructName: "User",
					FieldName:  "UpdatedAt",
					Name:       "updated_at",
					Type:       "TIMESTAMP",
				},
			},
		},
		{
			name: "inline with prefix",
			embeddedFields: []meta.EmbeddedField{
				{
					StructName:       "User",
					Mode:             "inline",
					Prefix:           "audit_",
					EmbeddedTypeName: "AuditInfo",
				},
			},
			allFields: []meta.SchemaField{
				{
					StructName: "AuditInfo",
					FieldName:  "By",
					Name:       "by",
					Type:       "TEXT",
				},
				{
					StructName: "AuditInfo",
					FieldName:  "Reason",
					Name:       "reason",
					Type:       "TEXT",
				},
			},
			structName: "User",
			expected: []meta.SchemaField{
				{
					StructName: "User",
					FieldName:  "By",
					Name:       "audit_by",
					Type:       "TEXT",
				},
				{
					StructName: "User",
					FieldName:  "Reason",
					Name:       "audit_reason",
					Type:       "TEXT",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := meta.ProcessEmbeddedFields(tt.embeddedFields, tt.allFields, tt.structName)

			c.Assert(len(result), qt.Equals, len(tt.expected))
			for i, expected := range tt.expected {
				c.Assert(result[i].StructName, qt.Equals, expected.StructName)
				c.Assert(result[i].FieldName, qt.Equals, expected.FieldName)
				c.Assert(result[i].Name, qt.Equals, expected.Name)
				c.Assert(result[i].Type, qt.Equals, expected.Type)
			}
		})
	}
}

func TestProcessEmbeddedFields_JsonMode(t *testing.T) {
	tests := []struct {
		name           string
		embeddedFields []meta.EmbeddedField
		structName     string
		expected       []meta.SchemaField
	}{
		{
			name: "json mode with default name and type",
			embeddedFields: []meta.EmbeddedField{
				{
					StructName:       "User",
					Mode:             "json",
					EmbeddedTypeName: "Meta",
				},
			},
			structName: "User",
			expected: []meta.SchemaField{
				{
					StructName: "User",
					FieldName:  "Meta",
					Name:       "meta_data",
					Type:       "JSONB",
					Nullable:   false,
				},
			},
		},
		{
			name: "json mode with custom name and type",
			embeddedFields: []meta.EmbeddedField{
				{
					StructName:       "User",
					Mode:             "json",
					Name:             "metadata",
					Type:             "JSON",
					EmbeddedTypeName: "Meta",
					Nullable:         true,
					Comment:          "User metadata",
				},
			},
			structName: "User",
			expected: []meta.SchemaField{
				{
					StructName: "User",
					FieldName:  "Meta",
					Name:       "metadata",
					Type:       "JSON",
					Nullable:   true,
					Comment:    "User metadata",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := meta.ProcessEmbeddedFields(tt.embeddedFields, nil, tt.structName)

			c.Assert(len(result), qt.Equals, len(tt.expected))
			for i, expected := range tt.expected {
				c.Assert(result[i].StructName, qt.Equals, expected.StructName)
				c.Assert(result[i].FieldName, qt.Equals, expected.FieldName)
				c.Assert(result[i].Name, qt.Equals, expected.Name)
				c.Assert(result[i].Type, qt.Equals, expected.Type)
				c.Assert(result[i].Nullable, qt.Equals, expected.Nullable)
				c.Assert(result[i].Comment, qt.Equals, expected.Comment)
			}
		})
	}
}

func TestProcessEmbeddedFields_RelationMode(t *testing.T) {
	tests := []struct {
		name           string
		embeddedFields []meta.EmbeddedField
		structName     string
		expected       []meta.SchemaField
	}{
		{
			name: "relation mode with integer reference",
			embeddedFields: []meta.EmbeddedField{
				{
					StructName:       "Post",
					Mode:             "relation",
					Field:            "user_id",
					Ref:              "users(id)",
					EmbeddedTypeName: "User",
					Comment:          "Reference to user",
				},
			},
			structName: "Post",
			expected: []meta.SchemaField{
				{
					StructName:     "Post",
					FieldName:      "User",
					Name:           "user_id",
					Type:           "INTEGER",
					Foreign:        "users(id)",
					ForeignKeyName: "fk_post_user_id",
					Comment:        "Reference to user",
				},
			},
		},
		{
			name: "relation mode with varchar reference",
			embeddedFields: []meta.EmbeddedField{
				{
					StructName:       "Post",
					Mode:             "relation",
					Field:            "category_id",
					Ref:              "categories(VARCHAR_uuid)",
					EmbeddedTypeName: "Category",
					Nullable:         true,
				},
			},
			structName: "Post",
			expected: []meta.SchemaField{
				{
					StructName:     "Post",
					FieldName:      "Category",
					Name:           "category_id",
					Type:           "VARCHAR(36)",
					Foreign:        "categories(VARCHAR_uuid)",
					ForeignKeyName: "fk_post_category_id",
					Nullable:       true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := meta.ProcessEmbeddedFields(tt.embeddedFields, nil, tt.structName)

			c.Assert(len(result), qt.Equals, len(tt.expected))
			for i, expected := range tt.expected {
				c.Assert(result[i].StructName, qt.Equals, expected.StructName)
				c.Assert(result[i].FieldName, qt.Equals, expected.FieldName)
				c.Assert(result[i].Name, qt.Equals, expected.Name)
				c.Assert(result[i].Type, qt.Equals, expected.Type)
				c.Assert(result[i].Foreign, qt.Equals, expected.Foreign)
				c.Assert(result[i].ForeignKeyName, qt.Equals, expected.ForeignKeyName)
				c.Assert(result[i].Nullable, qt.Equals, expected.Nullable)
				c.Assert(result[i].Comment, qt.Equals, expected.Comment)
			}
		})
	}
}

func TestProcessEmbeddedFields_SkipMode(t *testing.T) {
	c := qt.New(t)

	embeddedFields := []meta.EmbeddedField{
		{
			StructName:       "User",
			Mode:             "skip",
			EmbeddedTypeName: "Internal",
		},
	}

	result := meta.ProcessEmbeddedFields(embeddedFields, nil, "User")

	c.Assert(len(result), qt.Equals, 0)
}

func TestProcessEmbeddedFields_DefaultMode(t *testing.T) {
	c := qt.New(t)

	embeddedFields := []meta.EmbeddedField{
		{
			StructName:       "User",
			Mode:             "", // Empty mode should default to inline
			EmbeddedTypeName: "Timestamps",
		},
	}

	allFields := []meta.SchemaField{
		{
			StructName: "Timestamps",
			FieldName:  "CreatedAt",
			Name:       "created_at",
			Type:       "TIMESTAMP",
		},
	}

	result := meta.ProcessEmbeddedFields(embeddedFields, allFields, "User")

	c.Assert(len(result), qt.Equals, 1)
	c.Assert(result[0].StructName, qt.Equals, "User")
	c.Assert(result[0].Name, qt.Equals, "created_at")
}

func TestProcessEmbeddedFields_FiltersByStructName(t *testing.T) {
	c := qt.New(t)

	embeddedFields := []meta.EmbeddedField{
		{
			StructName:       "User",
			Mode:             "inline",
			EmbeddedTypeName: "Timestamps",
		},
		{
			StructName:       "Post", // Different struct - should be ignored
			Mode:             "inline",
			EmbeddedTypeName: "Timestamps",
		},
	}

	allFields := []meta.SchemaField{
		{
			StructName: "Timestamps",
			FieldName:  "CreatedAt",
			Name:       "created_at",
			Type:       "TIMESTAMP",
		},
	}

	result := meta.ProcessEmbeddedFields(embeddedFields, allFields, "User")

	c.Assert(len(result), qt.Equals, 1) // Only User embedded field processed
	c.Assert(result[0].StructName, qt.Equals, "User")
}

func TestProcessEmbeddedFields_RelationModeSkipsIncompleteFields(t *testing.T) {
	c := qt.New(t)

	embeddedFields := []meta.EmbeddedField{
		{
			StructName:       "Post",
			Mode:             "relation",
			Field:            "", // Missing field name
			Ref:              "users(id)",
			EmbeddedTypeName: "User",
		},
		{
			StructName:       "Post",
			Mode:             "relation",
			Field:            "user_id",
			Ref:              "", // Missing reference
			EmbeddedTypeName: "User",
		},
	}

	result := meta.ProcessEmbeddedFields(embeddedFields, nil, "Post")

	c.Assert(len(result), qt.Equals, 0) // Both should be skipped
}
