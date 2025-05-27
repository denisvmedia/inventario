package ast_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/ast"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/builders"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/renderers"
)

func TestCreateTableAST_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *ast.CreateTableNode
		contains []string
	}{
		{
			name: "simple table with basic columns",
			builder: func() *ast.CreateTableNode {
				return builders.NewTable("users").
					Column("id", "SERIAL").Primary().End().
					Column("name", "VARCHAR(255)").NotNull().End().
					Column("email", "VARCHAR(255)").NotNull().Unique().End().
					Build()
			},
			contains: []string{
				"CREATE TABLE users",
				"id SERIAL PRIMARY KEY",
				"name VARCHAR(255) NOT NULL",
				"email VARCHAR(255) NOT NULL UNIQUE",
			},
		},
		{
			name: "table with default values and constraints",
			builder: func() *ast.CreateTableNode {
				return builders.NewTable("products").
					Column("id", "SERIAL").Primary().End().
					Column("name", "VARCHAR(255)").NotNull().End().
					Column("price", "DECIMAL(10,2)").NotNull().Check("price > 0").End().
					Column("is_active", "BOOLEAN").Default("true").End().
					Column("created_at", "TIMESTAMP").DefaultFunction("NOW()").End().
					Build()
			},
			contains: []string{
				"CREATE TABLE products",
				"id SERIAL PRIMARY KEY",
				"name VARCHAR(255) NOT NULL",
				"price DECIMAL(10,2) NOT NULL CHECK (price > 0)",
				"is_active BOOLEAN DEFAULT 'true'",
				"created_at TIMESTAMP DEFAULT NOW()",
			},
		},
		{
			name: "table with composite primary key",
			builder: func() *ast.CreateTableNode {
				return builders.NewTable("order_items").
					Column("order_id", "INTEGER").NotNull().End().
					Column("product_id", "INTEGER").NotNull().End().
					Column("quantity", "INTEGER").NotNull().End().
					PrimaryKey("order_id", "product_id").
					Build()
			},
			contains: []string{
				"CREATE TABLE order_items",
				"order_id INTEGER NOT NULL",
				"product_id INTEGER NOT NULL",
				"quantity INTEGER NOT NULL",
				"PRIMARY KEY (order_id, product_id)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			table := tt.builder()
			c.Assert(table, qt.IsNotNil)
			c.Assert(table.Name, qt.Not(qt.Equals), "")

			// Test PostgreSQL rendering
			renderer := renderers.NewPostgreSQLRenderer()
			sql, err := renderer.Render(table)
			c.Assert(err, qt.IsNil)
			c.Assert(sql, qt.Not(qt.Equals), "")

			for _, expected := range tt.contains {
				c.Assert(sql, qt.Contains, expected)
			}
		})
	}
}

func TestCreateTableAST_UnhappyPath(t *testing.T) {
	tests := []struct {
		name        string
		builder     func() *ast.CreateTableNode
		expectError bool
	}{
		{
			name: "empty table name should still work",
			builder: func() *ast.CreateTableNode {
				return builders.NewTable("").
					Column("id", "SERIAL").Primary().End().
					Build()
			},
			expectError: false, // AST allows this, validation would be at a higher level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			table := tt.builder()
			c.Assert(table, qt.IsNotNil)

			renderer := renderers.NewPostgreSQLRenderer()
			sql, err := renderer.Render(table)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(sql, qt.Not(qt.Equals), "")
			}
		})
	}
}

func TestEnumAST_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *ast.EnumNode
		contains []string
	}{
		{
			name: "simple enum",
			builder: func() *ast.EnumNode {
				return ast.NewEnum("status", "active", "inactive", "pending")
			},
			contains: []string{
				"CREATE TYPE status AS ENUM",
				"'active'",
				"'inactive'",
				"'pending'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			enum := tt.builder()
			c.Assert(enum, qt.IsNotNil)
			c.Assert(enum.Name, qt.Not(qt.Equals), "")
			c.Assert(len(enum.Values), qt.Equals, 3)

			// Test PostgreSQL rendering (MySQL skips enums)
			renderer := renderers.NewPostgreSQLRenderer()
			sql, err := renderer.Render(enum)
			c.Assert(err, qt.IsNil)
			c.Assert(sql, qt.Not(qt.Equals), "")

			for _, expected := range tt.contains {
				c.Assert(sql, qt.Contains, expected)
			}
		})
	}
}

func TestSchemaBuilding_HappyPath(t *testing.T) {
	c := qt.New(t)

	// Build schema step by step to avoid complex chaining
	schemaBuilder := builders.NewSchema()
	schemaBuilder.Comment("Test database schema")
	schemaBuilder.Enum("user_role", "admin", "user", "guest")

	// Add users table
	usersTable := schemaBuilder.Table("users")
	usersTable.Column("id", "SERIAL").Primary().End()
	usersTable.Column("email", "VARCHAR(255)").NotNull().Unique().End()
	usersTable.Column("role", "user_role").NotNull().Default("user").End()
	schemaBuilder = usersTable.End()

	// Add posts table
	postsTable := schemaBuilder.Table("posts")
	postsTable.Column("id", "SERIAL").Primary().End()
	postsTable.Column("title", "VARCHAR(255)").NotNull().End()
	userIdCol := postsTable.Column("user_id", "INTEGER").NotNull()
	fkBuilder := userIdCol.ForeignKey("users", "id", "fk_posts_user")
	fkBuilder.OnDelete("CASCADE")
	postsTable = fkBuilder.End()
	schemaBuilder = postsTable.End()

	// Add index
	indexBuilder := schemaBuilder.Index("idx_posts_user", "posts", "user_id")
	schemaBuilder = indexBuilder.End()

	schema := schemaBuilder.Build()

	c.Assert(schema, qt.IsNotNil)
	c.Assert(len(schema.Statements), qt.Equals, 5) // comment + enum + 2 tables + index

	// Test PostgreSQL rendering
	renderer := renderers.NewPostgreSQLRenderer()
	sql, err := renderer.RenderSchema(schema)
	c.Assert(err, qt.IsNil)
	c.Assert(sql, qt.Not(qt.Equals), "")

	// Check that enums come first
	enumIndex := strings.Index(sql, "CREATE TYPE user_role")
	tableIndex := strings.Index(sql, "CREATE TABLE users")
	c.Assert(enumIndex, qt.Not(qt.Equals), -1)
	c.Assert(tableIndex, qt.Not(qt.Equals), -1)
	c.Assert(enumIndex < tableIndex, qt.IsTrue)

	// Check expected content
	expectedContent := []string{
		"-- Test database schema --",
		"CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest')",
		"CREATE TABLE users",
		"id SERIAL PRIMARY KEY",
		"email VARCHAR(255) NOT NULL UNIQUE",
		"role user_role NOT NULL DEFAULT 'user'",
		"CREATE TABLE posts",
		"title VARCHAR(255) NOT NULL",
		"user_id INTEGER NOT NULL",
		"CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE",
		"CREATE INDEX idx_posts_user ON posts (user_id)",
	}

	for _, expected := range expectedContent {
		c.Assert(sql, qt.Contains, expected)
	}
}

func TestAlterTableAST_HappyPath(t *testing.T) {
	c := qt.New(t)

	alterTable := &ast.AlterTableNode{
		Name: "users",
		Operations: []ast.AlterOperation{
			&ast.AddColumnOperation{
				Column: ast.NewColumn("phone", "VARCHAR(20)").SetNotNull(),
			},
			&ast.ModifyColumnOperation{
				Column: ast.NewColumn("email", "VARCHAR(320)").SetNotNull().SetUnique(),
			},
			&ast.DropColumnOperation{
				ColumnName: "old_field",
			},
		},
	}

	// Test PostgreSQL rendering
	renderer := renderers.NewPostgreSQLRenderer()
	sql, err := renderer.Render(alterTable)
	c.Assert(err, qt.IsNil)
	c.Assert(sql, qt.Not(qt.Equals), "")

	expectedContent := []string{
		"-- ALTER statements: --",
		"ALTER TABLE users ADD COLUMN phone VARCHAR(20) NOT NULL",
		"ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(320)",
		"ALTER TABLE users ALTER COLUMN email SET NOT NULL",
		"ALTER TABLE users DROP COLUMN old_field",
	}

	for _, expected := range expectedContent {
		c.Assert(sql, qt.Contains, expected)
	}
}

func TestDialectDifferences_HappyPath(t *testing.T) {
	c := qt.New(t)

	table := builders.NewTable("users").
		Column("id", "SERIAL").Primary().End().
		Column("name", "VARCHAR(255)").NotNull().End().
		Build()

	// Test PostgreSQL
	pgRenderer := renderers.NewPostgreSQLRenderer()
	pgSQL, err := pgRenderer.Render(table)
	c.Assert(err, qt.IsNil)
	c.Assert(pgSQL, qt.Contains, "SERIAL")

	// Test MySQL
	mysqlRenderer := renderers.NewMySQLRenderer()
	mysqlSQL, err := mysqlRenderer.Render(table)
	c.Assert(err, qt.IsNil)
	c.Assert(mysqlSQL, qt.Contains, "SERIAL") // Would be transformed in a real implementation

	// Both should contain the table structure
	for _, sql := range []string{pgSQL, mysqlSQL} {
		c.Assert(sql, qt.Contains, "CREATE TABLE users")
		c.Assert(sql, qt.Contains, "name VARCHAR(255) NOT NULL")
	}
}
