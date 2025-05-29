package ast_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/ast/mocks"
)

func TestNewCreateTable(t *testing.T) {
	c := qt.New(t)

	table := ast.NewCreateTable("users")

	c.Assert(table.Name, qt.Equals, "users")
	c.Assert(table.Columns, qt.IsNotNil)
	c.Assert(table.Columns, qt.HasLen, 0)
	c.Assert(table.Constraints, qt.IsNotNil)
	c.Assert(table.Constraints, qt.HasLen, 0)
	c.Assert(table.Options, qt.IsNotNil)
	c.Assert(table.Options, qt.HasLen, 0)
}

func TestCreateTableNode_FluentAPI(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("id", "INTEGER")
	constraint := ast.NewPrimaryKeyConstraint("id")

	table := ast.NewCreateTable("users").
		AddColumn(column).
		AddConstraint(constraint).
		SetOption("ENGINE", "InnoDB")

	c.Assert(table.Name, qt.Equals, "users")
	c.Assert(table.Columns, qt.HasLen, 1)
	c.Assert(table.Columns[0], qt.Equals, column)
	c.Assert(table.Constraints, qt.HasLen, 1)
	c.Assert(table.Constraints[0], qt.Equals, constraint)
	c.Assert(table.Options["ENGINE"], qt.Equals, "InnoDB")
}

func TestCreateTableNode_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	node := &ast.CreateTableNode{Name: "users"}

	err := node.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"CreateTable:users"})
}

func TestAlterTableNode_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	node := &ast.AlterTableNode{Name: "users"}

	err := node.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"AlterTable:users"})
}

func TestNewColumn(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("name", "VARCHAR(255)")

	c.Assert(column.Name, qt.Equals, "name")
	c.Assert(column.Type, qt.Equals, "VARCHAR(255)")
	c.Assert(column.Nullable, qt.IsTrue) // Default to nullable
	c.Assert(column.Primary, qt.IsFalse)
	c.Assert(column.Unique, qt.IsFalse)
	c.Assert(column.AutoInc, qt.IsFalse)
}

func TestColumnNode_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	node := &ast.ColumnNode{Name: "id"}

	err := node.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"Column:id"})
}

func TestColumnNode_SetPrimary(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("id", "INTEGER").SetPrimary()

	c.Assert(column.Primary, qt.IsTrue)
	c.Assert(column.Nullable, qt.IsFalse) // Primary keys are always NOT NULL
}

func TestColumnNode_SetNotNull(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("name", "VARCHAR(255)").SetNotNull()

	c.Assert(column.Nullable, qt.IsFalse)
}

func TestColumnNode_SetUnique(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("email", "VARCHAR(255)").SetUnique()

	c.Assert(column.Unique, qt.IsTrue)
}

func TestColumnNode_SetAutoIncrement(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("id", "INTEGER").SetAutoIncrement()

	c.Assert(column.AutoInc, qt.IsTrue)
}

func TestColumnNode_SetDefault(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("status", "VARCHAR(50)").SetDefault("'active'")

	c.Assert(column.Default, qt.IsNotNil)
	c.Assert(column.Default.Value, qt.Equals, "'active'")
	c.Assert(column.Default.Function, qt.Equals, "")
}

func TestColumnNode_SetDefaultFunction(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("created_at", "TIMESTAMP").SetDefaultFunction("NOW()")

	c.Assert(column.Default, qt.IsNotNil)
	c.Assert(column.Default.Function, qt.Equals, "NOW()")
	c.Assert(column.Default.Value, qt.Equals, "")
}

func TestColumnNode_SetCheck(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("age", "INTEGER").SetCheck("age >= 0")

	c.Assert(column.Check, qt.Equals, "age >= 0")
}

func TestColumnNode_SetComment(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("id", "INTEGER").SetComment("Primary key")

	c.Assert(column.Comment, qt.Equals, "Primary key")
}

func TestColumnNode_SetForeignKey(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("user_id", "INTEGER").SetForeignKey("users", "id", "fk_user")

	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.Table, qt.Equals, "users")
	c.Assert(column.ForeignKey.Column, qt.Equals, "id")
	c.Assert(column.ForeignKey.Name, qt.Equals, "fk_user")
}

func TestColumnNode_FluentAPI(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("id", "INTEGER").
		SetPrimary().
		SetAutoIncrement().
		SetComment("Auto-incrementing primary key")

	c.Assert(column.Name, qt.Equals, "id")
	c.Assert(column.Type, qt.Equals, "INTEGER")
	c.Assert(column.Primary, qt.IsTrue)
	c.Assert(column.AutoInc, qt.IsTrue)
	c.Assert(column.Nullable, qt.IsFalse) // SetPrimary() sets this to false
	c.Assert(column.Comment, qt.Equals, "Auto-incrementing primary key")
}

func TestConstraintNode_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	node := &ast.ConstraintNode{Name: "pk_users"}

	err := node.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"Constraint:pk_users"})
}

func TestNewIndex(t *testing.T) {
	c := qt.New(t)

	index := ast.NewIndex("idx_users_email", "users", "email")

	c.Assert(index.Name, qt.Equals, "idx_users_email")
	c.Assert(index.Table, qt.Equals, "users")
	c.Assert(index.Columns, qt.DeepEquals, []string{"email"})
	c.Assert(index.Unique, qt.IsFalse)
}

func TestIndexNode_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	node := &ast.IndexNode{Name: "idx_users_email"}

	err := node.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"Index:idx_users_email"})
}

func TestIndexNode_SetUnique(t *testing.T) {
	c := qt.New(t)

	index := ast.NewIndex("idx_users_email", "users", "email").SetUnique()

	c.Assert(index.Unique, qt.IsTrue)
}

func TestNewEnum(t *testing.T) {
	c := qt.New(t)

	enum := ast.NewEnum("status", "active", "inactive", "pending")

	c.Assert(enum.Name, qt.Equals, "status")
	c.Assert(enum.Values, qt.DeepEquals, []string{"active", "inactive", "pending"})
}

func TestEnumNode_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	node := &ast.EnumNode{Name: "status"}

	err := node.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"Enum:status"})
}

func TestNewComment(t *testing.T) {
	c := qt.New(t)

	comment := ast.NewComment("This is a test comment")

	c.Assert(comment.Text, qt.Equals, "This is a test comment")
}

func TestCommentNode_Accept(t *testing.T) {
	c := qt.New(t)

	visitor := &mocks.MockVisitor{}
	node := &ast.CommentNode{Text: "This is a comment"}

	err := node.Accept(visitor)

	c.Assert(err, qt.IsNil)
	c.Assert(visitor.VisitedNodes, qt.DeepEquals, []string{"Comment:This is a comment"})
}

// Test complex table creation with multiple elements
func TestComplexTableCreation(t *testing.T) {
	c := qt.New(t)

	// Create a complex table with multiple columns, constraints, and options
	table := ast.NewCreateTable("users").
		AddColumn(
			ast.NewColumn("id", "INTEGER").
				SetPrimary().
				SetAutoIncrement().
				SetComment("Primary key"),
		).
		AddColumn(
			ast.NewColumn("email", "VARCHAR(255)").
				SetNotNull().
				SetUnique().
				SetComment("User email address"),
		).
		AddColumn(
			ast.NewColumn("created_at", "TIMESTAMP").
				SetNotNull().
				SetDefaultFunction("CURRENT_TIMESTAMP"),
		).
		AddColumn(
			ast.NewColumn("status", "VARCHAR(20)").
				SetNotNull().
				SetDefault("'active'").
				SetCheck("status IN ('active', 'inactive', 'pending')"),
		).
		AddConstraint(ast.NewUniqueConstraint("uk_users_email", "email")).
		SetOption("ENGINE", "InnoDB").
		SetOption("CHARSET", "utf8mb4")

	c.Assert(table.Name, qt.Equals, "users")
	c.Assert(table.Columns, qt.HasLen, 4)
	c.Assert(table.Constraints, qt.HasLen, 1)
	c.Assert(table.Options, qt.HasLen, 2)

	// Verify first column (id)
	idCol := table.Columns[0]
	c.Assert(idCol.Name, qt.Equals, "id")
	c.Assert(idCol.Primary, qt.IsTrue)
	c.Assert(idCol.AutoInc, qt.IsTrue)
	c.Assert(idCol.Nullable, qt.IsFalse)
	c.Assert(idCol.Comment, qt.Equals, "Primary key")

	// Verify second column (email)
	emailCol := table.Columns[1]
	c.Assert(emailCol.Name, qt.Equals, "email")
	c.Assert(emailCol.Unique, qt.IsTrue)
	c.Assert(emailCol.Nullable, qt.IsFalse)

	// Verify third column (created_at)
	createdCol := table.Columns[2]
	c.Assert(createdCol.Default, qt.IsNotNil)
	c.Assert(createdCol.Default.Function, qt.Equals, "CURRENT_TIMESTAMP")

	// Verify fourth column (status)
	statusCol := table.Columns[3]
	c.Assert(statusCol.Default, qt.IsNotNil)
	c.Assert(statusCol.Default.Value, qt.Equals, "'active'")
	c.Assert(statusCol.Check, qt.Equals, "status IN ('active', 'inactive', 'pending')")

	// Verify constraint
	constraint := table.Constraints[0]
	c.Assert(constraint.Type, qt.Equals, ast.UniqueConstraint)
	c.Assert(constraint.Name, qt.Equals, "uk_users_email")

	// Verify options
	c.Assert(table.Options["ENGINE"], qt.Equals, "InnoDB")
	c.Assert(table.Options["CHARSET"], qt.Equals, "utf8mb4")
}

// Test multi-column index
func TestNewIndex_MultipleColumns(t *testing.T) {
	c := qt.New(t)

	index := ast.NewIndex("idx_users_name_email", "users", "last_name", "first_name", "email")

	c.Assert(index.Name, qt.Equals, "idx_users_name_email")
	c.Assert(index.Table, qt.Equals, "users")
	c.Assert(index.Columns, qt.DeepEquals, []string{"last_name", "first_name", "email"})
}

// Test enum with single value
func TestNewEnum_SingleValue(t *testing.T) {
	c := qt.New(t)

	enum := ast.NewEnum("boolean_status", "true")

	c.Assert(enum.Name, qt.Equals, "boolean_status")
	c.Assert(enum.Values, qt.DeepEquals, []string{"true"})
}

// Test enum with no values (edge case)
func TestNewEnum_NoValues(t *testing.T) {
	c := qt.New(t)

	enum := ast.NewEnum("empty_enum")

	c.Assert(enum.Name, qt.Equals, "empty_enum")
	c.Assert(enum.Values, qt.HasLen, 0)
}

// Test index with no columns (edge case)
func TestNewIndex_NoColumns(t *testing.T) {
	c := qt.New(t)

	index := ast.NewIndex("idx_empty", "users")

	c.Assert(index.Name, qt.Equals, "idx_empty")
	c.Assert(index.Table, qt.Equals, "users")
	c.Assert(index.Columns, qt.HasLen, 0)
}

// Test column with all properties set
func TestColumnNode_AllProperties(t *testing.T) {
	c := qt.New(t)

	column := ast.NewColumn("user_id", "INTEGER").
		SetNotNull().
		SetUnique().
		SetAutoIncrement().
		SetDefault("0").
		SetCheck("user_id > 0").
		SetComment("User identifier").
		SetForeignKey("users", "id", "fk_user_ref")

	c.Assert(column.Name, qt.Equals, "user_id")
	c.Assert(column.Type, qt.Equals, "INTEGER")
	c.Assert(column.Nullable, qt.IsFalse)
	c.Assert(column.Unique, qt.IsTrue)
	c.Assert(column.AutoInc, qt.IsTrue)
	c.Assert(column.Default, qt.IsNotNil)
	c.Assert(column.Default.Value, qt.Equals, "0")
	c.Assert(column.Check, qt.Equals, "user_id > 0")
	c.Assert(column.Comment, qt.Equals, "User identifier")
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.Table, qt.Equals, "users")
	c.Assert(column.ForeignKey.Column, qt.Equals, "id")
	c.Assert(column.ForeignKey.Name, qt.Equals, "fk_user_ref")
}
