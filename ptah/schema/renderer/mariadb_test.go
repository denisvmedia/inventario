package renderer_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/renderer"
)

func TestMariaDBRenderer_ImplementsVisitorInterface(t *testing.T) {
	c := qt.New(t)

	var _ ast.Visitor = (*renderer.MariaDBRenderer)(nil)
	
	renderer := renderer.NewMariaDBRenderer()
	c.Assert(renderer, qt.IsNotNil)
}

func TestMariaDBRenderer_VisitEnum(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMariaDBRenderer()
	enum := &ast.EnumNode{
		Name:   "status",
		Values: []string{"active", "inactive"},
	}

	err := renderer.VisitEnum(enum)

	c.Assert(err, qt.IsNil)
	// MariaDB renderer should do nothing for enum nodes (handled inline like MySQL)
	c.Assert(renderer.GetOutput(), qt.Equals, "")
}

func TestMariaDBRenderer_VisitCreateTable(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMariaDBRenderer()
	table := &ast.CreateTableNode{
		Name: "users",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				Nullable: false,
				AutoInc:  true,
			},
			{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Nullable: false,
				Unique:   true,
			},
		},
		Options: map[string]string{
			"ENGINE":  "InnoDB",
			"CHARSET": "utf8mb4",
		},
	}

	err := renderer.VisitCreateTable(table)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "-- MARIADB TABLE: users --")
	c.Assert(output, qt.Contains, "CREATE TABLE users (")
	c.Assert(output, qt.Contains, "id INTEGER PRIMARY KEY AUTO_INCREMENT")
	c.Assert(output, qt.Contains, "email VARCHAR(255) NOT NULL UNIQUE")
	c.Assert(output, qt.Contains, "); ENGINE=InnoDB CHARSET=utf8mb4")
}

func TestMariaDBRenderer_VisitCreateTable_WithComment(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMariaDBRenderer()
	table := &ast.CreateTableNode{
		Name:    "users",
		Comment: "User accounts table",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				Nullable: false,
			},
		},
	}

	err := renderer.VisitCreateTable(table)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "-- MARIADB TABLE: users (User accounts table) --")
}

func TestMariaDBRenderer_RenderAutoIncrement(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMariaDBRenderer()
	table := &ast.CreateTableNode{
		Name: "users",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				Nullable: false,
				AutoInc:  true,
			},
		},
	}

	err := renderer.VisitCreateTable(table)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "AUTO_INCREMENT")
}

func TestMariaDBRenderer_VisitAlterTable(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMariaDBRenderer()
	alterTable := &ast.AlterTableNode{
		Name: "users",
		Operations: []ast.AlterOperation{
			&ast.AddColumnOperation{
				Column: &ast.ColumnNode{
					Name:     "phone",
					Type:     "VARCHAR(20)",
					Nullable: true,
				},
			},
			&ast.DropColumnOperation{
				ColumnName: "old_field",
			},
			&ast.ModifyColumnOperation{
				Column: &ast.ColumnNode{
					Name:     "email",
					Type:     "VARCHAR(320)",
					Nullable: false,
				},
			},
		},
	}

	err := renderer.VisitAlterTable(alterTable)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "-- ALTER statements: --")
	c.Assert(output, qt.Contains, "ALTER TABLE users ADD COLUMN phone VARCHAR(20);")
	c.Assert(output, qt.Contains, "ALTER TABLE users DROP COLUMN old_field;")
	// MariaDB uses MODIFY COLUMN syntax like MySQL
	c.Assert(output, qt.Contains, "ALTER TABLE users MODIFY COLUMN email VARCHAR(320) NOT NULL;")
}

func TestMariaDBRenderer_RenderColumnWithEnums(t *testing.T) {
	tests := []struct {
		name       string
		column     *ast.ColumnNode
		enumValues []string
		expected   string
	}{
		{
			name: "Regular column",
			column: &ast.ColumnNode{
				Name:     "name",
				Type:     "VARCHAR(255)",
				Nullable: false,
			},
			expected: "name VARCHAR(255) NOT NULL",
		},
		{
			name: "Enum column",
			column: &ast.ColumnNode{
				Name:     "status",
				Type:     "status",
				Nullable: false,
			},
			enumValues: []string{"active", "inactive", "pending"},
			expected:   "status ENUM('active', 'inactive', 'pending') NOT NULL",
		},
		{
			name: "Primary key with auto increment",
			column: &ast.ColumnNode{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				Nullable: false,
				AutoInc:  true,
			},
			expected: "id INTEGER PRIMARY KEY AUTO_INCREMENT",
		},
		{
			name: "Column with comment",
			column: &ast.ColumnNode{
				Name:     "description",
				Type:     "TEXT",
				Nullable: true,
				Comment:  "User description",
			},
			expected: "description TEXT COMMENT 'User description'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMariaDBRenderer()
			
			// Test through VisitCreateTableWithEnums
			enums := make(map[string][]string)
			if len(tt.enumValues) > 0 {
				enums[tt.column.Type] = tt.enumValues
			}

			table := &ast.CreateTableNode{
				Name:    "test_table",
				Columns: []*ast.ColumnNode{tt.column},
			}

			err := renderer.VisitCreateTableWithEnums(table, enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, tt.expected)
		})
	}
}

func TestMariaDBRenderer_RenderSchema(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMariaDBRenderer()
	statements := &ast.StatementList{
		Statements: []ast.Node{
			&ast.EnumNode{
				Name:   "status",
				Values: []string{"active", "inactive"},
			},
			&ast.CreateTableNode{
				Name: "users",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
				},
			},
			&ast.CommentNode{
				Text: "Test comment",
			},
		},
	}

	output, err := renderer.RenderSchema(statements)

	c.Assert(err, qt.IsNil)
	// Enum nodes should be skipped for MariaDB (like MySQL)
	c.Assert(output, qt.Not(qt.Contains), "CREATE TYPE status")
	// Tables and comments should be rendered
	c.Assert(output, qt.Contains, "CREATE TABLE users")
	c.Assert(output, qt.Contains, "-- Test comment --")
}

func TestMariaDBRenderer_VisitCreateTableWithEnums(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMariaDBRenderer()
	enums := map[string][]string{
		"status":   {"active", "inactive", "pending"},
		"priority": {"low", "high"},
	}

	table := &ast.CreateTableNode{
		Name: "tasks",
		Columns: []*ast.ColumnNode{
			{Name: "id", Type: "INTEGER", Primary: true},
			{Name: "status", Type: "status", Nullable: false},
			{Name: "priority", Type: "priority", Nullable: true},
		},
	}

	err := renderer.VisitCreateTableWithEnums(table, enums)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "status ENUM('active', 'inactive', 'pending') NOT NULL")
	c.Assert(output, qt.Contains, "priority ENUM('low', 'high')")
}

func TestMariaDBRenderer_RenderTableOptions(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMariaDBRenderer()
	table := &ast.CreateTableNode{
		Name: "users",
		Columns: []*ast.ColumnNode{
			{Name: "id", Type: "INTEGER", Primary: true},
		},
		Options: map[string]string{
			"ENGINE":  "InnoDB",
			"CHARSET": "utf8mb4",
			"COMMENT": "User table",
		},
	}

	err := renderer.VisitCreateTable(table)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "ENGINE=InnoDB")
	c.Assert(output, qt.Contains, "CHARSET=utf8mb4")
	c.Assert(output, qt.Contains, "COMMENT=User table")
}

func TestMariaDBRenderer_InheritsFromMySQL(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMariaDBRenderer()
	
	// Test that MariaDB renderer inherits MySQL functionality
	// by testing a feature that should work the same way
	table := &ast.CreateTableNode{
		Name: "test_table",
		Columns: []*ast.ColumnNode{
			{
				Name:     "created_at",
				Type:     "TIMESTAMP",
				Nullable: false,
				Default: &ast.DefaultValue{
					Function: "NOW()",
				},
			},
		},
	}

	err := renderer.VisitCreateTable(table)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	// MariaDB uses the base renderer, not MySQL's enhanced conversion
	// So it should keep NOW() as-is
	c.Assert(output, qt.Contains, "DEFAULT NOW()")
}
