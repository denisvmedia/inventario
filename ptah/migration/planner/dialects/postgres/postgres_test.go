package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/migration/planner/dialects/postgres"
	"github.com/denisvmedia/inventario/ptah/migration/schemadiff/types"
)

func TestPlanner_GenerateMigrationSQL_EnumsAdded(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "single enum added",
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
				enumNode, ok := nodes[0].(*ast.EnumNode)
				if !ok {
					return false
				}
				return enumNode.Name == "user_status" &&
					len(enumNode.Values) == 2 &&
					enumNode.Values[0] == "active" &&
					enumNode.Values[1] == "inactive"
			},
		},
		{
			name: "multiple enums added",
			diff: &types.SchemaDiff{
				EnumsAdded: []string{"user_status", "order_status"},
			},
			generated: &goschema.Database{
				Enums: []goschema.Enum{
					{Name: "user_status", Values: []string{"active", "inactive"}},
					{Name: "order_status", Values: []string{"pending", "completed", "cancelled"}},
				},
			},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 2 {
					return false
				}

				// Check first enum
				enum1, ok := nodes[0].(*ast.EnumNode)
				if !ok || enum1.Name != "user_status" || len(enum1.Values) != 2 {
					return false
				}

				// Check second enum
				enum2, ok := nodes[1].(*ast.EnumNode)
				if !ok || enum2.Name != "order_status" || len(enum2.Values) != 3 {
					return false
				}

				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationSQL_EnumsModified(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "enum with values added",
			diff: &types.SchemaDiff{
				EnumsModified: []types.EnumDiff{
					{
						EnumName:    "user_status",
						ValuesAdded: []string{"suspended"},
					},
				},
			},
			generated: &goschema.Database{},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 1 {
					return false
				}
				alterNode, ok := nodes[0].(*ast.AlterTypeNode)
				if !ok {
					return false
				}
				return alterNode.Name == "user_status" && len(alterNode.Operations) == 1
			},
		},
		{
			name: "enum with values removed (should generate warning)",
			diff: &types.SchemaDiff{
				EnumsModified: []types.EnumDiff{
					{
						EnumName:      "user_status",
						ValuesRemoved: []string{"deprecated"},
					},
				},
			},
			generated: &goschema.Database{},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 2 {
					return false
				}

				// First should be ALTER TYPE
				alterNode, ok := nodes[0].(*ast.AlterTypeNode)
				if !ok || alterNode.Name != "user_status" {
					return false
				}

				// Second should be warning comment
				commentNode, ok := nodes[1].(*ast.CommentNode)
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

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationSQL_TablesAdded(t *testing.T) {
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
					{Name: "id", Type: "SERIAL", StructName: "User", Primary: true},
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

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationSQL_TablesModified(t *testing.T) {
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
				Tables: []goschema.Table{
					{Name: "users", StructName: "User"},
				},
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

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationSQL_IndexesAdded(t *testing.T) {
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
		{
			name: "unique index added",
			diff: &types.SchemaDiff{
				IndexesAdded: []string{"uk_users_email"},
			},
			generated: &goschema.Database{
				Indexes: []goschema.Index{
					{Name: "uk_users_email", StructName: "users", Fields: []string{"email"}, Unique: true},
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
				return indexNode.Name == "uk_users_email" && indexNode.Unique
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationSQL_IndexesRemoved(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "single index removed",
			diff: &types.SchemaDiff{
				IndexesRemoved: []string{"idx_old_index"},
			},
			generated: &goschema.Database{},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 1 {
					return false
				}
				dropIndexNode, ok := nodes[0].(*ast.DropIndexNode)
				if !ok {
					return false
				}
				return dropIndexNode.Name == "idx_old_index" && dropIndexNode.IfExists
			},
		},
		{
			name: "multiple indexes removed",
			diff: &types.SchemaDiff{
				IndexesRemoved: []string{"idx_old1", "idx_old2"},
			},
			generated: &goschema.Database{},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 2 {
					return false
				}

				drop1, ok := nodes[0].(*ast.DropIndexNode)
				if !ok || drop1.Name != "idx_old1" {
					return false
				}

				drop2, ok := nodes[1].(*ast.DropIndexNode)
				if !ok || drop2.Name != "idx_old2" {
					return false
				}

				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationSQL_TablesRemoved(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "single table removed",
			diff: &types.SchemaDiff{
				TablesRemoved: []string{"old_table"},
			},
			generated: &goschema.Database{},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 1 {
					return false
				}
				dropTableNode, ok := nodes[0].(*ast.DropTableNode)
				if !ok {
					return false
				}
				return dropTableNode.Name == "old_table" &&
					dropTableNode.IfExists &&
					dropTableNode.Cascade
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationSQL_EnumsRemoved(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "single enum removed",
			diff: &types.SchemaDiff{
				EnumsRemoved: []string{"old_enum"},
			},
			generated: &goschema.Database{},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 1 {
					return false
				}
				dropTypeNode, ok := nodes[0].(*ast.DropTypeNode)
				if !ok {
					return false
				}
				return dropTypeNode.Name == "old_enum" &&
					dropTypeNode.IfExists &&
					dropTypeNode.Cascade
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationSQL_ComplexScenario(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name: "complete migration with all operations",
			diff: &types.SchemaDiff{
				EnumsAdded:     []string{"user_status"},
				TablesAdded:    []string{"users"},
				IndexesAdded:   []string{"idx_users_email"},
				IndexesRemoved: []string{"idx_old"},
			},
			generated: &goschema.Database{
				Enums: []goschema.Enum{
					{Name: "user_status", Values: []string{"active", "inactive"}},
				},
				Tables: []goschema.Table{
					{Name: "users", StructName: "User"},
				},
				Fields: []goschema.Field{
					{Name: "id", Type: "SERIAL", StructName: "User", Primary: true},
					{Name: "email", Type: "VARCHAR(255)", StructName: "User", Nullable: false},
				},
				Indexes: []goschema.Index{
					{Name: "idx_users_email", StructName: "users", Fields: []string{"email"}},
				},
			},
			expected: func(nodes []ast.Node) bool {
				if len(nodes) != 4 {
					return false
				}

				// Should have enum, table, index, drop index in that order
				_, enumOk := nodes[0].(*ast.EnumNode)
				_, tableOk := nodes[1].(*ast.CreateTableNode)
				_, indexOk := nodes[2].(*ast.IndexNode)
				_, dropOk := nodes[3].(*ast.DropIndexNode)

				return enumOk && tableOk && indexOk && dropOk
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}

func TestPlanner_GenerateMigrationSQL_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		diff      *types.SchemaDiff
		generated *goschema.Database
		expected  func(nodes []ast.Node) bool
	}{
		{
			name:      "empty diff should return empty result",
			diff:      &types.SchemaDiff{},
			generated: &goschema.Database{},
			expected: func(nodes []ast.Node) bool {
				return len(nodes) == 0
			},
		},
		{
			name: "enum added but not found in generated schema",
			diff: &types.SchemaDiff{
				EnumsAdded: []string{"missing_enum"},
			},
			generated: &goschema.Database{
				Enums: []goschema.Enum{
					{Name: "other_enum", Values: []string{"value1"}},
				},
			},
			expected: func(nodes []ast.Node) bool {
				return len(nodes) == 0 // Should not generate anything for missing enum
			},
		},
		{
			name: "table added but not found in generated schema",
			diff: &types.SchemaDiff{
				TablesAdded: []string{"missing_table"},
			},
			generated: &goschema.Database{
				Tables: []goschema.Table{
					{Name: "other_table", StructName: "Other"},
				},
			},
			expected: func(nodes []ast.Node) bool {
				return len(nodes) == 0 // Should not generate anything for missing table
			},
		},
		{
			name: "index added but not found in generated schema",
			diff: &types.SchemaDiff{
				IndexesAdded: []string{"missing_index"},
			},
			generated: &goschema.Database{
				Indexes: []goschema.Index{
					{Name: "other_index", StructName: "other_table", Fields: []string{"field"}},
				},
			},
			expected: func(nodes []ast.Node) bool {
				return len(nodes) == 0 // Should not generate anything for missing index
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			planner := &postgres.Planner{}
			nodes := planner.GenerateMigrationAST(tt.diff, tt.generated)

			c.Assert(tt.expected(nodes), qt.IsTrue)
		})
	}
}
