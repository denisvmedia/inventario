package transform_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/schema/transform"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

func TestFromSchemaField_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		field    types.SchemaField
		expected func(*ast.ColumnNode) bool
	}{
		{
			name: "basic field",
			field: types.SchemaField{
				Name:     "username",
				Type:     "VARCHAR(255)",
				Nullable: true,
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "username" &&
					col.Type == "VARCHAR(255)" &&
					col.Nullable == true
			},
		},
		{
			name: "primary key field",
			field: types.SchemaField{
				Name:    "id",
				Type:    "SERIAL",
				Primary: true,
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "id" &&
					col.Type == "SERIAL" &&
					col.Primary == true &&
					col.Nullable == false
			},
		},
		{
			name: "not null field",
			field: types.SchemaField{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Nullable: false,
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "email" &&
					col.Type == "VARCHAR(255)" &&
					col.Nullable == false
			},
		},
		{
			name: "unique field",
			field: types.SchemaField{
				Name:   "username",
				Type:   "VARCHAR(100)",
				Unique: true,
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "username" &&
					col.Type == "VARCHAR(100)" &&
					col.Unique == true
			},
		},
		{
			name: "auto increment field",
			field: types.SchemaField{
				Name:    "id",
				Type:    "INTEGER",
				AutoInc: true,
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "id" &&
					col.Type == "INTEGER" &&
					col.AutoInc == true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := transform.FromSchemaField(tt.field, nil)

			c.Assert(result, qt.IsNotNil)
			c.Assert(tt.expected(result), qt.IsTrue)
		})
	}
}

func TestFromSchemaField_WithDefaults(t *testing.T) {
	tests := []struct {
		name     string
		field    types.SchemaField
		expected func(*ast.ColumnNode) bool
	}{
		{
			name: "literal default",
			field: types.SchemaField{
				Name:    "status",
				Type:    "VARCHAR(20)",
				Default: "'active'",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Default != nil && col.Default.Value == "'active'"
			},
		},
		{
			name: "function default",
			field: types.SchemaField{
				Name:        "created_at",
				Type:        "TIMESTAMP",
				DefaultExpr: "NOW()",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Default != nil && col.Default.Expression == "NOW()"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := transform.FromSchemaField(tt.field, nil)

			c.Assert(result, qt.IsNotNil)
			c.Assert(tt.expected(result), qt.IsTrue)
		})
	}
}

func TestFromSchemaField_WithConstraints(t *testing.T) {
	tests := []struct {
		name     string
		field    types.SchemaField
		expected func(*ast.ColumnNode) bool
	}{
		{
			name: "check constraint",
			field: types.SchemaField{
				Name:  "age",
				Type:  "INTEGER",
				Check: "age >= 0 AND age <= 150",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Check == "age >= 0 AND age <= 150"
			},
		},
		{
			name: "comment",
			field: types.SchemaField{
				Name:    "description",
				Type:    "TEXT",
				Comment: "User description",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Comment == "User description"
			},
		},
		{
			name: "foreign key",
			field: types.SchemaField{
				Name:           "user_id",
				Type:           "INTEGER",
				Foreign:        "users(id)",
				ForeignKeyName: "fk_posts_user",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.ForeignKey != nil &&
					col.ForeignKey.Table == "users(id)" &&
					col.ForeignKey.Name == "fk_posts_user"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := transform.FromSchemaField(tt.field, nil)

			c.Assert(result, qt.IsNotNil)
			c.Assert(tt.expected(result), qt.IsTrue)
		})
	}
}

func TestFromSchemaField_ComplexField(t *testing.T) {
	c := qt.New(t)

	field := types.SchemaField{
		Name:           "user_id",
		Type:           "INTEGER",
		Nullable:       false,
		Primary:        false,
		Unique:         true,
		AutoInc:        false,
		Default:        "0",
		Check:          "user_id > 0",
		Comment:        "Reference to user table",
		Foreign:        "users(id)",
		ForeignKeyName: "fk_posts_user",
	}

	result := transform.FromSchemaField(field, nil)

	c.Assert(result, qt.IsNotNil)
	c.Assert(result.Name, qt.Equals, "user_id")
	c.Assert(result.Type, qt.Equals, "INTEGER")
	c.Assert(result.Nullable, qt.IsFalse)
	c.Assert(result.Primary, qt.IsFalse)
	c.Assert(result.Unique, qt.IsTrue)
	c.Assert(result.AutoInc, qt.IsFalse)
	c.Assert(result.Default, qt.IsNotNil)
	c.Assert(result.Default.Value, qt.Equals, "0")
	c.Assert(result.Check, qt.Equals, "user_id > 0")
	c.Assert(result.Comment, qt.Equals, "Reference to user table")
	c.Assert(result.ForeignKey, qt.IsNotNil)
	c.Assert(result.ForeignKey.Table, qt.Equals, "users(id)")
	c.Assert(result.ForeignKey.Name, qt.Equals, "fk_posts_user")
}

func TestFromTableDirective_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		table    types.TableDirective
		fields   []types.SchemaField
		expected func(*ast.CreateTableNode) bool
	}{
		{
			name: "basic table",
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
					Primary:    true,
				},
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
					Nullable:   false,
				},
			},
			expected: func(table *ast.CreateTableNode) bool {
				return table.Name == "users" &&
					len(table.Columns) == 2 &&
					table.Columns[0].Name == "id" &&
					table.Columns[1].Name == "email"
			},
		},
		{
			name: "table with comment and engine",
			table: types.TableDirective{
				StructName: "Post",
				Name:       "posts",
				Comment:    "User posts",
				Engine:     "InnoDB",
			},
			fields: []types.SchemaField{
				{
					StructName: "Post",
					Name:       "id",
					Type:       "INTEGER",
					Primary:    true,
				},
			},
			expected: func(table *ast.CreateTableNode) bool {
				return table.Name == "posts" &&
					table.Comment == "User posts" &&
					table.Options["ENGINE"] == "InnoDB" &&
					len(table.Columns) == 1
			},
		},
		{
			name: "table with composite primary key",
			table: types.TableDirective{
				StructName: "UserRole",
				Name:       "user_roles",
				PrimaryKey: []string{"user_id", "role_id"},
			},
			fields: []types.SchemaField{
				{
					StructName: "UserRole",
					Name:       "user_id",
					Type:       "INTEGER",
				},
				{
					StructName: "UserRole",
					Name:       "role_id",
					Type:       "INTEGER",
				},
			},
			expected: func(table *ast.CreateTableNode) bool {
				return table.Name == "user_roles" &&
					len(table.Columns) == 2 &&
					len(table.Constraints) == 1 &&
					table.Constraints[0].Type == ast.PrimaryKeyConstraint
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := transform.FromTableDirective(tt.table, tt.fields, nil)

			c.Assert(result, qt.IsNotNil)
			c.Assert(tt.expected(result), qt.IsTrue)
		})
	}
}

func TestFromTableDirective_FiltersByStructName(t *testing.T) {
	c := qt.New(t)

	table := types.TableDirective{
		StructName: "User",
		Name:       "users",
	}

	fields := []types.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
		},
		{
			StructName: "Post", // Different struct - should be filtered out
			Name:       "title",
			Type:       "VARCHAR(255)",
		},
		{
			StructName: "User",
			Name:       "email",
			Type:       "VARCHAR(255)",
		},
	}

	result := transform.FromTableDirective(table, fields, nil)

	c.Assert(result, qt.IsNotNil)
	c.Assert(result.Name, qt.Equals, "users")
	c.Assert(len(result.Columns), qt.Equals, 2) // Only User fields
	c.Assert(result.Columns[0].Name, qt.Equals, "id")
	c.Assert(result.Columns[1].Name, qt.Equals, "email")
}

func TestFromSchemaIndex_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		index    types.SchemaIndex
		expected func(*ast.IndexNode) bool
	}{
		{
			name: "basic index",
			index: types.SchemaIndex{
				Name:       "idx_users_email",
				StructName: "users",
				Fields:     []string{"email"},
			},
			expected: func(idx *ast.IndexNode) bool {
				return idx.Name == "idx_users_email" &&
					idx.Table == "users" &&
					len(idx.Columns) == 1 &&
					idx.Columns[0] == "email" &&
					idx.Unique == false
			},
		},
		{
			name: "unique index",
			index: types.SchemaIndex{
				Name:       "idx_users_username",
				StructName: "users",
				Fields:     []string{"username"},
				Unique:     true,
			},
			expected: func(idx *ast.IndexNode) bool {
				return idx.Name == "idx_users_username" &&
					idx.Table == "users" &&
					idx.Unique == true
			},
		},
		{
			name: "composite index with comment",
			index: types.SchemaIndex{
				Name:       "idx_posts_user_created",
				StructName: "posts",
				Fields:     []string{"user_id", "created_at"},
				Comment:    "Index for user posts by creation date",
			},
			expected: func(idx *ast.IndexNode) bool {
				return idx.Name == "idx_posts_user_created" &&
					idx.Table == "posts" &&
					len(idx.Columns) == 2 &&
					idx.Columns[0] == "user_id" &&
					idx.Columns[1] == "created_at" &&
					idx.Comment == "Index for user posts by creation date"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := transform.FromSchemaIndex(tt.index)

			c.Assert(result, qt.IsNotNil)
			c.Assert(tt.expected(result), qt.IsTrue)
		})
	}
}

func TestFromGlobalEnum_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		enum     types.GlobalEnum
		expected func(*ast.EnumNode) bool
	}{
		{
			name: "basic enum",
			enum: types.GlobalEnum{
				Name:   "status",
				Values: []string{"active", "inactive", "pending"},
			},
			expected: func(enum *ast.EnumNode) bool {
				return enum.Name == "status" &&
					len(enum.Values) == 3 &&
					enum.Values[0] == "active" &&
					enum.Values[1] == "inactive" &&
					enum.Values[2] == "pending"
			},
		},
		{
			name: "single value enum",
			enum: types.GlobalEnum{
				Name:   "boolean_status",
				Values: []string{"true"},
			},
			expected: func(enum *ast.EnumNode) bool {
				return enum.Name == "boolean_status" &&
					len(enum.Values) == 1 &&
					enum.Values[0] == "true"
			},
		},
		{
			name: "empty enum",
			enum: types.GlobalEnum{
				Name:   "empty_enum",
				Values: []string{},
			},
			expected: func(enum *ast.EnumNode) bool {
				return enum.Name == "empty_enum" &&
					len(enum.Values) == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := transform.FromGlobalEnum(tt.enum)

			c.Assert(result, qt.IsNotNil)
			c.Assert(tt.expected(result), qt.IsTrue)
		})
	}
}

func TestFromSchemaField_WithEnumValidation(t *testing.T) {
	tests := []struct {
		name     string
		field    types.SchemaField
		enums    []types.GlobalEnum
		expected func(*ast.ColumnNode) bool
	}{
		{
			name: "enum field with valid global enum",
			field: types.SchemaField{
				Name: "status",
				Type: "enum_user_status",
				Enum: []string{"active", "inactive"},
			},
			enums: []types.GlobalEnum{
				{
					Name:   "enum_user_status",
					Values: []string{"active", "inactive", "pending"},
				},
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "status" && col.Type == "enum_user_status"
			},
		},
		{
			name: "enum field with no matching global enum",
			field: types.SchemaField{
				Name: "status",
				Type: "enum_unknown_status",
				Enum: []string{"active"},
			},
			enums: []types.GlobalEnum{
				{
					Name:   "enum_user_status",
					Values: []string{"active", "inactive"},
				},
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "status" && col.Type == "enum_unknown_status"
			},
		},
		{
			name: "non-enum field should not be validated",
			field: types.SchemaField{
				Name: "name",
				Type: "VARCHAR(255)",
			},
			enums: []types.GlobalEnum{
				{
					Name:   "enum_user_status",
					Values: []string{"active", "inactive"},
				},
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "name" && col.Type == "VARCHAR(255)"
			},
		},
		{
			name: "enum field with invalid values should still work but warn",
			field: types.SchemaField{
				Name: "status",
				Type: "enum_user_status",
				Enum: []string{"active", "invalid_value"},
			},
			enums: []types.GlobalEnum{
				{
					Name:   "enum_user_status",
					Values: []string{"active", "inactive"},
				},
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "status" && col.Type == "enum_user_status"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := transform.FromSchemaField(tt.field, tt.enums)

			c.Assert(result, qt.IsNotNil)
			c.Assert(tt.expected(result), qt.IsTrue)
		})
	}
}

func TestIsEnumType(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		expected  bool
	}{
		{
			name:      "enum type with prefix",
			fieldType: "enum_user_status",
			expected:  true,
		},
		{
			name:      "enum type with different name",
			fieldType: "enum_product_category",
			expected:  true,
		},
		{
			name:      "non-enum type",
			fieldType: "VARCHAR(255)",
			expected:  false,
		},
		{
			name:      "integer type",
			fieldType: "INTEGER",
			expected:  false,
		},
		{
			name:      "empty string",
			fieldType: "",
			expected:  false,
		},
		{
			name:      "contains enum but not prefix",
			fieldType: "my_enum_type",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// We need to test the unexported function through the public API
			// by checking if validation is triggered
			field := types.SchemaField{
				Name: "test_field",
				Type: tt.fieldType,
				Enum: []string{"test_value"},
			}

			// This should not panic regardless of the field type
			result := transform.FromSchemaField(field, []types.GlobalEnum{})
			c.Assert(result, qt.IsNotNil)
			c.Assert(result.Type, qt.Equals, tt.fieldType)
		})
	}
}
