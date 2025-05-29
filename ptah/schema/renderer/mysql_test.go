package renderer_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/renderer"
)

func TestMySQLRenderer_ImplementsVisitorInterface(t *testing.T) {
	c := qt.New(t)

	var _ ast.Visitor = (*renderer.MySQLRenderer)(nil)
	
	renderer := renderer.NewMySQLRenderer()
	c.Assert(renderer, qt.IsNotNil)
}

func TestMySQLRenderer_VisitEnum(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()
	enum := &ast.EnumNode{
		Name:   "status",
		Values: []string{"active", "inactive"},
	}

	err := renderer.VisitEnum(enum)

	c.Assert(err, qt.IsNil)
	// MySQL renderer should do nothing for enum nodes (handled inline)
	c.Assert(renderer.GetOutput(), qt.Equals, "")
}

func TestMySQLRenderer_ProcessFieldType(t *testing.T) {
	tests := []struct {
		name       string
		fieldType  string
		enumValues []string
		expected   string
	}{
		{
			name:      "SERIAL to INT",
			fieldType: "SERIAL",
			expected:  "INT",
		},
		{
			name:      "BOOLEAN unchanged",
			fieldType: "BOOLEAN",
			expected:  "BOOLEAN",
		},
		{
			name:      "Regular type unchanged",
			fieldType: "VARCHAR(255)",
			expected:  "VARCHAR(255)",
		},
		{
			name:       "Enum with values",
			fieldType:  "status",
			enumValues: []string{"active", "inactive", "pending"},
			expected:   "ENUM('active', 'inactive', 'pending')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()

			// Test through VisitCreateTableWithEnums since processFieldType is private
			enums := make(map[string][]string)
			if len(tt.enumValues) > 0 {
				enums[tt.fieldType] = tt.enumValues
			}

			table := &ast.CreateTableNode{
				Name: "test_table",
				Columns: []*ast.ColumnNode{
					{
						Name:     "test_col",
						Type:     tt.fieldType,
						Nullable: true,
					},
				},
			}

			err := renderer.VisitCreateTableWithEnums(table, enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, "test_col "+tt.expected)
		})
	}
}

func TestMySQLRenderer_ConvertDefaultFunction(t *testing.T) {
	tests := []struct {
		name     string
		function string
		expected string
	}{
		{
			name:     "NOW() to CURRENT_TIMESTAMP",
			function: "NOW()",
			expected: "CURRENT_TIMESTAMP",
		},
		{
			name:     "now() to CURRENT_TIMESTAMP (case insensitive)",
			function: "now()",
			expected: "CURRENT_TIMESTAMP",
		},
		{
			name:     "Other function unchanged",
			function: "UUID()",
			expected: "UUID()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()

			// Test through VisitCreateTableWithEnums since convertDefaultFunction is private
			table := &ast.CreateTableNode{
				Name: "test_table",
				Columns: []*ast.ColumnNode{
					{
						Name:     "test_col",
						Type:     "TIMESTAMP",
						Nullable: false,
						Default: &ast.DefaultValue{
							Function: tt.function,
						},
					},
				},
			}

			err := renderer.VisitCreateTableWithEnums(table, nil)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, "DEFAULT "+tt.expected)
		})
	}
}

func TestMySQLRenderer_ConvertDefaultValue(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		columnType string
		expected   string
	}{
		{
			name:       "Boolean true",
			value:      "true",
			columnType: "BOOLEAN",
			expected:   "TRUE",
		},
		{
			name:       "Boolean false",
			value:      "false",
			columnType: "BOOLEAN",
			expected:   "FALSE",
		},
		{
			name:       "Boolean 'true' quoted",
			value:      "'true'",
			columnType: "BOOLEAN",
			expected:   "TRUE",
		},
		{
			name:       "Regular value gets quoted",
			value:      "active",
			columnType: "VARCHAR(20)",
			expected:   "'active'",
		},
		{
			name:       "Already quoted value unchanged",
			value:      "'already_quoted'",
			columnType: "VARCHAR(20)",
			expected:   "'already_quoted'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()

			// Test through VisitCreateTableWithEnums since convertDefaultValue is private
			table := &ast.CreateTableNode{
				Name: "test_table",
				Columns: []*ast.ColumnNode{
					{
						Name:     "test_col",
						Type:     tt.columnType,
						Nullable: false,
						Default: &ast.DefaultValue{
							Value: tt.value,
						},
					},
				},
			}

			err := renderer.VisitCreateTableWithEnums(table, nil)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, "DEFAULT "+tt.expected)
		})
	}
}

func TestMySQLRenderer_VisitCreateTable(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()
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
	c.Assert(output, qt.Contains, "-- MYSQL TABLE: users --")
	c.Assert(output, qt.Contains, "CREATE TABLE users (")
	c.Assert(output, qt.Contains, "id INTEGER PRIMARY KEY AUTO_INCREMENT")
	c.Assert(output, qt.Contains, "email VARCHAR(255) NOT NULL UNIQUE")
	c.Assert(output, qt.Contains, "); ENGINE=InnoDB charset=utf8mb4")
}

func TestMySQLRenderer_RenderTableOptions(t *testing.T) {
	tests := []struct {
		name     string
		options  map[string]string
		expected []string
	}{
		{
			name: "ENGINE first",
			options: map[string]string{
				"CHARSET": "utf8mb4",
				"ENGINE":  "InnoDB",
				"COMMENT": "User table",
			},
			expected: []string{"ENGINE=InnoDB", "charset=utf8mb4", "COMMENT='User table'"},
		},
		{
			name: "Case insensitive keys",
			options: map[string]string{
				"engine":  "MyISAM",
				"charset": "latin1",
			},
			expected: []string{"ENGINE=MyISAM", "charset=latin1"},
		},
		{
			name: "Unknown options",
			options: map[string]string{
				"ENGINE":      "InnoDB",
				"CUSTOM_OPTION": "value",
			},
			expected: []string{"ENGINE=InnoDB", "CUSTOM_OPTION=value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()
			table := &ast.CreateTableNode{
				Name:    "test_table",
				Columns: []*ast.ColumnNode{{Name: "id", Type: "INTEGER"}},
				Options: tt.options,
			}

			err := renderer.VisitCreateTable(table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected)
			}
		})
	}
}

func TestMySQLRenderer_VisitAlterTable(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()
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
	// MySQL uses MODIFY COLUMN syntax
	c.Assert(output, qt.Contains, "ALTER TABLE users MODIFY COLUMN email VARCHAR(320) NOT NULL;")
}

func TestMySQLRenderer_RenderSchema(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()
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
	// Enum nodes should be skipped for MySQL
	c.Assert(output, qt.Not(qt.Contains), "CREATE TYPE status")
	// Tables and comments should be rendered
	c.Assert(output, qt.Contains, "CREATE TABLE users")
	c.Assert(output, qt.Contains, "-- Test comment --")
}

func TestMySQLRenderer_VisitCreateTableWithEnums(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()
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
