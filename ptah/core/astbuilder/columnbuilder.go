package astbuilder

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// ColumnBuilder provides a fluent API for building column definitions within table contexts.
//
// ColumnBuilder wraps an AST ColumnNode and provides a convenient fluent interface
// for configuring column properties such as constraints, default values, and foreign
// key references. It maintains a reference to its parent TableBuilder to enable
// method chaining that returns to the table context.
//
// The builder supports all standard SQL column attributes:
//   - Primary key designation
//   - NULL/NOT NULL constraints
//   - Unique constraints
//   - Auto-increment behavior
//   - Default values (literal and expressions)
//   - Check constraints
//   - Comments
//   - Foreign key references
//
// Example usage:
//
//	table := astbuilder.NewTable("users").
//		Column("id", "SERIAL").Primary().AutoIncrement().Comment("Primary key").End().
//		Column("email", "VARCHAR(255)").NotNull().Unique().End().
//		Column("status", "VARCHAR(20)").NotNull().Default("'active'").
//			Check("status IN ('active', 'inactive')").End().
//		Column("user_id", "INTEGER").NotNull().
//			ForeignKey("users", "id", "fk_posts_user").
//			OnDelete("CASCADE").End()
//
// All methods return the ColumnBuilder instance for chaining, except:
//   - ForeignKey() returns a ForeignKeyBuilder for configuring referential actions
//   - End() returns the parent TableBuilder to continue table construction
type ColumnBuilder struct {
	// column is the underlying AST node being configured
	column *ast.ColumnNode
	// table is the parent table builder for returning context
	table *TableBuilder
}

// Primary marks the column as a primary key and returns the ColumnBuilder for chaining.
//
// Setting a column as primary key automatically makes it NOT NULL, as primary keys
// cannot contain NULL values in SQL. This follows standard SQL semantics where
// primary key columns are implicitly NOT NULL.
//
// Example:
//
//	column := table.Column("id", "SERIAL").Primary()
//
// Note: For composite primary keys spanning multiple columns, use the table-level
// PrimaryKey() method instead.
func (cb *ColumnBuilder) Primary() *ColumnBuilder {
	cb.column.SetPrimary()
	return cb
}

// NotNull marks the column as NOT NULL and returns the ColumnBuilder for chaining.
//
// This explicitly prevents the column from accepting NULL values. Columns are
// nullable by default unless marked otherwise.
//
// Example:
//
//	column := table.Column("email", "VARCHAR(255)").NotNull()
func (cb *ColumnBuilder) NotNull() *ColumnBuilder {
	cb.column.SetNotNull()
	return cb
}

// Nullable explicitly marks the column as nullable and returns the ColumnBuilder for chaining.
//
// This is the default behavior for columns, so calling this method is typically
// only needed to override a previous constraint or for explicit documentation.
// Nullable columns can store NULL values.
//
// Example:
//
//	column := table.Column("description", "TEXT").Nullable()
func (cb *ColumnBuilder) Nullable() *ColumnBuilder {
	cb.column.Nullable = true
	return cb
}

// Unique marks the column as UNIQUE and returns the ColumnBuilder for chaining.
//
// This creates a column-level unique constraint, ensuring that all values in
// this column are distinct across all rows in the table. For multi-column
// unique constraints, use the table-level Unique() method instead.
//
// Example:
//
//	column := table.Column("email", "VARCHAR(255)").Unique()
func (cb *ColumnBuilder) Unique() *ColumnBuilder {
	cb.column.SetUnique()
	return cb
}

// AutoIncrement marks the column as auto-incrementing and returns the ColumnBuilder for chaining.
//
// Auto-increment behavior varies by database system:
//   - MySQL/MariaDB: AUTO_INCREMENT keyword
//   - PostgreSQL: SERIAL type or IDENTITY columns
//   - SQLite: AUTOINCREMENT keyword
//
// Auto-increment columns automatically generate unique sequential values when
// new rows are inserted without specifying a value for this column.
//
// Example:
//
//	column := table.Column("id", "INTEGER").AutoIncrement()
func (cb *ColumnBuilder) AutoIncrement() *ColumnBuilder {
	cb.column.SetAutoIncrement()
	return cb
}

// Default sets a literal default value and returns the ColumnBuilder for chaining.
//
// The value should be properly quoted for string literals (e.g., "'active'", "'2023-01-01'").
// For numeric values, quotes are not needed (e.g., "0", "42.5").
// For function calls or expressions, use DefaultExpression() instead.
//
// Examples:
//
//	column := table.Column("status", "VARCHAR(20)").Default("'active'")
//	column := table.Column("count", "INTEGER").Default("0")
//	column := table.Column("rate", "DECIMAL(5,2)").Default("1.00")
func (cb *ColumnBuilder) Default(value string) *ColumnBuilder {
	cb.column.SetDefault(value)
	return cb
}

// DefaultExpression sets a function or expression as the default value and returns the ColumnBuilder for chaining.
//
// This is used for database functions, expressions, or other non-literal default values.
// The expression will be used as-is in the generated SQL without additional quoting.
//
// Common examples:
//   - NOW(), CURRENT_TIMESTAMP: Current date/time
//   - UUID(), GEN_RANDOM_UUID(): Generate unique identifiers
//   - CURRENT_USER: Current database user
//   - Mathematical expressions: (price * 0.1)
//
// Examples:
//
//	column := table.Column("created_at", "TIMESTAMP").DefaultExpression("NOW()")
//	column := table.Column("id", "UUID").DefaultExpression("GEN_RANDOM_UUID()")
//	column := table.Column("updated_at", "TIMESTAMP").DefaultExpression("CURRENT_TIMESTAMP")
func (cb *ColumnBuilder) DefaultExpression(fn string) *ColumnBuilder {
	cb.column.SetDefaultExpression(fn)
	return cb
}

// Check sets a check constraint expression and returns the ColumnBuilder for chaining.
//
// Check constraints enforce domain integrity by limiting the values that can be
// stored in a column. The expression should be a valid SQL boolean expression
// that references the column name and evaluates to true for valid values.
//
// Examples:
//
//	column := table.Column("age", "INTEGER").Check("age >= 0 AND age <= 150")
//	column := table.Column("status", "VARCHAR(20)").Check("status IN ('active', 'inactive', 'pending')")
//	column := table.Column("price", "DECIMAL(10,2)").Check("price > 0")
//	column := table.Column("email", "VARCHAR(255)").Check("email LIKE '%@%'")
func (cb *ColumnBuilder) Check(expression string) *ColumnBuilder {
	cb.column.SetCheck(expression)
	return cb
}

// Comment sets a descriptive comment for the column and returns the ColumnBuilder for chaining.
//
// Comments are useful for documenting the purpose, format, or constraints of a column.
// Comment support varies by database system, but most modern databases support them.
//
// Examples:
//
//	column := table.Column("id", "SERIAL").Comment("Auto-incrementing primary key")
//	column := table.Column("email", "VARCHAR(255)").Comment("User's email address for login")
//	column := table.Column("created_at", "TIMESTAMP").Comment("Record creation timestamp")
func (cb *ColumnBuilder) Comment(comment string) *ColumnBuilder {
	cb.column.SetComment(comment)
	return cb
}

// ForeignKey sets a foreign key reference and returns a ForeignKeyBuilder for configuring referential actions.
//
// This creates a column-level foreign key constraint that references another table's column.
// The returned ForeignKeyBuilder allows configuration of referential actions (ON DELETE, ON UPDATE)
// before returning to the table context with End().
//
// Parameters:
//   - table: The referenced table name
//   - column: The referenced column name in the target table
//   - name: The constraint name for the foreign key
//
// Examples:
//
//	// Basic foreign key
//	column := table.Column("user_id", "INTEGER").
//		ForeignKey("users", "id", "fk_posts_user").End()
//
//	// Foreign key with referential actions
//	column := table.Column("category_id", "INTEGER").
//		ForeignKey("categories", "id", "fk_products_category").
//		OnDelete("CASCADE").
//		OnUpdate("RESTRICT").End()
func (cb *ColumnBuilder) ForeignKey(table, column, name string) *ForeignKeyBuilder {
	cb.column.SetForeignKey(table, column, name)

	ref := cb.column.ForeignKey
	return &ForeignKeyBuilder{
		ref:   ref,
		table: cb.table,
	}
}

// End completes the column definition and returns to the parent TableBuilder for chaining.
//
// This method is used to return to the table context after configuring all column
// properties, allowing continued table construction with additional columns or constraints.
//
// Example:
//
//	table := astbuilder.NewTable("users").
//		Column("id", "SERIAL").Primary().End().
//		Column("email", "VARCHAR(255)").NotNull().Unique().End().
//		Column("name", "VARCHAR(100)").NotNull().End()
func (cb *ColumnBuilder) End() *TableBuilder {
	return cb.table
}

// SchemaColumnBuilder provides a fluent API for building column definitions within schema contexts.
//
// SchemaColumnBuilder is similar to ColumnBuilder but operates within the context of
// a SchemaBuilder rather than a standalone TableBuilder. It wraps an AST ColumnNode
// and provides the same column configuration methods, but returns to a SchemaTableBuilder
// context when End() is called.
//
// This builder is used when constructing tables as part of a larger schema definition,
// allowing seamless navigation between schema, table, and column contexts.
//
// Example usage within a schema:
//
//	schema := astbuilder.NewSchema().
//		Table("users").
//			Column("id", "SERIAL").Primary().AutoIncrement().End().
//			Column("email", "VARCHAR(255)").NotNull().Unique().End().
//		End().
//		Table("posts").
//			Column("user_id", "INTEGER").NotNull().
//				ForeignKey("users", "id", "fk_posts_user").
//				OnDelete("CASCADE").End().
//		End()
//
// All methods return the SchemaColumnBuilder instance for chaining, except:
//   - ForeignKey() returns a SchemaForeignKeyBuilder for configuring referential actions
//   - End() returns the parent SchemaTableBuilder to continue table construction
type SchemaColumnBuilder struct {
	// column is the underlying AST node being configured
	column *ast.ColumnNode
	// schemaTable is the parent schema table builder for returning context
	schemaTable *SchemaTableBuilder
}

// Primary marks the column as a primary key and returns the SchemaColumnBuilder for chaining.
//
// Setting a column as primary key automatically makes it NOT NULL, as primary keys
// cannot contain NULL values in SQL. This follows standard SQL semantics where
// primary key columns are implicitly NOT NULL.
//
// Example:
//
//	column := schema.Table("users").Column("id", "SERIAL").Primary()
//
// Note: For composite primary keys spanning multiple columns, use the table-level
// PrimaryKey() method instead.
func (scb *SchemaColumnBuilder) Primary() *SchemaColumnBuilder {
	scb.column.SetPrimary()
	return scb
}

// NotNull marks the column as NOT NULL and returns the SchemaColumnBuilder for chaining.
//
// This explicitly prevents the column from accepting NULL values. Columns are
// nullable by default unless marked otherwise.
//
// Example:
//
//	column := schema.Table("users").Column("email", "VARCHAR(255)").NotNull()
func (scb *SchemaColumnBuilder) NotNull() *SchemaColumnBuilder {
	scb.column.SetNotNull()
	return scb
}

// Nullable explicitly marks the column as nullable and returns the SchemaColumnBuilder for chaining.
//
// This is the default behavior for columns, so calling this method is typically
// only needed to override a previous constraint or for explicit documentation.
// Nullable columns can store NULL values.
//
// Example:
//
//	column := schema.Table("users").Column("description", "TEXT").Nullable()
func (scb *SchemaColumnBuilder) Nullable() *SchemaColumnBuilder {
	scb.column.Nullable = true
	return scb
}

// Unique marks the column as UNIQUE and returns the SchemaColumnBuilder for chaining.
//
// This creates a column-level unique constraint, ensuring that all values in
// this column are distinct across all rows in the table. For multi-column
// unique constraints, use the table-level Unique() method instead.
//
// Example:
//
//	column := schema.Table("users").Column("email", "VARCHAR(255)").Unique()
func (scb *SchemaColumnBuilder) Unique() *SchemaColumnBuilder {
	scb.column.SetUnique()
	return scb
}

// AutoIncrement marks the column as auto-incrementing and returns the SchemaColumnBuilder for chaining.
//
// Auto-increment behavior varies by database system:
//   - MySQL/MariaDB: AUTO_INCREMENT keyword
//   - PostgreSQL: SERIAL type or IDENTITY columns
//   - SQLite: AUTOINCREMENT keyword
//
// Auto-increment columns automatically generate unique sequential values when
// new rows are inserted without specifying a value for this column.
//
// Example:
//
//	column := schema.Table("users").Column("id", "INTEGER").AutoIncrement()
func (scb *SchemaColumnBuilder) AutoIncrement() *SchemaColumnBuilder {
	scb.column.SetAutoIncrement()
	return scb
}

// Default sets a literal default value and returns the SchemaColumnBuilder for chaining.
//
// The value should be properly quoted for string literals (e.g., "'active'", "'2023-01-01'").
// For numeric values, quotes are not needed (e.g., "0", "42.5").
// For function calls or expressions, use DefaultExpression() instead.
//
// Examples:
//
//	column := schema.Table("users").Column("status", "VARCHAR(20)").Default("'active'")
//	column := schema.Table("users").Column("count", "INTEGER").Default("0")
//	column := schema.Table("users").Column("rate", "DECIMAL(5,2)").Default("1.00")
func (scb *SchemaColumnBuilder) Default(value string) *SchemaColumnBuilder {
	scb.column.SetDefault(value)
	return scb
}

// DefaultExpression sets a function or expression as the default value and returns the SchemaColumnBuilder for chaining.
//
// This is used for database functions, expressions, or other non-literal default values.
// The expression will be used as-is in the generated SQL without additional quoting.
//
// Common examples:
//   - NOW(), CURRENT_TIMESTAMP: Current date/time
//   - UUID(), GEN_RANDOM_UUID(): Generate unique identifiers
//   - CURRENT_USER: Current database user
//   - Mathematical expressions: (price * 0.1)
//
// Examples:
//
//	column := schema.Table("users").Column("created_at", "TIMESTAMP").DefaultExpression("NOW()")
//	column := schema.Table("users").Column("id", "UUID").DefaultExpression("GEN_RANDOM_UUID()")
//	column := schema.Table("users").Column("updated_at", "TIMESTAMP").DefaultExpression("CURRENT_TIMESTAMP")
func (scb *SchemaColumnBuilder) DefaultExpression(fn string) *SchemaColumnBuilder {
	scb.column.SetDefaultExpression(fn)
	return scb
}

// Check sets a check constraint expression and returns the SchemaColumnBuilder for chaining.
//
// Check constraints enforce domain integrity by limiting the values that can be
// stored in a column. The expression should be a valid SQL boolean expression
// that references the column name and evaluates to true for valid values.
//
// Examples:
//
//	column := schema.Table("users").Column("age", "INTEGER").Check("age >= 0 AND age <= 150")
//	column := schema.Table("users").Column("status", "VARCHAR(20)").Check("status IN ('active', 'inactive', 'pending')")
//	column := schema.Table("products").Column("price", "DECIMAL(10,2)").Check("price > 0")
//	column := schema.Table("users").Column("email", "VARCHAR(255)").Check("email LIKE '%@%'")
func (scb *SchemaColumnBuilder) Check(expression string) *SchemaColumnBuilder {
	scb.column.SetCheck(expression)
	return scb
}

// Comment sets a descriptive comment for the column and returns the SchemaColumnBuilder for chaining.
//
// Comments are useful for documenting the purpose, format, or constraints of a column.
// Comment support varies by database system, but most modern databases support them.
//
// Examples:
//
//	column := schema.Table("users").Column("id", "SERIAL").Comment("Auto-incrementing primary key")
//	column := schema.Table("users").Column("email", "VARCHAR(255)").Comment("User's email address for login")
//	column := schema.Table("users").Column("created_at", "TIMESTAMP").Comment("Record creation timestamp")
func (scb *SchemaColumnBuilder) Comment(comment string) *SchemaColumnBuilder {
	scb.column.SetComment(comment)
	return scb
}

// ForeignKey sets a foreign key reference and returns a SchemaForeignKeyBuilder for configuring referential actions.
//
// This creates a column-level foreign key constraint that references another table's column.
// The returned SchemaForeignKeyBuilder allows configuration of referential actions (ON DELETE, ON UPDATE)
// before returning to the schema table context with End().
//
// Parameters:
//   - table: The referenced table name
//   - column: The referenced column name in the target table
//   - name: The constraint name for the foreign key
//
// Examples:
//
//	// Basic foreign key
//	column := schema.Table("posts").Column("user_id", "INTEGER").
//		ForeignKey("users", "id", "fk_posts_user").End()
//
//	// Foreign key with referential actions
//	column := schema.Table("products").Column("category_id", "INTEGER").
//		ForeignKey("categories", "id", "fk_products_category").
//		OnDelete("CASCADE").
//		OnUpdate("RESTRICT").End()
func (scb *SchemaColumnBuilder) ForeignKey(table, column, name string) *SchemaForeignKeyBuilder {
	scb.column.SetForeignKey(table, column, name)

	ref := scb.column.ForeignKey
	return &SchemaForeignKeyBuilder{
		ref:         ref,
		schemaTable: scb.schemaTable,
	}
}

// End completes the column definition and returns to the parent SchemaTableBuilder for chaining.
//
// This method is used to return to the schema table context after configuring all column
// properties, allowing continued table construction with additional columns or constraints
// within the schema definition.
//
// Example:
//
//	schema := astbuilder.NewSchema().
//		Table("users").
//			Column("id", "SERIAL").Primary().End().
//			Column("email", "VARCHAR(255)").NotNull().Unique().End().
//			Column("name", "VARCHAR(100)").NotNull().End().
//		End()
func (scb *SchemaColumnBuilder) End() *SchemaTableBuilder {
	return scb.schemaTable
}
