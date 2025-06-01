package mysql_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/migration/planner/dialects/mysql"
	"github.com/denisvmedia/inventario/ptah/migration/schemadiff/types"
)

func TestPlanner_GenerateMigrationAST_EnumsAdded(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "enum added generates warning comment",
			diff: &types.SchemaDiff{
				EnumsAdded: []string{"user_status"},
			},
			generated: &goschema.Database{
				Enums: []goschema.Enum{
					{Name: "user_status", Values: []string{"active", "inactive"}},
				},
			},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 1 {
					return false
				}
				commentNode, ok := nodes[0].(*ast.CommentNode)
				if !ok {
					return false
				}
				return commentNode.Text != ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := mysql.New()
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationAST_EnumsModified(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "enum modification generates warning comments",
			diff: &types.SchemaDiff{
				EnumsModified: []types.EnumDiff{
					{
						EnumName:      "user_status",
						ValuesAdded:   []string{"suspended"},
						ValuesRemoved: []string{"deprecated"},
					},
				},
			},
			generated: &goschema.Database{},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 2 {
					return false
				}
				// Both should be warning comments for MySQL enum limitations
				comment1, ok := nodes[0].(*ast.CommentNode)
				if !ok {
					return false
				}
				comment2, ok := nodes[1].(*ast.CommentNode)
				if !ok {
					return false
				}
				return comment1.Text != "" && comment2.Text != ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := mysql.New()
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationAST_TablesAdded(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "single table added",
			diff: &types.SchemaDiff{
				TablesAdded: []string{"users"},
			},
			generated: &goschema.Database{
				Tables: []goschema.Table{
					{Name: "users", StructName: "User"},
				},
				Fields: []goschema.Field{
					{Name: "id", Type: "INT", StructName: "User", Primary: true, AutoInc: true},
					{Name: "email", Type: "VARCHAR(255)", StructName: "User", Nullable: false},
				},
			},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 1 {
					return false
				}
				tableNode, ok := nodes[0].(*ast.CreateTableNode)
				if !ok {
					return false
				}
				return tableNode.Name == "users" && len(tableNode.Columns) == 2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := mysql.New()
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationAST_TablesModified(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "table with columns added",
			diff: &types.SchemaDiff{
				TablesModified: []types.TableDiff{
					{
						TableName:    "users",
						ColumnsAdded: []string{"created_at"},
					},
				},
			},
			generated: &goschema.Database{
				Fields: []goschema.Field{
					{Name: "created_at", Type: "TIMESTAMP", StructName: "User", Nullable: false},
				},
			},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 2 {
					return false
				}

				// First should be comment
				_, ok := nodes[0].(*ast.CommentNode)
				if !ok {
					return false
				}

				// Second should be ALTER TABLE
				alterNode, ok := nodes[1].(*ast.AlterTableNode)
				if !ok {
					return false
				}

				return alterNode.Name == "users" && len(alterNode.Operations) == 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := mysql.New()
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationAST_IndexesAdded(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "single index added",
			diff: &types.SchemaDiff{
				IndexesAdded: []string{"idx_users_email"},
			},
			generated: &goschema.Database{
				Indexes: []goschema.Index{
					{Name: "idx_users_email", StructName: "users", Fields: []string{"email"}},
				},
			},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 1 {
					return false
				}
				indexNode, ok := nodes[0].(*ast.IndexNode)
				if !ok {
					return false
				}
				return indexNode.Name == "idx_users_email" &&
					indexNode.Table == "users" &&
					len(indexNode.Columns) == 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := mysql.New()
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationAST_EnumsRemoved(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "enum removed generates warning comment",
			diff: &types.SchemaDiff{
				EnumsRemoved: []string{"old_enum"},
			},
			generated: &goschema.Database{},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 1 {
					return false
				}
				commentNode, ok := nodes[0].(*ast.CommentNode)
				if !ok {
					return false
				}
				return commentNode.Text != ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := mysql.New()
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}
