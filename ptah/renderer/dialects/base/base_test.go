package base_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/renderer/dialects/base"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

func TestGenerator_NewGenerator(t *testing.T) {
	tests := []struct {
		name        string
		dialectName string
	}{
		{"postgres dialect", "postgres"},
		{"mysql dialect", "mysql"},
		{"custom dialect", "custom"},
		{"empty dialect", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			generator := base.NewGenerator(tt.dialectName)
			c.Assert(generator, qt.IsNotNil)
			c.Assert(generator.GetDialectName(), qt.Equals, tt.dialectName)
		})
	}
}

func TestGenerator_GetDialectName(t *testing.T) {
	c := qt.New(t)

	generator := base.NewGenerator("test-dialect")
	c.Assert(generator.GetDialectName(), qt.Equals, "test-dialect")
}

func TestGenerator_GenerateTableComment(t *testing.T) {
	tests := []struct {
		name        string
		dialectName string
		tableName   string
		expected    string
	}{
		{
			name:        "postgres table comment",
			dialectName: "postgres",
			tableName:   "users",
			expected:    "-- POSTGRES TABLE: users --",
		},
		{
			name:        "mysql table comment",
			dialectName: "mysql",
			tableName:   "products",
			expected:    "-- MYSQL TABLE: products --",
		},
		{
			name:        "empty dialect",
			dialectName: "",
			tableName:   "test",
			expected:    "--  TABLE: test --",
		},
		{
			name:        "empty table name",
			dialectName: "postgres",
			tableName:   "",
			expected:    "-- POSTGRES TABLE:  --",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			generator := base.NewGenerator(tt.dialectName)
			comment := generator.GenerateTableComment(tt.tableName)

			c.Assert(comment, qt.IsNotNil)
			c.Assert(comment.Text, qt.Equals, tt.expected)
		})
	}
}

func TestGenerator_GenerateColumn(t *testing.T) {
	generator := base.NewGenerator("test")

	tests := []struct {
		name      string
		field     types.SchemaField
		fieldType string
		enums     []types.GlobalEnum
	}{
		{
			name: "basic string field",
			field: types.SchemaField{
				StructName: "User",
				Name:       "name",
				Type:       "VARCHAR(255)",
				Nullable:   false,
			},
			fieldType: "VARCHAR(255)",
		},
		{
			name: "primary key field",
			field: types.SchemaField{
				StructName: "User",
				Name:       "id",
				Type:       "SERIAL",
				Primary:    true,
			},
			fieldType: "SERIAL",
		},
		{
			name: "nullable field",
			field: types.SchemaField{
				StructName: "User",
				Name:       "email",
				Type:       "VARCHAR(320)",
				Nullable:   true,
			},
			fieldType: "VARCHAR(320)",
		},
		{
			name: "enum field",
			field: types.SchemaField{
				StructName: "User",
				Name:       "status",
				Type:       "user_status_enum",
				Nullable:   false,
			},
			fieldType: "user_status_enum",
			enums: []types.GlobalEnum{
				{
					Name:   "user_status_enum",
					Values: []string{"active", "inactive"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			column := generator.GenerateColumn(tt.field, tt.fieldType, tt.enums)
			c.Assert(column, qt.IsNotNil)
			c.Assert(column.Name, qt.Equals, tt.field.Name)
			c.Assert(column.Type, qt.Equals, tt.field.Type)
			c.Assert(column.Nullable, qt.Equals, tt.field.Nullable)
			c.Assert(column.Primary, qt.Equals, tt.field.Primary)
		})
	}
}

func TestGenerator_GenerateCreateTable(t *testing.T) {
	c := qt.New(t)

	generator := base.NewGenerator("test")

	table := types.TableDirective{
		StructName: "User",
		Name:       "users",
		Comment:    "User accounts table",
	}

	fields := []types.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
		{
			StructName: "Post", // Different struct - should be ignored
			Name:       "title",
			Type:       "VARCHAR(255)",
		},
	}

	enums := []types.GlobalEnum{
		{
			Name:   "user_status_enum",
			Values: []string{"active", "inactive"},
		},
	}

	createTable := generator.GenerateCreateTable(table, fields, enums)

	c.Assert(createTable, qt.IsNotNil)
	c.Assert(createTable.Name, qt.Equals, "users")
	c.Assert(createTable.Comment, qt.Equals, "User accounts table")
	c.Assert(createTable.Columns, qt.HasLen, 2) // Only User struct fields

	// Check that only User struct fields are included
	columnNames := make([]string, len(createTable.Columns))
	for i, col := range createTable.Columns {
		columnNames[i] = col.Name
	}
	c.Assert(columnNames, qt.Contains, "id")
	c.Assert(columnNames, qt.Contains, "name")
	c.Assert(columnNames, qt.Not(qt.Contains), "title") // Post struct field should be excluded
}

func TestGenerator_GenerateIndexes(t *testing.T) {
	c := qt.New(t)

	generator := base.NewGenerator("test")

	table := types.TableDirective{
		StructName: "User",
		Name:       "users",
	}

	indexes := []types.SchemaIndex{
		{
			StructName: "User",
			Name:       "idx_users_email",
			Fields:     []string{"email"},
			Unique:     true,
		},
		{
			StructName: "User",
			Name:       "idx_users_name",
			Fields:     []string{"name"},
			Unique:     false,
		},
		{
			StructName: "Post", // Different struct - should be ignored
			Name:       "idx_posts_title",
			Fields:     []string{"title"},
		},
	}

	indexNodes := generator.GenerateIndexes(table, indexes)

	c.Assert(indexNodes, qt.HasLen, 2) // Only User struct indexes

	// Check index properties
	c.Assert(indexNodes[0].Name, qt.Equals, "idx_users_email")
	c.Assert(indexNodes[0].Unique, qt.IsTrue)
	c.Assert(indexNodes[0].Columns, qt.DeepEquals, []string{"email"})

	c.Assert(indexNodes[1].Name, qt.Equals, "idx_users_name")
	c.Assert(indexNodes[1].Unique, qt.IsFalse)
	c.Assert(indexNodes[1].Columns, qt.DeepEquals, []string{"name"})
}

func TestGenerator_GeneratePrimaryKeyConstraint(t *testing.T) {
	generator := base.NewGenerator("test")

	tests := []struct {
		name       string
		table      types.TableDirective
		expectNil  bool
		primaryKey []string
	}{
		{
			name: "single primary key - no constraint needed",
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
				PrimaryKey: []string{"id"},
			},
			expectNil: true,
		},
		{
			name: "composite primary key - constraint needed",
			table: types.TableDirective{
				StructName: "UserRole",
				Name:       "user_roles",
				PrimaryKey: []string{"user_id", "role_id"},
			},
			expectNil:  false,
			primaryKey: []string{"user_id", "role_id"},
		},
		{
			name: "no primary key defined",
			table: types.TableDirective{
				StructName: "Log",
				Name:       "logs",
				PrimaryKey: []string{},
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			constraint := generator.GeneratePrimaryKeyConstraint(tt.table)

			if tt.expectNil {
				c.Assert(constraint, qt.IsNil)
			} else {
				c.Assert(constraint, qt.IsNotNil)
				c.Assert(constraint.Type.String(), qt.Equals, "PRIMARY KEY")
				c.Assert(constraint.Columns, qt.DeepEquals, tt.primaryKey)
			}
		})
	}
}

func TestGenerator_GenerateSchema(t *testing.T) {
	c := qt.New(t)

	generator := base.NewGenerator("postgres")

	tables := []types.TableDirective{
		{
			StructName: "User",
			Name:       "users",
			Comment:    "User accounts",
		},
		{
			StructName: "Post",
			Name:       "posts",
			Comment:    "Blog posts",
		},
	}

	fields := []types.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
		{
			StructName: "Post",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "Post",
			Name:       "title",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
	}

	indexes := []types.SchemaIndex{
		{
			StructName: "User",
			Name:       "idx_users_name",
			Fields:     []string{"name"},
		},
	}

	enums := []types.GlobalEnum{
		{
			Name:   "user_status_enum",
			Values: []string{"active", "inactive"},
		},
	}

	schema := generator.GenerateSchema(tables, fields, indexes, enums)

	c.Assert(schema, qt.IsNotNil)
	c.Assert(schema.Statements, qt.Not(qt.HasLen), 0)

	// Check that schema contains expected elements
	// Note: The exact structure depends on the builders implementation
	// We're mainly testing that the method doesn't panic and returns a valid schema
}

func TestHelperFunctions(t *testing.T) {
	t.Run("QuoteList", func(t *testing.T) {
		tests := []struct {
			name     string
			input    []string
			expected []string
		}{
			{
				name:     "simple strings",
				input:    []string{"active", "inactive"},
				expected: []string{"'active'", "'inactive'"},
			},
			{
				name:     "strings with single quotes",
				input:    []string{"can't", "won't"},
				expected: []string{"'can''t'", "'won''t'"},
			},
			{
				name:     "empty list",
				input:    []string{},
				expected: []string{},
			},
			{
				name:     "empty strings",
				input:    []string{"", "test"},
				expected: []string{"''", "'test'"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := qt.New(t)
				result := base.QuoteList(tt.input)
				c.Assert(result, qt.DeepEquals, tt.expected)
			})
		}
	})

	t.Run("JoinSeps", func(t *testing.T) {
		tests := []struct {
			name     string
			input    []string
			sep      string
			expected string
		}{
			{
				name:     "comma separated",
				input:    []string{"a", "b", "c"},
				sep:      ", ",
				expected: "a, b, c",
			},
			{
				name:     "pipe separated",
				input:    []string{"x", "y"},
				sep:      " | ",
				expected: "x | y",
			},
			{
				name:     "empty list",
				input:    []string{},
				sep:      ",",
				expected: "",
			},
			{
				name:     "single item",
				input:    []string{"only"},
				sep:      ",",
				expected: "only",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := qt.New(t)
				result := base.JoinSeps(tt.input, tt.sep)
				c.Assert(result, qt.Equals, tt.expected)
			})
		}
	})
}
