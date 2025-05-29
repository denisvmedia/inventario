package renderer_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/renderer"
)

func TestPostgreSQLRenderer_ImplementsVisitorInterface(t *testing.T) {
	c := qt.New(t)

	var _ ast.Visitor = (*renderer.PostgreSQLRenderer)(nil)
	
	renderer := renderer.NewPostgreSQLRenderer()
	c.Assert(renderer, qt.IsNotNil)
}

func TestPostgreSQLRenderer_VisitEnum(t *testing.T) {
	tests := []struct {
		name     string
		enum     *ast.EnumNode
		expected string
	}{
		{
			name: "Simple enum",
			enum: &ast.EnumNode{
				Name:   "status",
				Values: []string{"active", "inactive"},
			},
			expected: "CREATE TYPE status AS ENUM ('active', 'inactive');",
		},
		{
			name: "Enum with multiple values",
			enum: &ast.EnumNode{
				Name:   "priority",
				Values: []string{"low", "medium", "high", "critical"},
			},
			expected: "CREATE TYPE priority AS ENUM ('low', 'medium', 'high', 'critical');",
		},
		{
			name: "Empty enum",
			enum: &ast.EnumNode{
				Name:   "empty_enum",
				Values: []string{},
			},
			expected: "CREATE TYPE empty_enum AS ENUM ();",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			err := renderer.VisitEnum(tt.enum)

			c.Assert(err, qt.IsNil)
			c.Assert(renderer.GetOutput(), qt.Equals, tt.expected+"\n")
		})
	}
}

func TestPostgreSQLRenderer_VisitCreateTable(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()
	table := &ast.CreateTableNode{
		Name: "users",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "SERIAL",
				Primary:  true,
				Nullable: false,
			},
			{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Nullable: false,
				Unique:   true,
			},
		},
	}

	err := renderer.VisitCreateTable(table)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "-- POSTGRES TABLE: users --")
	c.Assert(output, qt.Contains, "CREATE TABLE users (")
	c.Assert(output, qt.Contains, "id SERIAL PRIMARY KEY NOT NULL")
	c.Assert(output, qt.Contains, "email VARCHAR(255) UNIQUE NOT NULL")
	c.Assert(output, qt.Contains, ");")
}

func TestPostgreSQLRenderer_VisitCreateTable_WithForeignKey(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()
	table := &ast.CreateTableNode{
		Name: "profiles",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "SERIAL",
				Primary:  true,
				Nullable: false,
			},
			{
				Name:     "user_id",
				Type:     "INTEGER",
				Nullable: false,
				ForeignKey: &ast.ForeignKeyRef{
					Name:     "fk_profile_user",
					Table:    "users",
					Column:   "id",
					OnDelete: "CASCADE",
					OnUpdate: "RESTRICT",
				},
			},
		},
	}

	err := renderer.VisitCreateTable(table)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "user_id INTEGER NOT NULL")
	c.Assert(output, qt.Contains, "CONSTRAINT fk_profile_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE RESTRICT")
}

func TestPostgreSQLRenderer_VisitAlterTable(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()
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
					Default: &ast.DefaultValue{
						Value: "no-email@example.com",
					},
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
	// PostgreSQL-specific modify column syntax
	c.Assert(output, qt.Contains, "ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(320);")
	c.Assert(output, qt.Contains, "ALTER TABLE users ALTER COLUMN email SET NOT NULL;")
	c.Assert(output, qt.Contains, "ALTER TABLE users ALTER COLUMN email SET DEFAULT 'no-email@example.com';")
}

func TestPostgreSQLRenderer_ProcessFieldType(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		enums     []string
		expected  string
	}{
		{
			name:      "AUTO_INCREMENT to SERIAL",
			fieldType: "AUTO_INCREMENT",
			expected:  "SERIAL",
		},
		{
			name:      "BIGINT AUTO_INCREMENT to BIGSERIAL",
			fieldType: "BIGINT AUTO_INCREMENT",
			expected:  "BIGSERIAL",
		},
		{
			name:      "Enum type",
			fieldType: "status",
			enums:     []string{"status", "priority"},
			expected:  "status",
		},
		{
			name:      "Regular type",
			fieldType: "VARCHAR(255)",
			expected:  "VARCHAR(255)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			
			// Test through column rendering since processFieldType is private
			column := &ast.ColumnNode{
				Name:     "test_col",
				Type:     tt.fieldType,
				Nullable: true,
			}

			table := &ast.CreateTableNode{
				Name:    "test_table",
				Columns: []*ast.ColumnNode{column},
			}

			err := renderer.VisitCreateTable(table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, "test_col "+tt.expected)
		})
	}
}

func TestPostgreSQLRenderer_RenderTableOptions(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()
	table := &ast.CreateTableNode{
		Name: "users",
		Columns: []*ast.ColumnNode{
			{Name: "id", Type: "SERIAL", Primary: true},
		},
		Options: map[string]string{
			"ENGINE":  "InnoDB", // Should be filtered out
			"CHARSET": "utf8mb4",
		},
	}

	err := renderer.VisitCreateTable(table)

	c.Assert(err, qt.IsNil)
	output := renderer.GetOutput()
	// PostgreSQL doesn't render table options at all in this implementation
	c.Assert(output, qt.Not(qt.Contains), "ENGINE=InnoDB")
	c.Assert(output, qt.Not(qt.Contains), "CHARSET=utf8mb4")
}

func TestPostgreSQLRenderer_RenderSchema(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()
	statements := &ast.StatementList{
		Statements: []ast.Node{
			&ast.EnumNode{
				Name:   "status",
				Values: []string{"active", "inactive"},
			},
			&ast.CreateTableNode{
				Name: "users",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "status", Type: "status", Nullable: false},
				},
			},
			&ast.EnumNode{
				Name:   "priority",
				Values: []string{"low", "high"},
			},
		},
	}

	output, err := renderer.RenderSchema(statements)

	c.Assert(err, qt.IsNil)
	// Enums should be rendered first
	c.Assert(output, qt.Contains, "CREATE TYPE status AS ENUM ('active', 'inactive');")
	c.Assert(output, qt.Contains, "CREATE TYPE priority AS ENUM ('low', 'high');")
	// Then tables
	c.Assert(output, qt.Contains, "CREATE TABLE users (")
	
	// Check ordering: enums should come before tables
	statusPos := strings.Index(output, "CREATE TYPE status")
	tablePos := strings.Index(output, "CREATE TABLE users")
	c.Assert(statusPos < tablePos, qt.IsTrue, qt.Commentf("Enums should be rendered before tables"))
}

func TestPostgreSQLRenderer_Render(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()
	enum := &ast.EnumNode{
		Name:   "status",
		Values: []string{"active", "inactive"},
	}

	output, err := renderer.Render(enum)

	c.Assert(err, qt.IsNil)
	c.Assert(output, qt.Equals, "CREATE TYPE status AS ENUM ('active', 'inactive');\n")
}
