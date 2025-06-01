package toschema_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/convert/toschema"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
)

func TestToField_BasicProperties(t *testing.T) {
	tests := []struct {
		name       string
		column     *ast.ColumnNode
		structName string
		expected   func(goschema.Field) bool
	}{
		{
			name:       "basic field with name and type",
			column:     ast.NewColumn("email", "VARCHAR(255)"),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.Name == "email" &&
					field.Type == "VARCHAR(255)" &&
					field.StructName == "User" &&
					field.Nullable == true // Default nullable
			},
		},
		{
			name:       "non-nullable field",
			column:     ast.NewColumn("id", "INTEGER").SetNotNull(),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.Name == "id" &&
					field.Type == "INTEGER" &&
					field.Nullable == false
			},
		},
		{
			name:       "primary key field",
			column:     ast.NewColumn("id", "SERIAL").SetPrimary(),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.Name == "id" &&
					field.Primary == true
			},
		},
		{
			name:       "unique field",
			column:     ast.NewColumn("username", "VARCHAR(50)").SetUnique(),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.Name == "username" &&
					field.Unique == true
			},
		},
		{
			name:       "auto-increment field",
			column:     ast.NewColumn("id", "INTEGER").SetAutoIncrement(),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.Name == "id" &&
					field.AutoInc == true
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := toschema.ToField(test.column, test.structName, "")
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestToField_DefaultValues(t *testing.T) {
	tests := []struct {
		name       string
		column     *ast.ColumnNode
		structName string
		expected   func(goschema.Field) bool
	}{
		{
			name:       "literal default value",
			column:     ast.NewColumn("status", "VARCHAR(20)").SetDefault("'active'"),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.Default == "'active'" && field.DefaultExpr == ""
			},
		},
		{
			name:       "expression default value",
			column:     ast.NewColumn("created_at", "TIMESTAMP").SetDefaultExpression("NOW()"),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.DefaultExpr == "NOW()" && field.Default == ""
			},
		},
		{
			name:       "no default value",
			column:     ast.NewColumn("description", "TEXT"),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.Default == "" && field.DefaultExpr == ""
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := toschema.ToField(test.column, test.structName, "")
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestToField_ForeignKeys(t *testing.T) {
	tests := []struct {
		name       string
		column     *ast.ColumnNode
		structName string
		expected   func(goschema.Field) bool
	}{
		{
			name:       "foreign key with table and column",
			column:     ast.NewColumn("user_id", "INTEGER").SetForeignKey("users", "id", ""),
			structName: "Post",
			expected: func(field goschema.Field) bool {
				return field.Foreign == "users(id)" && field.ForeignKeyName == ""
			},
		},
		{
			name:       "foreign key with custom name",
			column:     ast.NewColumn("user_id", "INTEGER").SetForeignKey("users", "id", "fk_posts_user"),
			structName: "Post",
			expected: func(field goschema.Field) bool {
				return field.Foreign == "users(id)" && field.ForeignKeyName == "fk_posts_user"
			},
		},
		{
			name:       "foreign key with table only",
			column:     ast.NewColumn("category_id", "INTEGER").SetForeignKey("categories", "", ""),
			structName: "Product",
			expected: func(field goschema.Field) bool {
				return field.Foreign == "categories"
			},
		},
		{
			name:       "no foreign key",
			column:     ast.NewColumn("title", "VARCHAR(255)"),
			structName: "Post",
			expected: func(field goschema.Field) bool {
				return field.Foreign == ""
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := toschema.ToField(test.column, test.structName, "")
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestToField_CheckAndComment(t *testing.T) {
	tests := []struct {
		name       string
		column     *ast.ColumnNode
		structName string
		expected   func(goschema.Field) bool
	}{
		{
			name:       "field with check constraint",
			column:     ast.NewColumn("age", "INTEGER").SetCheck("age >= 0"),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.Check == "age >= 0"
			},
		},
		{
			name:       "field with comment",
			column:     ast.NewColumn("email", "VARCHAR(255)").SetComment("User email address"),
			structName: "User",
			expected: func(field goschema.Field) bool {
				return field.Comment == "User email address"
			},
		},
		{
			name: "field with both check and comment",
			column: ast.NewColumn("price", "DECIMAL(10,2)").
				SetCheck("price > 0").
				SetComment("Product price in USD"),
			structName: "Product",
			expected: func(field goschema.Field) bool {
				return field.Check == "price > 0" && field.Comment == "Product price in USD"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := toschema.ToField(test.column, test.structName, "")
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestToField_PlatformSource(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("data", "JSON")
	result := toschema.ToField(column, "Product", "mysql")

	c.Assert(result.Name, qt.Equals, "data")
	c.Assert(result.Type, qt.Equals, "JSON")
	c.Assert(result.StructName, qt.Equals, "Product")
	c.Assert(result.Overrides, qt.IsNotNil)
	c.Assert(len(result.Overrides), qt.Equals, 0) // Empty until merged
}

func TestToTable_BasicTable(t *testing.T) {
	tests := []struct {
		name     string
		table    *ast.CreateTableNode
		expected func(goschema.Table) bool
	}{
		{
			name:  "basic table",
			table: ast.NewCreateTable("users"),
			expected: func(table goschema.Table) bool {
				return table.Name == "users" &&
					table.StructName == "User" &&
					table.Comment == "" &&
					table.Engine == ""
			},
		},
		{
			name: "table with comment",
			table: func() *ast.CreateTableNode {
				table := ast.NewCreateTable("products")
				table.Comment = "Product catalog"
				return table
			}(),
			expected: func(table goschema.Table) bool {
				return table.Name == "products" &&
					table.StructName == "Product" &&
					table.Comment == "Product catalog"
			},
		},
		{
			name:  "table with engine",
			table: ast.NewCreateTable("logs").SetOption("ENGINE", "InnoDB"),
			expected: func(table goschema.Table) bool {
				return table.Name == "logs" &&
					table.StructName == "Log" &&
					table.Engine == "InnoDB"
			},
		},
		{
			name: "table with multiple options",
			table: func() *ast.CreateTableNode {
				table := ast.NewCreateTable("products").
					SetOption("ENGINE", "InnoDB").
					SetOption("CHARSET", "utf8mb4")
				table.Comment = "Product catalog"
				return table
			}(),
			expected: func(table goschema.Table) bool {
				return table.Name == "products" &&
					table.Engine == "InnoDB" &&
					table.Comment == "Product catalog"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := toschema.ToTable(test.table, "")
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestToTable_CompositePrimaryKey(t *testing.T) {
	c := qt.New(t)

	table := ast.NewCreateTable("user_roles").
		AddConstraint(ast.NewPrimaryKeyConstraint("user_id", "role_id"))

	result := toschema.ToTable(table, "")

	c.Assert(result.Name, qt.Equals, "user_roles")
	c.Assert(result.StructName, qt.Equals, "UserRole")
	c.Assert(len(result.PrimaryKey), qt.Equals, 2)
	c.Assert(result.PrimaryKey[0], qt.Equals, "user_id")
	c.Assert(result.PrimaryKey[1], qt.Equals, "role_id")
}

func TestToTable_PlatformSource(t *testing.T) {
	c := qt.New(t)

	table := ast.NewCreateTable("products").
		SetOption("ENGINE", "InnoDB").
		SetOption("CHARSET", "utf8mb4")
	table.Comment = "Product catalog"

	result := toschema.ToTable(table, "mysql")

	c.Assert(result.Name, qt.Equals, "products")
	c.Assert(result.Engine, qt.Equals, "InnoDB")
	c.Assert(result.Comment, qt.Equals, "Product catalog")
	c.Assert(result.Overrides, qt.IsNotNil)
	c.Assert(len(result.Overrides), qt.Equals, 1)
	c.Assert(result.Overrides["mysql"]["engine"], qt.Equals, "InnoDB")
	c.Assert(result.Overrides["mysql"]["charset"], qt.Equals, "utf8mb4")
	c.Assert(result.Overrides["mysql"]["comment"], qt.Equals, "Product catalog")
}

func TestToIndex_BasicIndex(t *testing.T) {
	tests := []struct {
		name     string
		index    *ast.IndexNode
		expected func(goschema.Index) bool
	}{
		{
			name:  "simple index",
			index: ast.NewIndex("idx_users_email", "users", "email"),
			expected: func(index goschema.Index) bool {
				return index.Name == "idx_users_email" &&
					index.StructName == "users" &&
					len(index.Fields) == 1 &&
					index.Fields[0] == "email" &&
					index.Unique == false
			},
		},
		{
			name:  "unique index",
			index: ast.NewIndex("idx_users_username", "users", "username").SetUnique(),
			expected: func(index goschema.Index) bool {
				return index.Name == "idx_users_username" &&
					index.Unique == true
			},
		},
		{
			name:  "composite index",
			index: ast.NewIndex("idx_posts_user_created", "posts", "user_id", "created_at"),
			expected: func(index goschema.Index) bool {
				return index.Name == "idx_posts_user_created" &&
					index.StructName == "posts" &&
					len(index.Fields) == 2 &&
					index.Fields[0] == "user_id" &&
					index.Fields[1] == "created_at"
			},
		},
		{
			name: "index with comment",
			index: func() *ast.IndexNode {
				index := ast.NewIndex("idx_products_price", "products", "price")
				index.Comment = "Index for price range queries"
				return index
			}(),
			expected: func(index goschema.Index) bool {
				return index.Name == "idx_products_price" &&
					index.Comment == "Index for price range queries"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := toschema.ToIndex(test.index)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestToEnum_BasicEnum(t *testing.T) {
	tests := []struct {
		name     string
		enum     *ast.EnumNode
		expected func(goschema.Enum) bool
	}{
		{
			name: "simple enum",
			enum: ast.NewEnum("status_type", "active", "inactive", "pending"),
			expected: func(enum goschema.Enum) bool {
				return enum.Name == "status_type" &&
					len(enum.Values) == 3 &&
					enum.Values[0] == "active" &&
					enum.Values[1] == "inactive" &&
					enum.Values[2] == "pending"
			},
		},
		{
			name: "user role enum",
			enum: ast.NewEnum("user_role", "admin", "moderator", "user", "guest"),
			expected: func(enum goschema.Enum) bool {
				return enum.Name == "user_role" &&
					len(enum.Values) == 4 &&
					enum.Values[0] == "admin" &&
					enum.Values[3] == "guest"
			},
		},
		{
			name: "empty enum",
			enum: ast.NewEnum("empty_enum"),
			expected: func(enum goschema.Enum) bool {
				return enum.Name == "empty_enum" &&
					len(enum.Values) == 0
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			result := toschema.ToEnum(test.enum)
			c.Assert(test.expected(result), qt.IsTrue)
		})
	}
}

func TestToDatabase_CompleteSchema(t *testing.T) {
	c := qt.New(t)

	usersTable := ast.NewCreateTable("users").
		AddColumn(ast.NewColumn("id", "SERIAL").SetPrimary()).
		AddColumn(ast.NewColumn("status", "user_status").SetNotNull())
	usersTable.Comment = "User accounts"

	statements := &ast.StatementList{
		Statements: []ast.Node{
			ast.NewEnum("user_status", "active", "inactive"),
			usersTable,
			ast.NewCreateTable("posts").
				AddColumn(ast.NewColumn("id", "SERIAL").SetPrimary()).
				AddColumn(ast.NewColumn("user_id", "INTEGER").SetForeignKey("users", "id", "")),
			ast.NewIndex("idx_users_status", "users", "status"),
			ast.NewIndex("idx_posts_user", "posts", "user_id"),
		},
	}

	result := toschema.ToDatabase(statements)

	c.Assert(len(result.Enums), qt.Equals, 1)
	c.Assert(result.Enums[0].Name, qt.Equals, "user_status")
	c.Assert(len(result.Enums[0].Values), qt.Equals, 2)

	c.Assert(len(result.Tables), qt.Equals, 2)
	c.Assert(result.Tables[0].Name, qt.Equals, "users")
	c.Assert(result.Tables[0].StructName, qt.Equals, "User")
	c.Assert(result.Tables[1].Name, qt.Equals, "posts")
	c.Assert(result.Tables[1].StructName, qt.Equals, "Post")

	c.Assert(len(result.Fields), qt.Equals, 4) // 2 fields per table
	c.Assert(result.Fields[0].Name, qt.Equals, "id")
	c.Assert(result.Fields[0].StructName, qt.Equals, "User")
	c.Assert(result.Fields[1].Name, qt.Equals, "status")
	c.Assert(result.Fields[1].StructName, qt.Equals, "User")
	c.Assert(result.Fields[2].Name, qt.Equals, "id")
	c.Assert(result.Fields[2].StructName, qt.Equals, "Post")
	c.Assert(result.Fields[3].Name, qt.Equals, "user_id")
	c.Assert(result.Fields[3].StructName, qt.Equals, "Post")
	c.Assert(result.Fields[3].Foreign, qt.Equals, "users(id)")

	c.Assert(len(result.Indexes), qt.Equals, 2)
	c.Assert(result.Indexes[0].Name, qt.Equals, "idx_users_status")
	c.Assert(result.Indexes[1].Name, qt.Equals, "idx_posts_user")
}

func TestToDatabase_EmptySchema(t *testing.T) {
	c := qt.New(t)

	statements := &ast.StatementList{
		Statements: []ast.Node{},
	}

	result := toschema.ToDatabase(statements)

	c.Assert(len(result.Enums), qt.Equals, 0)
	c.Assert(len(result.Tables), qt.Equals, 0)
	c.Assert(len(result.Fields), qt.Equals, 0)
	c.Assert(len(result.Indexes), qt.Equals, 0)
}

func TestMergeFieldOverrides_BasicMerging(t *testing.T) {
	c := qt.New(t)

	baseField := goschema.Field{
		Name:    "data",
		Type:    "JSONB",
		Comment: "JSON data",
	}

	platformFields := map[string]goschema.Field{
		"mysql": {
			Name:    "data",
			Type:    "JSON",
			Comment: "JSON data",
		},
		"mariadb": {
			Name:    "data",
			Type:    "LONGTEXT",
			Check:   "JSON_VALID(data)",
			Comment: "JSON data",
		},
	}

	result := toschema.MergeFieldOverrides(baseField, platformFields)

	c.Assert(result.Name, qt.Equals, "data")
	c.Assert(result.Type, qt.Equals, "JSONB") // Base type unchanged
	c.Assert(result.Overrides, qt.IsNotNil)
	c.Assert(len(result.Overrides), qt.Equals, 2)

	// Check MySQL overrides
	c.Assert(result.Overrides["mysql"]["type"], qt.Equals, "JSON")
	c.Assert(result.Overrides["mysql"]["check"], qt.Equals, "") // No check override
	c.Assert(result.Overrides["mysql"]["comment"], qt.Equals, "") // Same as base

	// Check MariaDB overrides
	c.Assert(result.Overrides["mariadb"]["type"], qt.Equals, "LONGTEXT")
	c.Assert(result.Overrides["mariadb"]["check"], qt.Equals, "JSON_VALID(data)")
}

func TestMergeFieldOverrides_DefaultValues(t *testing.T) {
	c := qt.New(t)

	baseField := goschema.Field{
		Name:        "created_at",
		Type:        "TIMESTAMP",
		DefaultExpr: "CURRENT_TIMESTAMP",
	}

	platformFields := map[string]goschema.Field{
		"mysql": {
			Name:        "created_at",
			Type:        "TIMESTAMP",
			DefaultExpr: "NOW()",
		},
		"postgres": {
			Name:    "created_at",
			Type:    "TIMESTAMP",
			Default: "NOW()",
		},
	}

	result := toschema.MergeFieldOverrides(baseField, platformFields)

	c.Assert(result.Overrides["mysql"]["default_expr"], qt.Equals, "NOW()")
	c.Assert(result.Overrides["postgres"]["default"], qt.Equals, "NOW()")
}

func TestMergeFieldOverrides_EmptyPlatforms(t *testing.T) {
	c := qt.New(t)

	baseField := goschema.Field{
		Name: "data",
		Type: "JSONB",
	}

	result := toschema.MergeFieldOverrides(baseField, map[string]goschema.Field{})

	c.Assert(result.Name, qt.Equals, "data")
	c.Assert(result.Type, qt.Equals, "JSONB")
	c.Assert(result.Overrides, qt.IsNil) // Should remain nil
}

func TestMergeTableOverrides_BasicMerging(t *testing.T) {
	c := qt.New(t)

	baseTable := goschema.Table{
		Name:    "products",
		Engine:  "InnoDB",
		Comment: "Product catalog",
	}

	platformTables := map[string]goschema.Table{
		"mariadb": {
			Name:    "products",
			Engine:  "InnoDB",
			Comment: "MariaDB product catalog",
		},
		"mysql": {
			Name:    "products",
			Engine:  "MyISAM",
			Comment: "Product catalog",
		},
	}

	result := toschema.MergeTableOverrides(baseTable, platformTables)

	c.Assert(result.Name, qt.Equals, "products")
	c.Assert(result.Engine, qt.Equals, "InnoDB") // Base engine unchanged
	c.Assert(result.Overrides, qt.IsNotNil)
	c.Assert(len(result.Overrides), qt.Equals, 2)

	// Check MariaDB overrides
	c.Assert(result.Overrides["mariadb"]["comment"], qt.Equals, "MariaDB product catalog")
	c.Assert(result.Overrides["mariadb"]["engine"], qt.Equals, "") // Same as base

	// Check MySQL overrides
	c.Assert(result.Overrides["mysql"]["engine"], qt.Equals, "MyISAM")
	c.Assert(result.Overrides["mysql"]["comment"], qt.Equals, "") // Same as base
}

func TestGenerateStructName_BasicConversions(t *testing.T) {
	tests := []struct {
		tableName  string
		structName string
	}{
		{"users", "User"},
		{"user_roles", "UserRole"},
		{"product_categories", "ProductCategory"},
		{"logs", "Log"},
		{"companies", "Company"},
		{"categories", "Category"},
		{"addresses", "Address"},
		{"", ""},
		{"single", "Single"},
		{"multi_word_table", "MultiWordTable"},
	}

	for _, test := range tests {
		t.Run(test.tableName, func(t *testing.T) {
			c := qt.New(t)
			// We need to test the generateStructName function, but it's not exported
			// So we'll test it indirectly through ToTable
			table := ast.NewCreateTable(test.tableName)
			result := toschema.ToTable(table, "")
			c.Assert(result.StructName, qt.Equals, test.structName)
		})
	}
}
