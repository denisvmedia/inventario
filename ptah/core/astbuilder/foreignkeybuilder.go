package astbuilder

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// ForeignKeyBuilder provides a fluent API for configuring foreign key constraints within table contexts.
//
// ForeignKeyBuilder is returned by ColumnBuilder.ForeignKey() and allows configuration of
// referential integrity actions (ON DELETE and ON UPDATE) for foreign key constraints.
// It wraps an AST ForeignKeyRef and maintains a reference to the parent TableBuilder
// to enable method chaining back to the table context.
//
// Foreign key constraints enforce referential integrity between tables by ensuring
// that values in the referencing column(s) exist in the referenced table's column(s).
// The builder allows configuration of what happens when referenced rows are updated
// or deleted.
//
// Supported referential actions:
//   - CASCADE: Automatically update/delete referencing rows when referenced row changes
//   - RESTRICT: Prevent update/delete of referenced row if referencing rows exist
//   - SET NULL: Set referencing columns to NULL when referenced row is updated/deleted
//   - SET DEFAULT: Set referencing columns to their default values
//   - NO ACTION: No action (database-specific behavior, often same as RESTRICT)
//
// Example usage:
//
//	table := astbuilder.NewTable("posts").
//		Column("user_id", "INTEGER").NotNull().
//			ForeignKey("users", "id", "fk_posts_user").
//			OnDelete("CASCADE").
//			OnUpdate("RESTRICT").
//			End().
//		Column("category_id", "INTEGER").
//			ForeignKey("categories", "id", "fk_posts_category").
//			OnDelete("SET NULL").
//			End()
//
// All methods return the ForeignKeyBuilder instance for chaining, except:
//   - End() returns the parent TableBuilder to continue table construction
type ForeignKeyBuilder struct {
	// ref is the underlying AST foreign key reference being configured
	ref *ast.ForeignKeyRef
	// table is the parent table builder for returning context
	table *TableBuilder
}

// OnDelete sets the ON DELETE referential action and returns the ForeignKeyBuilder for chaining.
//
// This configures what happens to referencing rows when a referenced row is deleted.
// The action must be one of the standard SQL referential actions.
//
// Supported actions:
//   - "CASCADE": Delete referencing rows when referenced row is deleted
//   - "RESTRICT": Prevent deletion of referenced row if referencing rows exist
//   - "SET NULL": Set foreign key columns to NULL when referenced row is deleted
//   - "SET DEFAULT": Set foreign key columns to their default values
//   - "NO ACTION": No action (database-specific, often same as RESTRICT)
//
// Examples:
//
//	// Delete posts when user is deleted
//	fk := column.ForeignKey("users", "id", "fk_posts_user").OnDelete("CASCADE")
//
//	// Prevent user deletion if posts exist
//	fk := column.ForeignKey("users", "id", "fk_posts_user").OnDelete("RESTRICT")
//
//	// Set user_id to NULL when user is deleted
//	fk := column.ForeignKey("users", "id", "fk_posts_user").OnDelete("SET NULL")
func (fkb *ForeignKeyBuilder) OnDelete(action string) *ForeignKeyBuilder {
	fkb.ref.OnDelete = action
	return fkb
}

// OnUpdate sets the ON UPDATE referential action and returns the ForeignKeyBuilder for chaining.
//
// This configures what happens to referencing rows when a referenced row's primary key is updated.
// The action must be one of the standard SQL referential actions.
//
// Supported actions:
//   - "CASCADE": Update foreign key values when referenced primary key changes
//   - "RESTRICT": Prevent update of referenced primary key if referencing rows exist
//   - "SET NULL": Set foreign key columns to NULL when referenced primary key changes
//   - "SET DEFAULT": Set foreign key columns to their default values
//   - "NO ACTION": No action (database-specific, often same as RESTRICT)
//
// Examples:
//
//	// Update foreign key when primary key changes
//	fk := column.ForeignKey("users", "id", "fk_posts_user").OnUpdate("CASCADE")
//
//	// Prevent primary key updates if foreign keys exist
//	fk := column.ForeignKey("users", "id", "fk_posts_user").OnUpdate("RESTRICT")
//
//	// Set foreign key to NULL when primary key changes
//	fk := column.ForeignKey("users", "id", "fk_posts_user").OnUpdate("SET NULL")
func (fkb *ForeignKeyBuilder) OnUpdate(action string) *ForeignKeyBuilder {
	fkb.ref.OnUpdate = action
	return fkb
}

// End completes the foreign key configuration and returns to the parent TableBuilder for chaining.
//
// This method is used to return to the table context after configuring all foreign key
// properties, allowing continued table construction with additional columns or constraints.
//
// Example:
//
//	table := astbuilder.NewTable("posts").
//		Column("user_id", "INTEGER").NotNull().
//			ForeignKey("users", "id", "fk_posts_user").
//			OnDelete("CASCADE").
//			OnUpdate("RESTRICT").
//			End().
//		Column("title", "VARCHAR(255)").NotNull().End()
func (fkb *ForeignKeyBuilder) End() *TableBuilder {
	return fkb.table
}

// SchemaForeignKeyBuilder provides a fluent API for configuring foreign key constraints within schema contexts.
//
// SchemaForeignKeyBuilder is similar to ForeignKeyBuilder but operates within the context of
// a SchemaBuilder rather than a standalone TableBuilder. It is returned by
// SchemaColumnBuilder.ForeignKey() and allows configuration of referential integrity actions
// (ON DELETE and ON UPDATE) for foreign key constraints within schema definitions.
//
// This builder is used when constructing foreign keys as part of a larger schema definition,
// allowing seamless navigation between schema, table, column, and foreign key contexts.
//
// Example usage within a schema:
//
//	schema := astbuilder.NewSchema().
//		Table("users").
//			Column("id", "SERIAL").Primary().End().
//		End().
//		Table("posts").
//			Column("user_id", "INTEGER").NotNull().
//				ForeignKey("users", "id", "fk_posts_user").
//				OnDelete("CASCADE").
//				OnUpdate("RESTRICT").
//				End().
//			Column("category_id", "INTEGER").
//				ForeignKey("categories", "id", "fk_posts_category").
//				OnDelete("SET NULL").
//				End().
//		End()
//
// All methods return the SchemaForeignKeyBuilder instance for chaining, except:
//   - End() returns the parent SchemaTableBuilder to continue table construction
type SchemaForeignKeyBuilder struct {
	// ref is the underlying AST foreign key reference being configured
	ref *ast.ForeignKeyRef
	// schemaTable is the parent schema table builder for returning context
	schemaTable *SchemaTableBuilder
}

// OnDelete sets the ON DELETE referential action and returns the SchemaForeignKeyBuilder for chaining.
//
// This configures what happens to referencing rows when a referenced row is deleted.
// The action must be one of the standard SQL referential actions.
//
// Supported actions:
//   - "CASCADE": Delete referencing rows when referenced row is deleted
//   - "RESTRICT": Prevent deletion of referenced row if referencing rows exist
//   - "SET NULL": Set foreign key columns to NULL when referenced row is deleted
//   - "SET DEFAULT": Set foreign key columns to their default values
//   - "NO ACTION": No action (database-specific, often same as RESTRICT)
//
// Examples:
//
//	// Delete posts when user is deleted
//	fk := schema.Table("posts").Column("user_id", "INTEGER").
//		ForeignKey("users", "id", "fk_posts_user").OnDelete("CASCADE")
//
//	// Prevent user deletion if posts exist
//	fk := schema.Table("posts").Column("user_id", "INTEGER").
//		ForeignKey("users", "id", "fk_posts_user").OnDelete("RESTRICT")
//
//	// Set user_id to NULL when user is deleted
//	fk := schema.Table("posts").Column("user_id", "INTEGER").
//		ForeignKey("users", "id", "fk_posts_user").OnDelete("SET NULL")
func (sfkb *SchemaForeignKeyBuilder) OnDelete(action string) *SchemaForeignKeyBuilder {
	sfkb.ref.OnDelete = action
	return sfkb
}

// OnUpdate sets the ON UPDATE referential action and returns the SchemaForeignKeyBuilder for chaining.
//
// This configures what happens to referencing rows when a referenced row's primary key is updated.
// The action must be one of the standard SQL referential actions.
//
// Supported actions:
//   - "CASCADE": Update foreign key values when referenced primary key changes
//   - "RESTRICT": Prevent update of referenced primary key if referencing rows exist
//   - "SET NULL": Set foreign key columns to NULL when referenced primary key changes
//   - "SET DEFAULT": Set foreign key columns to their default values
//   - "NO ACTION": No action (database-specific, often same as RESTRICT)
//
// Examples:
//
//	// Update foreign key when primary key changes
//	fk := schema.Table("posts").Column("user_id", "INTEGER").
//		ForeignKey("users", "id", "fk_posts_user").OnUpdate("CASCADE")
//
//	// Prevent primary key updates if foreign keys exist
//	fk := schema.Table("posts").Column("user_id", "INTEGER").
//		ForeignKey("users", "id", "fk_posts_user").OnUpdate("RESTRICT")
//
//	// Set foreign key to NULL when primary key changes
//	fk := schema.Table("posts").Column("user_id", "INTEGER").
//		ForeignKey("users", "id", "fk_posts_user").OnUpdate("SET NULL")
func (sfkb *SchemaForeignKeyBuilder) OnUpdate(action string) *SchemaForeignKeyBuilder {
	sfkb.ref.OnUpdate = action
	return sfkb
}

// End completes the foreign key configuration and returns to the parent SchemaTableBuilder for chaining.
//
// This method is used to return to the schema table context after configuring all foreign key
// properties, allowing continued table construction with additional columns or constraints
// within the schema definition.
//
// Example:
//
//	schema := astbuilder.NewSchema().
//		Table("posts").
//			Column("user_id", "INTEGER").NotNull().
//				ForeignKey("users", "id", "fk_posts_user").
//				OnDelete("CASCADE").
//				OnUpdate("RESTRICT").
//				End().
//			Column("title", "VARCHAR(255)").NotNull().End().
//		End()
func (sfkb *SchemaForeignKeyBuilder) End() *SchemaTableBuilder {
	return sfkb.schemaTable
}
