package fromschema_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/convert/fromschema"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
)

func TestFromField_BasicProperties(t *testing.T) {
	tests := []struct {
		name     string
		field    goschema.Field
		expected func(*ast.ColumnNode) bool
	}{
		{
			name: "basic field with name and type",
			field: goschema.Field{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Nullable: true, // Explicitly set to true
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "email" && col.Type == "VARCHAR(255)" && col.Nullable == true
			},
		},
		{
			name: "non-nullable field",
			field: goschema.Field{
				Name:     "id",
				Type:     "INTEGER",
				Nullable: false,
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "id" && col.Type == "INTEGER" && col.Nullable == false
			},
		},
		{
			name: "primary key field",
			field: goschema.Field{
				Name:    "id",
				Type:    "SERIAL",
				Primary: true,
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "id" && col.Primary == true
			},
		},
		{
			name: "unique field",
			field: goschema.Field{
				Name:   "username",
				Type:   "VARCHAR(50)",
				Unique: true,
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "username" && col.Unique == true
			},
		},
		{
			name: "auto-increment field",
			field: goschema.Field{
				Name:    "id",
				Type:    "INTEGER",
				AutoInc: true,
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "id" && col.AutoInc == true
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromField(test.field, nil, "")
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromField_DefaultValues(t *testing.T) {
	tests := []struct {
		name     string
		field    goschema.Field
		expected func(*ast.ColumnNode) bool
	}{
		{
			name: "literal default value",
			field: goschema.Field{
				Name:    "status",
				Type:    "VARCHAR(20)",
				Default: "'active'",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Default != nil && col.Default.Value == "'active'" && col.Default.Expression == ""
			},
		},
		{
			name: "expression default value",
			field: goschema.Field{
				Name:        "created_at",
				Type:        "TIMESTAMP",
				DefaultExpr: "NOW()",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Default != nil && col.Default.Expression == "NOW()" && col.Default.Value == ""
			},
		},
		{
			name: "no default value",
			field: goschema.Field{
				Name: "description",
				Type: "TEXT",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Default == nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromField(test.field, nil, "")
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromField_ForeignKeys(t *testing.T) {
	tests := []struct {
		name     string
		field    goschema.Field
		expected func(*ast.ColumnNode) bool
	}{
		{
			name: "foreign key with table and column",
			field: goschema.Field{
				Name:    "user_id",
				Type:    "INTEGER",
				Foreign: "users(id)",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.ForeignKey != nil &&
					col.ForeignKey.Table == "users" &&
					col.ForeignKey.Column == "id"
			},
		},
		{
			name: "foreign key with table only (defaults to id)",
			field: goschema.Field{
				Name:    "category_id",
				Type:    "INTEGER",
				Foreign: "categories",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.ForeignKey != nil &&
					col.ForeignKey.Table == "categories" &&
					col.ForeignKey.Column == "id"
			},
		},
		{
			name: "foreign key with custom name",
			field: goschema.Field{
				Name:           "user_id",
				Type:           "INTEGER",
				Foreign:        "users(id)",
				ForeignKeyName: "fk_posts_user",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.ForeignKey != nil &&
					col.ForeignKey.Name == "fk_posts_user"
			},
		},
		{
			name: "no foreign key",
			field: goschema.Field{
				Name: "title",
				Type: "VARCHAR(255)",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.ForeignKey == nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromField(test.field, nil, "")
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromField_EnumConversion(t *testing.T) {
	tests := []struct {
		name           string
		field          goschema.Field
		enums          []goschema.Enum
		targetPlatform string
		expectedType   string
	}{
		{
			name: "PostgreSQL keeps enum type name",
			field: goschema.Field{
				Name: "status",
				Type: "enum_user_status",
			},
			enums: []goschema.Enum{
				{Name: "enum_user_status", Values: []string{"active", "inactive", "suspended"}},
			},
			targetPlatform: "postgres",
			expectedType:   "enum_user_status",
		},
		{
			name: "MySQL converts to inline enum",
			field: goschema.Field{
				Name: "status",
				Type: "enum_user_status",
			},
			enums: []goschema.Enum{
				{Name: "enum_user_status", Values: []string{"active", "inactive", "suspended"}},
			},
			targetPlatform: "mysql",
			expectedType:   "ENUM('active', 'inactive', 'suspended')",
		},
		{
			name: "MariaDB converts to inline enum",
			field: goschema.Field{
				Name: "status",
				Type: "enum_user_status",
			},
			enums: []goschema.Enum{
				{Name: "enum_user_status", Values: []string{"active", "inactive", "suspended"}},
			},
			targetPlatform: "mariadb",
			expectedType:   "ENUM('active', 'inactive', 'suspended')",
		},
		{
			name: "Non-enum field unchanged",
			field: goschema.Field{
				Name: "name",
				Type: "VARCHAR(255)",
			},
			enums:          nil,
			targetPlatform: "mysql",
			expectedType:   "VARCHAR(255)",
		},
		{
			name: "Enum field without matching enum definition unchanged",
			field: goschema.Field{
				Name: "status",
				Type: "enum_unknown_status",
			},
			enums: []goschema.Enum{
				{Name: "enum_user_status", Values: []string{"active", "inactive"}},
			},
			targetPlatform: "mysql",
			expectedType:   "enum_unknown_status",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromField(test.field, test.enums, test.targetPlatform)
			c.Assert(result, qt.IsNotNil)
			c.Assert(result.Type, qt.Equals, test.expectedType)
		})
	}
}

func TestFromField_CheckAndComment(t *testing.T) {
	tests := []struct {
		name     string
		field    goschema.Field
		expected func(*ast.ColumnNode) bool
	}{
		{
			name: "field with check constraint",
			field: goschema.Field{
				Name:  "age",
				Type:  "INTEGER",
				Check: "age >= 0",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Check == "age >= 0"
			},
		},
		{
			name: "field with comment",
			field: goschema.Field{
				Name:    "email",
				Type:    "VARCHAR(255)",
				Comment: "User email address",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Comment == "User email address"
			},
		},
		{
			name: "field with both check and comment",
			field: goschema.Field{
				Name:    "price",
				Type:    "DECIMAL(10,2)",
				Check:   "price > 0",
				Comment: "Product price in USD",
			},
			expected: func(col *ast.ColumnNode) bool {
				return col.Check == "price > 0" && col.Comment == "Product price in USD"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromField(test.field, nil, "")
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromTable_BasicTable(t *testing.T) {
	tests := []struct {
		name     string
		table    goschema.Table
		fields   []goschema.Field
		expected func(*ast.CreateTableNode) bool
	}{
		{
			name: "basic table with columns",
			table: goschema.Table{
				StructName: "User",
				Name:       "users",
			},
			fields: []goschema.Field{
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
			name: "table with comment",
			table: goschema.Table{
				StructName: "Product",
				Name:       "products",
				Comment:    "Product catalog",
			},
			fields: []goschema.Field{
				{
					StructName: "Product",
					Name:       "id",
					Type:       "SERIAL",
					Primary:    true,
				},
			},
			expected: func(table *ast.CreateTableNode) bool {
				return table.Name == "products" &&
					table.Comment == "Product catalog" &&
					len(table.Columns) == 1
			},
		},
		{
			name: "table with engine option",
			table: goschema.Table{
				StructName: "Log",
				Name:       "logs",
				Engine:     "InnoDB",
			},
			fields: []goschema.Field{
				{
					StructName: "Log",
					Name:       "id",
					Type:       "BIGINT",
					Primary:    true,
				},
			},
			expected: func(table *ast.CreateTableNode) bool {
				return table.Name == "logs" &&
					table.Options["ENGINE"] == "InnoDB" &&
					len(table.Columns) == 1
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromTable(test.table, test.fields, nil, "")
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromTable_CompositePrimaryKey(t *testing.T) {
	c := qt.New(t)

	table := goschema.Table{
		StructName: "UserRole",
		Name:       "user_roles",
		PrimaryKey: []string{"user_id", "role_id"},
	}

	fields := []goschema.Field{
		{
			StructName: "UserRole",
			Name:       "user_id",
			Type:       "INTEGER",
			Foreign:    "users(id)",
		},
		{
			StructName: "UserRole",
			Name:       "role_id",
			Type:       "INTEGER",
			Foreign:    "roles(id)",
		},
	}

	result := fromschema.FromTable(table, fields, nil, "")

	c.Assert(result, qt.IsNotNil)
	c.Assert(result.Name, qt.Equals, "user_roles")
	c.Assert(len(result.Columns), qt.Equals, 2)
	c.Assert(len(result.Constraints), qt.Equals, 1)
	c.Assert(result.Constraints[0].Type, qt.Equals, ast.PrimaryKeyConstraint)
	c.Assert(result.Constraints[0].Columns, qt.DeepEquals, []string{"user_id", "role_id"})
}

func TestFromTable_FiltersByStructName(t *testing.T) {
	c := qt.New(t)

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
	}

	fields := []goschema.Field{
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

	result := fromschema.FromTable(table, fields, nil, "")

	c.Assert(result, qt.IsNotNil)
	c.Assert(result.Name, qt.Equals, "users")
	c.Assert(len(result.Columns), qt.Equals, 2) // Only User fields
	c.Assert(result.Columns[0].Name, qt.Equals, "id")
	c.Assert(result.Columns[1].Name, qt.Equals, "email")
}

func TestFromIndex_BasicIndex(t *testing.T) {
	tests := []struct {
		name     string
		index    goschema.Index
		expected func(*ast.IndexNode) bool
	}{
		{
			name: "simple index",
			index: goschema.Index{
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
			index: goschema.Index{
				Name:       "idx_users_username",
				StructName: "users",
				Fields:     []string{"username"},
				Unique:     true,
			},
			expected: func(idx *ast.IndexNode) bool {
				return idx.Name == "idx_users_username" &&
					idx.Unique == true
			},
		},
		{
			name: "composite index",
			index: goschema.Index{
				Name:       "idx_posts_user_created",
				StructName: "posts",
				Fields:     []string{"user_id", "created_at"},
			},
			expected: func(idx *ast.IndexNode) bool {
				return idx.Name == "idx_posts_user_created" &&
					idx.Table == "posts" &&
					len(idx.Columns) == 2 &&
					idx.Columns[0] == "user_id" &&
					idx.Columns[1] == "created_at"
			},
		},
		{
			name: "index with comment",
			index: goschema.Index{
				Name:       "idx_products_price",
				StructName: "products",
				Fields:     []string{"price"},
				Comment:    "Index for price range queries",
			},
			expected: func(idx *ast.IndexNode) bool {
				return idx.Name == "idx_products_price" &&
					idx.Comment == "Index for price range queries"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromIndex(test.index)
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromEnum_BasicEnum(t *testing.T) {
	tests := []struct {
		name     string
		enum     goschema.Enum
		expected func(*ast.EnumNode) bool
	}{
		{
			name: "simple enum",
			enum: goschema.Enum{
				Name:   "status_type",
				Values: []string{"active", "inactive", "pending"},
			},
			expected: func(enum *ast.EnumNode) bool {
				return enum.Name == "status_type" &&
					len(enum.Values) == 3 &&
					enum.Values[0] == "active" &&
					enum.Values[1] == "inactive" &&
					enum.Values[2] == "pending"
			},
		},
		{
			name: "user role enum",
			enum: goschema.Enum{
				Name:   "user_role",
				Values: []string{"admin", "moderator", "user", "guest"},
			},
			expected: func(enum *ast.EnumNode) bool {
				return enum.Name == "user_role" &&
					len(enum.Values) == 4 &&
					enum.Values[0] == "admin" &&
					enum.Values[3] == "guest"
			},
		},
		{
			name: "empty enum",
			enum: goschema.Enum{
				Name:   "empty_enum",
				Values: []string{},
			},
			expected: func(enum *ast.EnumNode) bool {
				return enum.Name == "empty_enum" &&
					len(enum.Values) == 0
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromEnum(test.enum)
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromDatabase_CompleteSchema(t *testing.T) {
	c := qt.New(t)

	database := goschema.Database{
		Enums: []goschema.Enum{
			{
				Name:   "user_status",
				Values: []string{"active", "inactive"},
			},
		},
		Tables: []goschema.Table{
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
		},
		Fields: []goschema.Field{
			{
				StructName: "User",
				Name:       "id",
				Type:       "SERIAL",
				Primary:    true,
			},
			{
				StructName: "User",
				Name:       "status",
				Type:       "user_status",
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
				Name:       "user_id",
				Type:       "INTEGER",
				Foreign:    "users(id)",
			},
		},
		Indexes: []goschema.Index{
			{
				Name:       "idx_users_status",
				StructName: "users",
				Fields:     []string{"status"},
			},
			{
				Name:       "idx_posts_user",
				StructName: "posts",
				Fields:     []string{"user_id"},
			},
		},
	}

	result := fromschema.FromDatabase(database, "")

	c.Assert(result, qt.IsNotNil)
	c.Assert(len(result.Statements), qt.Equals, 5) // 1 enum + 2 tables + 2 indexes

	// Check statement ordering: enums first, then tables, then indexes
	enumNode, ok := result.Statements[0].(*ast.EnumNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(enumNode.Name, qt.Equals, "user_status")

	table1Node, ok := result.Statements[1].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(table1Node.Name, qt.Equals, "users")

	table2Node, ok := result.Statements[2].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(table2Node.Name, qt.Equals, "posts")

	index1Node, ok := result.Statements[3].(*ast.IndexNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(index1Node.Name, qt.Equals, "idx_users_status")

	index2Node, ok := result.Statements[4].(*ast.IndexNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(index2Node.Name, qt.Equals, "idx_posts_user")
}

func TestFromDatabase_EmptySchema(t *testing.T) {
	c := qt.New(t)

	database := goschema.Database{
		Enums:   []goschema.Enum{},
		Tables:  []goschema.Table{},
		Fields:  []goschema.Field{},
		Indexes: []goschema.Index{},
	}

	result := fromschema.FromDatabase(database, "")

	c.Assert(result, qt.IsNotNil)
	c.Assert(len(result.Statements), qt.Equals, 0)
}

func TestFromField_PlatformOverrides(t *testing.T) {
	tests := []struct {
		name           string
		field          goschema.Field
		targetPlatform string
		expected       func(*ast.ColumnNode) bool
	}{
		{
			name: "MySQL type override",
			field: goschema.Field{
				Name: "data",
				Type: "JSONB",
				Overrides: map[string]map[string]string{
					"mysql": {"type": "JSON"},
				},
			},
			targetPlatform: "mysql",
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "data" && col.Type == "JSON"
			},
		},
		{
			name: "MariaDB type and check override",
			field: goschema.Field{
				Name:  "data",
				Type:  "JSONB",
				Check: "",
				Overrides: map[string]map[string]string{
					"mariadb": {
						"type":  "LONGTEXT",
						"check": "JSON_VALID(data)",
					},
				},
			},
			targetPlatform: "mariadb",
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "data" &&
					col.Type == "LONGTEXT" &&
					col.Check == "JSON_VALID(data)"
			},
		},
		{
			name: "PostgreSQL no override (uses default)",
			field: goschema.Field{
				Name: "data",
				Type: "JSONB",
				Overrides: map[string]map[string]string{
					"mysql": {"type": "JSON"},
				},
			},
			targetPlatform: "postgres",
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "data" && col.Type == "JSONB" // Uses default
			},
		},
		{
			name: "Comment override",
			field: goschema.Field{
				Name:    "status",
				Type:    "VARCHAR(20)",
				Comment: "Default comment",
				Overrides: map[string]map[string]string{
					"mysql": {"comment": "MySQL-specific comment"},
				},
			},
			targetPlatform: "mysql",
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "status" &&
					col.Comment == "MySQL-specific comment"
			},
		},
		{
			name: "Default value override",
			field: goschema.Field{
				Name:    "created_at",
				Type:    "TIMESTAMP",
				Default: "CURRENT_TIMESTAMP",
				Overrides: map[string]map[string]string{
					"postgres": {"default": "NOW()"},
				},
			},
			targetPlatform: "postgres",
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "created_at" &&
					col.Default != nil &&
					col.Default.Value == "NOW()"
			},
		},
		{
			name: "Default expression override",
			field: goschema.Field{
				Name:        "updated_at",
				Type:        "TIMESTAMP",
				DefaultExpr: "CURRENT_TIMESTAMP",
				Overrides: map[string]map[string]string{
					"mysql": {"default_expr": "NOW()"},
				},
			},
			targetPlatform: "mysql",
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "updated_at" &&
					col.Default != nil &&
					col.Default.Expression == "NOW()"
			},
		},
		{
			name: "No platform specified (uses defaults)",
			field: goschema.Field{
				Name: "data",
				Type: "JSONB",
				Overrides: map[string]map[string]string{
					"mysql": {"type": "JSON"},
				},
			},
			targetPlatform: "",
			expected: func(col *ast.ColumnNode) bool {
				return col.Name == "data" && col.Type == "JSONB" // Uses default
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromField(test.field, nil, test.targetPlatform)
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromTable_PlatformOverrides(t *testing.T) {
	tests := []struct {
		name           string
		table          goschema.Table
		fields         []goschema.Field
		targetPlatform string
		expected       func(*ast.CreateTableNode) bool
	}{
		{
			name: "MySQL engine and comment override",
			table: goschema.Table{
				StructName: "Product",
				Name:       "products",
				Comment:    "Default comment",
				Engine:     "MyISAM",
				Overrides: map[string]map[string]string{
					"mysql": {
						"engine":  "InnoDB",
						"comment": "MySQL-specific comment",
					},
				},
			},
			fields:         []goschema.Field{},
			targetPlatform: "mysql",
			expected: func(table *ast.CreateTableNode) bool {
				return table.Name == "products" &&
					table.Comment == "MySQL-specific comment" &&
					table.Options["ENGINE"] == "InnoDB"
			},
		},
		{
			name: "MariaDB charset override",
			table: goschema.Table{
				StructName: "User",
				Name:       "users",
				Overrides: map[string]map[string]string{
					"mariadb": {
						"charset":   "utf8mb4",
						"collation": "utf8mb4_unicode_ci",
					},
				},
			},
			fields:         []goschema.Field{},
			targetPlatform: "mariadb",
			expected: func(table *ast.CreateTableNode) bool {
				return table.Name == "users" &&
					table.Options["CHARSET"] == "utf8mb4" &&
					table.Options["COLLATION"] == "utf8mb4_unicode_ci"
			},
		},
		{
			name: "PostgreSQL no override (uses defaults)",
			table: goschema.Table{
				StructName: "Log",
				Name:       "logs",
				Comment:    "Default comment",
				Engine:     "InnoDB",
				Overrides: map[string]map[string]string{
					"mysql": {"engine": "MyISAM"},
				},
			},
			fields:         []goschema.Field{},
			targetPlatform: "postgres",
			expected: func(table *ast.CreateTableNode) bool {
				return table.Name == "logs" &&
					table.Comment == "Default comment" &&
					table.Options["ENGINE"] == "InnoDB" // Uses default
			},
		},
		{
			name: "No platform specified (uses defaults)",
			table: goschema.Table{
				StructName: "Category",
				Name:       "categories",
				Comment:    "Default comment",
				Overrides: map[string]map[string]string{
					"mysql": {"comment": "MySQL comment"},
				},
			},
			fields:         []goschema.Field{},
			targetPlatform: "",
			expected: func(table *ast.CreateTableNode) bool {
				return table.Name == "categories" &&
					table.Comment == "Default comment" // Uses default
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromTable(test.table, test.fields, nil, test.targetPlatform)
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromDatabase_PlatformOverrides(t *testing.T) {
	c := qt.New(t)

	database := goschema.Database{
		Tables: []goschema.Table{
			{
				StructName: "Product",
				Name:       "products",
				Overrides: map[string]map[string]string{
					"mysql": {"engine": "InnoDB"},
				},
			},
		},
		Fields: []goschema.Field{
			{
				StructName: "Product",
				Name:       "data",
				Type:       "JSONB",
				Overrides: map[string]map[string]string{
					"mysql": {"type": "JSON"},
				},
			},
		},
		Indexes: []goschema.Index{},
		Enums:   []goschema.Enum{},
	}

	// Test MySQL platform
	mysqlResult := fromschema.FromDatabase(database, "mysql")
	c.Assert(mysqlResult, qt.IsNotNil)
	c.Assert(len(mysqlResult.Statements), qt.Equals, 1)

	tableNode, ok := mysqlResult.Statements[0].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(tableNode.Name, qt.Equals, "products")
	c.Assert(tableNode.Options["ENGINE"], qt.Equals, "InnoDB")
	c.Assert(len(tableNode.Columns), qt.Equals, 1)
	c.Assert(tableNode.Columns[0].Type, qt.Equals, "JSON") // Overridden type

	// Test PostgreSQL platform (no overrides)
	postgresResult := fromschema.FromDatabase(database, "postgres")
	c.Assert(postgresResult, qt.IsNotNil)
	c.Assert(len(postgresResult.Statements), qt.Equals, 1)

	tableNode2, ok := postgresResult.Statements[0].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(tableNode2.Name, qt.Equals, "products")
	c.Assert(tableNode2.Options["ENGINE"], qt.Equals, "") // No engine for PostgreSQL
	c.Assert(len(tableNode2.Columns), qt.Equals, 1)
	c.Assert(tableNode2.Columns[0].Type, qt.Equals, "JSONB") // Default type
}

func TestFromDatabase_EmbeddedFields_InlineMode(t *testing.T) {
	tests := []struct {
		name     string
		database goschema.Database
		expected func(*ast.StatementList) bool
	}{
		{
			name: "inline mode without prefix",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "User",
						Name:       "users",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "User",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
					{
						StructName: "Timestamps",
						Name:       "created_at",
						Type:       "TIMESTAMP",
						Nullable:   false,
					},
					{
						StructName: "Timestamps",
						Name:       "updated_at",
						Type:       "TIMESTAMP",
						Nullable:   false,
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "User",
						Mode:             "inline",
						EmbeddedTypeName: "Timestamps",
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have 3 columns: id + created_at + updated_at
				return tableNode.Name == "users" &&
					len(tableNode.Columns) == 3 &&
					tableNode.Columns[0].Name == "id" &&
					tableNode.Columns[1].Name == "created_at" &&
					tableNode.Columns[2].Name == "updated_at"
			},
		},
		{
			name: "inline mode with prefix",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "Article",
						Name:       "articles",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "Article",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
					{
						StructName: "AuditInfo",
						Name:       "by",
						Type:       "TEXT",
					},
					{
						StructName: "AuditInfo",
						Name:       "reason",
						Type:       "TEXT",
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "Article",
						Mode:             "inline",
						Prefix:           "audit_",
						EmbeddedTypeName: "AuditInfo",
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have 3 columns: id + audit_by + audit_reason
				return tableNode.Name == "articles" &&
					len(tableNode.Columns) == 3 &&
					tableNode.Columns[0].Name == "id" &&
					tableNode.Columns[1].Name == "audit_by" &&
					tableNode.Columns[2].Name == "audit_reason"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromDatabase(test.database, "")
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromDatabase_EmbeddedFields_JsonMode(t *testing.T) {
	tests := []struct {
		name     string
		database goschema.Database
		expected func(*ast.StatementList) bool
	}{
		{
			name: "json mode with explicit name and type",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "User",
						Name:       "users",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "User",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "User",
						Mode:             "json",
						Name:             "metadata",
						Type:             "JSONB",
						EmbeddedTypeName: "UserMeta",
						Comment:          "User metadata in JSON format",
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have 2 columns: id + metadata
				return tableNode.Name == "users" &&
					len(tableNode.Columns) == 2 &&
					tableNode.Columns[0].Name == "id" &&
					tableNode.Columns[1].Name == "metadata" &&
					tableNode.Columns[1].Type == "JSONB" &&
					tableNode.Columns[1].Comment == "User metadata in JSON format"
			},
		},
		{
			name: "json mode with auto-generated name and default type",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "Product",
						Name:       "products",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "Product",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "Product",
						Mode:             "json",
						EmbeddedTypeName: "Meta", // Should generate "meta_data" column name
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have 2 columns: id + meta_data
				return tableNode.Name == "products" &&
					len(tableNode.Columns) == 2 &&
					tableNode.Columns[0].Name == "id" &&
					tableNode.Columns[1].Name == "meta_data" &&
					tableNode.Columns[1].Type == "JSONB" // Default type
			},
		},
		{
			name: "json mode with platform overrides",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "Article",
						Name:       "articles",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "Article",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "Article",
						Mode:             "json",
						Name:             "content_data",
						Type:             "JSONB",
						EmbeddedTypeName: "Content",
						Overrides: map[string]map[string]string{
							"mysql":   {"type": "JSON"},
							"mariadb": {"type": "LONGTEXT"},
						},
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have 2 columns: id + content_data
				return tableNode.Name == "articles" &&
					len(tableNode.Columns) == 2 &&
					tableNode.Columns[0].Name == "id" &&
					tableNode.Columns[1].Name == "content_data" &&
					tableNode.Columns[1].Type == "JSONB" // Default type (no platform specified)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromDatabase(test.database, "")
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromDatabase_EmbeddedFields_RelationMode(t *testing.T) {
	tests := []struct {
		name     string
		database goschema.Database
		expected func(*ast.StatementList) bool
	}{
		{
			name: "relation mode with integer reference",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "Post",
						Name:       "posts",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "Post",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "Post",
						Mode:             "relation",
						Field:            "user_id",
						Ref:              "users(id)",
						EmbeddedTypeName: "User",
						Comment:          "Reference to user",
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have 2 columns: id + user_id
				return tableNode.Name == "posts" &&
					len(tableNode.Columns) == 2 &&
					tableNode.Columns[0].Name == "id" &&
					tableNode.Columns[1].Name == "user_id" &&
					tableNode.Columns[1].Type == "INTEGER" &&
					tableNode.Columns[1].ForeignKey != nil &&
					tableNode.Columns[1].ForeignKey.Table == "users" &&
					tableNode.Columns[1].ForeignKey.Column == "id" &&
					tableNode.Columns[1].Comment == "Reference to user"
			},
		},
		{
			name: "relation mode with UUID reference",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "Order",
						Name:       "orders",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "Order",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "Order",
						Mode:             "relation",
						Field:            "customer_uuid",
						Ref:              "customers(uuid)",
						EmbeddedTypeName: "Customer",
						Nullable:         true,
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have 2 columns: id + customer_uuid
				return tableNode.Name == "orders" &&
					len(tableNode.Columns) == 2 &&
					tableNode.Columns[0].Name == "id" &&
					tableNode.Columns[1].Name == "customer_uuid" &&
					tableNode.Columns[1].Type == "VARCHAR(36)" && // UUID type inference
					tableNode.Columns[1].Nullable == true &&
					tableNode.Columns[1].ForeignKey != nil &&
					tableNode.Columns[1].ForeignKey.Table == "customers" &&
					tableNode.Columns[1].ForeignKey.Column == "uuid"
			},
		},
		{
			name: "relation mode with incomplete definition (should be skipped)",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "Comment",
						Name:       "comments",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "Comment",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "Comment",
						Mode:             "relation",
						Field:            "", // Missing field name
						Ref:              "posts(id)",
						EmbeddedTypeName: "Post",
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have only 1 column: id (relation field skipped)
				return tableNode.Name == "comments" &&
					len(tableNode.Columns) == 1 &&
					tableNode.Columns[0].Name == "id"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromDatabase(test.database, "")
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromDatabase_EmbeddedFields_SkipAndDefaultModes(t *testing.T) {
	tests := []struct {
		name     string
		database goschema.Database
		expected func(*ast.StatementList) bool
	}{
		{
			name: "skip mode ignores embedded field",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "User",
						Name:       "users",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "User",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
					{
						StructName: "Internal",
						Name:       "debug_info",
						Type:       "TEXT",
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "User",
						Mode:             "skip",
						EmbeddedTypeName: "Internal",
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have only 1 column: id (Internal fields skipped)
				return tableNode.Name == "users" &&
					len(tableNode.Columns) == 1 &&
					tableNode.Columns[0].Name == "id"
			},
		},
		{
			name: "default mode falls back to inline behavior",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "User",
						Name:       "users",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "User",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
					{
						StructName: "Timestamps",
						Name:       "created_at",
						Type:       "TIMESTAMP",
						Nullable:   false,
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "User",
						Mode:             "", // Empty mode should default to inline
						EmbeddedTypeName: "Timestamps",
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have 2 columns: id + created_at (inline behavior)
				return tableNode.Name == "users" &&
					len(tableNode.Columns) == 2 &&
					tableNode.Columns[0].Name == "id" &&
					tableNode.Columns[1].Name == "created_at"
			},
		},
		{
			name: "unrecognized mode falls back to inline behavior",
			database: goschema.Database{
				Tables: []goschema.Table{
					{
						StructName: "Product",
						Name:       "products",
					},
				},
				Fields: []goschema.Field{
					{
						StructName: "Product",
						Name:       "id",
						Type:       "SERIAL",
						Primary:    true,
					},
					{
						StructName: "Audit",
						Name:       "created_by",
						Type:       "VARCHAR(100)",
					},
				},
				EmbeddedFields: []goschema.EmbeddedField{
					{
						StructName:       "Product",
						Mode:             "unknown_mode", // Unrecognized mode
						EmbeddedTypeName: "Audit",
					},
				},
			},
			expected: func(result *ast.StatementList) bool {
				if len(result.Statements) != 1 {
					return false
				}
				tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				// Should have 2 columns: id + created_by (inline behavior)
				return tableNode.Name == "products" &&
					len(tableNode.Columns) == 2 &&
					tableNode.Columns[0].Name == "id" &&
					tableNode.Columns[1].Name == "created_by"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := fromschema.FromDatabase(test.database, "")
			c.Assert(result, qt.IsNotNil)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestFromDatabase_EmbeddedFields_ComplexScenario(t *testing.T) {
	c := qt.New(t)

	// Complex scenario with multiple embedded fields using different modes
	database := goschema.Database{
		Tables: []goschema.Table{
			{
				StructName: "Article",
				Name:       "articles",
				Comment:    "Blog articles",
			},
		},
		Fields: []goschema.Field{
			// Article fields
			{
				StructName: "Article",
				Name:       "id",
				Type:       "SERIAL",
				Primary:    true,
			},
			{
				StructName: "Article",
				Name:       "title",
				Type:       "VARCHAR(255)",
				Nullable:   false,
			},
			// Timestamps fields (for inline mode)
			{
				StructName: "Timestamps",
				Name:       "created_at",
				Type:       "TIMESTAMP",
				Nullable:   false,
			},
			{
				StructName: "Timestamps",
				Name:       "updated_at",
				Type:       "TIMESTAMP",
				Nullable:   false,
			},
			// AuditInfo fields (for inline mode with prefix)
			{
				StructName: "AuditInfo",
				Name:       "by",
				Type:       "TEXT",
			},
			{
				StructName: "AuditInfo",
				Name:       "reason",
				Type:       "TEXT",
			},
		},
		EmbeddedFields: []goschema.EmbeddedField{
			// Mode 1: inline without prefix
			{
				StructName:       "Article",
				Mode:             "inline",
				EmbeddedTypeName: "Timestamps",
			},
			// Mode 2: inline with prefix
			{
				StructName:       "Article",
				Mode:             "inline",
				Prefix:           "audit_",
				EmbeddedTypeName: "AuditInfo",
			},
			// Mode 3: json mode
			{
				StructName:       "Article",
				Mode:             "json",
				Name:             "meta_data",
				Type:             "JSONB",
				EmbeddedTypeName: "Meta",
				Comment:          "Article metadata",
			},
			// Mode 4: relation mode
			{
				StructName:       "Article",
				Mode:             "relation",
				Field:            "author_id",
				Ref:              "users(id)",
				EmbeddedTypeName: "User",
				Comment:          "Article author",
			},
			// Mode 5: skip mode
			{
				StructName:       "Article",
				Mode:             "skip",
				EmbeddedTypeName: "Internal",
			},
		},
	}

	result := fromschema.FromDatabase(database, "")

	c.Assert(result, qt.IsNotNil)
	c.Assert(len(result.Statements), qt.Equals, 1)

	tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(tableNode.Name, qt.Equals, "articles")
	c.Assert(tableNode.Comment, qt.Equals, "Blog articles")

	// Should have 8 columns total:
	// 1. id (original)
	// 2. title (original)
	// 3. created_at (from Timestamps inline)
	// 4. updated_at (from Timestamps inline)
	// 5. audit_by (from AuditInfo inline with prefix)
	// 6. audit_reason (from AuditInfo inline with prefix)
	// 7. meta_data (from Meta json mode)
	// 8. author_id (from User relation mode)
	// Note: Internal fields are skipped
	c.Assert(len(tableNode.Columns), qt.Equals, 8)

	// Verify each column
	columns := make(map[string]*ast.ColumnNode)
	for _, col := range tableNode.Columns {
		columns[col.Name] = col
	}

	// Original fields
	c.Assert(columns["id"], qt.IsNotNil)
	c.Assert(columns["id"].Type, qt.Equals, "SERIAL")
	c.Assert(columns["id"].Primary, qt.IsTrue)

	c.Assert(columns["title"], qt.IsNotNil)
	c.Assert(columns["title"].Type, qt.Equals, "VARCHAR(255)")
	c.Assert(columns["title"].Nullable, qt.IsFalse)

	// Inline mode fields
	c.Assert(columns["created_at"], qt.IsNotNil)
	c.Assert(columns["created_at"].Type, qt.Equals, "TIMESTAMP")

	c.Assert(columns["updated_at"], qt.IsNotNil)
	c.Assert(columns["updated_at"].Type, qt.Equals, "TIMESTAMP")

	// Inline mode with prefix fields
	c.Assert(columns["audit_by"], qt.IsNotNil)
	c.Assert(columns["audit_by"].Type, qt.Equals, "TEXT")

	c.Assert(columns["audit_reason"], qt.IsNotNil)
	c.Assert(columns["audit_reason"].Type, qt.Equals, "TEXT")

	// JSON mode field
	c.Assert(columns["meta_data"], qt.IsNotNil)
	c.Assert(columns["meta_data"].Type, qt.Equals, "JSONB")
	c.Assert(columns["meta_data"].Comment, qt.Equals, "Article metadata")

	// Relation mode field
	c.Assert(columns["author_id"], qt.IsNotNil)
	c.Assert(columns["author_id"].Type, qt.Equals, "INTEGER")
	c.Assert(columns["author_id"].Comment, qt.Equals, "Article author")
	c.Assert(columns["author_id"].ForeignKey, qt.IsNotNil)
	c.Assert(columns["author_id"].ForeignKey.Table, qt.Equals, "users")
	c.Assert(columns["author_id"].ForeignKey.Column, qt.Equals, "id")
}
