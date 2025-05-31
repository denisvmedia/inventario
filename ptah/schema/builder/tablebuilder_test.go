package builder_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

func TestNewTable(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users")

	c.Assert(table, qt.IsNotNil)

	result := table.Build()
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.Name, qt.Equals, "users")
	c.Assert(len(result.Columns), qt.Equals, 0)
	c.Assert(len(result.Constraints), qt.Equals, 0)
}

func TestTableBuilder_Comment(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Comment("User accounts table")

	result := table.Build()

	c.Assert(result.Comment, qt.Equals, "User accounts table")
}

func TestTableBuilder_Engine(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Engine("InnoDB")

	result := table.Build()

	c.Assert(result.Options["ENGINE"], qt.Equals, "InnoDB")
}

func TestTableBuilder_Option(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Option("CHARSET", "utf8mb4").
		Option("COLLATE", "utf8mb4_unicode_ci")

	result := table.Build()

	c.Assert(result.Options["CHARSET"], qt.Equals, "utf8mb4")
	c.Assert(result.Options["COLLATE"], qt.Equals, "utf8mb4_unicode_ci")
}

func TestTableBuilder_Column(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("id", "SERIAL").Primary().End().
		Column("email", "VARCHAR(255)").NotNull().Unique().End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 2)

	// Check first column
	c.Assert(result.Columns[0].Name, qt.Equals, "id")
	c.Assert(result.Columns[0].Type, qt.Equals, "SERIAL")
	c.Assert(result.Columns[0].Primary, qt.IsTrue)

	// Check second column
	c.Assert(result.Columns[1].Name, qt.Equals, "email")
	c.Assert(result.Columns[1].Type, qt.Equals, "VARCHAR(255)")
	c.Assert(result.Columns[1].Nullable, qt.IsFalse)
	c.Assert(result.Columns[1].Unique, qt.IsTrue)
}

func TestTableBuilder_PrimaryKey(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("user_roles").
		Column("user_id", "INTEGER").End().
		Column("role_id", "INTEGER").End().
		PrimaryKey("user_id", "role_id")

	result := table.Build()

	c.Assert(len(result.Constraints), qt.Equals, 1)
	c.Assert(result.Constraints[0].Type, qt.Equals, ast.PrimaryKeyConstraint)
	c.Assert(result.Constraints[0].Columns, qt.DeepEquals, []string{"user_id", "role_id"})
}

func TestTableBuilder_Unique(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("email", "VARCHAR(255)").End().
		Column("username", "VARCHAR(100)").End().
		Unique("uk_users_email_username", "email", "username")

	result := table.Build()

	c.Assert(len(result.Constraints), qt.Equals, 1)
	c.Assert(result.Constraints[0].Type, qt.Equals, ast.UniqueConstraint)
	c.Assert(result.Constraints[0].Name, qt.Equals, "uk_users_email_username")
	c.Assert(result.Constraints[0].Columns, qt.DeepEquals, []string{"email", "username"})
}

func TestTableBuilder_ForeignKey(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").End().
		ForeignKey("fk_posts_user", []string{"user_id"}, "users", "id").
		OnDelete("CASCADE").
		OnUpdate("RESTRICT").
		End()

	result := table.Build()

	c.Assert(len(result.Constraints), qt.Equals, 1)
	c.Assert(result.Constraints[0].Type, qt.Equals, ast.ForeignKeyConstraint)
	c.Assert(result.Constraints[0].Name, qt.Equals, "fk_posts_user")
	c.Assert(result.Constraints[0].Columns, qt.DeepEquals, []string{"user_id"})
	c.Assert(result.Constraints[0].Reference, qt.IsNotNil)
	c.Assert(result.Constraints[0].Reference.Table, qt.Equals, "users")
	c.Assert(result.Constraints[0].Reference.Column, qt.Equals, "id")
	c.Assert(result.Constraints[0].Reference.OnDelete, qt.Equals, "CASCADE")
	c.Assert(result.Constraints[0].Reference.OnUpdate, qt.Equals, "RESTRICT")
}

func TestTableBuilder_ComplexTable(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("posts").
		Comment("Blog posts table").
		Engine("InnoDB").
		Option("CHARSET", "utf8mb4").
		Column("id", "SERIAL").Primary().Comment("Primary key").End().
		Column("user_id", "INTEGER").NotNull().ForeignKey("users", "id", "fk_posts_user").End().
		Column("title", "VARCHAR(255)").NotNull().End().
		Column("slug", "VARCHAR(255)").NotNull().End().
		Column("content", "TEXT").End().
		Column("status", "ENUM('draft','published','archived')").NotNull().Default("'draft'").End().
		Column("created_at", "TIMESTAMP").NotNull().DefaultExpression("NOW()").End().
		Unique("uk_posts_slug", "slug").
		PrimaryKey("id")

	result := table.Build()

	// Check table properties
	c.Assert(result.Name, qt.Equals, "posts")
	c.Assert(result.Comment, qt.Equals, "Blog posts table")
	c.Assert(result.Options["ENGINE"], qt.Equals, "InnoDB")
	c.Assert(result.Options["CHARSET"], qt.Equals, "utf8mb4")

	// Check columns
	c.Assert(len(result.Columns), qt.Equals, 7)

	// Check constraints
	c.Assert(len(result.Constraints), qt.Equals, 2) // unique + primary key

	// Check unique constraint
	uniqueConstraint := result.Constraints[0]
	c.Assert(uniqueConstraint.Type, qt.Equals, ast.UniqueConstraint)
	c.Assert(uniqueConstraint.Name, qt.Equals, "uk_posts_slug")

	// Check primary key constraint
	pkConstraint := result.Constraints[1]
	c.Assert(pkConstraint.Type, qt.Equals, ast.PrimaryKeyConstraint)
	c.Assert(pkConstraint.Columns, qt.DeepEquals, []string{"id"})
}

func TestTableBuilder_FluentChaining(t *testing.T) {
	c := qt.New(t)

	// Test that all methods return the table builder for chaining
	table := builder.NewTable("test")

	result1 := table.Comment("test comment")
	c.Assert(result1, qt.Equals, table)

	result2 := table.Engine("InnoDB")
	c.Assert(result2, qt.Equals, table)

	result3 := table.Option("CHARSET", "utf8")
	c.Assert(result3, qt.Equals, table)

	result4 := table.PrimaryKey("id")
	c.Assert(result4, qt.Equals, table)

	result5 := table.Unique("uk_test", "name")
	c.Assert(result5, qt.Equals, table)

	// Column returns a column builder, but End() should return the table
	columnBuilder := table.Column("id", "INTEGER")
	c.Assert(columnBuilder, qt.IsNotNil)

	result6 := columnBuilder.End()
	c.Assert(result6, qt.Equals, table)

	// ForeignKey returns a foreign key builder, but End() should return the table
	fkBuilder := table.ForeignKey("fk_test", []string{"ref_id"}, "ref_table", "id")
	c.Assert(fkBuilder, qt.IsNotNil)

	result7 := fkBuilder.End()
	c.Assert(result7, qt.Equals, table)
}

func TestTableBuilder_MultipleConstraints(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("test").
		Column("id", "INTEGER").End().
		Column("email", "VARCHAR(255)").End().
		Column("username", "VARCHAR(100)").End().
		PrimaryKey("id").
		Unique("uk_email", "email").
		Unique("uk_username", "username")

	result := table.Build()

	c.Assert(len(result.Constraints), qt.Equals, 3)

	// Check primary key
	c.Assert(result.Constraints[0].Type, qt.Equals, ast.PrimaryKeyConstraint)

	// Check unique constraints
	c.Assert(result.Constraints[1].Type, qt.Equals, ast.UniqueConstraint)
	c.Assert(result.Constraints[1].Name, qt.Equals, "uk_email")

	c.Assert(result.Constraints[2].Type, qt.Equals, ast.UniqueConstraint)
	c.Assert(result.Constraints[2].Name, qt.Equals, "uk_username")
}
