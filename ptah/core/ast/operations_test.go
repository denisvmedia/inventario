package ast_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/ast/mocks"
)

func TestAddColumnOperation_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	column := &ast.ColumnNode{Name: "new_column"}
	op := &ast.AddColumnOperation{Column: column}

	err := op.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"Column:new_column"})
}

func TestAddColumnOperation_AcceptError(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{ReturnError: true}
	column := &ast.ColumnNode{Name: "new_column"}
	op := &ast.AddColumnOperation{Column: column}

	err := op.Accept(visitor)

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, "mock error")
}

func TestDropColumnOperation_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	op := &ast.DropColumnOperation{ColumnName: "old_column"}

	err := op.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.HasLen, 0) // DropColumnOperation returns nil without visiting
}

func TestDropColumnOperation_AcceptError(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{ReturnError: true}
	op := &ast.DropColumnOperation{ColumnName: "old_column"}

	err := op.Accept(visitor)

	c.Assert(err, qt.IsNil) // DropColumnOperation always returns nil
	c.Assert(visitor.VisitedNodes, qt.HasLen, 0)
}

func TestModifyColumnOperation_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	column := &ast.ColumnNode{Name: "modified_column"}
	op := &ast.ModifyColumnOperation{Column: column}

	err := op.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"Column:modified_column"})
}

func TestModifyColumnOperation_AcceptError(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{ReturnError: true}
	column := &ast.ColumnNode{Name: "modified_column"}
	op := &ast.ModifyColumnOperation{Column: column}

	err := op.Accept(visitor)

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, "mock error")
}

// Test that alter operations implement the AlterOperation interface
func TestAlterOperations_ImplementInterface(t *testing.T) {
	c := qt.New(t)

	var ops []ast.AlterOperation

	// Test that all alter operations can be assigned to AlterOperation interface
	// This is a compile-time check - if they don't implement the interface, this won't compile
	ops = append(ops, &ast.AddColumnOperation{Column: ast.NewColumn("test", "INTEGER")})
	ops = append(ops, &ast.DropColumnOperation{ColumnName: "test"})
	ops = append(ops, &ast.ModifyColumnOperation{Column: ast.NewColumn("test", "INTEGER")})

	c.Assert(ops, qt.HasLen, 3)

	// Test that they all implement the Node interface as well (since AlterOperation embeds Node)
	for _, op := range ops {
		// This tests that Accept method exists and can be called
		visitor := &mocks.MockVisitor{}
		err := op.Accept(visitor)
		c.Assert(err, qt.IsNil)
	}
}

// Test that operations implement the AlterOperation interface (compile-time check)
func TestAlterOperations_InterfaceCompliance(t *testing.T) {
	c := qt.New(t)

	// Test that types implement the interface - this is a compile-time check
	var _ ast.AlterOperation = &ast.AddColumnOperation{}
	var _ ast.AlterOperation = &ast.DropColumnOperation{}
	var _ ast.AlterOperation = &ast.ModifyColumnOperation{}

	// Test that they also implement Node interface
	var _ ast.Node = &ast.AddColumnOperation{}
	var _ ast.Node = &ast.DropColumnOperation{}
	var _ ast.Node = &ast.ModifyColumnOperation{}

	// If we get here, all interfaces are implemented correctly
	c.Assert(true, qt.IsTrue)
}

// Test AddColumnOperation with complex column
func TestAddColumnOperation_ComplexColumn(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	column := ast.NewColumn("user_id", "INTEGER").
		SetNotNull().
		SetForeignKey("users", "id", "fk_user").
		SetComment("Foreign key to users table")

	op := &ast.AddColumnOperation{Column: column}

	err := op.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"Column:user_id"})

	// Verify the column properties are preserved
	c.Assert(op.Column.Name, qt.Equals, "user_id")
	c.Assert(op.Column.Type, qt.Equals, "INTEGER")
	c.Assert(op.Column.Nullable, qt.IsFalse)
	c.Assert(op.Column.ForeignKey, qt.IsNotNil)
	c.Assert(op.Column.Comment, qt.Equals, "Foreign key to users table")
}

// Test ModifyColumnOperation with complex column
func TestModifyColumnOperation_ComplexColumn(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	column := ast.NewColumn("status", "VARCHAR(20)").
		SetNotNull().
		SetDefault("'active'").
		SetCheck("status IN ('active', 'inactive', 'pending')").
		SetComment("User status")

	op := &ast.ModifyColumnOperation{Column: column}

	err := op.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"Column:status"})

	// Verify the column properties are preserved
	c.Assert(op.Column.Name, qt.Equals, "status")
	c.Assert(op.Column.Type, qt.Equals, "VARCHAR(20)")
	c.Assert(op.Column.Nullable, qt.IsFalse)
	c.Assert(op.Column.Default, qt.IsNotNil)
	c.Assert(op.Column.Default.Value, qt.Equals, "'active'")
	c.Assert(op.Column.Check, qt.Equals, "status IN ('active', 'inactive', 'pending')")
	c.Assert(op.Column.Comment, qt.Equals, "User status")
}

// Test DropColumnOperation with different column names
func TestDropColumnOperation_VariousNames(t *testing.T) {
	tests := []struct {
		name       string
		columnName string
	}{
		{
			name:       "SimpleColumn",
			columnName: "id",
		},
		{
			name:       "UnderscoreColumn",
			columnName: "user_id",
		},
		{
			name:       "LongColumn",
			columnName: "very_long_column_name_with_many_underscores",
		},
		{
			name:       "EmptyColumn",
			columnName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			visitor := &mocks.MockVisitor{}
			op := &ast.DropColumnOperation{ColumnName: tt.columnName}

			err := op.Accept(visitor)

			c.Assert(err, qt.IsNil)
			c.Assert(visitor.VisitedNodes, qt.HasLen, 0)
			c.Assert(op.ColumnName, qt.Equals, tt.columnName)
		})
	}
}

// Test operations with nil columns (edge case)
func TestAlterOperations_NilColumn(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}

	// Test AddColumnOperation with nil column - this will panic
	addOp := &ast.AddColumnOperation{Column: nil}
	c.Assert(func() { _ = addOp.Accept(visitor) }, qt.PanicMatches, ".*")

	// Test ModifyColumnOperation with nil column - this will panic
	modifyOp := &ast.ModifyColumnOperation{Column: nil}
	c.Assert(func() { _ = modifyOp.Accept(visitor) }, qt.PanicMatches, ".*")
}

// Test that operations can be used in AlterTableNode
func TestAlterOperations_InAlterTable(t *testing.T) {
	c := qt.New(t)

	addOp := &ast.AddColumnOperation{
		Column: ast.NewColumn("new_col", "VARCHAR(255)"),
	}
	dropOp := &ast.DropColumnOperation{
		ColumnName: "old_col",
	}
	modifyOp := &ast.ModifyColumnOperation{
		Column: ast.NewColumn("existing_col", "TEXT"),
	}

	alterTable := &ast.AlterTableNode{
		Name:       "users",
		Operations: []ast.AlterOperation{addOp, dropOp, modifyOp},
	}

	c.Assert(alterTable.Name, qt.Equals, "users")
	c.Assert(alterTable.Operations, qt.HasLen, 3)

	// Verify operations are correctly stored
	c.Assert(alterTable.Operations[0], qt.Equals, addOp)
	c.Assert(alterTable.Operations[1], qt.Equals, dropOp)
	c.Assert(alterTable.Operations[2], qt.Equals, modifyOp)
}
