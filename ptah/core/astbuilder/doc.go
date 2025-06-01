// Package astbuilder provides fluent APIs for building SQL DDL Abstract Syntax Trees (AST).
//
// This package is part of the Ptah schema management tool and provides a convenient
// fluent interface for constructing SQL DDL statements through AST nodes. It serves
// as a higher-level abstraction over the core AST package, making it easier to build
// complex database schemas programmatically.
//
// # Architecture
//
// The package follows the builder pattern with fluent interfaces, providing these
// main components:
//
//   - SchemaBuilder: Top-level builder for complete database schemas
//   - TableBuilder: Builder for CREATE TABLE statements
//   - ColumnBuilder: Builder for column definitions with constraints
//   - IndexBuilder: Builder for CREATE INDEX statements
//   - ForeignKeyBuilder: Builder for foreign key constraints
//
// Each builder provides method chaining for a fluent API and integrates seamlessly
// with the underlying AST nodes from the ptah/core/ast package.
//
// # Schema Builder
//
// The SchemaBuilder provides the entry point for building complete database schemas:
//
//	schema := astbuilder.NewSchema().
//		Comment("User management schema").
//		Enum("user_status", "active", "inactive", "suspended").
//		Table("users").
//			Column("id", "SERIAL").Primary().End().
//			Column("email", "VARCHAR(255)").NotNull().Unique().End().
//			Column("status", "user_status").NotNull().Default("'active'").End().
//		End().
//		Index("idx_users_email", "users", "email").Unique().End()
//
//	result := schema.Build()
//
// # Table Builder
//
// The TableBuilder handles CREATE TABLE statements with columns and constraints:
//
//	table := astbuilder.NewTable("posts").
//		Comment("Blog posts table").
//		Engine("InnoDB").
//		Option("CHARSET", "utf8mb4").
//		Column("id", "SERIAL").Primary().End().
//		Column("user_id", "INTEGER").NotNull().
//			ForeignKey("users", "id", "fk_posts_user").
//			OnDelete("CASCADE").End().
//		Column("title", "VARCHAR(255)").NotNull().End().
//		Unique("uk_posts_title", "title")
//
//	result := table.Build()
//
// # Column Builder
//
// The ColumnBuilder provides detailed column configuration:
//
//	column := table.Column("created_at", "TIMESTAMP").
//		NotNull().
//		DefaultExpression("NOW()").
//		Comment("Record creation timestamp").
//		End()
//
// Supported column attributes:
//   - Primary(): Mark as primary key
//   - NotNull(): Set NOT NULL constraint
//   - Unique(): Add unique constraint
//   - Default(value): Set default value
//   - DefaultExpression(expr): Set default expression
//   - AutoIncrement(): Enable auto-increment
//   - Comment(text): Add column comment
//   - ForeignKey(table, column, name): Add foreign key reference
//
// # Index Builder
//
// The IndexBuilder handles CREATE INDEX statements:
//
//	index := astbuilder.NewIndex("idx_users_status", "users", "status", "created_at").
//		Unique().
//		Type("BTREE").
//		Comment("Index for user status queries")
//
//	result := index.Build()
//
// # Foreign Key Builder
//
// The ForeignKeyBuilder configures foreign key constraints:
//
//	fk := column.ForeignKey("users", "id", "fk_posts_user").
//		OnDelete("CASCADE").
//		OnUpdate("RESTRICT").
//		End()
//
// Supported referential actions:
//   - CASCADE: Cascade changes to referenced rows
//   - RESTRICT: Prevent changes that would violate constraint
//   - SET NULL: Set referencing columns to NULL
//   - SET DEFAULT: Set referencing columns to default values
//   - NO ACTION: No action (database-specific behavior)
//
// # Fluent Interface Design
//
// All builders support method chaining and provide End() methods to return to
// parent builders, enabling nested construction:
//
//	schema := astbuilder.NewSchema().
//		Table("users").
//			Column("id", "SERIAL").Primary().End().
//			Column("email", "VARCHAR(255)").NotNull().End().
//		End().
//		Table("posts").
//			Column("user_id", "INTEGER").
//				ForeignKey("users", "id", "fk_posts_user").
//				OnDelete("CASCADE").End().
//		End()
//
// # Integration with AST
//
// All builders produce AST nodes from the ptah/core/ast package:
//
//   - SchemaBuilder.Build() returns *ast.StatementList
//   - TableBuilder.Build() returns *ast.CreateTableNode
//   - IndexBuilder.Build() returns *ast.IndexNode
//   - Column and foreign key builders modify existing AST nodes
//
// These AST nodes can then be processed by dialect-specific renderers to generate
// SQL for different database platforms (PostgreSQL, MySQL, MariaDB, etc.).
//
// # Usage Examples
//
// Simple table creation:
//
//	table := astbuilder.NewTable("users").
//		Column("id", "INTEGER").Primary().AutoIncrement().End().
//		Column("name", "VARCHAR(100)").NotNull().End().
//		Column("email", "VARCHAR(255)").NotNull().Unique().End()
//
// Complex schema with relationships:
//
//	schema := astbuilder.NewSchema().
//		Table("categories").
//			Column("id", "SERIAL").Primary().End().
//			Column("name", "VARCHAR(100)").NotNull().Unique().End().
//		End().
//		Table("products").
//			Column("id", "SERIAL").Primary().End().
//			Column("category_id", "INTEGER").NotNull().
//				ForeignKey("categories", "id", "fk_products_category").
//				OnDelete("RESTRICT").End().
//			Column("name", "VARCHAR(200)").NotNull().End().
//			Column("price", "DECIMAL(10,2)").NotNull().End().
//		End().
//		Index("idx_products_category", "products", "category_id").End().
//		Index("idx_products_name", "products", "name").End()
//
// Database-specific features:
//
//	// MySQL/MariaDB table with engine and charset
//	table := astbuilder.NewTable("logs").
//		Engine("InnoDB").
//		Option("CHARSET", "utf8mb4").
//		Option("COLLATE", "utf8mb4_unicode_ci").
//		Column("id", "BIGINT").Primary().AutoIncrement().End().
//		Column("message", "TEXT").NotNull().End()
//
//	// PostgreSQL with enum
//	schema := astbuilder.NewSchema().
//		Enum("status_type", "active", "inactive", "pending").
//		Table("users").
//			Column("status", "status_type").NotNull().Default("'active'").End().
//		End()
//
// # Error Handling
//
// The builders themselves do not perform validation - they construct AST nodes
// that can be validated later by the rendering pipeline. Invalid configurations
// will be caught during SQL generation or database execution.
//
// # Thread Safety
//
// The builders are not thread-safe and should not be used concurrently from
// multiple goroutines. Each builder instance should be used by a single goroutine.
//
// # Integration with Ptah
//
// This package integrates with other Ptah components:
//
//   - ptah/core/ast: Provides the underlying AST node types
//   - ptah/core/renderer: Renders AST nodes to dialect-specific SQL
//   - ptah/migration/schemadiff: Compares schemas for migration generation
//   - ptah/core/goschema: Parses Go structs to generate builders
//   - ptah/core/parser: Parses SQL DDL to generate AST nodes
//
// The astbuilder package serves as the programmatic interface for schema construction
// in the Ptah ecosystem, bridging high-level schema definitions and low-level
// AST manipulation.
package astbuilder

