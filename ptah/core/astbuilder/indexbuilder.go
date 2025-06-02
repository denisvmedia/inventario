package astbuilder

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// IndexBuilder provides a fluent API for building CREATE INDEX statements.
//
// IndexBuilder wraps an AST IndexNode and provides a convenient fluent interface
// for configuring index properties such as uniqueness, index type, and comments.
// It can be used standalone or as part of a larger schema definition.
//
// Indexes improve query performance by creating optimized data structures that
// allow faster lookups on specified columns. They can be unique (enforcing
// uniqueness constraints) or non-unique (for performance only).
//
// Index types vary by database system:
//   - BTREE: Balanced tree index (default for most databases)
//   - HASH: Hash-based index for equality lookups
//   - GIN/GIST: PostgreSQL specialized indexes
//   - FULLTEXT: Full-text search indexes
//
// Example usage:
//
//	// Simple index
//	index := astbuilder.NewIndex("idx_users_email", "users", "email")
//
//	// Unique index with type and comment
//	index := astbuilder.NewIndex("idx_users_username", "users", "username").
//		Unique().
//		Type("BTREE").
//		Comment("Unique index for username lookups")
//
//	// Multi-column index
//	index := astbuilder.NewIndex("idx_posts_user_date", "posts", "user_id", "created_at").
//		Type("BTREE").
//		Comment("Composite index for user posts by date")
//
// All methods return the IndexBuilder instance for chaining, except:
//   - Build() returns the completed AST IndexNode
type IndexBuilder struct {
	// index is the underlying AST node being configured
	index *ast.IndexNode
}

// NewIndex creates a new IndexBuilder for the specified index name, table, and columns.
//
// The index will be created on the specified table and columns. Multiple columns
// can be specified to create a composite index. The order of columns matters for
// query optimization.
//
// Parameters:
//   - name: The index name (must be unique within the database)
//   - table: The table name to create the index on
//   - columns: One or more column names to include in the index
//
// Examples:
//
//	// Single column index
//	index := astbuilder.NewIndex("idx_users_email", "users", "email")
//
//	// Multi-column composite index
//	index := astbuilder.NewIndex("idx_orders_user_date", "orders", "user_id", "order_date")
//
//	// Index on multiple columns for complex queries
//	index := astbuilder.NewIndex("idx_products_cat_price", "products", "category_id", "price", "name")
func NewIndex(name, table string, columns ...string) *IndexBuilder {
	return &IndexBuilder{
		index: ast.NewIndex(name, table, columns...),
	}
}

// Unique marks the index as unique and returns the IndexBuilder for chaining.
//
// Unique indexes enforce uniqueness constraints on the indexed columns,
// preventing duplicate values. They also provide performance benefits for
// lookups. A unique index on a single column ensures all values in that
// column are distinct. A unique index on multiple columns ensures the
// combination of values is unique.
//
// Examples:
//
//	// Unique index on email column
//	index := astbuilder.NewIndex("idx_users_email", "users", "email").Unique()
//
//	// Unique composite index
//	index := astbuilder.NewIndex("idx_user_posts_slug", "posts", "user_id", "slug").Unique()
func (ib *IndexBuilder) Unique() *IndexBuilder {
	ib.index.SetUnique()
	return ib
}

// Type sets the index type and returns the IndexBuilder for chaining.
//
// The index type determines the underlying data structure and algorithm used
// for the index. Different types are optimized for different query patterns
// and data characteristics. Support varies by database system.
//
// Common index types:
//   - "BTREE": Balanced tree (default, good for range queries and equality)
//   - "HASH": Hash table (fast equality lookups, no range queries)
//   - "GIN": PostgreSQL generalized inverted index (arrays, JSON, full-text)
//   - "GIST": PostgreSQL generalized search tree (geometric data, full-text)
//   - "FULLTEXT": Full-text search index (MySQL, some other databases)
//
// Examples:
//
//	// B-tree index for range queries
//	index := astbuilder.NewIndex("idx_orders_date", "orders", "order_date").Type("BTREE")
//
//	// Hash index for fast equality lookups
//	index := astbuilder.NewIndex("idx_sessions_token", "sessions", "token").Type("HASH")
//
//	// PostgreSQL GIN index for JSON data
//	index := astbuilder.NewIndex("idx_products_tags", "products", "tags").Type("GIN")
func (ib *IndexBuilder) Type(indexType string) *IndexBuilder {
	ib.index.Type = indexType
	return ib
}

// Comment sets a descriptive comment for the index and returns the IndexBuilder for chaining.
//
// Comments are useful for documenting the purpose, usage patterns, or performance
// characteristics of an index. Comment support varies by database system, but most
// modern databases support them.
//
// Examples:
//
//	index := astbuilder.NewIndex("idx_users_email", "users", "email").
//		Comment("Unique index for user email lookups and authentication")
//
//	index := astbuilder.NewIndex("idx_orders_status_date", "orders", "status", "created_at").
//		Comment("Composite index for order status reports and date range queries")
//
//	index := astbuilder.NewIndex("idx_products_search", "products", "name", "description").
//		Type("FULLTEXT").
//		Comment("Full-text search index for product search functionality")
func (ib *IndexBuilder) Comment(comment string) *IndexBuilder {
	ib.index.Comment = comment
	return ib
}

// Build returns the completed CREATE INDEX AST node.
//
// This method finalizes the index configuration and returns the underlying
// AST IndexNode that can be processed by dialect-specific renderers to
// generate SQL CREATE INDEX statements.
//
// Example:
//
//	index := astbuilder.NewIndex("idx_users_email", "users", "email").
//		Unique().
//		Type("BTREE").
//		Comment("Unique email index")
//
//	node := index.Build()
//	// node can now be rendered to SQL by a dialect-specific renderer
func (ib *IndexBuilder) Build() *ast.IndexNode {
	return ib.index
}

// SchemaIndexBuilder provides a fluent API for building indexes within schema contexts.
//
// SchemaIndexBuilder wraps an IndexBuilder and operates within the context of
// a SchemaBuilder rather than standalone usage. It provides the same index
// configuration methods but returns to a SchemaBuilder context when End() is called.
//
// This builder is used when constructing indexes as part of a larger schema definition,
// allowing seamless navigation between schema and index contexts. It embeds IndexBuilder
// to inherit all index configuration functionality.
//
// Example usage within a schema:
//
//	schema := astbuilder.NewSchema().
//		Table("users").
//			Column("id", "SERIAL").Primary().End().
//			Column("email", "VARCHAR(255)").NotNull().Unique().End().
//		End().
//		Index("idx_users_email", "users", "email").
//			Unique().
//			Type("BTREE").
//			Comment("Unique index for email lookups").
//			End().
//		Index("idx_users_created", "users", "created_at").
//			Type("BTREE").
//			Comment("Index for date-based queries").
//			End()
//
// All methods return the SchemaIndexBuilder instance for chaining, except:
//   - End() returns the parent SchemaBuilder to continue schema construction
type SchemaIndexBuilder struct {
	// IndexBuilder provides all index configuration methods
	*IndexBuilder
	// schema is the parent schema builder for returning context
	schema *SchemaBuilder
}

// Unique marks the index as unique and returns the SchemaIndexBuilder for chaining.
//
// Unique indexes enforce uniqueness constraints on the indexed columns,
// preventing duplicate values. They also provide performance benefits for
// lookups. This method delegates to the embedded IndexBuilder.
//
// Examples:
//
//	// Unique index within schema
//	schema.Index("idx_users_email", "users", "email").Unique()
//
//	// Unique composite index within schema
//	schema.Index("idx_user_posts_slug", "posts", "user_id", "slug").Unique()
func (sib *SchemaIndexBuilder) Unique() *SchemaIndexBuilder {
	sib.IndexBuilder.Unique()
	return sib
}

// Type sets the index type and returns the SchemaIndexBuilder for chaining.
//
// The index type determines the underlying data structure and algorithm used
// for the index. Different types are optimized for different query patterns
// and data characteristics. This method delegates to the embedded IndexBuilder.
//
// Common index types:
//   - "BTREE": Balanced tree (default, good for range queries and equality)
//   - "HASH": Hash table (fast equality lookups, no range queries)
//   - "GIN": PostgreSQL generalized inverted index (arrays, JSON, full-text)
//   - "GIST": PostgreSQL generalized search tree (geometric data, full-text)
//   - "FULLTEXT": Full-text search index (MySQL, some other databases)
//
// Examples:
//
//	// B-tree index within schema
//	schema.Index("idx_orders_date", "orders", "order_date").Type("BTREE")
//
//	// PostgreSQL GIN index within schema
//	schema.Index("idx_products_tags", "products", "tags").Type("GIN")
func (sib *SchemaIndexBuilder) Type(indexType string) *SchemaIndexBuilder {
	sib.IndexBuilder.Type(indexType)
	return sib
}

// Comment sets a descriptive comment for the index and returns the SchemaIndexBuilder for chaining.
//
// Comments are useful for documenting the purpose, usage patterns, or performance
// characteristics of an index. This method delegates to the embedded IndexBuilder.
//
// Examples:
//
//	schema.Index("idx_users_email", "users", "email").
//		Comment("Unique index for user email lookups and authentication")
//
//	schema.Index("idx_orders_status_date", "orders", "status", "created_at").
//		Comment("Composite index for order status reports and date range queries")
func (sib *SchemaIndexBuilder) Comment(comment string) *SchemaIndexBuilder {
	sib.IndexBuilder.Comment(comment)
	return sib
}

// End completes the index definition and returns to the parent SchemaBuilder for chaining.
//
// This method finalizes the index configuration, adds the completed index to the
// schema's statement list, and returns to the schema context for continued schema
// construction with additional tables, indexes, or other schema elements.
//
// Example:
//
//	schema := astbuilder.NewSchema().
//		Table("users").
//			Column("id", "SERIAL").Primary().End().
//			Column("email", "VARCHAR(255)").NotNull().End().
//		End().
//		Index("idx_users_email", "users", "email").
//			Unique().
//			Type("BTREE").
//			Comment("Email lookup index").
//			End().
//		Table("posts").
//			Column("id", "SERIAL").Primary().End().
//		End()
func (sib *SchemaIndexBuilder) End() *SchemaBuilder {
	sib.schema.statements = append(sib.schema.statements, sib.IndexBuilder.Build())
	return sib.schema
}
