package ast

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

func (op *DropColumnOperation) Accept(_visitor Visitor) error {
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
