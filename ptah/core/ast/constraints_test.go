package ast_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/ast/mocks"
)

func TestNewPrimaryKeyConstraint(t *testing.T) {
	c := qt.New(t)

	constraint := ast.NewPrimaryKeyConstraint("id", "tenant_id")

	c.Assert(constraint.Type, qt.Equals, ast.PrimaryKeyConstraint)
	c.Assert(constraint.Columns, qt.DeepEquals, []string{"id", "tenant_id"})
	c.Assert(constraint.Name, qt.Equals, "")
	c.Assert(constraint.Reference, qt.IsNil)
	c.Assert(constraint.Expression, qt.Equals, "")
}

func TestNewPrimaryKeyConstraint_SingleColumn(t *testing.T) {
	c := qt.New(t)

	constraint := ast.NewPrimaryKeyConstraint("id")

	c.Assert(constraint.Type, qt.Equals, ast.PrimaryKeyConstraint)
	c.Assert(constraint.Columns, qt.DeepEquals, []string{"id"})
	c.Assert(constraint.Name, qt.Equals, "")
}

func TestNewPrimaryKeyConstraint_NoColumns(t *testing.T) {
	c := qt.New(t)

	constraint := ast.NewPrimaryKeyConstraint()

	c.Assert(constraint.Type, qt.Equals, ast.PrimaryKeyConstraint)
	c.Assert(constraint.Columns, qt.HasLen, 0)
	c.Assert(constraint.Name, qt.Equals, "")
}

func TestNewUniqueConstraint(t *testing.T) {
	c := qt.New(t)

	constraint := ast.NewUniqueConstraint("uk_email", "email")

	c.Assert(constraint.Type, qt.Equals, ast.UniqueConstraint)
	c.Assert(constraint.Name, qt.Equals, "uk_email")
	c.Assert(constraint.Columns, qt.DeepEquals, []string{"email"})
	c.Assert(constraint.Reference, qt.IsNil)
	c.Assert(constraint.Expression, qt.Equals, "")
}

func TestNewUniqueConstraint_MultipleColumns(t *testing.T) {
	c := qt.New(t)

	constraint := ast.NewUniqueConstraint("uk_user_email", "user_id", "email")

	c.Assert(constraint.Type, qt.Equals, ast.UniqueConstraint)
	c.Assert(constraint.Name, qt.Equals, "uk_user_email")
	c.Assert(constraint.Columns, qt.DeepEquals, []string{"user_id", "email"})
}

func TestNewUniqueConstraint_NoColumns(t *testing.T) {
	c := qt.New(t)

	constraint := ast.NewUniqueConstraint("uk_empty")

	c.Assert(constraint.Type, qt.Equals, ast.UniqueConstraint)
	c.Assert(constraint.Name, qt.Equals, "uk_empty")
	c.Assert(constraint.Columns, qt.HasLen, 0)
}

func TestNewForeignKeyConstraint(t *testing.T) {
	c := qt.New(t)

	ref := &ast.ForeignKeyRef{
		Table:    "users",
		Column:   "id",
		OnDelete: "CASCADE",
		OnUpdate: "RESTRICT",
		Name:     "fk_user",
	}
	constraint := ast.NewForeignKeyConstraint("fk_posts_user", []string{"user_id"}, ref)

	c.Assert(constraint.Type, qt.Equals, ast.ForeignKeyConstraint)
	c.Assert(constraint.Name, qt.Equals, "fk_posts_user")
	c.Assert(constraint.Columns, qt.DeepEquals, []string{"user_id"})
	c.Assert(constraint.Reference, qt.Equals, ref)
	c.Assert(constraint.Expression, qt.Equals, "")
}

func TestNewForeignKeyConstraint_MultipleColumns(t *testing.T) {
	c := qt.New(t)

	ref := &ast.ForeignKeyRef{
		Table:  "users",
		Column: "id,tenant_id",
		Name:   "fk_user_tenant",
	}
	constraint := ast.NewForeignKeyConstraint("fk_posts_user_tenant", []string{"user_id", "tenant_id"}, ref)

	c.Assert(constraint.Type, qt.Equals, ast.ForeignKeyConstraint)
	c.Assert(constraint.Name, qt.Equals, "fk_posts_user_tenant")
	c.Assert(constraint.Columns, qt.DeepEquals, []string{"user_id", "tenant_id"})
	c.Assert(constraint.Reference, qt.Equals, ref)
}

func TestNewForeignKeyConstraint_NilReference(t *testing.T) {
	c := qt.New(t)

	constraint := ast.NewForeignKeyConstraint("fk_test", []string{"col1"}, nil)

	c.Assert(constraint.Type, qt.Equals, ast.ForeignKeyConstraint)
	c.Assert(constraint.Name, qt.Equals, "fk_test")
	c.Assert(constraint.Columns, qt.DeepEquals, []string{"col1"})
	c.Assert(constraint.Reference, qt.IsNil)
}

func TestNewForeignKeyConstraint_EmptyColumns(t *testing.T) {
	c := qt.New(t)

	ref := &ast.ForeignKeyRef{
		Table:  "users",
		Column: "id",
		Name:   "fk_user",
	}
	constraint := ast.NewForeignKeyConstraint("fk_empty", []string{}, ref)

	c.Assert(constraint.Type, qt.Equals, ast.ForeignKeyConstraint)
	c.Assert(constraint.Name, qt.Equals, "fk_empty")
	c.Assert(constraint.Columns, qt.HasLen, 0)
	c.Assert(constraint.Reference, qt.Equals, ref)
}

// Test constraint builders with edge cases
func TestConstraintBuilders_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		buildFunc    func() *ast.ConstraintNode
		expectedType ast.ConstraintType
		expectedName string
		expectedCols []string
	}{
		{
			name: "PrimaryKeyWithManyColumns",
			buildFunc: func() *ast.ConstraintNode {
				return ast.NewPrimaryKeyConstraint("col1", "col2", "col3", "col4", "col5")
			},
			expectedType: ast.PrimaryKeyConstraint,
			expectedName: "",
			expectedCols: []string{"col1", "col2", "col3", "col4", "col5"},
		},
		{
			name: "UniqueWithEmptyName",
			buildFunc: func() *ast.ConstraintNode {
				return ast.NewUniqueConstraint("", "email")
			},
			expectedType: ast.UniqueConstraint,
			expectedName: "",
			expectedCols: []string{"email"},
		},
		{
			name: "UniqueWithLongName",
			buildFunc: func() *ast.ConstraintNode {
				return ast.NewUniqueConstraint("very_long_unique_constraint_name_with_many_words", "column")
			},
			expectedType: ast.UniqueConstraint,
			expectedName: "very_long_unique_constraint_name_with_many_words",
			expectedCols: []string{"column"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			constraint := tt.buildFunc()

			c.Assert(constraint.Type, qt.Equals, tt.expectedType)
			c.Assert(constraint.Name, qt.Equals, tt.expectedName)
			c.Assert(constraint.Columns, qt.DeepEquals, tt.expectedCols)
		})
	}
}

// Test that constraints can be used in CreateTableNode
func TestConstraints_InCreateTable(t *testing.T) {
	c := qt.New(t)

	pkConstraint := ast.NewPrimaryKeyConstraint("id")
	ukConstraint := ast.NewUniqueConstraint("uk_email", "email")
	fkRef := &ast.ForeignKeyRef{
		Table:  "users",
		Column: "id",
		Name:   "fk_user",
	}
	fkConstraint := ast.NewForeignKeyConstraint("fk_posts_user", []string{"user_id"}, fkRef)

	table := ast.NewCreateTable("posts").
		AddConstraint(pkConstraint).
		AddConstraint(ukConstraint).
		AddConstraint(fkConstraint)

	c.Assert(table.Constraints, qt.HasLen, 3)
	c.Assert(table.Constraints[0], qt.Equals, pkConstraint)
	c.Assert(table.Constraints[1], qt.Equals, ukConstraint)
	c.Assert(table.Constraints[2], qt.Equals, fkConstraint)
}

// Test constraint Accept method
func TestConstraints_Accept(t *testing.T) {
	tests := []struct {
		name       string
		constraint *ast.ConstraintNode
		expected   string
	}{
		{
			name:       "PrimaryKey",
			constraint: ast.NewPrimaryKeyConstraint("id"),
			expected:   "Constraint:",
		},
		{
			name:       "Unique",
			constraint: ast.NewUniqueConstraint("uk_email", "email"),
			expected:   "Constraint:uk_email",
		},
		{
			name: "ForeignKey",
			constraint: ast.NewForeignKeyConstraint("fk_user", []string{"user_id"}, &ast.ForeignKeyRef{
				Table:  "users",
				Column: "id",
				Name:   "fk_user",
			}),
			expected: "Constraint:fk_user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			visitor := &mocks.MockVisitor{}
			err := tt.constraint.Accept(visitor)

			c.Assert(err, qt.IsNil)
			c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{tt.expected})
		})
	}
}

// Test constraint Accept with error
func TestConstraints_AcceptError(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{ReturnError: true}
	constraint := ast.NewPrimaryKeyConstraint("id")

	err := constraint.Accept(visitor)

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Equals, "mock error")
}

// Test foreign key reference variations
func TestForeignKeyConstraint_ReferenceVariations(t *testing.T) {
	tests := []struct {
		name string
		ref  *ast.ForeignKeyRef
	}{
		{
			name: "BasicReference",
			ref: &ast.ForeignKeyRef{
				Table:  "users",
				Column: "id",
				Name:   "fk_basic",
			},
		},
		{
			name: "CascadeReference",
			ref: &ast.ForeignKeyRef{
				Table:    "users",
				Column:   "id",
				OnDelete: "CASCADE",
				OnUpdate: "CASCADE",
				Name:     "fk_cascade",
			},
		},
		{
			name: "SetNullReference",
			ref: &ast.ForeignKeyRef{
				Table:    "users",
				Column:   "id",
				OnDelete: "SET NULL",
				OnUpdate: "RESTRICT",
				Name:     "fk_set_null",
			},
		},
		{
			name: "NoActionReference",
			ref: &ast.ForeignKeyRef{
				Table:    "users",
				Column:   "id",
				OnDelete: "NO ACTION",
				OnUpdate: "NO ACTION",
				Name:     "fk_no_action",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			constraint := ast.NewForeignKeyConstraint("fk_test", []string{"user_id"}, tt.ref)

			c.Assert(constraint.Type, qt.Equals, ast.ForeignKeyConstraint)
			c.Assert(constraint.Reference, qt.Equals, tt.ref)
			c.Assert(constraint.Reference.Table, qt.Equals, tt.ref.Table)
			c.Assert(constraint.Reference.Column, qt.Equals, tt.ref.Column)
			c.Assert(constraint.Reference.OnDelete, qt.Equals, tt.ref.OnDelete)
			c.Assert(constraint.Reference.OnUpdate, qt.Equals, tt.ref.OnUpdate)
			c.Assert(constraint.Reference.Name, qt.Equals, tt.ref.Name)
		})
	}
}
