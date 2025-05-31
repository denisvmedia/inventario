package ast

// Visitor defines the interface for visiting AST nodes using the visitor pattern.
//
// The visitor pattern allows for dialect-specific rendering of SQL statements
// without modifying the AST node structures. Each visitor method corresponds
// to a specific node type and is responsible for generating the appropriate
// SQL representation for that node.
//
// Implementations of this interface should handle the rendering logic for
// their specific database dialect (PostgreSQL, MySQL, MariaDB, etc.).
type Visitor interface {
	// VisitCreateTable renders a CREATE TABLE statement
	VisitCreateTable(*CreateTableNode) error
	// VisitAlterTable renders an ALTER TABLE statement
	VisitAlterTable(*AlterTableNode) error
	// VisitColumn renders a column definition (typically called from other visitors)
	VisitColumn(*ColumnNode) error
	// VisitConstraint renders a constraint definition (typically called from other visitors)
	VisitConstraint(*ConstraintNode) error
	// VisitIndex renders a CREATE INDEX statement
	VisitIndex(*IndexNode) error
	// VisitEnum renders an enum type definition (PostgreSQL-specific)
	VisitEnum(*EnumNode) error
	// VisitComment renders a SQL comment
	VisitComment(*CommentNode) error
}

// DefaultValue represents different types of default values for table columns.
//
// A default value can be either a literal value (like 'active', 42, true) or
// a function call (like NOW(), CURRENT_TIMESTAMP, UUID()). Only one of Value
// or Function should be set.
type DefaultValue struct {
	// Value contains literal default values like 'default_value', '42', 'true'
	Value string
	// Function contains function calls like NOW(), UUID()
	Expression string
}

// ForeignKeyRef represents a foreign key reference with optional referential actions.
//
// This structure defines the target table and column for a foreign key constraint,
// along with optional ON DELETE and ON UPDATE actions that specify what should
// happen when the referenced row is deleted or updated.
type ForeignKeyRef struct {
	// Table is the name of the referenced table
	Table string
	// Column is the name of the referenced column
	Column string
	// OnDelete specifies the action when the referenced row is deleted (CASCADE, SET NULL, etc.)
	OnDelete string
	// OnUpdate specifies the action when the referenced row is updated (CASCADE, SET NULL, etc.)
	OnUpdate string
	// Name is the constraint name for the foreign key
	Name string
}

// ConstraintType represents the different types of table constraints.
//
// This enumeration covers the standard SQL constraint types that can be
// applied at the table level, including primary keys, unique constraints,
// foreign keys, and check constraints.
type ConstraintType int

const (
	// PrimaryKeyConstraint represents a PRIMARY KEY constraint
	PrimaryKeyConstraint ConstraintType = iota
	// UniqueConstraint represents a UNIQUE constraint
	UniqueConstraint
	// ForeignKeyConstraint represents a FOREIGN KEY constraint
	ForeignKeyConstraint
	// CheckConstraint represents a CHECK constraint
	CheckConstraint
)

// String returns the SQL representation of the constraint type.
//
// This method converts the ConstraintType enumeration value to its
// corresponding SQL keyword that would appear in DDL statements.
func (ct ConstraintType) String() string {
	switch ct {
	case PrimaryKeyConstraint:
		return "PRIMARY KEY"
	case UniqueConstraint:
		return "UNIQUE"
	case ForeignKeyConstraint:
		return "FOREIGN KEY"
	case CheckConstraint:
		return "CHECK"
	default:
		return "UNKNOWN"
	}
}
