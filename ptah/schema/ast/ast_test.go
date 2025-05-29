package ast_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/ast/mocks"
)

// TestVisitorInterface_HappyPath tests that the visitor interface works correctly
func TestVisitorInterface_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		node         ast.Node
		expectedCall string
	}{
		{
			name:         "CreateTableNode",
			node:         &ast.CreateTableNode{Name: "users"},
			expectedCall: "CreateTable:users",
		},
		{
			name:         "AlterTableNode",
			node:         &ast.AlterTableNode{Name: "users"},
			expectedCall: "AlterTable:users",
		},
		{
			name:         "ColumnNode",
			node:         &ast.ColumnNode{Name: "id"},
			expectedCall: "Column:id",
		},
		{
			name:         "ConstraintNode",
			node:         &ast.ConstraintNode{Name: "pk_users"},
			expectedCall: "Constraint:pk_users",
		},
		{
			name:         "IndexNode",
			node:         &ast.IndexNode{Name: "idx_users_email"},
			expectedCall: "Index:idx_users_email",
		},
		{
			name:         "EnumNode",
			node:         &ast.EnumNode{Name: "status"},
			expectedCall: "Enum:status",
		},
		{
			name:         "CommentNode",
			node:         &ast.CommentNode{Text: "This is a comment"},
			expectedCall: "Comment:This is a comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			visitor := &mocks.MockVisitor{}
			err := tt.node.Accept(visitor)

			c.Assert(err, qt.IsNil)
			c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{tt.expectedCall})
		})
	}
}

// TestVisitorInterface_ErrorPath tests error propagation in visitor pattern
func TestVisitorInterface_ErrorPath(t *testing.T) {
	tests := []struct {
		name string
		node ast.Node
	}{
		{
			name: "CreateTableNode",
			node: &ast.CreateTableNode{Name: "users"},
		},
		{
			name: "AlterTableNode",
			node: &ast.AlterTableNode{Name: "users"},
		},
		{
			name: "ColumnNode",
			node: &ast.ColumnNode{Name: "id"},
		},
		{
			name: "ConstraintNode",
			node: &ast.ConstraintNode{Name: "pk_users"},
		},
		{
			name: "IndexNode",
			node: &ast.IndexNode{Name: "idx_users_email"},
		},
		{
			name: "EnumNode",
			node: &ast.EnumNode{Name: "status"},
		},
		{
			name: "CommentNode",
			node: &ast.CommentNode{Text: "This is a comment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			visitor := &mocks.MockVisitor{ReturnError: true}
			err := tt.node.Accept(visitor)

			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Equals, "mock error")
		})
	}
}

func TestStatementList_Accept_HappyPath(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	statements := []ast.Node{
		&ast.CreateTableNode{Name: "users"},
		&ast.IndexNode{Name: "idx_users"},
		&ast.CommentNode{Text: "test comment"},
	}

	sl := &ast.StatementList{Statements: statements}
	err := sl.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{
		"CreateTable:users",
		"Index:idx_users",
		"Comment:test comment",
	})
}

func TestStatementList_Accept_ErrorPath(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{ReturnError: true}
	statements := []ast.Node{
		&ast.CreateTableNode{Name: "users"},
		&ast.IndexNode{Name: "idx_users"},
	}

	sl := &ast.StatementList{Statements: statements}
	err := sl.Accept(visitor)

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "error visiting statement")
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"CreateTable:users"})
}

func TestStatementList_Accept_EmptyList(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	sl := &ast.StatementList{Statements: []ast.Node{}}

	err := sl.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.HasLen, 0)
}

func TestConstraintType_String(t *testing.T) {
	tests := []struct {
		name     string
		ct       ast.ConstraintType
		expected string
	}{
		{
			name:     "PrimaryKeyConstraint",
			ct:       ast.PrimaryKeyConstraint,
			expected: "PRIMARY KEY",
		},
		{
			name:     "UniqueConstraint",
			ct:       ast.UniqueConstraint,
			expected: "UNIQUE",
		},
		{
			name:     "ForeignKeyConstraint",
			ct:       ast.ForeignKeyConstraint,
			expected: "FOREIGN KEY",
		},
		{
			name:     "CheckConstraint",
			ct:       ast.CheckConstraint,
			expected: "CHECK",
		},
		{
			name:     "UnknownConstraint",
			ct:       ast.ConstraintType(999),
			expected: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := tt.ct.String()
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestDefaultValue_LiteralValue(t *testing.T) {
	c := qt.New(t)

	dv := &ast.DefaultValue{
		Value:    "'default_string'",
		Function: "",
	}

	c.Assert(dv.Value, qt.Equals, "'default_string'")
	c.Assert(dv.Function, qt.Equals, "")
}

func TestDefaultValue_FunctionValue(t *testing.T) {
	c := qt.New(t)

	dv := &ast.DefaultValue{
		Value:    "",
		Function: "NOW()",
	}

	c.Assert(dv.Value, qt.Equals, "")
	c.Assert(dv.Function, qt.Equals, "NOW()")
}

func TestDefaultValue_BothValues(t *testing.T) {
	c := qt.New(t)

	// Edge case: both values set (should not happen in normal usage)
	dv := &ast.DefaultValue{
		Value:    "'literal'",
		Function: "NOW()",
	}

	c.Assert(dv.Value, qt.Equals, "'literal'")
	c.Assert(dv.Function, qt.Equals, "NOW()")
}

func TestForeignKeyRef_BasicFields(t *testing.T) {
	c := qt.New(t)

	fkRef := &ast.ForeignKeyRef{
		Table:  "users",
		Column: "id",
		Name:   "fk_user",
	}

	c.Assert(fkRef.Table, qt.Equals, "users")
	c.Assert(fkRef.Column, qt.Equals, "id")
	c.Assert(fkRef.Name, qt.Equals, "fk_user")
	c.Assert(fkRef.OnDelete, qt.Equals, "")
	c.Assert(fkRef.OnUpdate, qt.Equals, "")
}

func TestForeignKeyRef_AllFields(t *testing.T) {
	c := qt.New(t)

	fkRef := &ast.ForeignKeyRef{
		Table:    "users",
		Column:   "id",
		OnDelete: "CASCADE",
		OnUpdate: "RESTRICT",
		Name:     "fk_user",
	}

	c.Assert(fkRef.Table, qt.Equals, "users")
	c.Assert(fkRef.Column, qt.Equals, "id")
	c.Assert(fkRef.OnDelete, qt.Equals, "CASCADE")
	c.Assert(fkRef.OnUpdate, qt.Equals, "RESTRICT")
	c.Assert(fkRef.Name, qt.Equals, "fk_user")
}

func TestForeignKeyRef_EmptyFields(t *testing.T) {
	c := qt.New(t)

	fkRef := &ast.ForeignKeyRef{}

	c.Assert(fkRef.Table, qt.Equals, "")
	c.Assert(fkRef.Column, qt.Equals, "")
	c.Assert(fkRef.OnDelete, qt.Equals, "")
	c.Assert(fkRef.OnUpdate, qt.Equals, "")
	c.Assert(fkRef.Name, qt.Equals, "")
}

// Test foreign key reference with all fields through column
func TestForeignKeyRef_ThroughColumn(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("user_id", "INTEGER").SetForeignKey("users", "id", "fk_user")

	// Manually set additional FK properties to test the struct
	column.ForeignKey.OnDelete = "CASCADE"
	column.ForeignKey.OnUpdate = "RESTRICT"

	c.Assert(column.ForeignKey.Table, qt.Equals, "users")
	c.Assert(column.ForeignKey.Column, qt.Equals, "id")
	c.Assert(column.ForeignKey.Name, qt.Equals, "fk_user")
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "CASCADE")
	c.Assert(column.ForeignKey.OnUpdate, qt.Equals, "RESTRICT")
}

// Test constraint type constants
func TestConstraintType_Constants(t *testing.T) {
	c := qt.New(t)

	// Test that constants have expected values
	c.Assert(int(ast.PrimaryKeyConstraint), qt.Equals, 0)
	c.Assert(int(ast.UniqueConstraint), qt.Equals, 1)
	c.Assert(int(ast.ForeignKeyConstraint), qt.Equals, 2)
	c.Assert(int(ast.CheckConstraint), qt.Equals, 3)
}

// Test that constraint types are distinct
func TestConstraintType_Distinct(t *testing.T) {
	c := qt.New(t)

	types := []ast.ConstraintType{
		ast.PrimaryKeyConstraint,
		ast.UniqueConstraint,
		ast.ForeignKeyConstraint,
		ast.CheckConstraint,
	}

	// Verify all types are different
	for i, t1 := range types {
		for j, t2 := range types {
			if i != j {
				c.Assert(t1, qt.Not(qt.Equals), t2)
			}
		}
	}
}

// Test default value edge cases
func TestDefaultValue_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		function string
	}{
		{
			name:     "EmptyValue",
			value:    "",
			function: "",
		},
		{
			name:     "NumericValue",
			value:    "42",
			function: "",
		},
		{
			name:     "BooleanValue",
			value:    "true",
			function: "",
		},
		{
			name:     "NullValue",
			value:    "NULL",
			function: "",
		},
		{
			name:     "ComplexFunction",
			value:    "",
			function: "COALESCE(column1, 'default')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			dv := &ast.DefaultValue{
				Value:    tt.value,
				Function: tt.function,
			}

			c.Assert(dv.Value, qt.Equals, tt.value)
			c.Assert(dv.Function, qt.Equals, tt.function)
		})
	}
}

// Test foreign key reference edge cases
func TestForeignKeyRef_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		table    string
		column   string
		onDelete string
		onUpdate string
		fkName   string
	}{
		{
			name:     "MinimalFK",
			table:    "t",
			column:   "c",
			onDelete: "",
			onUpdate: "",
			fkName:   "",
		},
		{
			name:     "LongNames",
			table:    "very_long_table_name_with_underscores",
			column:   "very_long_column_name_with_underscores",
			onDelete: "SET NULL",
			onUpdate: "NO ACTION",
			fkName:   "very_long_foreign_key_constraint_name",
		},
		{
			name:     "SpecialActions",
			table:    "parent",
			column:   "id",
			onDelete: "SET DEFAULT",
			onUpdate: "CASCADE",
			fkName:   "fk_special",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			fkRef := &ast.ForeignKeyRef{
				Table:    tt.table,
				Column:   tt.column,
				OnDelete: tt.onDelete,
				OnUpdate: tt.onUpdate,
				Name:     tt.fkName,
			}

			c.Assert(fkRef.Table, qt.Equals, tt.table)
			c.Assert(fkRef.Column, qt.Equals, tt.column)
			c.Assert(fkRef.OnDelete, qt.Equals, tt.onDelete)
			c.Assert(fkRef.OnUpdate, qt.Equals, tt.onUpdate)
			c.Assert(fkRef.Name, qt.Equals, tt.fkName)
		})
	}
}
