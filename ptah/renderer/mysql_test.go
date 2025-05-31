package renderer_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/renderer"
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

func TestMySQLRenderer_ConvertDefaultExpression(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		expected   string
	}{
		{
			name:       "NOW() to CURRENT_TIMESTAMP",
			expression: "NOW()",
			expected:   "CURRENT_TIMESTAMP",
		},
		{
			name:       "now() to CURRENT_TIMESTAMP (case insensitive)",
			expression: "now()",
			expected:   "CURRENT_TIMESTAMP",
		},
		{
			name:       "Other function unchanged",
			expression: "UUID()",
			expected:   "UUID()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			r := renderer.NewMySQLRenderer()

			// Test through VisitCreateTableWithEnums since convertDefaultExpression is private
			table := &ast.CreateTableNode{
				Name: "test_table",
				Columns: []*ast.ColumnNode{
					{
						Name:     "test_col",
						Type:     "TIMESTAMP",
						Nullable: false,
						Default: &ast.DefaultValue{
							Expression: tt.expression,
						},
					},
				},
			}

			err := r.VisitCreateTableWithEnums(table, nil)
			c.Assert(err, qt.IsNil)

			output := r.GetOutput()
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

			r := renderer.NewMySQLRenderer()

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

			err := r.VisitCreateTableWithEnums(table, nil)
			c.Assert(err, qt.IsNil)

			output := r.GetOutput()
			c.Assert(output, qt.Contains, "DEFAULT "+tt.expected)
		})
	}
}

func TestMySQLRenderer_VisitCreateTable(t *testing.T) {
	tests := []struct {
		name     string
		table    *ast.CreateTableNode
		expected []string
	}{
		{
			name: "Table without comment",
			table: &ast.CreateTableNode{
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
			},
			expected: []string{
				"-- MYSQL TABLE: users --",
				"CREATE TABLE users (",
				"id INTEGER PRIMARY KEY AUTO_INCREMENT",
				"email VARCHAR(255) NOT NULL UNIQUE",
				"); ENGINE=InnoDB charset=utf8mb4",
			},
		},
		{
			name: "Table with comment",
			table: &ast.CreateTableNode{
				Name:    "products",
				Comment: "Product catalog table",
				Columns: []*ast.ColumnNode{
					{
						Name:     "id",
						Type:     "INTEGER",
						Primary:  true,
						Nullable: false,
						AutoInc:  true,
					},
					{
						Name:     "name",
						Type:     "VARCHAR(255)",
						Nullable: false,
					},
				},
				Options: map[string]string{
					"ENGINE": "InnoDB",
				},
			},
			expected: []string{
				"-- MYSQL TABLE: products (Product catalog table) --",
				"CREATE TABLE products (",
				"id INTEGER PRIMARY KEY AUTO_INCREMENT",
				"name VARCHAR(255) NOT NULL",
				"); ENGINE=InnoDB",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()
			err := renderer.VisitCreateTable(tt.table)

			c.Assert(err, qt.IsNil)
			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected)
			}
		})
	}
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
				"ENGINE":        "InnoDB",
				"CUSTOM_OPTION": "value",
			},
			expected: []string{"ENGINE=InnoDB", "CUSTOM_OPTION=value"},
		},
		{
			name: "CHARACTER SET option",
			options: map[string]string{
				"CHARACTER SET": "utf8mb4",
				"ENGINE":        "InnoDB",
			},
			expected: []string{"ENGINE=InnoDB", "CHARACTER SET=utf8mb4"},
		},
		{
			name: "COLLATE option",
			options: map[string]string{
				"COLLATE": "utf8mb4_unicode_ci",
				"ENGINE":  "InnoDB",
			},
			expected: []string{"ENGINE=InnoDB", "COLLATE=utf8mb4_unicode_ci"},
		},
		{
			name: "CHARACTER SET and COLLATE together",
			options: map[string]string{
				"CHARACTER SET": "utf8mb4",
				"COLLATE":       "utf8mb4_unicode_ci",
				"ENGINE":        "InnoDB",
			},
			expected: []string{"ENGINE=InnoDB", "CHARACTER SET=utf8mb4", "COLLATE=utf8mb4_unicode_ci"},
		},
		{
			name: "Case insensitive CHARACTER SET and COLLATE",
			options: map[string]string{
				"character set": "latin1",
				"collate":       "latin1_swedish_ci",
				"engine":        "MyISAM",
			},
			expected: []string{"ENGINE=MyISAM", "CHARACTER SET=latin1", "COLLATE=latin1_swedish_ci"},
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

func TestMySQLRenderer_VisitIndex(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()
	index := &ast.IndexNode{
		Name:    "idx_users_email",
		Table:   "users",
		Columns: []string{"email"},
		Unique:  true,
	}

	err := renderer.VisitIndex(index)

	c.Assert(err, qt.IsNil)
	c.Assert(renderer.GetOutput(), qt.Equals, "CREATE UNIQUE INDEX idx_users_email ON users (email);\n")
}

func TestMySQLRenderer_HelperMethods(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()

	// Test isEnumType and getEnumValues through VisitCreateTableWithEnums
	enums := map[string][]string{
		"status": {"active", "inactive"},
	}

	table := &ast.CreateTableNode{
		Name: "test_table",
		Columns: []*ast.ColumnNode{
			{Name: "status", Type: "status", Nullable: false},
			{Name: "name", Type: "VARCHAR(255)", Nullable: false},
		},
	}

	err := renderer.VisitCreateTableWithEnums(table, enums)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "status ENUM('active', 'inactive')")
	c.Assert(output, qt.Contains, "name VARCHAR(255)")
}

func TestMySQLRenderer_RenderAutoIncrement(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()

	// Test through a column with auto increment
	table := &ast.CreateTableNode{
		Name: "test_table",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				AutoInc:  true,
				Nullable: false,
			},
		},
	}

	err := renderer.VisitCreateTable(table)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "AUTO_INCREMENT")
}

func TestMySQLRenderer_IndirectHelperMethods(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()

	// Test renderAutoIncrement indirectly through renderColumnWithEnums
	table := &ast.CreateTableNode{
		Name: "test_table",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				AutoInc:  true,
				Nullable: false,
			},
		},
	}

	err := renderer.VisitCreateTableWithEnums(table, nil)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "AUTO_INCREMENT")

	// Test isEnumType and getEnumValues indirectly through VisitCreateTableWithEnums
	renderer.Reset()
	enums := map[string][]string{
		"status":   {"active", "inactive"},
		"priority": {"low", "high"},
	}

	enumTable := &ast.CreateTableNode{
		Name: "enum_test_table",
		Columns: []*ast.ColumnNode{
			{Name: "status", Type: "status", Nullable: false},
			{Name: "priority", Type: "priority", Nullable: true},
			{Name: "name", Type: "VARCHAR(255)", Nullable: false}, // Non-enum type
		},
	}

	err = renderer.VisitCreateTableWithEnums(enumTable, enums)
	c.Assert(err, qt.IsNil)

	output = renderer.GetOutput()
	// These should be converted to ENUM types
	c.Assert(output, qt.Contains, "status ENUM('active', 'inactive')")
	c.Assert(output, qt.Contains, "priority ENUM('low', 'high')")
	// This should remain as VARCHAR
	c.Assert(output, qt.Contains, "name VARCHAR(255)")

	// Test with nil enums map
	renderer.Reset()
	err = renderer.VisitCreateTableWithEnums(enumTable, nil)
	c.Assert(err, qt.IsNil)

	output = renderer.GetOutput()
	// Without enum definitions, types should remain unchanged
	c.Assert(output, qt.Contains, "status status")
	c.Assert(output, qt.Contains, "priority priority")
}

func TestMySQLRenderer_VisitCreateTableWithEnums_ComprehensivePaths(t *testing.T) {
	tests := []struct {
		name     string
		table    *ast.CreateTableNode
		enums    map[string][]string
		expected []string
	}{
		{
			name: "Table with comment and enums",
			table: &ast.CreateTableNode{
				Name:    "tasks",
				Comment: "Task management table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true, AutoInc: true},
					{Name: "status", Type: "status", Nullable: false},
					{Name: "priority", Type: "priority", Nullable: true},
				},
				Options: map[string]string{
					"ENGINE":  "InnoDB",
					"CHARSET": "utf8mb4",
				},
			},
			enums: map[string][]string{
				"status":   {"active", "inactive", "pending"},
				"priority": {"low", "medium", "high"},
			},
			expected: []string{
				"-- MYSQL TABLE: tasks (Task management table) --",
				"status ENUM('active', 'inactive', 'pending') NOT NULL",
				"priority ENUM('low', 'medium', 'high')",
				"); ENGINE=InnoDB",
			},
		},
		{
			name: "Table with constraints only (no column foreign keys in base renderer)",
			table: &ast.CreateTableNode{
				Name: "user_profiles",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true, AutoInc: true},
					{Name: "user_id", Type: "INTEGER", Nullable: false},
				},
				Constraints: []*ast.ConstraintNode{
					{
						Type:    ast.UniqueConstraint,
						Name:    "uk_user_profile",
						Columns: []string{"user_id"},
					},
					{
						Type:    ast.ForeignKeyConstraint,
						Name:    "fk_user_profile",
						Columns: []string{"user_id"},
						Reference: &ast.ForeignKeyRef{
							Table:    "users",
							Column:   "id",
							OnDelete: "CASCADE",
						},
					},
				},
			},
			expected: []string{
				"-- MYSQL TABLE: user_profiles --",
				"user_id INTEGER NOT NULL",
				"CONSTRAINT fk_user_profile FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE",
				"CONSTRAINT uk_user_profile UNIQUE (user_id)",
			},
		},
		{
			name: "Table with empty options",
			table: &ast.CreateTableNode{
				Name: "simple_table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "INTEGER", Primary: true},
				},
				Options: map[string]string{}, // Empty options
			},
			expected: []string{
				"-- MYSQL TABLE: simple_table --",
				"id INTEGER PRIMARY KEY",
				"\n);", // Should end with just );
			},
		},
		{
			name: "Table with boolean default values",
			table: &ast.CreateTableNode{
				Name: "settings",
				Columns: []*ast.ColumnNode{
					{
						Name:     "is_active",
						Type:     "BOOLEAN",
						Nullable: false,
						Default: &ast.DefaultValue{
							Value: "true",
						},
					},
					{
						Name:     "is_deleted",
						Type:     "BOOLEAN",
						Nullable: false,
						Default: &ast.DefaultValue{
							Value: "false",
						},
					},
				},
			},
			expected: []string{
				"is_active BOOLEAN NOT NULL DEFAULT TRUE",
				"is_deleted BOOLEAN NOT NULL DEFAULT FALSE",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()
			err := renderer.VisitCreateTableWithEnums(tt.table, tt.enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}
		})
	}
}

func TestMySQLRenderer_VisitCreateTable_ConstraintRendering(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewMySQLRenderer()

	// Test the regular VisitCreateTable method with constraints
	table := &ast.CreateTableNode{
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
		},
	}

	err := renderer.VisitCreateTable(table)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "-- MYSQL TABLE: users_with_constraints --")
	c.Assert(output, qt.Contains, "CONSTRAINT uk_users_email UNIQUE (email)")
	c.Assert(output, qt.Contains, "CONSTRAINT chk_email_format CHECK (email LIKE '%@%')")
}

func TestMySQLRenderer_VisitCreateTable_OptionsPath(t *testing.T) {
	tests := []struct {
		name     string
		options  map[string]string
		expected []string
	}{
		{
			name: "Options with empty values",
			options: map[string]string{
				"ENGINE":       "InnoDB",
				"EMPTY_OPTION": "", // This will render to "EMPTY_OPTION=" which is not empty
			},
			expected: []string{
				"ENGINE=InnoDB",
				"EMPTY_OPTION=",
			},
		},
		{
			name:    "No options",
			options: nil,
			expected: []string{
				");", // Should end with just );
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()
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
		})
	}
}

func TestMySQLRenderer_IsEnumType(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		enums     map[string][]string
		expected  bool
	}{
		{
			name:      "Nil enums map",
			fieldType: "status",
			enums:     nil,
			expected:  false,
		},
		{
			name:      "Empty enums map",
			fieldType: "status",
			enums:     map[string][]string{},
			expected:  false,
		},
		{
			name:      "Field type exists in enums",
			fieldType: "status",
			enums: map[string][]string{
				"status":   {"active", "inactive"},
				"priority": {"low", "high"},
			},
			expected: true,
		},
		{
			name:      "Field type does not exist in enums",
			fieldType: "unknown_type",
			enums: map[string][]string{
				"status":   {"active", "inactive"},
				"priority": {"low", "high"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()

			// Since isEnumType is private, we need to test it indirectly
			// We'll create a table and check if enum types are properly handled
			enums := tt.enums
			table := &ast.CreateTableNode{
				Name: "test_table",
				Columns: []*ast.ColumnNode{
					{Name: "test_field", Type: tt.fieldType, Nullable: false},
				},
			}

			err := renderer.VisitCreateTableWithEnums(table, enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()

			// Check if the field type was converted to ENUM or left as-is
			if tt.expected {
				// Should be converted to ENUM format
				c.Assert(output, qt.Contains, "ENUM(")
			} else {
				// Should remain as original type (not converted to ENUM)
				c.Assert(output, qt.Not(qt.Contains), "ENUM(")
				c.Assert(output, qt.Contains, "test_field "+tt.fieldType)
			}
		})
	}
}

func TestMySQLRenderer_GetEnumValues(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		enums     map[string][]string
		expected  []string
	}{
		{
			name:      "Nil enums map",
			fieldType: "status",
			enums:     nil,
			expected:  nil,
		},
		{
			name:      "Empty enums map",
			fieldType: "status",
			enums:     map[string][]string{},
			expected:  nil,
		},
		{
			name:      "Field type exists in enums",
			fieldType: "status",
			enums: map[string][]string{
				"status":   {"active", "inactive", "pending"},
				"priority": {"low", "high"},
			},
			expected: []string{"active", "inactive", "pending"},
		},
		{
			name:      "Field type does not exist in enums",
			fieldType: "unknown_type",
			enums: map[string][]string{
				"status":   {"active", "inactive"},
				"priority": {"low", "high"},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()

			// Since getEnumValues is private, we need to test it indirectly
			// We'll create a table and check if enum values are properly used
			enums := tt.enums
			table := &ast.CreateTableNode{
				Name: "test_table",
				Columns: []*ast.ColumnNode{
					{Name: "test_field", Type: tt.fieldType, Nullable: false},
				},
			}

			err := renderer.VisitCreateTableWithEnums(table, enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()

			// Check if the enum values are properly rendered
			if tt.expected != nil {
				// Should contain ENUM with the expected values
				c.Assert(output, qt.Contains, "ENUM(")
				for _, value := range tt.expected {
					c.Assert(output, qt.Contains, "'"+value+"'")
				}
			} else {
				// Should not contain ENUM format
				c.Assert(output, qt.Not(qt.Contains), "ENUM(")
			}
		})
	}
}

func TestMySQLRenderer_RenderColumnWithEnums_UniqueConstraint(t *testing.T) {
	tests := []struct {
		name     string
		column   *ast.ColumnNode
		enums    map[string][]string
		expected []string
	}{
		{
			name: "Non-primary column with unique constraint",
			column: &ast.ColumnNode{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Nullable: false,
				Unique:   true,
			},
			enums: nil,
			expected: []string{
				"email VARCHAR(255)",
				"UNIQUE",
				"NOT NULL",
			},
		},
		{
			name: "Unique enum column",
			column: &ast.ColumnNode{
				Name:     "status",
				Type:     "status",
				Nullable: false,
				Unique:   true,
			},
			enums: map[string][]string{
				"status": {"active", "inactive", "pending"},
			},
			expected: []string{
				"status ENUM('active', 'inactive', 'pending')",
				"UNIQUE",
				"NOT NULL",
			},
		},
		{
			name: "Unique nullable column",
			column: &ast.ColumnNode{
				Name:     "username",
				Type:     "VARCHAR(50)",
				Nullable: true,
				Unique:   true,
			},
			enums: nil,
			expected: []string{
				"username VARCHAR(50)",
				"UNIQUE",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()
			table := &ast.CreateTableNode{
				Name:    "test_table",
				Columns: []*ast.ColumnNode{tt.column},
			}

			err := renderer.VisitCreateTableWithEnums(table, tt.enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}
		})
	}
}

func TestMySQLRenderer_RenderColumnWithEnums_CheckConstraint(t *testing.T) {
	tests := []struct {
		name     string
		column   *ast.ColumnNode
		enums    map[string][]string
		expected []string
	}{
		{
			name: "Column with check constraint",
			column: &ast.ColumnNode{
				Name:     "age",
				Type:     "INTEGER",
				Nullable: false,
				Check:    "age >= 0 AND age <= 120",
			},
			enums: nil,
			expected: []string{
				"age INTEGER",
				"NOT NULL",
				"CHECK (age >= 0 AND age <= 120)",
			},
		},
		{
			name: "Unique column with check constraint",
			column: &ast.ColumnNode{
				Name:     "score",
				Type:     "DECIMAL(5,2)",
				Nullable: false,
				Unique:   true,
				Check:    "score >= 0.0 AND score <= 100.0",
			},
			enums: nil,
			expected: []string{
				"score DECIMAL(5,2)",
				"UNIQUE",
				"NOT NULL",
				"CHECK (score >= 0.0 AND score <= 100.0)",
			},
		},
		{
			name: "Enum column with check constraint",
			column: &ast.ColumnNode{
				Name:     "priority",
				Type:     "priority",
				Nullable: false,
				Check:    "priority IN ('low', 'medium', 'high')",
			},
			enums: map[string][]string{
				"priority": {"low", "medium", "high"},
			},
			expected: []string{
				"priority ENUM('low', 'medium', 'high')",
				"NOT NULL",
				"CHECK (priority IN ('low', 'medium', 'high'))",
			},
		},
		{
			name: "Primary key column with check constraint",
			column: &ast.ColumnNode{
				Name:    "id",
				Type:    "INTEGER",
				Primary: true,
				AutoInc: true,
				Check:   "id > 0",
			},
			enums: nil,
			expected: []string{
				"id INTEGER",
				"PRIMARY KEY",
				"AUTO_INCREMENT",
				"CHECK (id > 0)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewMySQLRenderer()
			table := &ast.CreateTableNode{
				Name:    "test_table",
				Columns: []*ast.ColumnNode{tt.column},
			}

			err := renderer.VisitCreateTableWithEnums(table, tt.enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}
		})
	}
}
