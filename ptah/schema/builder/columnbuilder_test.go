package builder_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

func TestColumnBuilder_Primary(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("id", "SERIAL").Primary().End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.Primary, qt.IsTrue)
	c.Assert(column.Nullable, qt.IsFalse) // Primary keys are automatically NOT NULL
}

func TestColumnBuilder_NotNull(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("email", "VARCHAR(255)").NotNull().End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.Nullable, qt.IsFalse)
}

func TestColumnBuilder_Nullable(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("description", "TEXT").Nullable().End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.Nullable, qt.IsTrue)
}

func TestColumnBuilder_Unique(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("email", "VARCHAR(255)").Unique().End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.Unique, qt.IsTrue)
}

func TestColumnBuilder_AutoIncrement(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("id", "INTEGER").AutoIncrement().End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.AutoInc, qt.IsTrue)
}

func TestColumnBuilder_Default(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("status", "VARCHAR(20)").Default("'active'").End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.Default, qt.IsNotNil)
	c.Assert(column.Default.Value, qt.Equals, "'active'")
	c.Assert(column.Default.Expression, qt.Equals, "")
}

func TestColumnBuilder_DefaultFunction(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("created_at", "TIMESTAMP").DefaultExpression("NOW()").End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.Default, qt.IsNotNil)
	c.Assert(column.Default.Expression, qt.Equals, "NOW()")
	c.Assert(column.Default.Value, qt.Equals, "")
}

func TestColumnBuilder_Check(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("age", "INTEGER").Check("age >= 0 AND age <= 150").End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.Check, qt.Equals, "age >= 0 AND age <= 150")
}

func TestColumnBuilder_Comment(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("id", "SERIAL").Comment("Auto-incrementing primary key").End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.Comment, qt.Equals, "Auto-incrementing primary key")
}

func TestColumnBuilder_ForeignKey(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").ForeignKey("users", "id", "fk_posts_user").
		OnDelete("CASCADE").
		OnUpdate("RESTRICT").
		End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.Table, qt.Equals, "users")
	c.Assert(column.ForeignKey.Column, qt.Equals, "id")
	c.Assert(column.ForeignKey.Name, qt.Equals, "fk_posts_user")
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "CASCADE")
	c.Assert(column.ForeignKey.OnUpdate, qt.Equals, "RESTRICT")
}

func TestColumnBuilder_ComplexColumn(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").
		NotNull().
		Check("user_id > 0").
		Comment("Reference to user table").
		ForeignKey("users", "id", "fk_posts_user").
		OnDelete("CASCADE").
		End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]

	c.Assert(column.Name, qt.Equals, "user_id")
	c.Assert(column.Type, qt.Equals, "INTEGER")
	c.Assert(column.Nullable, qt.IsFalse)
	c.Assert(column.Check, qt.Equals, "user_id > 0")
	c.Assert(column.Comment, qt.Equals, "Reference to user table")
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.Table, qt.Equals, "users")
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "CASCADE")
}

func TestColumnBuilder_FluentChaining(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("test")
	columnBuilder := table.Column("test_col", "INTEGER")

	// Test that all methods return the column builder for chaining
	result1 := columnBuilder.Primary()
	c.Assert(result1, qt.Equals, columnBuilder)

	result2 := columnBuilder.NotNull()
	c.Assert(result2, qt.Equals, columnBuilder)

	result3 := columnBuilder.Nullable()
	c.Assert(result3, qt.Equals, columnBuilder)

	result4 := columnBuilder.Unique()
	c.Assert(result4, qt.Equals, columnBuilder)

	result5 := columnBuilder.AutoIncrement()
	c.Assert(result5, qt.Equals, columnBuilder)

	result6 := columnBuilder.Default("'test'")
	c.Assert(result6, qt.Equals, columnBuilder)

	result7 := columnBuilder.DefaultExpression("NOW()")
	c.Assert(result7, qt.Equals, columnBuilder)

	result8 := columnBuilder.Check("test_col > 0")
	c.Assert(result8, qt.Equals, columnBuilder)

	result9 := columnBuilder.Comment("test comment")
	c.Assert(result9, qt.Equals, columnBuilder)

	// ForeignKey returns a foreign key builder, but End() should return the table builder
	fkBuilder := columnBuilder.ForeignKey("ref_table", "ref_col", "fk_name")
	c.Assert(fkBuilder, qt.IsNotNil)

	result10 := fkBuilder.End()
	c.Assert(result10, qt.Equals, table)

	// End() should return the table builder
	result11 := columnBuilder.End()
	c.Assert(result11, qt.Equals, table)
}

func TestColumnBuilder_MultipleColumns(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("users").
		Column("id", "SERIAL").Primary().AutoIncrement().Comment("Primary key").End().
		Column("email", "VARCHAR(255)").NotNull().Unique().Comment("User email").End().
		Column("username", "VARCHAR(100)").NotNull().Unique().End().
		Column("password_hash", "VARCHAR(255)").NotNull().End().
		Column("created_at", "TIMESTAMP").NotNull().DefaultExpression("NOW()").End().
		Column("updated_at", "TIMESTAMP").DefaultExpression("NOW()").End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 6)

	// Check id column
	idCol := result.Columns[0]
	c.Assert(idCol.Name, qt.Equals, "id")
	c.Assert(idCol.Primary, qt.IsTrue)
	c.Assert(idCol.AutoInc, qt.IsTrue)
	c.Assert(idCol.Comment, qt.Equals, "Primary key")

	// Check email column
	emailCol := result.Columns[1]
	c.Assert(emailCol.Name, qt.Equals, "email")
	c.Assert(emailCol.Nullable, qt.IsFalse)
	c.Assert(emailCol.Unique, qt.IsTrue)
	c.Assert(emailCol.Comment, qt.Equals, "User email")

	// Check created_at column
	createdAtCol := result.Columns[4]
	c.Assert(createdAtCol.Name, qt.Equals, "created_at")
	c.Assert(createdAtCol.Nullable, qt.IsFalse)
	c.Assert(createdAtCol.Default, qt.IsNotNil)
	c.Assert(createdAtCol.Default.Expression, qt.Equals, "NOW()")
}

func TestColumnBuilder_PrimaryKeyOverridesNullable(t *testing.T) {
	c := qt.New(t)

	table := builder.NewTable("test").
		Column("id", "INTEGER").Nullable().Primary().End()

	result := table.Build()

	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]

	// Primary key should override nullable setting
	c.Assert(column.Primary, qt.IsTrue)
	c.Assert(column.Nullable, qt.IsFalse)
}
