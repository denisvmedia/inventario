package ast

// Visitor defines the interface for visiting AST nodes
type Visitor interface {
	VisitCreateTable(*CreateTableNode) error
	VisitAlterTable(*AlterTableNode) error
	VisitColumn(*ColumnNode) error
	VisitConstraint(*ConstraintNode) error
	VisitIndex(*IndexNode) error
	VisitEnum(*EnumNode) error
	VisitComment(*CommentNode) error
}

// DefaultValue represents different types of default values
type DefaultValue struct {
	Value    string // For literal values like 'default_value'
	Function string // For function calls like NOW(), CURRENT_TIMESTAMP
}

// ForeignKeyRef represents a foreign key reference
type ForeignKeyRef struct {
	Table    string
	Column   string
	OnDelete string
	OnUpdate string
	Name     string // Constraint name
}

// ConstraintType represents different types of constraints
type ConstraintType int

const (
	PrimaryKeyConstraint ConstraintType = iota
	UniqueConstraint
	ForeignKeyConstraint
	CheckConstraint
)

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
