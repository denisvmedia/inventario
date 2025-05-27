package ast

import "fmt"

// Node represents any SQL AST node
type Node interface {
	// Accept implements the visitor pattern for rendering
	Accept(visitor Visitor) error
}

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

// CreateTableNode represents a CREATE TABLE statement
type CreateTableNode struct {
	Name        string
	Columns     []*ColumnNode
	Constraints []*ConstraintNode
	Options     map[string]string // For dialect-specific options like ENGINE
	Comment     string
}

func (n *CreateTableNode) Accept(visitor Visitor) error {
	return visitor.VisitCreateTable(n)
}

// AlterTableNode represents ALTER TABLE statements
type AlterTableNode struct {
	Name        string
	Operations  []AlterOperation
}

func (n *AlterTableNode) Accept(visitor Visitor) error {
	return visitor.VisitAlterTable(n)
}

// AlterOperation represents different types of ALTER operations
type AlterOperation interface {
	Node
	alterOperation() // marker method
}

// AddColumnOperation represents ADD COLUMN
type AddColumnOperation struct {
	Column *ColumnNode
}

func (op *AddColumnOperation) Accept(visitor Visitor) error {
	return op.Column.Accept(visitor)
}

func (op *AddColumnOperation) alterOperation() {}

// DropColumnOperation represents DROP COLUMN
type DropColumnOperation struct {
	ColumnName string
}

func (op *DropColumnOperation) Accept(visitor Visitor) error {
	// This would be handled by the visitor's VisitAlterTable method
	return nil
}

func (op *DropColumnOperation) alterOperation() {}

// ModifyColumnOperation represents ALTER COLUMN/MODIFY COLUMN
type ModifyColumnOperation struct {
	Column *ColumnNode
}

func (op *ModifyColumnOperation) Accept(visitor Visitor) error {
	return op.Column.Accept(visitor)
}

func (op *ModifyColumnOperation) alterOperation() {}

// ColumnNode represents a table column definition
type ColumnNode struct {
	Name         string
	Type         string
	Nullable     bool
	Primary      bool
	Unique       bool
	AutoInc      bool
	Default      *DefaultValue
	Check        string
	Comment      string
	ForeignKey   *ForeignKeyRef
}

func (n *ColumnNode) Accept(visitor Visitor) error {
	return visitor.VisitColumn(n)
}

// DefaultValue represents different types of default values
type DefaultValue struct {
	Value    string // For literal values like 'default_value'
	Function string // For function calls like NOW(), CURRENT_TIMESTAMP
}

// ForeignKeyRef represents a foreign key reference
type ForeignKeyRef struct {
	Table      string
	Column     string
	OnDelete   string
	OnUpdate   string
	Name       string // Constraint name
}

// ConstraintNode represents table-level constraints
type ConstraintNode struct {
	Type       ConstraintType
	Name       string
	Columns    []string
	Reference  *ForeignKeyRef // For foreign key constraints
	Expression string         // For check constraints
}

func (n *ConstraintNode) Accept(visitor Visitor) error {
	return visitor.VisitConstraint(n)
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

// IndexNode represents a CREATE INDEX statement
type IndexNode struct {
	Name     string
	Table    string
	Columns  []string
	Unique   bool
	Type     string // For index type like BTREE, HASH, etc.
	Comment  string
}

func (n *IndexNode) Accept(visitor Visitor) error {
	return visitor.VisitIndex(n)
}

// EnumNode represents a CREATE TYPE ... AS ENUM statement (PostgreSQL)
type EnumNode struct {
	Name   string
	Values []string
}

func (n *EnumNode) Accept(visitor Visitor) error {
	return visitor.VisitEnum(n)
}

// CommentNode represents SQL comments
type CommentNode struct {
	Text string
}

func (n *CommentNode) Accept(visitor Visitor) error {
	return visitor.VisitComment(n)
}

// StatementList represents a list of SQL statements
type StatementList struct {
	Statements []Node
}

func (sl *StatementList) Accept(visitor Visitor) error {
	for _, stmt := range sl.Statements {
		if err := stmt.Accept(visitor); err != nil {
			return fmt.Errorf("error visiting statement: %w", err)
		}
	}
	return nil
}

// Add convenience methods for building common structures

// NewCreateTable creates a new CREATE TABLE node
func NewCreateTable(name string) *CreateTableNode {
	return &CreateTableNode{
		Name:        name,
		Columns:     make([]*ColumnNode, 0),
		Constraints: make([]*ConstraintNode, 0),
		Options:     make(map[string]string),
	}
}

// AddColumn adds a column to the CREATE TABLE statement
func (ct *CreateTableNode) AddColumn(column *ColumnNode) *CreateTableNode {
	ct.Columns = append(ct.Columns, column)
	return ct
}

// AddConstraint adds a constraint to the CREATE TABLE statement
func (ct *CreateTableNode) AddConstraint(constraint *ConstraintNode) *CreateTableNode {
	ct.Constraints = append(ct.Constraints, constraint)
	return ct
}

// SetOption sets a table option (like ENGINE for MySQL)
func (ct *CreateTableNode) SetOption(key, value string) *CreateTableNode {
	ct.Options[key] = value
	return ct
}

// NewColumn creates a new column node
func NewColumn(name, dataType string) *ColumnNode {
	return &ColumnNode{
		Name:     name,
		Type:     dataType,
		Nullable: true, // Default to nullable
	}
}

// SetPrimary marks the column as primary key
func (c *ColumnNode) SetPrimary() *ColumnNode {
	c.Primary = true
	c.Nullable = false // Primary keys are always NOT NULL
	return c
}

// SetNotNull marks the column as NOT NULL
func (c *ColumnNode) SetNotNull() *ColumnNode {
	c.Nullable = false
	return c
}

// SetUnique marks the column as UNIQUE
func (c *ColumnNode) SetUnique() *ColumnNode {
	c.Unique = true
	return c
}

// SetAutoIncrement marks the column as auto-incrementing
func (c *ColumnNode) SetAutoIncrement() *ColumnNode {
	c.AutoInc = true
	return c
}

// SetDefault sets a literal default value
func (c *ColumnNode) SetDefault(value string) *ColumnNode {
	c.Default = &DefaultValue{Value: value}
	return c
}

// SetDefaultFunction sets a function as default value
func (c *ColumnNode) SetDefaultFunction(fn string) *ColumnNode {
	c.Default = &DefaultValue{Function: fn}
	return c
}

// SetCheck sets a check constraint
func (c *ColumnNode) SetCheck(expression string) *ColumnNode {
	c.Check = expression
	return c
}

// SetComment sets a column comment
func (c *ColumnNode) SetComment(comment string) *ColumnNode {
	c.Comment = comment
	return c
}

// SetForeignKey sets a foreign key reference
func (c *ColumnNode) SetForeignKey(table, column, name string) *ColumnNode {
	c.ForeignKey = &ForeignKeyRef{
		Table:  table,
		Column: column,
		Name:   name,
	}
	return c
}

// NewPrimaryKeyConstraint creates a primary key constraint
func NewPrimaryKeyConstraint(columns ...string) *ConstraintNode {
	return &ConstraintNode{
		Type:    PrimaryKeyConstraint,
		Columns: columns,
	}
}

// NewUniqueConstraint creates a unique constraint
func NewUniqueConstraint(name string, columns ...string) *ConstraintNode {
	return &ConstraintNode{
		Type:    UniqueConstraint,
		Name:    name,
		Columns: columns,
	}
}

// NewForeignKeyConstraint creates a foreign key constraint
func NewForeignKeyConstraint(name string, columns []string, ref *ForeignKeyRef) *ConstraintNode {
	return &ConstraintNode{
		Type:      ForeignKeyConstraint,
		Name:      name,
		Columns:   columns,
		Reference: ref,
	}
}

// NewIndex creates a new index node
func NewIndex(name, table string, columns ...string) *IndexNode {
	return &IndexNode{
		Name:    name,
		Table:   table,
		Columns: columns,
	}
}

// SetUnique marks the index as unique
func (i *IndexNode) SetUnique() *IndexNode {
	i.Unique = true
	return i
}

// NewEnum creates a new enum node
func NewEnum(name string, values ...string) *EnumNode {
	return &EnumNode{
		Name:   name,
		Values: values,
	}
}

// NewComment creates a new comment node
func NewComment(text string) *CommentNode {
	return &CommentNode{Text: text}
}
