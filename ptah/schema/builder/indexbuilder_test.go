package builder_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

func TestNewIndex(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_users_email", "users", "email")
	
	c.Assert(index, qt.IsNotNil)
	
	result := index.Build()
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.Name, qt.Equals, "idx_users_email")
	c.Assert(result.Table, qt.Equals, "users")
	c.Assert(result.Columns, qt.DeepEquals, []string{"email"})
	c.Assert(result.Unique, qt.IsFalse)
	c.Assert(result.Type, qt.Equals, "")
	c.Assert(result.Comment, qt.Equals, "")
}

func TestIndexBuilder_Unique(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_users_email", "users", "email").
		Unique()
	
	result := index.Build()
	
	c.Assert(result.Unique, qt.IsTrue)
}

func TestIndexBuilder_Type(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_users_email", "users", "email").
		Type("BTREE")
	
	result := index.Build()
	
	c.Assert(result.Type, qt.Equals, "BTREE")
}

func TestIndexBuilder_Comment(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_users_email", "users", "email").
		Comment("Index for fast email lookups")
	
	result := index.Build()
	
	c.Assert(result.Comment, qt.Equals, "Index for fast email lookups")
}

func TestIndexBuilder_MultipleColumns(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_posts_user_created", "posts", "user_id", "created_at")
	
	result := index.Build()
	
	c.Assert(result.Name, qt.Equals, "idx_posts_user_created")
	c.Assert(result.Table, qt.Equals, "posts")
	c.Assert(result.Columns, qt.DeepEquals, []string{"user_id", "created_at"})
}

func TestIndexBuilder_ComplexIndex(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_users_email_status", "users", "email", "status").
		Unique().
		Type("BTREE").
		Comment("Unique index on email and status for fast user lookups")
	
	result := index.Build()
	
	c.Assert(result.Name, qt.Equals, "idx_users_email_status")
	c.Assert(result.Table, qt.Equals, "users")
	c.Assert(result.Columns, qt.DeepEquals, []string{"email", "status"})
	c.Assert(result.Unique, qt.IsTrue)
	c.Assert(result.Type, qt.Equals, "BTREE")
	c.Assert(result.Comment, qt.Equals, "Unique index on email and status for fast user lookups")
}

func TestIndexBuilder_FluentChaining(t *testing.T) {
	c := qt.New(t)
	
	// Test that all methods return the index builder for chaining
	index := builder.NewIndex("test_index", "test_table", "col1")
	
	result1 := index.Unique()
	c.Assert(result1, qt.Equals, index)
	
	result2 := index.Type("HASH")
	c.Assert(result2, qt.Equals, index)
	
	result3 := index.Comment("test comment")
	c.Assert(result3, qt.Equals, index)
}

func TestIndexBuilder_SingleColumn(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_users_username", "users", "username").
		Unique().
		Comment("Unique username index")
	
	result := index.Build()
	
	c.Assert(result.Name, qt.Equals, "idx_users_username")
	c.Assert(result.Table, qt.Equals, "users")
	c.Assert(len(result.Columns), qt.Equals, 1)
	c.Assert(result.Columns[0], qt.Equals, "username")
	c.Assert(result.Unique, qt.IsTrue)
	c.Assert(result.Comment, qt.Equals, "Unique username index")
}

func TestIndexBuilder_NoColumns(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_empty", "test_table")
	
	result := index.Build()
	
	c.Assert(result.Name, qt.Equals, "idx_empty")
	c.Assert(result.Table, qt.Equals, "test_table")
	c.Assert(len(result.Columns), qt.Equals, 0)
}

func TestIndexBuilder_HashIndex(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_users_id_hash", "users", "id").
		Type("HASH").
		Comment("Hash index for exact ID lookups")
	
	result := index.Build()
	
	c.Assert(result.Name, qt.Equals, "idx_users_id_hash")
	c.Assert(result.Type, qt.Equals, "HASH")
	c.Assert(result.Comment, qt.Equals, "Hash index for exact ID lookups")
	c.Assert(result.Unique, qt.IsFalse) // Not unique by default
}

func TestIndexBuilder_GinIndex(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_posts_tags", "posts", "tags").
		Type("GIN").
		Comment("GIN index for array/JSON searches")
	
	result := index.Build()
	
	c.Assert(result.Name, qt.Equals, "idx_posts_tags")
	c.Assert(result.Type, qt.Equals, "GIN")
	c.Assert(result.Comment, qt.Equals, "GIN index for array/JSON searches")
}

func TestIndexBuilder_PartialIndex(t *testing.T) {
	c := qt.New(t)
	
	// Note: Partial index conditions would typically be handled in the renderer
	// This test just ensures the basic structure works
	index := builder.NewIndex("idx_users_active_email", "users", "email").
		Unique().
		Comment("Unique email index for active users only")
	
	result := index.Build()
	
	c.Assert(result.Name, qt.Equals, "idx_users_active_email")
	c.Assert(result.Unique, qt.IsTrue)
	c.Assert(result.Comment, qt.Equals, "Unique email index for active users only")
}

func TestIndexBuilder_CompositeUniqueIndex(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_user_roles_unique", "user_roles", "user_id", "role_id").
		Unique().
		Comment("Ensure each user can have each role only once")
	
	result := index.Build()
	
	c.Assert(result.Name, qt.Equals, "idx_user_roles_unique")
	c.Assert(result.Table, qt.Equals, "user_roles")
	c.Assert(result.Columns, qt.DeepEquals, []string{"user_id", "role_id"})
	c.Assert(result.Unique, qt.IsTrue)
	c.Assert(result.Comment, qt.Equals, "Ensure each user can have each role only once")
}

func TestIndexBuilder_PerformanceIndex(t *testing.T) {
	c := qt.New(t)
	
	index := builder.NewIndex("idx_orders_customer_date", "orders", "customer_id", "order_date").
		Type("BTREE").
		Comment("Performance index for customer order history queries")
	
	result := index.Build()
	
	c.Assert(result.Name, qt.Equals, "idx_orders_customer_date")
	c.Assert(result.Table, qt.Equals, "orders")
	c.Assert(result.Columns, qt.DeepEquals, []string{"customer_id", "order_date"})
	c.Assert(result.Type, qt.Equals, "BTREE")
	c.Assert(result.Unique, qt.IsFalse)
	c.Assert(result.Comment, qt.Equals, "Performance index for customer order history queries")
}
