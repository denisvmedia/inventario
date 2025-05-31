package renderer_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/renderer"
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
	// PostgreSQL should filter out ENGINE but include other options
	c.Assert(output, qt.Not(qt.Contains), "ENGINE=InnoDB")
	c.Assert(output, qt.Contains, "CHARSET=utf8mb4")
}

func TestPostgreSQLRenderer_RenderTableOptions_Comprehensive(t *testing.T) {
	tests := []struct {
		name        string
		options     map[string]string
		contains    []string
		notContains []string
	}{
		{
			name:        "Empty options",
			options:     map[string]string{},
			contains:    []string{");"},
			notContains: []string{"ENGINE=", "CHARSET="},
		},
		{
			name: "ENGINE option only (should be filtered out)",
			options: map[string]string{
				"ENGINE": "InnoDB",
			},
			contains:    []string{");"},
			notContains: []string{"ENGINE=InnoDB"},
		},
		{
			name: "Non-ENGINE options should be included",
			options: map[string]string{
				"CHARSET":    "utf8mb4",
				"TABLESPACE": "pg_default",
			},
			contains:    []string{"CHARSET=utf8mb4", "TABLESPACE=pg_default"},
			notContains: []string{"ENGINE="},
		},
		{
			name: "Mixed options with ENGINE (ENGINE should be filtered)",
			options: map[string]string{
				"ENGINE":     "InnoDB",
				"CHARSET":    "utf8mb4",
				"TABLESPACE": "pg_default",
			},
			contains:    []string{"CHARSET=utf8mb4", "TABLESPACE=pg_default"},
			notContains: []string{"ENGINE=InnoDB"},
		},
		{
			name: "Single non-ENGINE option",
			options: map[string]string{
				"CHARSET": "utf8mb4",
			},
			contains:    []string{"CHARSET=utf8mb4"},
			notContains: []string{"ENGINE="},
		},
		{
			name: "PostgreSQL-specific options",
			options: map[string]string{
				"TABLESPACE": "pg_default",
				"FILLFACTOR": "80",
				"WITH":       "OIDS",
			},
			contains:    []string{"TABLESPACE=pg_default", "FILLFACTOR=80", "WITH=OIDS"},
			notContains: []string{"ENGINE="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			table := &ast.CreateTableNode{
				Name: "test_table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
				},
				Options: tt.options,
			}

			err := renderer.VisitCreateTable(table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()

			// Check that expected content is present
			for _, expected := range tt.contains {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}

			// Check that unwanted content is not present
			for _, notExpected := range tt.notContains {
				c.Assert(output, qt.Not(qt.Contains), notExpected, qt.Commentf("Did not expect %q in output", notExpected))
			}
		})
	}
}

func TestPostgreSQLRenderer_VisitCreateTableWithEnums(t *testing.T) {
	tests := []struct {
		name        string
		table       *ast.CreateTableNode
		enums       []string
		contains    []string
		notContains []string
	}{
		{
			name: "Table with enum columns",
			table: &ast.CreateTableNode{
				Name: "users",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "status", Type: "user_status", Nullable: false},
					{Name: "priority", Type: "task_priority", Nullable: true},
					{Name: "name", Type: "VARCHAR(255)", Nullable: false},
				},
			},
			enums: []string{"user_status", "task_priority"},
			contains: []string{
				"-- POSTGRES TABLE: users --",
				"CREATE TABLE users (",
				"id SERIAL PRIMARY KEY NOT NULL",
				"status user_status NOT NULL",
				"priority task_priority",
				"name VARCHAR(255) NOT NULL",
				");",
			},
			notContains: []string{"ENUM("},
		},
		{
			name: "Table with no enum columns",
			table: &ast.CreateTableNode{
				Name: "products",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "name", Type: "VARCHAR(255)", Nullable: false},
					{Name: "price", Type: "DECIMAL(10,2)", Nullable: false},
				},
			},
			enums: []string{"user_status", "task_priority"},
			contains: []string{
				"-- POSTGRES TABLE: products --",
				"CREATE TABLE products (",
				"id SERIAL PRIMARY KEY NOT NULL",
				"name VARCHAR(255) NOT NULL",
				"price DECIMAL(10,2) NOT NULL",
				");",
			},
			notContains: []string{"user_status", "task_priority"},
		},
		{
			name: "Table with mixed enum and regular columns",
			table: &ast.CreateTableNode{
				Name: "tasks",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "BIGSERIAL", Primary: true},
					{Name: "title", Type: "TEXT", Nullable: false},
					{Name: "status", Type: "task_status", Nullable: false},
					{Name: "created_at", Type: "TIMESTAMP", Nullable: false},
				},
			},
			enums: []string{"task_status"},
			contains: []string{
				"-- POSTGRES TABLE: tasks --",
				"id BIGSERIAL PRIMARY KEY NOT NULL",
				"title TEXT NOT NULL",
				"status task_status NOT NULL",
				"created_at TIMESTAMP NOT NULL",
			},
			notContains: []string{"ENUM("},
		},
		{
			name: "Table with empty enums list",
			table: &ast.CreateTableNode{
				Name: "simple_table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "status", Type: "status", Nullable: false},
				},
			},
			enums: []string{},
			contains: []string{
				"-- POSTGRES TABLE: simple_table --",
				"id SERIAL PRIMARY KEY NOT NULL",
				"status status NOT NULL", // Should remain as-is
			},
			notContains: []string{"ENUM("},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			err := renderer.VisitCreateTableWithEnums(tt.table, tt.enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()

			// Check that expected content is present
			for _, expected := range tt.contains {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}

			// Check that unwanted content is not present
			for _, notExpected := range tt.notContains {
				c.Assert(output, qt.Not(qt.Contains), notExpected, qt.Commentf("Did not expect %q in output", notExpected))
			}
		})
	}
}

func TestPostgreSQLRenderer_VisitCreateTableWithEnums_Advanced(t *testing.T) {
	tests := []struct {
		name        string
		table       *ast.CreateTableNode
		enums       []string
		contains    []string
		notContains []string
	}{
		{
			name: "Table with constraints and enums",
			table: &ast.CreateTableNode{
				Name: "users_with_constraints",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "status", Type: "user_status", Nullable: false},
					{Name: "email", Type: "VARCHAR(255)", Nullable: false, Unique: true},
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
			},
			enums: []string{"user_status"},
			contains: []string{
				"-- POSTGRES TABLE: users_with_constraints --",
				"id SERIAL PRIMARY KEY NOT NULL",
				"status user_status NOT NULL",
				"email VARCHAR(255) UNIQUE NOT NULL",
				"CONSTRAINT uk_users_email UNIQUE (email)",
				"CONSTRAINT chk_email_format CHECK (email LIKE '%@%')",
			},
			notContains: []string{"ENUM("},
		},
		{
			name: "Table with foreign keys and enums",
			table: &ast.CreateTableNode{
				Name: "orders",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "status", Type: "order_status", Nullable: false},
					{
						Name:     "user_id",
						Type:     "INTEGER",
						Nullable: false,
						ForeignKey: &ast.ForeignKeyRef{
							Name:     "fk_orders_user",
							Table:    "users",
							Column:   "id",
							OnDelete: "CASCADE",
							OnUpdate: "RESTRICT",
						},
					},
				},
			},
			enums: []string{"order_status"},
			contains: []string{
				"-- POSTGRES TABLE: orders --",
				"id SERIAL PRIMARY KEY NOT NULL",
				"status order_status NOT NULL",
				"user_id INTEGER NOT NULL",
				"CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE RESTRICT",
			},
			notContains: []string{"ENUM("},
		},
		{
			name: "Table with table options and enums",
			table: &ast.CreateTableNode{
				Name: "products",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "category", Type: "product_category", Nullable: false},
				},
				Options: map[string]string{
					"ENGINE":     "InnoDB", // Should be filtered out
					"TABLESPACE": "pg_default",
					"FILLFACTOR": "80",
				},
			},
			enums: []string{"product_category"},
			contains: []string{
				"-- POSTGRES TABLE: products --",
				"id SERIAL PRIMARY KEY NOT NULL",
				"category product_category NOT NULL",
				"TABLESPACE=pg_default",
				"FILLFACTOR=80",
			},
			notContains: []string{"ENGINE=InnoDB", "ENUM("},
		},
		{
			name: "Table with nil enums",
			table: &ast.CreateTableNode{
				Name: "simple_table",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "status", Type: "status", Nullable: false},
				},
			},
			enums: nil,
			contains: []string{
				"-- POSTGRES TABLE: simple_table --",
				"id SERIAL PRIMARY KEY NOT NULL",
				"status status NOT NULL", // Should remain as-is
			},
			notContains: []string{"ENUM("},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			err := renderer.VisitCreateTableWithEnums(tt.table, tt.enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()

			// Check that expected content is present
			for _, expected := range tt.contains {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}

			// Check that unwanted content is not present
			for _, notExpected := range tt.notContains {
				c.Assert(output, qt.Not(qt.Contains), notExpected, qt.Commentf("Did not expect %q in output", notExpected))
			}
		})
	}
}

func TestPostgreSQLRenderer_VisitCreateTableWithEnums_ColumnFeatures(t *testing.T) {
	tests := []struct {
		name     string
		table    *ast.CreateTableNode
		enums    []string
		contains []string
	}{
		{
			name: "Table with enum columns with all features",
			table: &ast.CreateTableNode{
				Name: "comprehensive_table",
				Columns: []*ast.ColumnNode{
					{
						Name:     "id",
						Type:     "SERIAL",
						Primary:  true,
						Nullable: false,
					},
					{
						Name:     "status",
						Type:     "task_status",
						Nullable: false,
						Unique:   true,
						Default: &ast.DefaultValue{
							Value: "pending",
						},
						Check: "status IN ('pending', 'active', 'completed')",
					},
					{
						Name:     "priority",
						Type:     "priority_level",
						Nullable: true,
						Default: &ast.DefaultValue{
							Expression: "get_default_priority()",
						},
					},
					{
						Name:     "category",
						Type:     "VARCHAR(50)",
						Nullable: false,
						Default: &ast.DefaultValue{
							Value: "general",
						},
					},
				},
			},
			enums: []string{"task_status", "priority_level"},
			contains: []string{
				"-- POSTGRES TABLE: comprehensive_table --",
				"id SERIAL PRIMARY KEY NOT NULL",
				"status task_status UNIQUE NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'completed'))",
				"priority priority_level DEFAULT get_default_priority()",
				"category VARCHAR(50) NOT NULL DEFAULT 'general'",
			},
		},
		{
			name: "Table with AUTO_INCREMENT conversion and enums",
			table: &ast.CreateTableNode{
				Name: "conversion_table",
				Columns: []*ast.ColumnNode{
					{
						Name:     "id",
						Type:     "AUTO_INCREMENT",
						Primary:  true,
						Nullable: false,
					},
					{
						Name:     "big_id",
						Type:     "BIGINT AUTO_INCREMENT",
						Nullable: false,
					},
					{
						Name:     "status",
						Type:     "record_status",
						Nullable: false,
					},
				},
			},
			enums: []string{"record_status"},
			contains: []string{
				"-- POSTGRES TABLE: conversion_table --",
				"id SERIAL PRIMARY KEY NOT NULL",
				"big_id BIGSERIAL NOT NULL",
				"status record_status NOT NULL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			err := renderer.VisitCreateTableWithEnums(tt.table, tt.enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()

			// Check that expected content is present
			for _, expected := range tt.contains {
				c.Assert(output, qt.Contains, expected, qt.Commentf("Expected %q in output", expected))
			}
		})
	}
}

func TestPostgreSQLRenderer_VisitCreateTableWithEnums_TableComments(t *testing.T) {
	tests := []struct {
		name     string
		table    *ast.CreateTableNode
		enums    []string
		expected string
	}{
		{
			name: "Table with comment",
			table: &ast.CreateTableNode{
				Name:    "users",
				Comment: "User accounts and profiles",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "status", Type: "user_status", Nullable: false},
				},
			},
			enums:    []string{"user_status"},
			expected: "-- POSTGRES TABLE: users (User accounts and profiles) --",
		},
		{
			name: "Table without comment",
			table: &ast.CreateTableNode{
				Name: "products",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "category", Type: "product_category", Nullable: false},
				},
			},
			enums:    []string{"product_category"},
			expected: "-- POSTGRES TABLE: products --",
		},
		{
			name: "Table with empty comment",
			table: &ast.CreateTableNode{
				Name:    "orders",
				Comment: "",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "status", Type: "order_status", Nullable: false},
				},
			},
			enums:    []string{"order_status"},
			expected: "-- POSTGRES TABLE: orders --",
		},
		{
			name: "Table with special characters in comment",
			table: &ast.CreateTableNode{
				Name:    "logs",
				Comment: "System logs & audit trail (2024)",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "level", Type: "log_level", Nullable: false},
				},
			},
			enums:    []string{"log_level"},
			expected: "-- POSTGRES TABLE: logs (System logs & audit trail (2024)) --",
		},
		{
			name: "Table with multiword comment",
			table: &ast.CreateTableNode{
				Name:    "task_assignments",
				Comment: "Task assignment tracking for project management",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "priority", Type: "task_priority", Nullable: false},
				},
			},
			enums:    []string{"task_priority"},
			expected: "-- POSTGRES TABLE: task_assignments (Task assignment tracking for project management) --",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			err := renderer.VisitCreateTableWithEnums(tt.table, tt.enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, tt.expected, qt.Commentf("Expected table comment %q in output", tt.expected))

			// Also verify that the table comment appears at the beginning
			lines := strings.Split(strings.TrimSpace(output), "\n")
			c.Assert(len(lines), qt.Not(qt.Equals), 0)
			c.Assert(lines[0], qt.Equals, tt.expected, qt.Commentf("Table comment should be the first line"))
		})
	}
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

func TestPostgreSQLRenderer_VisitMethods(t *testing.T) {
	tests := []struct {
		name   string
		test   func(*renderer.PostgreSQLRenderer) error
		output string
	}{
		{
			name: "VisitColumn",
			test: func(r *renderer.PostgreSQLRenderer) error {
				return r.VisitColumn(&ast.ColumnNode{Name: "test"})
			},
			output: "",
		},
		{
			name: "VisitConstraint",
			test: func(r *renderer.PostgreSQLRenderer) error {
				return r.VisitConstraint(&ast.ConstraintNode{Name: "test"})
			},
			output: "",
		},
		{
			name: "VisitIndex",
			test: func(r *renderer.PostgreSQLRenderer) error {
				return r.VisitIndex(&ast.IndexNode{Name: "test", Table: "table", Columns: []string{"col"}})
			},
			output: "CREATE INDEX test ON table (col);\n",
		},
		{
			name: "VisitComment",
			test: func(r *renderer.PostgreSQLRenderer) error {
				return r.VisitComment(&ast.CommentNode{Text: "test"})
			},
			output: "-- test --\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			err := tt.test(renderer)
			c.Assert(err, qt.IsNil)
			c.Assert(renderer.GetOutput(), qt.Equals, tt.output)
		})
	}
}

func TestPostgreSQLRenderer_VisitDropTable(t *testing.T) {
	tests := []struct {
		name     string
		node     *ast.DropTableNode
		expected string
	}{
		{
			name: "Basic DROP TABLE",
			node: ast.NewDropTable("users"),
			expected: "DROP TABLE users;\n",
		},
		{
			name: "DROP TABLE IF EXISTS",
			node: ast.NewDropTable("users").SetIfExists(),
			expected: "DROP TABLE IF EXISTS users;\n",
		},
		{
			name: "DROP TABLE CASCADE",
			node: ast.NewDropTable("users").SetCascade(),
			expected: "DROP TABLE users CASCADE;\n",
		},
		{
			name: "DROP TABLE with all options",
			node: ast.NewDropTable("users").SetIfExists().SetCascade().SetComment("Dangerous operation"),
			expected: "-- Dangerous operation\nDROP TABLE IF EXISTS users CASCADE;\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			err := renderer.VisitDropTable(tt.node)

			c.Assert(err, qt.IsNil)
			c.Assert(renderer.GetOutput(), qt.Equals, tt.expected)
		})
	}
}

func TestPostgreSQLRenderer_VisitDropType(t *testing.T) {
	tests := []struct {
		name     string
		node     *ast.DropTypeNode
		expected string
	}{
		{
			name: "Basic DROP TYPE",
			node: ast.NewDropType("status_enum"),
			expected: "DROP TYPE status_enum;\n",
		},
		{
			name: "DROP TYPE IF EXISTS",
			node: ast.NewDropType("status_enum").SetIfExists(),
			expected: "DROP TYPE IF EXISTS status_enum;\n",
		},
		{
			name: "DROP TYPE CASCADE",
			node: ast.NewDropType("status_enum").SetCascade(),
			expected: "DROP TYPE status_enum CASCADE;\n",
		},
		{
			name: "DROP TYPE with all options",
			node: ast.NewDropType("status_enum").SetIfExists().SetCascade().SetComment("Remove unused enum"),
			expected: "-- Remove unused enum\nDROP TYPE IF EXISTS status_enum CASCADE;\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
			err := renderer.VisitDropType(tt.node)

			c.Assert(err, qt.IsNil)
			c.Assert(renderer.GetOutput(), qt.Equals, tt.expected)
		})
	}
}

func TestPostgreSQLRenderer_RenderAutoIncrement(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()

	// Test through a column with auto increment
	table := &ast.CreateTableNode{
		Name: "test_table",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "SERIAL",
				Primary:  true,
				AutoInc:  true,
				Nullable: false,
			},
		},
	}

	err := renderer.VisitCreateTable(table)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	// PostgreSQL uses SERIAL type, not AUTO_INCREMENT
	c.Assert(output, qt.Contains, "id SERIAL PRIMARY KEY")
	c.Assert(output, qt.Not(qt.Contains), "AUTO_INCREMENT")
}

func TestPostgreSQLRenderer_IsEnumType(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()

	// Test through processFieldType which calls isEnumType
	table := &ast.CreateTableNode{
		Name: "test_table",
		Columns: []*ast.ColumnNode{
			{Name: "status", Type: "status", Nullable: false},
			{Name: "name", Type: "VARCHAR(255)", Nullable: false},
		},
	}

	err := renderer.VisitCreateTable(table)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	// Both should be rendered as-is since we don't have enum definitions
	c.Assert(output, qt.Contains, "status status")
	c.Assert(output, qt.Contains, "name VARCHAR(255)")
}

func TestPostgreSQLRenderer_ProcessFieldType_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
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
			name:      "Regular type unchanged",
			fieldType: "TEXT",
			expected:  "TEXT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()

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

			err := renderer.VisitCreateTable(table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, "test_col "+tt.expected)
		})
	}
}

func TestPostgreSQLRenderer_IndirectHelperMethods(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()

	// Test renderAutoIncrement indirectly - PostgreSQL doesn't use AUTO_INCREMENT
	// It's handled by SERIAL types, so renderAutoIncrement should return empty string
	// We can test this by checking that AUTO_INCREMENT doesn't appear in output
	table := &ast.CreateTableNode{
		Name: "test_table",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "SERIAL",
				Primary:  true,
				AutoInc:  true,
				Nullable: false,
			},
		},
	}

	err := renderer.VisitCreateTable(table)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	c.Assert(output, qt.Not(qt.Contains), "AUTO_INCREMENT")
	c.Assert(output, qt.Contains, "SERIAL")

	// Test renderTableOptions indirectly - PostgreSQL VisitCreateTable doesn't call it
	// But we can test the logic by creating a custom scenario
	// Since renderTableOptions is private, we test the behavior through processFieldType

	// Test isEnumType indirectly through processFieldType
	renderer.Reset()
	enumTable := &ast.CreateTableNode{
		Name: "enum_test_table",
		Columns: []*ast.ColumnNode{
			{Name: "status", Type: "status", Nullable: false},
			{Name: "name", Type: "VARCHAR(255)", Nullable: false},
		},
	}

	err = renderer.VisitCreateTable(enumTable)
	c.Assert(err, qt.IsNil)

	output = renderer.GetOutput()
	// Without enum definitions, types should remain unchanged
	c.Assert(output, qt.Contains, "status status")
	c.Assert(output, qt.Contains, "name VARCHAR(255)")
}

func TestPostgreSQLRenderer_RenderPostgreSQLModifyColumn(t *testing.T) {
	tests := []struct {
		name     string
		column   *ast.ColumnNode
		expected []string
	}{
		{
			name: "Column with default value",
			column: &ast.ColumnNode{
				Name:     "email",
				Type:     "VARCHAR(320)",
				Nullable: false,
				Default: &ast.DefaultValue{
					Value: "no-email@example.com",
				},
			},
			expected: []string{
				"ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(320);",
				"ALTER TABLE users ALTER COLUMN email SET NOT NULL;",
				"ALTER TABLE users ALTER COLUMN email SET DEFAULT 'no-email@example.com';",
			},
		},
		{
			name: "Column with default function",
			column: &ast.ColumnNode{
				Name:     "created_at",
				Type:     "TIMESTAMP",
				Nullable: true,
				Default: &ast.DefaultValue{
					Expression: "NOW()",
				},
			},
			expected: []string{
				"ALTER TABLE users ALTER COLUMN created_at TYPE TIMESTAMP;",
				"ALTER TABLE users ALTER COLUMN created_at DROP NOT NULL;",
				"ALTER TABLE users ALTER COLUMN created_at SET DEFAULT NOW();",
			},
		},
		{
			name: "Column without default",
			column: &ast.ColumnNode{
				Name:     "description",
				Type:     "TEXT",
				Nullable: true,
			},
			expected: []string{
				"ALTER TABLE users ALTER COLUMN description TYPE TEXT;",
				"ALTER TABLE users ALTER COLUMN description DROP NOT NULL;",
				"ALTER TABLE users ALTER COLUMN description DROP DEFAULT;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()

			// Test renderPostgreSQLModifyColumn indirectly through VisitAlterTable
			alterTable := &ast.AlterTableNode{
				Name: "users",
				Operations: []ast.AlterOperation{
					&ast.ModifyColumnOperation{
						Column: tt.column,
					},
				},
			}

			err := renderer.VisitAlterTable(alterTable)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			for _, expected := range tt.expected {
				c.Assert(output, qt.Contains, expected)
			}
		})
	}
}

func TestPostgreSQLRenderer_ComprehensiveColumnRendering(t *testing.T) {
	tests := []struct {
		name     string
		column   *ast.ColumnNode
		expected []string
	}{
		{
			name: "Column with all PostgreSQL features",
			column: &ast.ColumnNode{
				Name:     "user_data",
				Type:     "JSONB",
				Nullable: false,
				Unique:   true,
				Check:    "user_data IS NOT NULL",
				Default: &ast.DefaultValue{
					Value: "{}",
				},
				ForeignKey: &ast.ForeignKeyRef{
					Name:     "fk_user_data",
					Table:    "users",
					Column:   "id",
					OnDelete: "SET NULL",
					OnUpdate: "CASCADE",
				},
			},
			expected: []string{
				"user_data JSONB UNIQUE NOT NULL",
				"DEFAULT '{}'",
				"CHECK (user_data IS NOT NULL)",
				"CONSTRAINT fk_user_data FOREIGN KEY (user_data) REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE",
			},
		},
		{
			name: "BIGSERIAL column",
			column: &ast.ColumnNode{
				Name:     "big_id",
				Type:     "BIGINT AUTO_INCREMENT",
				Primary:  true,
				Nullable: false,
			},
			expected: []string{
				"big_id BIGSERIAL PRIMARY KEY",
			},
		},
		{
			name: "Column with default function",
			column: &ast.ColumnNode{
				Name:     "created_at",
				Type:     "TIMESTAMP",
				Nullable: false,
				Default: &ast.DefaultValue{
					Expression: "CURRENT_TIMESTAMP",
				},
			},
			expected: []string{
				"created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()
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

func TestPostgreSQLRenderer_RenderSchema_ComprehensiveOrdering(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()
	statements := &ast.StatementList{
		Statements: []ast.Node{
			&ast.CreateTableNode{
				Name: "users",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "status", Type: "user_status", Nullable: false},
				},
			},
			&ast.EnumNode{
				Name:   "user_status",
				Values: []string{"active", "inactive", "pending"},
			},
			&ast.CreateTableNode{
				Name: "profiles",
				Columns: []*ast.ColumnNode{
					{Name: "id", Type: "SERIAL", Primary: true},
					{Name: "priority", Type: "task_priority", Nullable: true},
				},
			},
			&ast.EnumNode{
				Name:   "task_priority",
				Values: []string{"low", "high"},
			},
			&ast.CommentNode{
				Text: "Schema complete",
			},
		},
	}

	output, err := renderer.RenderSchema(statements)

	c.Assert(err, qt.IsNil)

	// Check that enums are rendered first
	userStatusPos := strings.Index(output, "CREATE TYPE user_status")
	taskPriorityPos := strings.Index(output, "CREATE TYPE task_priority")
	usersTablePos := strings.Index(output, "CREATE TABLE users")
	profilesTablePos := strings.Index(output, "CREATE TABLE profiles")
	commentPos := strings.Index(output, "-- Schema complete --")

	// Enums should come before tables
	c.Assert(userStatusPos < usersTablePos, qt.IsTrue, qt.Commentf("user_status enum should come before users table"))
	c.Assert(taskPriorityPos < profilesTablePos, qt.IsTrue, qt.Commentf("task_priority enum should come before profiles table"))

	// Comment should be at the end
	c.Assert(commentPos > usersTablePos, qt.IsTrue, qt.Commentf("Comment should come after tables"))
	c.Assert(commentPos > profilesTablePos, qt.IsTrue, qt.Commentf("Comment should come after tables"))
}

func TestPostgreSQLRenderer_VisitCreateTable_ConstraintRendering(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()

	// Test the regular VisitCreateTable method with constraints
	table := &ast.CreateTableNode{
		Name: "users_with_constraints",
		Columns: []*ast.ColumnNode{
			{Name: "id", Type: "SERIAL", Primary: true},
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
					OnDelete: "SET NULL",
					OnUpdate: "CASCADE",
				},
			},
		},
	}

	err := renderer.VisitCreateTable(table)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	c.Assert(output, qt.Contains, "-- POSTGRES TABLE: users_with_constraints --")
	c.Assert(output, qt.Contains, "CONSTRAINT uk_users_email UNIQUE (email)")
	c.Assert(output, qt.Contains, "CONSTRAINT chk_email_format CHECK (email LIKE '%@%')")
	c.Assert(output, qt.Contains, "CONSTRAINT fk_user_profile FOREIGN KEY (id) REFERENCES profiles(user_id) ON DELETE SET NULL ON UPDATE CASCADE")
}

func TestPostgreSQLRenderer_VisitCreateTable_OptionsHandling(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()

	// PostgreSQL should render table options but filter out MySQL-specific ones like ENGINE
	table := &ast.CreateTableNode{
		Name: "test_table",
		Columns: []*ast.ColumnNode{
			{Name: "id", Type: "SERIAL", Primary: true},
		},
		Options: map[string]string{
			"ENGINE":  "InnoDB", // Should be ignored
			"CHARSET": "utf8mb4",
		},
	}

	err := renderer.VisitCreateTable(table)
	c.Assert(err, qt.IsNil)

	output := renderer.GetOutput()
	// PostgreSQL should filter out ENGINE but include other options
	c.Assert(output, qt.Contains, "-- POSTGRES TABLE: test_table --")
	c.Assert(output, qt.Contains, "id SERIAL PRIMARY KEY")
	c.Assert(output, qt.Not(qt.Contains), "ENGINE")  // ENGINE should be filtered out
	c.Assert(output, qt.Contains, "CHARSET=utf8mb4") // CHARSET should be included
}

func TestPostgreSQLRenderer_ProcessFieldType_MissingPaths(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		enums     []string
		expected  string
	}{
		{
			name:      "Enum type with enum list",
			fieldType: "user_status",
			enums:     []string{"user_status", "priority"},
			expected:  "user_status",
		},
		{
			name:      "Non-enum type with enum list",
			fieldType: "VARCHAR(255)",
			enums:     []string{"user_status", "priority"},
			expected:  "VARCHAR(255)",
		},
		{
			name:      "Type with empty enum list",
			fieldType: "user_status",
			enums:     []string{},
			expected:  "user_status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()

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

			err := renderer.VisitCreateTable(table)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			c.Assert(output, qt.Contains, "test_col "+tt.expected)
		})
	}
}

func TestPostgreSQLRenderer_IsEnumType_DirectTesting(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		enums     []string
		expected  string
	}{
		{
			name:      "Field type matches enum in list",
			fieldType: "user_status",
			enums:     []string{"user_status", "priority"},
			expected:  "user_status", // Should return the enum type directly
		},
		{
			name:      "Field type matches first enum",
			fieldType: "priority",
			enums:     []string{"priority", "user_status"},
			expected:  "priority",
		},
		{
			name:      "Field type does not match any enum",
			fieldType: "VARCHAR(255)",
			enums:     []string{"user_status", "priority"},
			expected:  "VARCHAR(255)", // Should return original type
		},
		{
			name:      "Field type with empty enum list",
			fieldType: "user_status",
			enums:     []string{},
			expected:  "user_status", // Should return original type
		},
		{
			name:      "Field type with nil enum list",
			fieldType: "user_status",
			enums:     nil,
			expected:  "user_status", // Should return original type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer := renderer.NewPostgreSQLRenderer()

			// Create a table that would use the enum type
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

			// Use VisitCreateTableWithEnums to test isEnumType method
			err := renderer.VisitCreateTableWithEnums(table, tt.enums)
			c.Assert(err, qt.IsNil)

			output := renderer.GetOutput()
			// The enum checking should now happen through isEnumType
			c.Assert(output, qt.Contains, "test_col "+tt.expected)
		})
	}
}

func TestPostgreSQLRenderer_NeedsQuotedDefault(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.NewPostgreSQLRenderer()

	// Test numeric types (should not need quotes)
	c.Assert(renderer.NeedsQuotedDefault("INTEGER"), qt.IsFalse)
	c.Assert(renderer.NeedsQuotedDefault("BIGINT"), qt.IsFalse)
	c.Assert(renderer.NeedsQuotedDefault("SERIAL"), qt.IsFalse)
	c.Assert(renderer.NeedsQuotedDefault("DECIMAL(10,2)"), qt.IsFalse)
	c.Assert(renderer.NeedsQuotedDefault("BOOLEAN"), qt.IsFalse)
	c.Assert(renderer.NeedsQuotedDefault("FLOAT"), qt.IsFalse)
	c.Assert(renderer.NeedsQuotedDefault("DOUBLE PRECISION"), qt.IsFalse)

	// Test string types (should need quotes)
	c.Assert(renderer.NeedsQuotedDefault("VARCHAR(255)"), qt.IsTrue)
	c.Assert(renderer.NeedsQuotedDefault("TEXT"), qt.IsTrue)
	c.Assert(renderer.NeedsQuotedDefault("CHAR(10)"), qt.IsTrue)

	// Test date/time types (should need quotes)
	c.Assert(renderer.NeedsQuotedDefault("TIMESTAMP"), qt.IsTrue)
	c.Assert(renderer.NeedsQuotedDefault("DATE"), qt.IsTrue)
	c.Assert(renderer.NeedsQuotedDefault("TIME"), qt.IsTrue)
	c.Assert(renderer.NeedsQuotedDefault("TIMESTAMP WITH TIME ZONE"), qt.IsTrue)

	// Test enum types (should need quotes)
	renderer.CurrentEnums = []string{"user_status", "priority_level"}
	c.Assert(renderer.NeedsQuotedDefault("user_status"), qt.IsTrue)
	c.Assert(renderer.NeedsQuotedDefault("priority_level"), qt.IsTrue)
	c.Assert(renderer.NeedsQuotedDefault("unknown_enum"), qt.IsTrue) // Unknown types default to quoted
}
