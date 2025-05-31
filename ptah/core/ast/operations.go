package ast

// AlterOperation represents different types of operations that can be performed in ALTER TABLE statements.
//
// This interface extends the Node interface and includes a marker method to ensure
// type safety. All ALTER operations must implement both the visitor pattern
// and the marker method.
type AlterOperation interface {
	Node
	alterOperation() // marker method to ensure type safety
}

// AddColumnOperation represents an ADD COLUMN operation in ALTER TABLE statements.
//
// This operation adds a new column to an existing table with all the specified
// column attributes (type, constraints, defaults, etc.).
type AddColumnOperation struct {
	// Column contains the complete column definition to add
	Column *ColumnNode
}

// Accept implements the Node interface for AddColumnOperation.
//
// The visitor typically handles this by delegating to the column's Accept method
// or by processing it within the VisitAlterTable method.
func (op *AddColumnOperation) Accept(visitor Visitor) error {
	return op.Column.Accept(visitor)
}

// alterOperation implements the marker method for type safety.
func (op *AddColumnOperation) alterOperation() {}

// DropColumnOperation represents a DROP COLUMN operation in ALTER TABLE statements.
//
// This operation removes an existing column from a table. Note that dropping
// columns may have cascading effects on indexes, constraints, and foreign keys.
type DropColumnOperation struct {
	// ColumnName is the name of the column to drop
	ColumnName string
}

// Accept implements the Node interface for DropColumnOperation.
//
// The actual rendering is typically handled by the visitor's VisitAlterTable method
// rather than delegating to a separate visitor method.
func (op *DropColumnOperation) Accept(_visitor Visitor) error {
	// This would be handled by the visitor's VisitAlterTable method
	return nil
}

// alterOperation implements the marker method for type safety.
func (op *DropColumnOperation) alterOperation() {}

// ModifyColumnOperation represents an ALTER COLUMN/MODIFY COLUMN operation in ALTER TABLE statements.
//
// This operation changes the definition of an existing column. The exact syntax
// varies between database systems (ALTER COLUMN vs MODIFY COLUMN).
type ModifyColumnOperation struct {
	// Column contains the new column definition
	Column *ColumnNode
}

// Accept implements the Node interface for ModifyColumnOperation.
//
// The visitor typically handles this by delegating to the column's Accept method
// or by processing it within the VisitAlterTable method.
func (op *ModifyColumnOperation) Accept(visitor Visitor) error {
	return op.Column.Accept(visitor)
}

// alterOperation implements the marker method for type safety.
func (op *ModifyColumnOperation) alterOperation() {}
