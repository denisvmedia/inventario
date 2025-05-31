package renderer_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/renderer"
)

func TestBaseRenderer_BasicOperations(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*renderer.BaseRenderer)
		expected string
	}{
		{
			name: "Write",
			setup: func(r *renderer.BaseRenderer) {
				r.Write("Hello")
			},
			expected: "Hello",
		},
		{
			name: "WriteLine",
			setup: func(r *renderer.BaseRenderer) {
				r.WriteLine("Hello")
			},
			expected: "Hello\n",
		},
		{
			name: "Writef",
			setup: func(r *renderer.BaseRenderer) {
				r.Writef("Hello %s", "World")
			},
			expected: "Hello World",
		},
		{
			name: "WriteLinef",
			setup: func(r *renderer.BaseRenderer) {
				r.WriteLinef("Hello %s", "World")
			},
			expected: "Hello World\n",
		},
		{
			name: "Multiple operations",
			setup: func(r *renderer.BaseRenderer) {
				r.Write("Hello")
				r.Write(" ")
				r.WriteLine("World")
				r.WriteLinef("Count: %d", 42)
			},
			expected: "Hello World\nCount: 42\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewBaseRenderer("test")
			tt.setup(renderer)

			c.Assert(renderer.GetOutput(), qt.Equals, tt.expected)
		})
	}
}

func TestBaseRenderer_Reset(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("test")
	renderer.Write("Hello")
	c.Assert(renderer.GetOutput(), qt.Equals, "Hello")

	renderer.Reset()
	c.Assert(renderer.GetOutput(), qt.Equals, "")

	renderer.Write("World")
	c.Assert(renderer.GetOutput(), qt.Equals, "World")
}

func TestBaseRenderer_VisitComment(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("test")
	comment := &ast.CommentNode{Text: "This is a test comment"}

	err := renderer.VisitComment(comment)

	c.Assert(err, qt.IsNil)
	c.Assert(renderer.GetOutput(), qt.Equals, "-- This is a test comment --\n")
}

func TestBaseRenderer_VisitCreateTable_Simple(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("test")
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
				Name:     "name",
				Type:     "VARCHAR(255)",
				Nullable: false,
			},
		},
	}

	err := renderer.VisitCreateTable(table)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "-- TEST TABLE: users --")
	c.Assert(output, qt.Contains, "CREATE TABLE users (")
	c.Assert(output, qt.Contains, "id INTEGER PRIMARY KEY AUTO_INCREMENT")
	c.Assert(output, qt.Contains, "name VARCHAR(255) NOT NULL")
	c.Assert(output, qt.Contains, ");")
}

func TestBaseRenderer_VisitCreateTable_WithComment(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("mysql")
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
	c.Assert(output, qt.Contains, "-- MYSQL TABLE: users (User accounts table) --")
}

func TestBaseRenderer_VisitCreateTable_WithOptions(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("mysql")
	table := &ast.CreateTableNode{
		Name: "users",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				Nullable: false,
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
	// Base renderer doesn't guarantee order, just check both options are present
	c.Assert(output, qt.Contains, "ENGINE=InnoDB")
	c.Assert(output, qt.Contains, "CHARSET=utf8mb4")
}

func TestBaseRenderer_RenderColumn_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		column   *ast.ColumnNode
		expected string
	}{
		{
			name: "Simple column",
			column: &ast.ColumnNode{
				Name:     "name",
				Type:     "VARCHAR(255)",
				Nullable: true,
			},
			expected: "  name VARCHAR(255)",
		},
		{
			name: "Primary key column",
			column: &ast.ColumnNode{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				Nullable: false,
			},
			expected: "  id INTEGER PRIMARY KEY",
		},
		{
			name: "Not null column",
			column: &ast.ColumnNode{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Nullable: false,
			},
			expected: "  email VARCHAR(255) NOT NULL",
		},
		{
			name: "Unique column",
			column: &ast.ColumnNode{
				Name:     "username",
				Type:     "VARCHAR(100)",
				Nullable: false,
				Unique:   true,
			},
			expected: "  username VARCHAR(100) NOT NULL UNIQUE",
		},
		{
			name: "Auto increment column",
			column: &ast.ColumnNode{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				Nullable: false,
				AutoInc:  true,
			},
			expected: "  id INTEGER PRIMARY KEY AUTO_INCREMENT",
		},
		{
			name: "Column with default value",
			column: &ast.ColumnNode{
				Name:     "status",
				Type:     "VARCHAR(20)",
				Nullable: false,
				Default: &ast.DefaultValue{
					Value: "active",
				},
			},
			expected: "  status VARCHAR(20) NOT NULL DEFAULT 'active'",
		},
		{
			name: "Column with default function",
			column: &ast.ColumnNode{
				Name:     "created_at",
				Type:     "TIMESTAMP",
				Nullable: false,
				Default: &ast.DefaultValue{
					Expression: "NOW()",
				},
			},
			expected: "  created_at TIMESTAMP NOT NULL DEFAULT NOW()",
		},
		{
			name: "Column with check constraint",
			column: &ast.ColumnNode{
				Name:     "age",
				Type:     "INTEGER",
				Nullable: false,
				Check:    "age >= 0",
			},
			expected: "  age INTEGER NOT NULL CHECK (age >= 0)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewBaseRenderer("test")

			// Test through VisitCreateTable since renderColumn is private
			table := &ast.CreateTableNode{
				Name:    "test_table",
				Columns: []*ast.ColumnNode{tt.column},
			}

			err := renderer.VisitCreateTable(table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, tt.expected)
		})
	}
}

func TestBaseRenderer_VisitConstraint_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		constraint *ast.ConstraintNode
		expected   string
	}{
		{
			name: "Primary key constraint",
			constraint: &ast.ConstraintNode{
				Type:    ast.PrimaryKeyConstraint,
				Columns: []string{"id"},
			},
			expected: "  PRIMARY KEY (id)",
		},
		{
			name: "Unique constraint with name",
			constraint: &ast.ConstraintNode{
				Type:    ast.UniqueConstraint,
				Name:    "uk_users_email",
				Columns: []string{"email"},
			},
			expected: "  CONSTRAINT uk_users_email UNIQUE (email)",
		},
		{
			name: "Unique constraint without name",
			constraint: &ast.ConstraintNode{
				Type:    ast.UniqueConstraint,
				Columns: []string{"username"},
			},
			expected: "  UNIQUE (username)",
		},
		{
			name: "Check constraint with name",
			constraint: &ast.ConstraintNode{
				Type:       ast.CheckConstraint,
				Name:       "chk_age_positive",
				Expression: "age >= 0",
			},
			expected: "  CONSTRAINT chk_age_positive CHECK (age >= 0)",
		},
		{
			name: "Check constraint without name",
			constraint: &ast.ConstraintNode{
				Type:       ast.CheckConstraint,
				Expression: "status IN ('active', 'inactive')",
			},
			expected: "  CHECK (status IN ('active', 'inactive'))",
		},
		{
			name: "Foreign key constraint",
			constraint: &ast.ConstraintNode{
				Type:    ast.ForeignKeyConstraint,
				Name:    "fk_user_profile",
				Columns: []string{"user_id"},
				Reference: &ast.ForeignKeyRef{
					Table:    "users",
					Column:   "id",
					OnDelete: "CASCADE",
					OnUpdate: "RESTRICT",
				},
			},
			expected: "  CONSTRAINT fk_user_profile FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE RESTRICT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewBaseRenderer("test")

			// Test through VisitCreateTable since renderConstraint is private
			table := &ast.CreateTableNode{
				Name:        "test_table",
				Columns:     []*ast.ColumnNode{{Name: "id", Type: "INTEGER"}},
				Constraints: []*ast.ConstraintNode{tt.constraint},
			}

			err := renderer.VisitCreateTable(table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, tt.expected)
		})
	}
}

func TestBaseRenderer_VisitIndex(t *testing.T) {
	tests := []struct {
		name     string
		index    *ast.IndexNode
		expected string
	}{
		{
			name: "Simple index",
			index: &ast.IndexNode{
				Name:    "idx_users_email",
				Table:   "users",
				Columns: []string{"email"},
			},
			expected: "CREATE INDEX idx_users_email ON users (email);",
		},
		{
			name: "Unique index",
			index: &ast.IndexNode{
				Name:    "idx_users_username",
				Table:   "users",
				Columns: []string{"username"},
				Unique:  true,
			},
			expected: "CREATE UNIQUE INDEX idx_users_username ON users (username);",
		},
		{
			name: "Composite index",
			index: &ast.IndexNode{
				Name:    "idx_users_name_email",
				Table:   "users",
				Columns: []string{"first_name", "last_name", "email"},
			},
			expected: "CREATE INDEX idx_users_name_email ON users (first_name, last_name, email);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewBaseRenderer("test")
			err := renderer.VisitIndex(tt.index)

			c.Assert(err, qt.IsNil)
			c.Assert(renderer.GetOutput(), qt.Equals, tt.expected+"\n")
		})
	}
}

func TestBaseRenderer_VisitAlterTable(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("test")
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
	c.Assert(output, qt.Contains, "ALTER TABLE users ALTER COLUMN email VARCHAR(320) NOT NULL;")
}

func TestBaseRenderer_VisitEnum(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("test")
	enum := &ast.EnumNode{
		Name:   "status",
		Values: []string{"active", "inactive", "pending"},
	}

	err := renderer.VisitEnum(enum)

	c.Assert(err, qt.IsNil)
	// Base renderer should do nothing for enums
	c.Assert(renderer.GetOutput(), qt.Equals, "")
}

func TestBaseRenderer_Render(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("test")
	comment := &ast.CommentNode{Text: "Test comment"}

	output, err := renderer.Render(comment)

	c.Assert(err, qt.IsNil)
	c.Assert(output, qt.Equals, "-- Test comment --\n")
}

func TestBaseRenderer_VisitColumn_VisitConstraint(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("test")

	// These methods should not error but also don't produce output directly
	err := renderer.VisitColumn(&ast.ColumnNode{Name: "test"})
	c.Assert(err, qt.IsNil)

	err = renderer.VisitConstraint(&ast.ConstraintNode{Name: "test"})
	c.Assert(err, qt.IsNil)

	// Output should be empty since these are helper methods
	c.Assert(renderer.GetOutput(), qt.Equals, "")
}

// Unhappy path tests
func TestBaseRenderer_RenderConstraint_UnhappyPath(t *testing.T) {
	tests := []struct {
		name       string
		constraint *ast.ConstraintNode
		expectErr  bool
	}{
		{
			name: "Foreign key without reference",
			constraint: &ast.ConstraintNode{
				Type:    ast.ForeignKeyConstraint,
				Name:    "fk_test",
				Columns: []string{"user_id"},
				// Missing Reference field
			},
			expectErr: true,
		},
		{
			name: "Unknown constraint type",
			constraint: &ast.ConstraintNode{
				Type:    ast.ConstraintType(999), // Invalid type
				Name:    "unknown",
				Columns: []string{"test"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewBaseRenderer("test")
			table := &ast.CreateTableNode{
				Name:        "test_table",
				Columns:     []*ast.ColumnNode{{Name: "id", Type: "INTEGER"}},
				Constraints: []*ast.ConstraintNode{tt.constraint},
			}

			err := renderer.VisitCreateTable(table)

			if tt.expectErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

// Note: Testing unknown alter operation types is not possible from outside the package
// since the alterOperation() method is unexported

func TestBaseRenderer_VisitCreateTable_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		table *ast.CreateTableNode
	}{
		{
			name: "Table with no columns",
			table: &ast.CreateTableNode{
				Name:    "empty_table",
				Columns: []*ast.ColumnNode{},
			},
		},
		{
			name: "Table with only constraints",
			table: &ast.CreateTableNode{
				Name:    "constraint_table",
				Columns: []*ast.ColumnNode{{Name: "id", Type: "INTEGER"}},
				Constraints: []*ast.ConstraintNode{
					{
						Type:       ast.CheckConstraint,
						Name:       "chk_test",
						Expression: "id > 0",
					},
				},
			},
		},
		{
			name: "Table with foreign key column",
			table: &ast.CreateTableNode{
				Name: "fk_table",
				Columns: []*ast.ColumnNode{
					{
						Name:     "user_id",
						Type:     "INTEGER",
						Nullable: false,
						ForeignKey: &ast.ForeignKeyRef{
							Name:     "fk_user",
							Table:    "users",
							Column:   "id",
							OnDelete: "CASCADE",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewBaseRenderer("test")
			err := renderer.VisitCreateTable(tt.table)

			c.Assert(err, qt.IsNil)
			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, "CREATE TABLE "+tt.table.Name)
		})
	}
}

func TestBaseRenderer_VisitAlterTable_EdgeCases(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("test")

	// Test with empty operations
	alterTable := &ast.AlterTableNode{
		Name:       "users",
		Operations: []ast.AlterOperation{},
	}

	err := renderer.VisitAlterTable(alterTable)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "-- ALTER statements: --")
}

func TestBaseRenderer_Render_EdgeCases(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewBaseRenderer("test")

	// Test with a valid comment node
	comment := &ast.CommentNode{Text: "test comment"}
	output, err := renderer.Render(comment)

	c.Assert(err, qt.IsNil)
	c.Assert(output, qt.Equals, "-- test comment --\n")
}

func TestBaseRenderer_ConstraintRendering_ComprehensivePaths(t *testing.T) {
	tests := []struct {
		name       string
		constraint *ast.ConstraintNode
		expected   string
	}{
		{
			name: "Foreign key with all options",
			constraint: &ast.ConstraintNode{
				Type:    ast.ForeignKeyConstraint,
				Name:    "fk_user_profile_complete",
				Columns: []string{"user_id", "profile_id"},
				Reference: &ast.ForeignKeyRef{
					Table:    "users",
					Column:   "id",
					OnDelete: "CASCADE",
					OnUpdate: "RESTRICT",
				},
			},
			expected: "CONSTRAINT fk_user_profile_complete FOREIGN KEY (user_id, profile_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE RESTRICT",
		},
		{
			name: "Foreign key with minimal options",
			constraint: &ast.ConstraintNode{
				Type:    ast.ForeignKeyConstraint,
				Columns: []string{"user_id"},
				Reference: &ast.ForeignKeyRef{
					Table:  "users",
					Column: "id",
				},
			},
			expected: "FOREIGN KEY (user_id) REFERENCES users(id)",
		},
		{
			name: "Multi-column primary key",
			constraint: &ast.ConstraintNode{
				Type:    ast.PrimaryKeyConstraint,
				Columns: []string{"tenant_id", "user_id"},
			},
			expected: "PRIMARY KEY (tenant_id, user_id)",
		},
		{
			name: "Multi-column unique constraint",
			constraint: &ast.ConstraintNode{
				Type:    ast.UniqueConstraint,
				Name:    "uk_email_tenant",
				Columns: []string{"email", "tenant_id"},
			},
			expected: "CONSTRAINT uk_email_tenant UNIQUE (email, tenant_id)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewBaseRenderer("test")

			table := &ast.CreateTableNode{
				Name:        "test_table",
				Columns:     []*ast.ColumnNode{{Name: "id", Type: "INTEGER"}},
				Constraints: []*ast.ConstraintNode{tt.constraint},
			}

			err := renderer.VisitCreateTable(table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, tt.expected)
		})
	}
}

func TestBaseRenderer_ColumnRendering_AllFeatures(t *testing.T) {
	tests := []struct {
		name     string
		column   *ast.ColumnNode
		expected []string
	}{
		{
			name: "Column with all constraints (no foreign key in base renderer)",
			column: &ast.ColumnNode{
				Name:     "user_id",
				Type:     "INTEGER",
				Nullable: false,
				Unique:   true,
				Check:    "user_id > 0",
				Default: &ast.DefaultValue{
					Value: "1",
				},
			},
			expected: []string{
				"user_id INTEGER NOT NULL UNIQUE",
				"DEFAULT '1'",
				"CHECK (user_id > 0)",
			},
		},
		{
			name: "Primary key column with auto increment",
			column: &ast.ColumnNode{
				Name:     "id",
				Type:     "INTEGER",
				Primary:  true,
				AutoInc:  true,
				Nullable: false, // This should be ignored for primary keys
			},
			expected: []string{
				"id INTEGER PRIMARY KEY AUTO_INCREMENT",
			},
		},
		{
			name: "Column with default function",
			column: &ast.ColumnNode{
				Name:     "created_at",
				Type:     "TIMESTAMP",
				Nullable: false,
				Default: &ast.DefaultValue{
					Expression: "NOW()",
				},
			},
			expected: []string{
				"created_at TIMESTAMP NOT NULL DEFAULT NOW()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewBaseRenderer("test")

			table := &ast.CreateTableNode{
				Name:    "test_table",
				Columns: []*ast.ColumnNode{tt.column},
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
