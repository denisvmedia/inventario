package renderer_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/renderer"
)

func TestSupportedDialects(t *testing.T) {
	c := qt.New(t)

	dialects := renderer.SupportedDialects()
	expected := []string{"postgresql", "postgres", "mysql", "mariadb"}

	c.Assert(dialects, qt.DeepEquals, expected)
}

func TestNewRenderer_SupportedDialects(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		expected string
	}{
		{
			name:     "PostgreSQL",
			dialect:  "postgresql",
			expected: "postgres",
		},
		{
			name:     "Postgres alias",
			dialect:  "postgres",
			expected: "postgres",
		},
		{
			name:     "MySQL",
			dialect:  "mysql",
			expected: "mysql",
		},
		{
			name:     "MariaDB",
			dialect:  "mariadb",
			expected: "mariadb",
		},
		{
			name:     "Case insensitive PostgreSQL",
			dialect:  "POSTGRESQL",
			expected: "postgres",
		},
		{
			name:     "Case insensitive MySQL",
			dialect:  "MySQL",
			expected: "mysql",
		},
		{
			name:     "Whitespace handling",
			dialect:  "  postgresql  ",
			expected: "postgres",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer, err := renderer.NewRenderer(tt.dialect)
			c.Assert(err, qt.IsNil)
			c.Assert(renderer, qt.IsNotNil)
			c.Assert(renderer.GetDialect(), qt.Equals, tt.expected)
		})
	}
}

func TestNewRenderer_UnsupportedDialects(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
	}{
		{
			name:    "SQLite",
			dialect: "sqlite",
		},
		{
			name:    "Oracle",
			dialect: "oracle",
		},
		{
			name:    "SQL Server",
			dialect: "sqlserver",
		},
		{
			name:    "Empty string",
			dialect: "",
		},
		{
			name:    "Random string",
			dialect: "random",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			renderer, err := renderer.NewRenderer(tt.dialect)
			c.Assert(err, qt.IsNotNil)
			c.Assert(renderer, qt.IsNil)
			c.Assert(err.Error(), qt.Contains, "unsupported database dialect")
			c.Assert(err.Error(), qt.Contains, tt.dialect)
		})
	}
}

func TestMustNewRenderer_Success(t *testing.T) {
	c := qt.New(t)

	renderer := renderer.MustNewRenderer("postgresql")
	c.Assert(renderer, qt.IsNotNil)
	c.Assert(renderer.GetDialect(), qt.Equals, "postgres")
}

func TestMustNewRenderer_Panic(t *testing.T) {
	c := qt.New(t)

	c.Assert(func() {
		renderer.MustNewRenderer("unsupported")
	}, qt.PanicMatches, "failed to create renderer: .*")
}

func TestValidateDialect(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		expected bool
	}{
		{
			name:     "Valid PostgreSQL",
			dialect:  "postgresql",
			expected: true,
		},
		{
			name:     "Valid MySQL",
			dialect:  "mysql",
			expected: true,
		},
		{
			name:     "Valid MariaDB",
			dialect:  "mariadb",
			expected: true,
		},
		{
			name:     "Invalid SQLite",
			dialect:  "sqlite",
			expected: false,
		},
		{
			name:     "Invalid empty",
			dialect:  "",
			expected: false,
		},
		{
			name:     "Case insensitive valid",
			dialect:  "MYSQL",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := renderer.ValidateDialect(tt.dialect)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestRenderSQL_Success(t *testing.T) {
	c := qt.New(t)

	// Create a simple comment node for testing
	comment := &ast.CommentNode{Text: "Test comment"}

	sql, err := renderer.RenderSQL("postgresql", comment)
	c.Assert(err, qt.IsNil)
	c.Assert(sql, qt.Contains, "Test comment")
}

func TestRenderSQL_UnsupportedDialect(t *testing.T) {
	c := qt.New(t)

	comment := &ast.CommentNode{Text: "Test comment"}

	sql, err := renderer.RenderSQL("unsupported", comment)
	c.Assert(err, qt.IsNotNil)
	c.Assert(sql, qt.Equals, "")
	c.Assert(err.Error(), qt.Contains, "unsupported database dialect")
}

func TestRenderer_Interface(t *testing.T) {
	// Test that all dialect renderers implement the Renderer interface
	dialects := []string{"postgresql", "mysql", "mariadb"}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			c := qt.New(t)

			r, err := renderer.NewRenderer(dialect)
			c.Assert(err, qt.IsNil)

			// Test interface methods
			c.Assert(r.GetDialect(), qt.IsNotNil)
			c.Assert(r.GetOutput(), qt.Equals, "")

			// Test Reset
			r.Reset()
			c.Assert(r.GetOutput(), qt.Equals, "")

			// Test Render with a simple node
			comment := &ast.CommentNode{Text: "Test"}
			sql, err := r.Render(comment)
			c.Assert(err, qt.IsNil)
			c.Assert(sql, qt.IsNotNil)
		})
	}
}

func TestRenderer_BasicRendering(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		node     ast.Node
		contains []string
	}{
		{
			name:     "PostgreSQL comment",
			dialect:  "postgresql",
			node:     &ast.CommentNode{Text: "PostgreSQL comment"},
			contains: []string{"PostgreSQL comment"},
		},
		{
			name:     "MySQL comment",
			dialect:  "mysql",
			node:     &ast.CommentNode{Text: "MySQL comment"},
			contains: []string{"MySQL comment"},
		},
		{
			name:     "MariaDB comment",
			dialect:  "mariadb",
			node:     &ast.CommentNode{Text: "MariaDB comment"},
			contains: []string{"MariaDB comment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			r, err := renderer.NewRenderer(tt.dialect)
			c.Assert(err, qt.IsNil)

			sql, err := r.Render(tt.node)
			c.Assert(err, qt.IsNil)

			for _, expected := range tt.contains {
				c.Assert(sql, qt.Contains, expected)
			}
		})
	}
}

func TestRenderer_CreateTable(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		contains []string
	}{
		{
			name:     "PostgreSQL CREATE TABLE",
			dialect:  "postgresql",
			contains: []string{"CREATE TABLE", "users", "POSTGRES TABLE"},
		},
		{
			name:     "MySQL CREATE TABLE",
			dialect:  "mysql",
			contains: []string{"CREATE TABLE", "users", "MYSQL TABLE"},
		},
		{
			name:     "MariaDB CREATE TABLE",
			dialect:  "mariadb",
			contains: []string{"CREATE TABLE", "users", "MARIADB TABLE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			r, err := renderer.NewRenderer(tt.dialect)
			c.Assert(err, qt.IsNil)

			table := &ast.CreateTableNode{
				Name: "users",
				Columns: []*ast.ColumnNode{
					{
						Name:     "id",
						Type:     "INTEGER",
						Primary:  true,
						Nullable: false,
					},
					{
						Name:     "email",
						Type:     "VARCHAR(255)",
						Unique:   true,
						Nullable: false,
					},
				},
			}

			sql, err := r.Render(table)
			c.Assert(err, qt.IsNil)

			for _, expected := range tt.contains {
				c.Assert(sql, qt.Contains, expected)
			}
		})
	}
}
