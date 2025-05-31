package renderer_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/renderer"
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
	// Check both options are present (order may vary)
	c.Assert(output, qt.Contains, "ENGINE=InnoDB")
	c.Assert(output, qt.Contains, "CHARSET=utf8mb4")
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
					Expression: "NOW()",
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

func TestMariaDBRenderer_RenderColumnWithEnums_ComprehensivePaths(t *testing.T) {
	tests := []struct {
		name       string
		column     *ast.ColumnNode
		enumValues []string
		expected   []string
	}{
		{
			name: "Non-primary key with auto increment",
			column: &ast.ColumnNode{
				Name:     "sequence_id",
				Type:     "INTEGER",
				Primary:  false,
				AutoInc:  true,
				Nullable: false,
				Unique:   true,
			},
			expected: []string{"sequence_id INTEGER", "NOT NULL", "UNIQUE", "AUTO_INCREMENT"},
		},
		{
			name: "Column with all constraints",
			column: &ast.ColumnNode{
				Name:     "status",
				Type:     "status",
				Primary:  false,
				Nullable: false,
				Unique:   true,
				Check:    "status IN ('active', 'inactive')",
				Comment:  "User status field",
				Default: &ast.DefaultValue{
					Value: "active",
				},
			},
			enumValues: []string{"active", "inactive", "pending"},
			expected: []string{
				"status ENUM('active', 'inactive', 'pending')",
				"NOT NULL",
				"UNIQUE",
				"DEFAULT 'active'",
				"CHECK (status IN ('active', 'inactive'))",
				"COMMENT 'User status field'",
			},
		},
		{
			name: "Primary key with auto increment and comment",
			column: &ast.ColumnNode{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				AutoInc:  true,
				Nullable: false,
				Comment:  "Primary key identifier",
			},
			expected: []string{
				"id INTEGER",
				"PRIMARY KEY",
				"AUTO_INCREMENT",
				"COMMENT 'Primary key identifier'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMariaDBRenderer()

			table := &ast.CreateTableNode{
				Name:    "test_table",
				Columns: []*ast.ColumnNode{tt.column},
			}

			enums := make(map[string][]string)
			if len(tt.enumValues) > 0 {
				enums[tt.column.Type] = tt.enumValues
			}

			err := renderer.VisitCreateTableWithEnums(table, enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}
		})
	}
}

func TestMariaDBRenderer_VisitCreateTableWithEnums_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		table    *ast.CreateTableNode
		enums    map[string][]string
		expected []string
	}{
		{
			name: "Table with comment and constraints",
			table: &ast.CreateTableNode{
				Name:    "users",
				Comment: "User accounts table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
					{Name: "email", Type: "VARCHAR(255)", Nullable: false, Unique: true},
				},
				Constraints: []*ast.ConstraintNode{
					{
						Type:       ast.CheckConstraint,
						Name:       "chk_email_format",
						Expression: "email LIKE '%@%'",
					},
				},
			},
			expected: []string{
				"-- MARIADB TABLE: users (User accounts table) --",
				"CONSTRAINT chk_email_format CHECK (email LIKE '%@%')",
			},
		},
		{
			name: "Table with no options",
			table: &ast.CreateTableNode{
				Name: "simple_table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
				},
				// No Options field set
			},
			expected: []string{
				"-- MARIADB TABLE: simple_table --",
				"CREATE TABLE simple_table (",
				"id INTEGER PRIMARY KEY",
				"\n);", // Should end with just );
			},
		},
		{
			name: "Table with empty options map",
			table: &ast.CreateTableNode{
				Name: "empty_options_table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
				},
				Options: map[string]string{}, // Empty options
			},
			expected: []string{
				"-- MARIADB TABLE: empty_options_table --",
				"id INTEGER PRIMARY KEY",
				"\n);", // Should end with just );
			},
		},
		{
			name: "Table with options that have values",
			table: &ast.CreateTableNode{
				Name: "options_table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
				},
				Options: map[string]string{
					"ENGINE":  "InnoDB",
					"CHARSET": "utf8mb4",
				},
			},
			expected: []string{
				"-- MARIADB TABLE: options_table --",
				"id INTEGER PRIMARY KEY",
				"ENGINE=InnoDB", // Should have options (order may vary)
				"CHARSET=utf8mb4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMariaDBRenderer()
			err := renderer.VisitCreateTableWithEnums(tt.table, tt.enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}
		})
	}
}

func TestMariaDBRenderer_VisitCreateTable_ConstraintRendering(t *testing.T) {
	tests := []struct {
		name     string
		table    *ast.CreateTableNode
		expected []string
	}{
		{
			name: "Table with constraints using regular VisitCreateTable",
			table: &ast.CreateTableNode{
				Name: "users_with_constraints",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
					{Name: "email", Type: "VARCHAR(255)", Nullable: false},
				},
				Constraints: []*ast.ConstraintNode{
					{
						Type:    ast.UniqueConstraint,
						Name:    "uk_users_email",
						Columns: []string{"email"},
					},
					{
						Type:       ast.CheckConstraint,
						Name:       "chk_email_format",
						Expression: "email LIKE '%@%'",
					},
					{
						Type:    ast.ForeignKeyConstraint,
						Name:    "fk_user_profile",
						Columns: []string{"id"},
						Reference: &ast.ForeignKeyRef{
							Table:    "profiles",
							Column:   "user_id",
							OnDelete: "CASCADE",
						},
					},
				},
			},
			expected: []string{
				"-- MARIADB TABLE: users_with_constraints --",
				"CONSTRAINT uk_users_email UNIQUE (email)",
				"CONSTRAINT chk_email_format CHECK (email LIKE '%@%')",
				"CONSTRAINT fk_user_profile FOREIGN KEY (id) REFERENCES profiles(user_id) ON DELETE CASCADE",
			},
		},
		{
			name: "Table with empty constraint that returns empty string",
			table: &ast.CreateTableNode{
				Name: "test_empty_constraint",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
				},
				Constraints: []*ast.ConstraintNode{
					{
						Type:    ast.UniqueConstraint,
						Columns: []string{"email"}, // Valid constraint
					},
				},
			},
			expected: []string{
				"-- MARIADB TABLE: test_empty_constraint --",
				"UNIQUE (email)",
			},
		},
		{
			name: "Table with options that render to empty string",
			table: &ast.CreateTableNode{
				Name: "test_empty_options",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
				},
				Options: map[string]string{
					"EMPTY_OPTION": "", // This will render to "EMPTY_OPTION=" which is not empty
				},
			},
			expected: []string{
				"-- MARIADB TABLE: test_empty_options --",
				"id INTEGER PRIMARY KEY",
				"EMPTY_OPTION=",
			},
		},
		{
			name: "Table with no options (nil map)",
			table: &ast.CreateTableNode{
				Name: "test_no_options",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
				},
				// Options is nil
			},
			expected: []string{
				"-- MARIADB TABLE: test_no_options --",
				"id INTEGER PRIMARY KEY",
				");", // Should end with just );
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMariaDBRenderer()
			// Use the regular VisitCreateTable method, not VisitCreateTableWithEnums
			err := renderer.VisitCreateTable(tt.table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}
		})
	}
}

func TestMariaDBRenderer_VisitCreateTable_OptionsPath(t *testing.T) {
	tests := []struct {
		name        string
		options     map[string]string
		expected    []string
		notExpected []string
	}{
		{
			name: "Options that produce non-empty string",
			options: map[string]string{
				"ENGINE":  "InnoDB",
				"CHARSET": "utf8mb4",
			},
			expected: []string{
				"ENGINE=InnoDB",
				"CHARSET=utf8mb4",
			},
			notExpected: []string{
				"\n);\n", // Should not end with just ); on its own line
			},
		},
		{
			name:    "Empty options map",
			options: map[string]string{},
			expected: []string{
				");", // Should end with just );
			},
			notExpected: []string{
				"ENGINE=",
				"CHARSET=",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMariaDBRenderer()
			table := &ast.CreateTableNode{
				Name: "test_table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
				},
				Options: tt.options,
			}

			err := renderer.VisitCreateTable(table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}
			for _, notExpected := range tt.notExpected {
				c.Assert(output, qt.Not(qt.Contains), notExpected, qt.Commentf("Did not expect %q in output", notExpected))
			}
		})
	}
}
