package ast

import (
	"fmt"
)

// Node represents any SQL AST node
type Node interface {
	// Accept implements the visitor pattern for rendering
	Accept(visitor Visitor) error
}

// AlterTableNode represents ALTER TABLE statements
type AlterTableNode struct {
	Name       string
	Operations []AlterOperation
}

func (n *AlterTableNode) Accept(visitor Visitor) error {
	return visitor.VisitAlterTable(n)
}

// EnumNode represents a CREATE TYPE ... AS ENUM statement (PostgreSQL)
type EnumNode struct {
	Name   string
	Values []string
}

// NewEnum creates a new enum node
func NewEnum(name string, values ...string) *EnumNode {
	return &EnumNode{
		Name:   name,
		Values: values,
	}
}

func (n *EnumNode) Accept(visitor Visitor) error {
	return visitor.VisitEnum(n)
}

// CreateTableNode represents a CREATE TABLE statement
type CreateTableNode struct {
	Name        string
	Columns     []*ColumnNode
	Constraints []*ConstraintNode
	Options     map[string]string // For dialect-specific options like ENGINE
	Comment     string
}

// NewCreateTable creates a new CREATE TABLE node
func NewCreateTable(name string) *CreateTableNode {
	return &CreateTableNode{
		Name:        name,
		Columns:     make([]*ColumnNode, 0),
		Constraints: make([]*ConstraintNode, 0),
		Options:     make(map[string]string),
	}
}

func (n *CreateTableNode) Accept(visitor Visitor) error {
	return visitor.VisitCreateTable(n)
}

// AddColumn adds a column to the CREATE TABLE statement
func (n *CreateTableNode) AddColumn(column *ColumnNode) *CreateTableNode {
	n.Columns = append(n.Columns, column)
	return n
}

// AddConstraint adds a constraint to the CREATE TABLE statement
func (n *CreateTableNode) AddConstraint(constraint *ConstraintNode) *CreateTableNode {
	n.Constraints = append(n.Constraints, constraint)
	return n
}

// SetOption sets a table option (like ENGINE for MySQL)
func (n *CreateTableNode) SetOption(key, value string) *CreateTableNode {
	n.Options[key] = value
	return n
}

// ColumnNode represents a table column definition
type ColumnNode struct {
	Name       string
	Type       string
	Nullable   bool
	Primary    bool
	Unique     bool
	AutoInc    bool
	Default    *DefaultValue
	Check      string
	Comment    string
	ForeignKey *ForeignKeyRef
}

// NewColumn creates a new column node
func NewColumn(name, dataType string) *ColumnNode {
	return &ColumnNode{
		Name:     name,
		Type:     dataType,
		Nullable: true, // Default to nullable
	}
}

func (n *ColumnNode) Accept(visitor Visitor) error {
	return visitor.VisitColumn(n)
}

// SetPrimary marks the column as primary key
func (n *ColumnNode) SetPrimary() *ColumnNode {
	n.Primary = true
	n.Nullable = false // Primary keys are always NOT NULL
	return n
}

// SetNotNull marks the column as NOT NULL
func (n *ColumnNode) SetNotNull() *ColumnNode {
	n.Nullable = false
	return n
}

// SetUnique marks the column as UNIQUE
func (n *ColumnNode) SetUnique() *ColumnNode {
	n.Unique = true
	return n
}

// SetAutoIncrement marks the column as auto-incrementing
func (n *ColumnNode) SetAutoIncrement() *ColumnNode {
	n.AutoInc = true
	return n
}

// SetDefault sets a literal default value
func (n *ColumnNode) SetDefault(value string) *ColumnNode {
	n.Default = &DefaultValue{Value: value}
	return n
}

// SetDefaultFunction sets a function as default value
func (n *ColumnNode) SetDefaultFunction(fn string) *ColumnNode {
	n.Default = &DefaultValue{Function: fn}
	return n
}

// SetCheck sets a check constraint
func (n *ColumnNode) SetCheck(expression string) *ColumnNode {
	n.Check = expression
	return n
}

// SetComment sets a column comment
func (n *ColumnNode) SetComment(comment string) *ColumnNode {
	n.Comment = comment
	return n
}

// SetForeignKey sets a foreign key reference
func (n *ColumnNode) SetForeignKey(table, column, name string) *ColumnNode {
	n.ForeignKey = &ForeignKeyRef{
		Table:  table,
		Column: column,
		Name:   name,
	}
	return n
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

// IndexNode represents a CREATE INDEX statement
type IndexNode struct {
	Name    string
	Table   string
	Columns []string
	Unique  bool
	Type    string // For index type like BTREE, HASH, etc.
	Comment string
}

// NewIndex creates a new index node
func NewIndex(name, table string, columns ...string) *IndexNode {
	return &IndexNode{
		Name:    name,
		Table:   table,
		Columns: columns,
	}
}

func (n *IndexNode) Accept(visitor Visitor) error {
	return visitor.VisitIndex(n)
}

// SetUnique marks the index as unique
func (n *IndexNode) SetUnique() *IndexNode {
	n.Unique = true
	return n
}

// CommentNode represents SQL comments
type CommentNode struct {
	Text string
}

// NewComment creates a new comment node
func NewComment(text string) *CommentNode {
	return &CommentNode{Text: text}
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
