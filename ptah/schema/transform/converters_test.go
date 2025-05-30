package transform_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
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
				Name:      "created_at",
				Type:      "TIMESTAMP",
				DefaultFn: "NOW()",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Default != nil && col.Default.Function == "NOW()"
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
