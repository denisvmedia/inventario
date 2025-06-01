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
