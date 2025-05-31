package astbuilder

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// TableBuilder provides a fluent API for building CREATE TABLE statements.
//
// TableBuilder wraps an AST CreateTableNode and provides a convenient fluent interface
// for configuring table properties such as columns, constraints, comments, and
// database-specific options. It can be used standalone or as part of a larger
// schema definition.
//
// The builder supports all standard SQL table elements:
//   - Columns: Individual column definitions with constraints
//   - Table-level constraints: Primary keys, unique constraints, foreign keys
//   - Comments: Table documentation
//   - Database-specific options: Engine, charset, collation, etc.
//
// Table construction follows a hierarchical pattern where the TableBuilder
// delegates to specialized builders (ColumnBuilder, ForeignKeyBuilder) that
// can return to the table context when their definitions are complete.
//
// Example usage:
//
//	table := astbuilder.NewTable("users").
//		Comment("User account information").
//		Engine("InnoDB").
//		Option("CHARSET", "utf8mb4").
//		Column("id", "SERIAL").Primary().AutoIncrement().Comment("Primary key").End().
//		Column("email", "VARCHAR(255)").NotNull().Unique().Comment("Login email").End().
//		Column("username", "VARCHAR(50)").NotNull().End().
//		Column("password_hash", "VARCHAR(255)").NotNull().End().
//		Column("created_at", "TIMESTAMP").NotNull().DefaultExpression("NOW()").End().
//		Column("updated_at", "TIMESTAMP").DefaultExpression("NOW()").End().
//		PrimaryKey("id").
//		Unique("uk_users_email", "email").
//		Unique("uk_users_username", "username")
//
//	result := table.Build()
//
// All methods return the TableBuilder instance for chaining, except:
//   - Column() returns a ColumnBuilder for column construction
//   - ForeignKey() returns a ForeignKeyBuilder for foreign key construction
//   - Build() returns the completed AST CreateTableNode
type TableBuilder struct {
	// table is the underlying AST node being configured
	table *ast.CreateTableNode
}

// NewTable creates a new TableBuilder for the specified table name.
//
// The returned builder has an empty table definition and is ready to accept
// columns, constraints, and other table properties through its fluent API methods.
//
// Parameters:
//   - name: The table name
//
// Example:
//
//	table := astbuilder.NewTable("products")
//	// Begin adding columns and constraints...
func NewTable(name string) *TableBuilder {
	return &TableBuilder{
		table: ast.NewCreateTable(name),
	}
}

// Comment sets a descriptive comment for the table and returns the TableBuilder for chaining.
//
// Comments are useful for documenting the purpose, usage, or other metadata
// about the table. Comment support varies by database system, but most modern
// databases support them.
//
// Examples:
//
//	table := astbuilder.NewTable("users").
//		Comment("User account and profile information")
//
//	table := astbuilder.NewTable("audit_log").
//		Comment("System audit trail for security and compliance tracking")
func (tb *TableBuilder) Comment(comment string) *TableBuilder {
	tb.table.Comment = comment
	return tb
}

// Engine sets the table storage engine and returns the TableBuilder for chaining.
//
// This is primarily used for MySQL and MariaDB databases where different storage
// engines provide different features and performance characteristics. Common engines
// include InnoDB (default, supports transactions and foreign keys) and MyISAM
// (faster for read-heavy workloads but no transactions).
//
// Examples:
//
//	// InnoDB for transactional tables
//	table := astbuilder.NewTable("orders").Engine("InnoDB")
//
//	// MyISAM for read-heavy lookup tables
//	table := astbuilder.NewTable("zip_codes").Engine("MyISAM")
//
//	// Memory engine for temporary tables
//	table := astbuilder.NewTable("session_cache").Engine("MEMORY")
func (tb *TableBuilder) Engine(engine string) *TableBuilder {
	tb.table.SetOption("ENGINE", engine)
	return tb
}

// Option sets a custom table option and returns the TableBuilder for chaining.
//
// Table options are database-specific settings that control various aspects of
// table behavior, storage, and performance. Different databases support different
// options.
//
// Common MySQL/MariaDB options:
//   - CHARSET: Character set (utf8mb4, latin1, etc.)
//   - COLLATE: Collation rules (utf8mb4_unicode_ci, etc.)
//   - ROW_FORMAT: Storage format (DYNAMIC, COMPRESSED, etc.)
//   - AUTO_INCREMENT: Starting value for auto-increment columns
//
// Examples:
//
//	// MySQL/MariaDB character set and collation
//	table := astbuilder.NewTable("posts").
//		Option("CHARSET", "utf8mb4").
//		Option("COLLATE", "utf8mb4_unicode_ci")
//
//	// Row format for compression
//	table := astbuilder.NewTable("large_data").
//		Option("ROW_FORMAT", "COMPRESSED")
//
//	// Auto-increment starting value
//	table := astbuilder.NewTable("products").
//		Option("AUTO_INCREMENT", "1000")
func (tb *TableBuilder) Option(key, value string) *TableBuilder {
	tb.table.SetOption(key, value)
	return tb
}

// Column begins a column definition and returns a ColumnBuilder for detailed column construction.
//
// This method creates a new column builder that operates within the table context.
// The returned ColumnBuilder provides methods for configuring column properties
// such as constraints, default values, and comments. When the column definition
// is complete, calling End() on the column builder will return to this TableBuilder.
//
// Parameters:
//   - name: The column name
//   - dataType: The column data type (e.g., "INTEGER", "VARCHAR(255)", "TIMESTAMP")
//
// Examples:
//
//	// Simple column
//	table.Column("name", "VARCHAR(100)")
//
//	// Column with constraints
//	table.Column("id", "SERIAL").Primary().AutoIncrement().End()
//
//	// Column with default and comment
//	table.Column("status", "VARCHAR(20)").NotNull().Default("'active'").
//		Comment("Account status").End()
//
//	// Foreign key column
//	table.Column("user_id", "INTEGER").NotNull().
//		ForeignKey("users", "id", "fk_posts_user").
//		OnDelete("CASCADE").End()
func (tb *TableBuilder) Column(name, dataType string) *ColumnBuilder {
	column := ast.NewColumn(name, dataType)
	tb.table.AddColumn(column)
	return &ColumnBuilder{
		column: column,
		table:  tb,
	}
}

// PrimaryKey adds a composite primary key constraint and returns the TableBuilder for chaining.
//
// This creates a table-level primary key constraint that spans multiple columns.
// For single-column primary keys, it's often more convenient to use the Primary()
// method on the column builder instead.
//
// A primary key constraint ensures that the combination of values in the specified
// columns is unique across all rows and that none of the columns can be NULL.
//
// Parameters:
//   - columns: One or more column names that form the primary key
//
// Examples:
//
//	// Single column primary key (table-level)
//	table := astbuilder.NewTable("users").
//		Column("id", "SERIAL").End().
//		PrimaryKey("id")
//
//	// Composite primary key
//	table := astbuilder.NewTable("user_roles").
//		Column("user_id", "INTEGER").End().
//		Column("role_id", "INTEGER").End().
//		PrimaryKey("user_id", "role_id")
//
//	// Multi-column primary key for junction table
//	table := astbuilder.NewTable("post_tags").
//		Column("post_id", "INTEGER").End().
//		Column("tag_id", "INTEGER").End().
//		Column("created_at", "TIMESTAMP").End().
//		PrimaryKey("post_id", "tag_id")
func (tb *TableBuilder) PrimaryKey(columns ...string) *TableBuilder {
	constraint := ast.NewPrimaryKeyConstraint(columns...)
	tb.table.AddConstraint(constraint)
	return tb
}

// Unique adds a unique constraint and returns the TableBuilder for chaining.
//
// This creates a table-level unique constraint that ensures the combination of
// values in the specified columns is unique across all rows. Unlike primary keys,
// unique constraints allow NULL values (though the combination must still be unique).
//
// Parameters:
//   - name: The constraint name (for referencing in error messages and metadata)
//   - columns: One or more column names that must be unique together
//
// Examples:
//
//	// Single column unique constraint
//	table := astbuilder.NewTable("users").
//		Column("email", "VARCHAR(255)").End().
//		Unique("uk_users_email", "email")
//
//	// Multi-column unique constraint
//	table := astbuilder.NewTable("products").
//		Column("category_id", "INTEGER").End().
//		Column("sku", "VARCHAR(50)").End().
//		Unique("uk_products_category_sku", "category_id", "sku")
//
//	// Multiple unique constraints
//	table := astbuilder.NewTable("users").
//		Column("email", "VARCHAR(255)").End().
//		Column("username", "VARCHAR(50)").End().
//		Unique("uk_users_email", "email").
//		Unique("uk_users_username", "username")
func (tb *TableBuilder) Unique(name string, columns ...string) *TableBuilder {
	constraint := ast.NewUniqueConstraint(name, columns...)
	tb.table.AddConstraint(constraint)
	return tb
}

// ForeignKey adds a table-level foreign key constraint and returns a ForeignKeyBuilder for configuration.
//
// This creates a table-level foreign key constraint that references another table's
// column(s). The returned ForeignKeyBuilder allows configuration of referential
// actions (ON DELETE, ON UPDATE) before returning to the table context.
//
// Table-level foreign keys are useful for multi-column foreign keys or when you
// want to define the constraint separately from the column definitions.
//
// Parameters:
//   - name: The constraint name
//   - columns: The local column names that reference the foreign table
//   - refTable: The referenced table name
//   - refColumn: The referenced column name in the target table
//
// Examples:
//
//	// Single column foreign key
//	table := astbuilder.NewTable("posts").
//		Column("user_id", "INTEGER").End().
//		ForeignKey("fk_posts_user", []string{"user_id"}, "users", "id").
//		OnDelete("CASCADE").End()
//
//	// Multi-column foreign key
//	table := astbuilder.NewTable("order_items").
//		Column("product_id", "INTEGER").End().
//		Column("variant_id", "INTEGER").End().
//		ForeignKey("fk_order_items_product", []string{"product_id", "variant_id"}, "product_variants", "product_id").
//		OnDelete("RESTRICT").End()
//
// Note: For single-column foreign keys, it's often more convenient to use the
// ForeignKey() method on the column builder instead.
func (tb *TableBuilder) ForeignKey(name string, columns []string, refTable, refColumn string) *ForeignKeyBuilder {
	ref := &ast.ForeignKeyRef{
		Table:  refTable,
		Column: refColumn,
		Name:   name,
	}
	constraint := ast.NewForeignKeyConstraint(name, columns, ref)
	tb.table.AddConstraint(constraint)

	return &ForeignKeyBuilder{
		ref:   ref,
		table: tb,
	}
}

// Build returns the completed CREATE TABLE AST node.
//
// This method finalizes the table configuration and returns the underlying
// AST CreateTableNode that can be processed by dialect-specific renderers to
// generate SQL CREATE TABLE statements.
//
// Example:
//
//	table := astbuilder.NewTable("users").
//		Comment("User accounts").
//		Column("id", "SERIAL").Primary().End().
//		Column("email", "VARCHAR(255)").NotNull().Unique().End().
//		Unique("uk_users_email", "email")
//
//	node := table.Build()
//	// node can now be rendered to SQL by a dialect-specific renderer
func (tb *TableBuilder) Build() *ast.CreateTableNode {
	return tb.table
}

// SchemaTableBuilder provides a fluent API for building tables within schema contexts.
//
// SchemaTableBuilder wraps a TableBuilder and operates within the context of
// a SchemaBuilder rather than standalone usage. It embeds TableBuilder to inherit
// all table configuration functionality and provides schema-specific column building
// that returns to the schema context.
//
// This builder is used when constructing tables as part of a larger schema definition,
// allowing seamless navigation between schema, table, and column contexts. It provides
// the same table configuration methods as TableBuilder but with schema-aware navigation.
//
// Example usage within a schema:
//
//	schema := astbuilder.NewSchema().
//		Comment("E-commerce database schema").
//		Table("users").
//			Comment("Customer accounts").
//			Engine("InnoDB").
//			Option("CHARSET", "utf8mb4").
//			Column("id", "SERIAL").Primary().AutoIncrement().End().
//			Column("email", "VARCHAR(255)").NotNull().Unique().End().
//			Column("created_at", "TIMESTAMP").NotNull().DefaultExpression("NOW()").End().
//			PrimaryKey("id").
//			Unique("uk_users_email", "email").
//		End().
//		Table("orders").
//			Column("id", "SERIAL").Primary().End().
//			Column("user_id", "INTEGER").NotNull().
//				ForeignKey("users", "id", "fk_orders_user").
//				OnDelete("RESTRICT").End().
//		End()
//
// All inherited methods from TableBuilder return the SchemaTableBuilder instance for chaining, except:
//   - Column() returns a SchemaColumnBuilder for schema-aware column construction
//   - End() returns the parent SchemaBuilder to continue schema construction
type SchemaTableBuilder struct {
	// TableBuilder provides all table configuration methods
	*TableBuilder
	// schema is the parent schema builder for returning context
	schema *SchemaBuilder
}

// Column begins a column definition and returns a SchemaColumnBuilder for detailed column construction.
//
// This method creates a new schema-aware column builder that operates within the
// schema table context. The returned SchemaColumnBuilder provides methods for
// configuring column properties and can return to this SchemaTableBuilder when
// the column definition is complete.
//
// This method overrides the embedded TableBuilder.Column() method to provide
// schema-aware navigation while maintaining the same column configuration capabilities.
//
// Parameters:
//   - name: The column name
//   - dataType: The column data type (e.g., "INTEGER", "VARCHAR(255)", "TIMESTAMP")
//
// Examples:
//
//	// Simple column within schema
//	schema.Table("users").Column("name", "VARCHAR(100)").NotNull().End()
//
//	// Column with constraints within schema
//	schema.Table("users").Column("id", "SERIAL").Primary().AutoIncrement().End()
//
//	// Foreign key column within schema
//	schema.Table("posts").Column("user_id", "INTEGER").NotNull().
//		ForeignKey("users", "id", "fk_posts_user").
//		OnDelete("CASCADE").End()
func (stb *SchemaTableBuilder) Column(name, dataType string) *SchemaColumnBuilder {
	column := ast.NewColumn(name, dataType)
	stb.TableBuilder.table.AddColumn(column)
	return &SchemaColumnBuilder{
		column:      column,
		schemaTable: stb,
	}
}

// End completes the table definition and returns to the parent SchemaBuilder for chaining.
//
// This method finalizes the table configuration, adds the completed table to the
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
//		Table("posts").
//			Column("id", "SERIAL").Primary().End().
//			Column("user_id", "INTEGER").NotNull().End().
//		End().
//		Index("idx_posts_user", "posts", "user_id").End()
func (stb *SchemaTableBuilder) End() *SchemaBuilder {
	stb.schema.statements = append(stb.schema.statements, stb.TableBuilder.Build())
	return stb.schema
}
