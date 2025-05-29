package builder_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

func TestForeignKeyBuilder_OnDelete(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").ForeignKey("users", "id", "fk_posts_user").
			OnDelete("CASCADE").
		End()
	
	result := table.Build()
	
	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "CASCADE")
}

func TestForeignKeyBuilder_OnUpdate(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").ForeignKey("users", "id", "fk_posts_user").
			OnUpdate("RESTRICT").
		End()
	
	result := table.Build()
	
	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.OnUpdate, qt.Equals, "RESTRICT")
}

func TestForeignKeyBuilder_OnDeleteAndUpdate(t *testing.T) {
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
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "CASCADE")
	c.Assert(column.ForeignKey.OnUpdate, qt.Equals, "RESTRICT")
}

func TestForeignKeyBuilder_FluentChaining(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("posts")
	columnBuilder := table.Column("user_id", "INTEGER")
	fkBuilder := columnBuilder.ForeignKey("users", "id", "fk_posts_user")
	
	// Test that all methods return the foreign key builder for chaining
	result1 := fkBuilder.OnDelete("CASCADE")
	c.Assert(result1, qt.Equals, fkBuilder)
	
	result2 := fkBuilder.OnUpdate("RESTRICT")
	c.Assert(result2, qt.Equals, fkBuilder)
	
	// End() should return the table builder
	result3 := fkBuilder.End()
	c.Assert(result3, qt.Equals, table)
}

func TestForeignKeyBuilder_SetNoAction(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").ForeignKey("users", "id", "fk_posts_user").
			OnDelete("NO ACTION").
			OnUpdate("NO ACTION").
		End()
	
	result := table.Build()
	
	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "NO ACTION")
	c.Assert(column.ForeignKey.OnUpdate, qt.Equals, "NO ACTION")
}

func TestForeignKeyBuilder_SetNull(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").ForeignKey("users", "id", "fk_posts_user").
			OnDelete("SET NULL").
			OnUpdate("SET NULL").
		End()
	
	result := table.Build()
	
	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "SET NULL")
	c.Assert(column.ForeignKey.OnUpdate, qt.Equals, "SET NULL")
}

func TestForeignKeyBuilder_SetDefault(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").ForeignKey("users", "id", "fk_posts_user").
			OnDelete("SET DEFAULT").
			OnUpdate("SET DEFAULT").
		End()
	
	result := table.Build()
	
	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "SET DEFAULT")
	c.Assert(column.ForeignKey.OnUpdate, qt.Equals, "SET DEFAULT")
}

func TestForeignKeyBuilder_TableLevelConstraint(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").End().
		ForeignKey("fk_posts_user", []string{"user_id"}, "users", "id").
			OnDelete("CASCADE").
			OnUpdate("RESTRICT").
		End()
	
	result := table.Build()
	
	c.Assert(len(result.Constraints), qt.Equals, 1)
	constraint := result.Constraints[0]
	c.Assert(constraint.Reference, qt.IsNotNil)
	c.Assert(constraint.Reference.OnDelete, qt.Equals, "CASCADE")
	c.Assert(constraint.Reference.OnUpdate, qt.Equals, "RESTRICT")
}

func TestForeignKeyBuilder_MultipleForeignKeys(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").ForeignKey("users", "id", "fk_posts_user").
			OnDelete("CASCADE").
		End().
		Column("category_id", "INTEGER").ForeignKey("categories", "id", "fk_posts_category").
			OnDelete("SET NULL").
			OnUpdate("CASCADE").
		End()
	
	result := table.Build()
	
	c.Assert(len(result.Columns), qt.Equals, 2)
	
	// Check first foreign key
	userIdColumn := result.Columns[0]
	c.Assert(userIdColumn.ForeignKey, qt.IsNotNil)
	c.Assert(userIdColumn.ForeignKey.Table, qt.Equals, "users")
	c.Assert(userIdColumn.ForeignKey.OnDelete, qt.Equals, "CASCADE")
	
	// Check second foreign key
	categoryIdColumn := result.Columns[1]
	c.Assert(categoryIdColumn.ForeignKey, qt.IsNotNil)
	c.Assert(categoryIdColumn.ForeignKey.Table, qt.Equals, "categories")
	c.Assert(categoryIdColumn.ForeignKey.OnDelete, qt.Equals, "SET NULL")
	c.Assert(categoryIdColumn.ForeignKey.OnUpdate, qt.Equals, "CASCADE")
}

func TestForeignKeyBuilder_ComplexForeignKey(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("order_items").
		Column("order_id", "INTEGER").NotNull().ForeignKey("orders", "id", "fk_order_items_order").
			OnDelete("CASCADE").
			OnUpdate("RESTRICT").
		End().
		Column("product_id", "INTEGER").NotNull().ForeignKey("products", "id", "fk_order_items_product").
			OnDelete("RESTRICT").
			OnUpdate("CASCADE").
		End()
	
	result := table.Build()
	
	c.Assert(len(result.Columns), qt.Equals, 2)
	
	// Check order foreign key
	orderColumn := result.Columns[0]
	c.Assert(orderColumn.Name, qt.Equals, "order_id")
	c.Assert(orderColumn.Nullable, qt.IsFalse)
	c.Assert(orderColumn.ForeignKey, qt.IsNotNil)
	c.Assert(orderColumn.ForeignKey.Table, qt.Equals, "orders")
	c.Assert(orderColumn.ForeignKey.Column, qt.Equals, "id")
	c.Assert(orderColumn.ForeignKey.Name, qt.Equals, "fk_order_items_order")
	c.Assert(orderColumn.ForeignKey.OnDelete, qt.Equals, "CASCADE")
	c.Assert(orderColumn.ForeignKey.OnUpdate, qt.Equals, "RESTRICT")
	
	// Check product foreign key
	productColumn := result.Columns[1]
	c.Assert(productColumn.Name, qt.Equals, "product_id")
	c.Assert(productColumn.Nullable, qt.IsFalse)
	c.Assert(productColumn.ForeignKey, qt.IsNotNil)
	c.Assert(productColumn.ForeignKey.Table, qt.Equals, "products")
	c.Assert(productColumn.ForeignKey.Column, qt.Equals, "id")
	c.Assert(productColumn.ForeignKey.Name, qt.Equals, "fk_order_items_product")
	c.Assert(productColumn.ForeignKey.OnDelete, qt.Equals, "RESTRICT")
	c.Assert(productColumn.ForeignKey.OnUpdate, qt.Equals, "CASCADE")
}

func TestForeignKeyBuilder_SelfReferencing(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("categories").
		Column("id", "SERIAL").Primary().End().
		Column("parent_id", "INTEGER").ForeignKey("categories", "id", "fk_categories_parent").
			OnDelete("SET NULL").
		End()
	
	result := table.Build()
	
	c.Assert(len(result.Columns), qt.Equals, 2)
	
	parentColumn := result.Columns[1]
	c.Assert(parentColumn.Name, qt.Equals, "parent_id")
	c.Assert(parentColumn.ForeignKey, qt.IsNotNil)
	c.Assert(parentColumn.ForeignKey.Table, qt.Equals, "categories")
	c.Assert(parentColumn.ForeignKey.Column, qt.Equals, "id")
	c.Assert(parentColumn.ForeignKey.Name, qt.Equals, "fk_categories_parent")
	c.Assert(parentColumn.ForeignKey.OnDelete, qt.Equals, "SET NULL")
}

func TestForeignKeyBuilder_NoActions(t *testing.T) {
	c := qt.New(t)
	
	table := builder.NewTable("posts").
		Column("user_id", "INTEGER").ForeignKey("users", "id", "fk_posts_user").End()
	
	result := table.Build()
	
	c.Assert(len(result.Columns), qt.Equals, 1)
	column := result.Columns[0]
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "") // Default empty
	c.Assert(column.ForeignKey.OnUpdate, qt.Equals, "") // Default empty
}
