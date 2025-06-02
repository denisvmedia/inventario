package astbuilder_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/astbuilder"
)

func TestNewSchema(t *testing.T) {
	c := qt.New(t)

	schema := astbuilder.NewSchema()

	c.Assert(schema, qt.IsNotNil)

	// Build should return empty statement list
	result := schema.Build()
	c.Assert(result, qt.IsNotNil)
	c.Assert(len(result.Statements), qt.Equals, 0)
}

func TestSchemaBuilder_Comment(t *testing.T) {
	c := qt.New(t)

	schema := astbuilder.NewSchema().
		Comment("This is a test schema")

	result := schema.Build()

	c.Assert(len(result.Statements), qt.Equals, 1)

	// Check that it's a comment node
	commentNode, ok := result.Statements[0].(*ast.CommentNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(commentNode.Text, qt.Equals, "This is a test schema")
}

func TestSchemaBuilder_Enum(t *testing.T) {
	c := qt.New(t)

	schema := astbuilder.NewSchema().
		Enum("status", "active", "inactive", "pending")

	result := schema.Build()

	c.Assert(len(result.Statements), qt.Equals, 1)

	// Check that it's an enum node
	enumNode, ok := result.Statements[0].(*ast.EnumNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(enumNode.Name, qt.Equals, "status")
	c.Assert(enumNode.Values, qt.DeepEquals, []string{"active", "inactive", "pending"})
}

func TestSchemaBuilder_Table(t *testing.T) {
	c := qt.New(t)

	schema := astbuilder.NewSchema().
		Table("users").
		Column("id", "SERIAL").Primary().End().
		Column("email", "VARCHAR(255)").NotNull().Unique().End().
		End()

	result := schema.Build()

	c.Assert(len(result.Statements), qt.Equals, 1)

	// Check that it's a create table node
	tableNode, ok := result.Statements[0].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(tableNode.Name, qt.Equals, "users")
	c.Assert(len(tableNode.Columns), qt.Equals, 2)

	// Check first column
	c.Assert(tableNode.Columns[0].Name, qt.Equals, "id")
	c.Assert(tableNode.Columns[0].Type, qt.Equals, "SERIAL")
	c.Assert(tableNode.Columns[0].Primary, qt.IsTrue)

	// Check second column
	c.Assert(tableNode.Columns[1].Name, qt.Equals, "email")
	c.Assert(tableNode.Columns[1].Type, qt.Equals, "VARCHAR(255)")
	c.Assert(tableNode.Columns[1].Nullable, qt.IsFalse)
	c.Assert(tableNode.Columns[1].Unique, qt.IsTrue)
}

func TestSchemaBuilder_Index(t *testing.T) {
	c := qt.New(t)

	schema := astbuilder.NewSchema().
		Index("idx_users_email", "users", "email").
		Unique().
		Comment("Unique index on email").
		End()

	result := schema.Build()

	c.Assert(len(result.Statements), qt.Equals, 1)

	// Check that it's an index node
	indexNode, ok := result.Statements[0].(*ast.IndexNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(indexNode.Name, qt.Equals, "idx_users_email")
	c.Assert(indexNode.Table, qt.Equals, "users")
	c.Assert(indexNode.Columns, qt.DeepEquals, []string{"email"})
	c.Assert(indexNode.Unique, qt.IsTrue)
	c.Assert(indexNode.Comment, qt.Equals, "Unique index on email")
}

func TestSchemaBuilder_ComplexSchema(t *testing.T) {
	c := qt.New(t)

	schema := astbuilder.NewSchema().
		Comment("User management schema").
		Enum("user_status", "active", "inactive", "suspended").
		Table("users").
		Column("id", "SERIAL").Primary().End().
		Column("email", "VARCHAR(255)").NotNull().Unique().End().
		Column("status", "user_status").NotNull().Default("'active'").End().
		Column("created_at", "TIMESTAMP").NotNull().DefaultExpression("NOW()").End().
		End().
		Index("idx_users_status", "users", "status").End().
		Table("posts").
		Column("id", "SERIAL").Primary().End().
		Column("user_id", "INTEGER").NotNull().ForeignKey("users", "id", "fk_posts_user").End().
		Column("title", "VARCHAR(255)").NotNull().End().
		Column("content", "TEXT").End().
		End()

	result := schema.Build()

	c.Assert(len(result.Statements), qt.Equals, 5) // comment + enum + 2 tables + 1 index

	// Check comment
	commentNode, ok := result.Statements[0].(*ast.CommentNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(commentNode.Text, qt.Equals, "User management schema")

	// Check enum
	enumNode, ok := result.Statements[1].(*ast.EnumNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(enumNode.Name, qt.Equals, "user_status")

	// Check users table
	usersTable, ok := result.Statements[2].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(usersTable.Name, qt.Equals, "users")
	c.Assert(len(usersTable.Columns), qt.Equals, 4)

	// Check index
	indexNode, ok := result.Statements[3].(*ast.IndexNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(indexNode.Name, qt.Equals, "idx_users_status")

	// Check posts table
	postsTable, ok := result.Statements[4].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(postsTable.Name, qt.Equals, "posts")
	c.Assert(len(postsTable.Columns), qt.Equals, 4)

	// Check foreign key on user_id column
	userIDColumn := postsTable.Columns[1]
	c.Assert(userIDColumn.Name, qt.Equals, "user_id")
	c.Assert(userIDColumn.ForeignKey, qt.IsNotNil)
	c.Assert(userIDColumn.ForeignKey.Table, qt.Equals, "users")
	c.Assert(userIDColumn.ForeignKey.Column, qt.Equals, "id")
	c.Assert(userIDColumn.ForeignKey.Name, qt.Equals, "fk_posts_user")
}

func TestSchemaBuilder_FluentChaining(t *testing.T) {
	c := qt.New(t)

	// Test that all methods return the schema builder for chaining
	schema := astbuilder.NewSchema()

	result1 := schema.Comment("test")
	c.Assert(result1, qt.Equals, schema)

	result2 := schema.Enum("test_enum", "value1")
	c.Assert(result2, qt.Equals, schema)

	// Table and Index return wrapped builders, but End() should return the schema
	tableBuilder := schema.Table("test_table")
	c.Assert(tableBuilder, qt.IsNotNil)

	result3 := tableBuilder.End()
	c.Assert(result3, qt.Equals, schema)

	indexBuilder := schema.Index("test_index", "test_table", "col1")
	c.Assert(indexBuilder, qt.IsNotNil)

	result4 := indexBuilder.End()
	c.Assert(result4, qt.Equals, schema)
}

func TestSchemaBuilder_MultipleComments(t *testing.T) {
	c := qt.New(t)

	schema := astbuilder.NewSchema().
		Comment("First comment").
		Comment("Second comment")

	result := schema.Build()

	c.Assert(len(result.Statements), qt.Equals, 2)

	comment1, ok := result.Statements[0].(*ast.CommentNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(comment1.Text, qt.Equals, "First comment")

	comment2, ok := result.Statements[1].(*ast.CommentNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(comment2.Text, qt.Equals, "Second comment")
}

func TestSchemaBuilder_MultipleEnums(t *testing.T) {
	c := qt.New(t)

	schema := astbuilder.NewSchema().
		Enum("status", "active", "inactive").
		Enum("role", "admin", "user", "guest")

	result := schema.Build()

	c.Assert(len(result.Statements), qt.Equals, 2)

	enum1, ok := result.Statements[0].(*ast.EnumNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(enum1.Name, qt.Equals, "status")
	c.Assert(enum1.Values, qt.DeepEquals, []string{"active", "inactive"})

	enum2, ok := result.Statements[1].(*ast.EnumNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(enum2.Name, qt.Equals, "role")
	c.Assert(enum2.Values, qt.DeepEquals, []string{"admin", "user", "guest"})
}
