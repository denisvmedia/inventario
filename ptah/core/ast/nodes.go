package ast

import (
	"fmt"
)

// Node represents any SQL AST node that can be visited by a Visitor.
//
// All AST nodes implement this interface to participate in the visitor pattern.
// The Accept method allows visitors to traverse the AST and generate
// dialect-specific SQL output.
type Node interface {
	// Accept implements the visitor pattern for rendering
	Accept(visitor Visitor) error
}

// AlterTableNode represents ALTER TABLE statements with one or more operations.
//
// This node can contain multiple operations like adding columns, dropping columns,
// or modifying existing columns. Each operation is represented by a specific
// AlterOperation implementation.
type AlterTableNode struct {
	// Name is the name of the table to alter
	Name string
	// Operations contains the list of operations to perform on the table
	Operations []AlterOperation
}

// Accept implements the Node interface for AlterTableNode.
func (n *AlterTableNode) Accept(visitor Visitor) error {
	return visitor.VisitAlterTable(n)
}

// EnumNode represents a CREATE TYPE ... AS ENUM statement (PostgreSQL-specific).
//
// Enums are primarily supported by PostgreSQL. Other databases may handle
// enum-like functionality differently (e.g., CHECK constraints with IN clauses).
type EnumNode struct {
	// Name is the name of the enum type
	Name string
	// Values contains the list of allowed enum values
	Values []string
}

// NewEnum creates a new enum node with the specified name and values.
//
// Example:
//
//	enum := NewEnum("status", "active", "inactive", "pending")
func NewEnum(name string, values ...string) *EnumNode {
	return &EnumNode{
		Name:   name,
		Values: values,
	}
}

// Accept implements the Node interface for EnumNode.
func (n *EnumNode) Accept(visitor Visitor) error {
	return visitor.VisitEnum(n)
}

// CreateTableNode represents a CREATE TABLE statement with all its components.
//
// This node contains the complete definition of a table including columns,
// constraints, dialect-specific options, and optional comments. It supports
// a fluent API for easy construction.
type CreateTableNode struct {
	// Name is the name of the table to create
	Name string
	// Columns contains all column definitions for the table
	Columns []*ColumnNode
	// Constraints contains table-level constraints (PRIMARY KEY, UNIQUE, FOREIGN KEY, CHECK)
	Constraints []*ConstraintNode
	// Options contains dialect-specific table options like ENGINE for MySQL
	Options map[string]string
	// Comment is an optional table comment
	Comment string
}

// NewCreateTable creates a new CREATE TABLE node with the specified table name.
//
// The returned node has empty slices for columns and constraints, and an empty
// options map. Use the fluent API methods to add columns, constraints, and options.
//
// Example:
//
//	table := NewCreateTable("users")
func NewCreateTable(name string) *CreateTableNode {
	return &CreateTableNode{
		Name:        name,
		Columns:     make([]*ColumnNode, 0),
		Constraints: make([]*ConstraintNode, 0),
		Options:     make(map[string]string),
	}
}

// Accept implements the Node interface for CreateTableNode.
func (n *CreateTableNode) Accept(visitor Visitor) error {
	return visitor.VisitCreateTable(n)
}

// AddColumn adds a column to the CREATE TABLE statement and returns the table node for chaining.
//
// Example:
//
//	table.AddColumn(NewColumn("id", "INTEGER").SetPrimary())
func (n *CreateTableNode) AddColumn(column *ColumnNode) *CreateTableNode {
	n.Columns = append(n.Columns, column)
	return n
}

// AddConstraint adds a table-level constraint and returns the table node for chaining.
//
// Example:
//
//	table.AddConstraint(NewUniqueConstraint("uk_email", "email"))
func (n *CreateTableNode) AddConstraint(constraint *ConstraintNode) *CreateTableNode {
	n.Constraints = append(n.Constraints, constraint)
	return n
}

// SetOption sets a dialect-specific table option and returns the table node for chaining.
//
// Common options include:
//   - MySQL/MariaDB: ENGINE, CHARSET, COLLATE
//   - PostgreSQL: TABLESPACE, WITH
//
// Example:
//
//	table.SetOption("ENGINE", "InnoDB")
func (n *CreateTableNode) SetOption(key, value string) *CreateTableNode {
	n.Options[key] = value
	return n
}

// ColumnNode represents a table column definition with all its attributes.
//
// This node contains the complete specification of a table column including
// its data type, constraints, default values, and other properties. It supports
// a fluent API for easy configuration.
type ColumnNode struct {
	// Name is the column name
	Name string
	// Type is the column data type (e.g., "INTEGER", "VARCHAR(255)", "TIMESTAMP")
	Type string
	// Nullable indicates whether the column allows NULL values (default: true)
	Nullable bool
	// Primary indicates whether this column is part of the primary key
	Primary bool
	// Unique indicates whether this column has a unique constraint
	Unique bool
	// AutoInc indicates whether this column is auto-incrementing
	AutoInc bool
	// Default contains the default value specification (literal or function)
	Default *DefaultValue
	// Check contains a check constraint expression for this column
	Check string
	// Comment is an optional column comment
	Comment string
	// ForeignKey contains foreign key reference information if this column references another table
	ForeignKey *ForeignKeyRef
}

// NewColumn creates a new column node with the specified name and data type.
//
// The column is created with nullable=true by default. Use the fluent API
// methods to configure other properties.
//
// Example:
//
//	column := NewColumn("email", "VARCHAR(255)")
func NewColumn(name, dataType string) *ColumnNode {
	return &ColumnNode{
		Name:     name,
		Type:     dataType,
		Nullable: true, // Default to nullable
	}
}

// Accept implements the Node interface for ColumnNode.
func (n *ColumnNode) Accept(visitor Visitor) error {
	return visitor.VisitColumn(n)
}

// SetPrimary marks the column as a primary key and returns the column for chaining.
//
// Setting a column as primary automatically makes it NOT NULL, as primary keys
// cannot contain NULL values in SQL.
//
// Example:
//
//	column.SetPrimary()
func (n *ColumnNode) SetPrimary() *ColumnNode {
	n.Primary = true
	n.Nullable = false // Primary keys are always NOT NULL
	return n
}

// SetNotNull marks the column as NOT NULL and returns the column for chaining.
//
// Example:
//
//	column.SetNotNull()
func (n *ColumnNode) SetNotNull() *ColumnNode {
	n.Nullable = false
	return n
}

// SetUnique marks the column as UNIQUE and returns the column for chaining.
//
// This creates a column-level unique constraint. For multi-column unique
// constraints, use table-level constraints instead.
//
// Example:
//
//	column.SetUnique()
func (n *ColumnNode) SetUnique() *ColumnNode {
	n.Unique = true
	return n
}

// SetAutoIncrement marks the column as auto-incrementing and returns the column for chaining.
//
// Auto-increment behavior varies by database:
//   - MySQL/MariaDB: AUTO_INCREMENT
//   - PostgreSQL: SERIAL or IDENTITY
//   - SQLite: AUTOINCREMENT
//
// Example:
//
//	column.SetAutoIncrement()
func (n *ColumnNode) SetAutoIncrement() *ColumnNode {
	n.AutoInc = true
	return n
}

// SetDefault sets a literal default value and returns the column for chaining.
//
// The value should be properly quoted for string literals (e.g., "'active'").
// For function calls, use SetDefaultExpression instead.
//
// Example:
//
//	column.SetDefault("'active'")
//	column.SetDefault("0")
func (n *ColumnNode) SetDefault(value string) *ColumnNode {
	n.Default = &DefaultValue{Value: value}
	return n
}

// SetDefaultExpression sets a function as the default value and returns the column for chaining.
//
// Common functions include NOW(), CURRENT_TIMESTAMP, UUID(), etc.
//
// Example:
//
//	column.SetDefaultExpression("NOW()")
//	column.SetDefaultExpression("UUID()")
func (n *ColumnNode) SetDefaultExpression(fn string) *ColumnNode {
	n.Default = &DefaultValue{Expression: fn}
	return n
}

// SetCheck sets a check constraint expression and returns the column for chaining.
//
// The expression should be a valid SQL boolean expression that references
// the column.
//
// Example:
//
//	column.SetCheck("status IN ('active', 'inactive')")
//	column.SetCheck("price > 0")
func (n *ColumnNode) SetCheck(expression string) *ColumnNode {
	n.Check = expression
	return n
}

// SetComment sets a column comment and returns the column for chaining.
//
// Example:
//
//	column.SetComment("User's email address")
func (n *ColumnNode) SetComment(comment string) *ColumnNode {
	n.Comment = comment
	return n
}

// SetForeignKey sets a foreign key reference and returns the column for chaining.
//
// This creates a column-level foreign key constraint. The name parameter
// is the constraint name.
//
// Example:
//
//	column.SetForeignKey("users", "id", "fk_orders_user")
func (n *ColumnNode) SetForeignKey(table, column, name string) *ColumnNode {
	n.ForeignKey = &ForeignKeyRef{
		Table:  table,
		Column: column,
		Name:   name,
	}
	return n
}

// ConstraintNode represents table-level constraints (PRIMARY KEY, UNIQUE, FOREIGN KEY, CHECK).
//
// Table-level constraints can span multiple columns and are defined separately
// from column definitions. This is different from column-level constraints
// which are defined as part of the column specification.
type ConstraintNode struct {
	// Type specifies the constraint type (PRIMARY KEY, UNIQUE, etc.)
	Type ConstraintType
	// Name is the constraint name (optional for some constraint types)
	Name string
	// Columns contains the list of column names involved in the constraint
	Columns []string
	// Reference contains foreign key reference information (only for FOREIGN KEY constraints)
	Reference *ForeignKeyRef
	// Expression contains the check expression (only for CHECK constraints)
	Expression string
}

// Accept implements the Node interface for ConstraintNode.
func (n *ConstraintNode) Accept(visitor Visitor) error {
	return visitor.VisitConstraint(n)
}

// IndexNode represents a CREATE INDEX statement.
//
// Indexes can be unique or non-unique and may specify an index type
// depending on the database system capabilities.
type IndexNode struct {
	// Name is the index name
	Name string
	// Table is the name of the table to index
	Table string
	// Columns contains the list of column names to include in the index
	Columns []string
	// Unique indicates whether this is a unique index
	Unique bool
	// Type specifies the index type (BTREE, HASH, etc.) - database-specific
	Type string
	// Comment is an optional index comment
	Comment string
}

// NewIndex creates a new index node with the specified name, table, and columns.
//
// Example:
//
//	index := NewIndex("idx_user_email", "users", "email")
//	index := NewIndex("idx_user_name_status", "users", "name", "status")
func NewIndex(name, table string, columns ...string) *IndexNode {
	return &IndexNode{
		Name:    name,
		Table:   table,
		Columns: columns,
	}
}

// Accept implements the Node interface for IndexNode.
func (n *IndexNode) Accept(visitor Visitor) error {
	return visitor.VisitIndex(n)
}

// SetUnique marks the index as unique and returns the index for chaining.
//
// Unique indexes enforce uniqueness constraints on the indexed columns.
//
// Example:
//
//	index.SetUnique()
func (n *IndexNode) SetUnique() *IndexNode {
	n.Unique = true
	return n
}

// CommentNode represents SQL comments that can be included in generated scripts.
//
// Comments are useful for documenting generated SQL and providing context
// about the schema structure.
type CommentNode struct {
	// Text is the comment content
	Text string
}

// NewComment creates a new comment node with the specified text.
//
// Example:
//
//	comment := NewComment("User management tables")
func NewComment(text string) *CommentNode {
	return &CommentNode{Text: text}
}

// Accept implements the Node interface for CommentNode.
func (n *CommentNode) Accept(visitor Visitor) error {
	return visitor.VisitComment(n)
}

// DropTableNode represents a DROP TABLE statement.
//
// This node supports various DROP TABLE options including IF EXISTS,
// CASCADE/RESTRICT, and dialect-specific features.
type DropTableNode struct {
	// Name is the name of the table to drop
	Name string
	// IfExists indicates whether to use IF EXISTS clause
	IfExists bool
	// Cascade indicates whether to use CASCADE option (PostgreSQL)
	Cascade bool
	// Comment is an optional comment for the drop operation
	Comment string
}

// NewDropTable creates a new DROP TABLE node with the specified table name.
//
// The node is created with IfExists=false and Cascade=false by default.
// Use the fluent API methods to configure these options.
//
// Example:
//
//	dropTable := NewDropTable("users").SetIfExists().SetCascade()
func NewDropTable(name string) *DropTableNode {
	return &DropTableNode{
		Name:     name,
		IfExists: false,
		Cascade:  false,
	}
}

// SetIfExists sets the IF EXISTS option for the DROP TABLE statement.
//
// This makes the statement safe to execute even if the table doesn't exist.
func (n *DropTableNode) SetIfExists() *DropTableNode {
	n.IfExists = true
	return n
}

// SetCascade sets the CASCADE option for the DROP TABLE statement.
//
// This is primarily used in PostgreSQL to automatically drop dependent objects.
func (n *DropTableNode) SetCascade() *DropTableNode {
	n.Cascade = true
	return n
}

// SetComment sets a comment for the DROP TABLE operation.
//
// This comment can be used for documentation or warnings.
func (n *DropTableNode) SetComment(comment string) *DropTableNode {
	n.Comment = comment
	return n
}

// Accept implements the Node interface for DropTableNode.
func (n *DropTableNode) Accept(visitor Visitor) error {
	return visitor.VisitDropTable(n)
}

// DropTypeNode represents a DROP TYPE statement (PostgreSQL-specific).
//
// This node is used to drop custom types, particularly enum types in PostgreSQL.
// Other databases may not support this operation or handle it differently.
type DropTypeNode struct {
	// Name is the name of the type to drop
	Name string
	// IfExists indicates whether to use IF EXISTS clause
	IfExists bool
	// Cascade indicates whether to use CASCADE option
	Cascade bool
	// Comment is an optional comment for the drop operation
	Comment string
}

// NewDropType creates a new DROP TYPE node with the specified type name.
//
// The node is created with IfExists=false and Cascade=false by default.
// Use the fluent API methods to configure these options.
//
// Example:
//
//	dropType := NewDropType("status_enum").SetIfExists().SetCascade()
func NewDropType(name string) *DropTypeNode {
	return &DropTypeNode{
		Name:     name,
		IfExists: false,
		Cascade:  false,
	}
}

// SetIfExists sets the IF EXISTS option for the DROP TYPE statement.
//
// This makes the statement safe to execute even if the type doesn't exist.
func (n *DropTypeNode) SetIfExists() *DropTypeNode {
	n.IfExists = true
	return n
}

// SetCascade sets the CASCADE option for the DROP TYPE statement.
//
// This automatically drops dependent objects that use this type.
func (n *DropTypeNode) SetCascade() *DropTypeNode {
	n.Cascade = true
	return n
}

// SetComment sets a comment for the DROP TYPE operation.
//
// This comment can be used for documentation or warnings.
func (n *DropTypeNode) SetComment(comment string) *DropTypeNode {
	n.Comment = comment
	return n
}

// Accept implements the Node interface for DropTypeNode.
func (n *DropTypeNode) Accept(visitor Visitor) error {
	return visitor.VisitDropType(n)
}

// StatementList represents a collection of SQL statements that should be executed together.
//
// This is typically used to represent a complete schema or migration script
// that contains multiple DDL statements. The visitor will process each
// statement in order.
type StatementList struct {
	// Statements contains the ordered list of SQL statements
	Statements []Node
}

// Accept implements the Node interface for StatementList.
//
// This method visits each statement in the list in order. If any statement
// fails to be visited, the process stops and returns the error.
func (sl *StatementList) Accept(visitor Visitor) error {
	for _, stmt := range sl.Statements {
		if err := stmt.Accept(visitor); err != nil {
			return fmt.Errorf("error visiting statement: %w", err)
		}
	}
	return nil
}
