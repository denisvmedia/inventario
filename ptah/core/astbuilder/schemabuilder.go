package astbuilder

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// SchemaBuilder provides a fluent API for building complete database schemas.
//
// SchemaBuilder is the top-level builder in the astbuilder package hierarchy and
// serves as the entry point for constructing comprehensive database schemas. It
// maintains a collection of AST statements (tables, indexes, enums, comments) and
// provides methods to add various schema elements in a fluent, chainable manner.
//
// The builder supports all major schema elements:
//   - Comments: Documentation and schema descriptions
//   - Enums: Custom data types (PostgreSQL-specific)
//   - Tables: Complete table definitions with columns and constraints
//   - Indexes: Performance optimization indexes
//
// Schema construction follows a hierarchical pattern where the SchemaBuilder
// delegates to specialized builders (SchemaTableBuilder, SchemaIndexBuilder)
// that can return to the schema context when their definitions are complete.
//
// Example usage:
//
//	schema := astbuilder.NewSchema().
//		Comment("User management and content system").
//		Enum("user_status", "active", "inactive", "suspended").
//		Table("users").
//			Column("id", "SERIAL").Primary().End().
//			Column("email", "VARCHAR(255)").NotNull().Unique().End().
//			Column("status", "user_status").NotNull().Default("'active'").End().
//			Column("created_at", "TIMESTAMP").NotNull().DefaultExpression("NOW()").End().
//		End().
//		Table("posts").
//			Column("id", "SERIAL").Primary().End().
//			Column("user_id", "INTEGER").NotNull().
//				ForeignKey("users", "id", "fk_posts_user").
//				OnDelete("CASCADE").End().
//			Column("title", "VARCHAR(255)").NotNull().End().
//			Column("content", "TEXT").End().
//			Column("created_at", "TIMESTAMP").NotNull().DefaultExpression("NOW()").End().
//		End().
//		Index("idx_posts_user_date", "posts", "user_id", "created_at").
//			Type("BTREE").
//			Comment("Index for user posts by date").
//			End().
//		Index("idx_users_email", "users", "email").
//			Unique().
//			Comment("Unique index for email lookups").
//			End()
//
//	result := schema.Build()
//
// All methods return the SchemaBuilder instance for chaining, except:
//   - Table() returns a SchemaTableBuilder for table construction
//   - Index() returns a SchemaIndexBuilder for index construction
//   - Build() returns the completed AST StatementList
type SchemaBuilder struct {
	// statements contains all schema elements (tables, indexes, enums, comments)
	statements []ast.Node
}

// NewSchema creates a new SchemaBuilder for constructing database schemas.
//
// The returned builder has an empty statement list and is ready to accept
// schema elements through its fluent API methods.
//
// Example:
//
//	schema := astbuilder.NewSchema()
//	// Begin adding schema elements...
func NewSchema() *SchemaBuilder {
	return &SchemaBuilder{
		statements: make([]ast.Node, 0),
	}
}

// Comment adds a documentation comment to the schema and returns the SchemaBuilder for chaining.
//
// Comments are useful for documenting the purpose, version, or other metadata
// about the schema. They will be included in the generated SQL as SQL comments.
// Multiple comments can be added and will appear in the order they were added.
//
// Examples:
//
//	schema := astbuilder.NewSchema().
//		Comment("User management schema v2.1").
//		Comment("Created: 2023-12-01").
//		Comment("Contains user accounts, profiles, and authentication data")
func (sb *SchemaBuilder) Comment(text string) *SchemaBuilder {
	sb.statements = append(sb.statements, ast.NewComment(text))
	return sb
}

// Enum adds a custom enumeration type definition and returns the SchemaBuilder for chaining.
//
// Enums are custom data types that restrict values to a predefined set of options.
// This feature is primarily supported by PostgreSQL, though some other databases
// have similar functionality. The enum will be created before any tables that
// reference it.
//
// Parameters:
//   - name: The enum type name
//   - values: One or more string values that the enum can contain
//
// Examples:
//
//	// Simple status enum
//	schema := astbuilder.NewSchema().
//		Enum("user_status", "active", "inactive", "suspended")
//
//	// Multiple enums for different purposes
//	schema := astbuilder.NewSchema().
//		Enum("user_role", "admin", "moderator", "user", "guest").
//		Enum("post_status", "draft", "published", "archived").
//		Enum("priority_level", "low", "medium", "high", "critical")
//
// Note: Enum support varies by database. PostgreSQL has native enum support,
// while other databases may need to use CHECK constraints or lookup tables.
func (sb *SchemaBuilder) Enum(name string, values ...string) *SchemaBuilder {
	sb.statements = append(sb.statements, ast.NewEnum(name, values...))
	return sb
}

// Table begins a table definition and returns a SchemaTableBuilder for detailed table construction.
//
// This method creates a new table builder that operates within the schema context.
// The returned SchemaTableBuilder provides methods for adding columns, constraints,
// and other table properties. When the table definition is complete, calling End()
// on the table builder will add the table to the schema and return to this SchemaBuilder.
//
// Parameters:
//   - name: The table name
//
// Examples:
//
//	// Simple table with basic columns
//	schema := astbuilder.NewSchema().
//		Table("users").
//			Column("id", "SERIAL").Primary().End().
//			Column("name", "VARCHAR(100)").NotNull().End().
//		End()
//
//	// Complex table with relationships and constraints
//	schema := astbuilder.NewSchema().
//		Table("orders").
//			Comment("Customer order records").
//			Column("id", "SERIAL").Primary().End().
//			Column("customer_id", "INTEGER").NotNull().
//				ForeignKey("customers", "id", "fk_orders_customer").
//				OnDelete("RESTRICT").End().
//			Column("total", "DECIMAL(10,2)").NotNull().Check("total >= 0").End().
//			Column("status", "order_status").NotNull().Default("'pending'").End().
//			Column("created_at", "TIMESTAMP").NotNull().DefaultExpression("NOW()").End().
//		End()
func (sb *SchemaBuilder) Table(name string) *SchemaTableBuilder {
	tb := NewTable(name)
	// We'll add the table to statements when End() is called
	return &SchemaTableBuilder{
		TableBuilder: tb,
		schema:       sb,
	}
}

// Index begins an index definition and returns a SchemaIndexBuilder for detailed index construction.
//
// This method creates a new index builder that operates within the schema context.
// The returned SchemaIndexBuilder provides methods for configuring index properties
// such as uniqueness, type, and comments. When the index definition is complete,
// calling End() on the index builder will add the index to the schema and return
// to this SchemaBuilder.
//
// Parameters:
//   - name: The index name (must be unique within the database)
//   - table: The table name to create the index on
//   - columns: One or more column names to include in the index
//
// Examples:
//
//	// Simple performance index
//	schema := astbuilder.NewSchema().
//		Index("idx_users_email", "users", "email").
//			Type("BTREE").
//			Comment("Index for email lookups").
//			End()
//
//	// Unique composite index
//	schema := astbuilder.NewSchema().
//		Index("idx_user_posts_slug", "posts", "user_id", "slug").
//			Unique().
//			Comment("Ensure unique slugs per user").
//			End()
//
//	// Multiple indexes for different query patterns
//	schema := astbuilder.NewSchema().
//		Index("idx_orders_customer", "orders", "customer_id").End().
//		Index("idx_orders_date", "orders", "created_at").End().
//		Index("idx_orders_status_date", "orders", "status", "created_at").
//			Comment("Composite index for status reports").
//			End()
func (sb *SchemaBuilder) Index(name, table string, columns ...string) *SchemaIndexBuilder {
	ib := NewIndex(name, table, columns...)
	// We'll add the index to statements when End() is called
	return &SchemaIndexBuilder{
		IndexBuilder: ib,
		schema:       sb,
	}
}

// Build returns the completed schema as an AST StatementList.
//
// This method finalizes the schema construction and returns the underlying
// AST StatementList containing all schema elements (comments, enums, tables,
// indexes) in the order they were added. The returned StatementList can be
// processed by dialect-specific renderers to generate SQL DDL statements.
//
// Example:
//
//	schema := astbuilder.NewSchema().
//		Comment("Application schema").
//		Table("users").
//			Column("id", "SERIAL").Primary().End().
//		End().
//		Index("idx_users_id", "users", "id").End()
//
//	result := schema.Build()
//	// result contains AST nodes that can be rendered to SQL
func (sb *SchemaBuilder) Build() *ast.StatementList {
	return &ast.StatementList{
		Statements: sb.statements,
	}
}
