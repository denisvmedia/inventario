package renderer_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
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

			r := renderer.NewRenderer(tt.dialect)
			c.Assert(r, qt.IsNotNil)
			c.Assert(r.GetDialect(), qt.Equals, tt.expected)
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

			c.Assert(func() {
				renderer.NewRenderer(tt.dialect)
			}, qt.PanicMatches, "unsupported database dialect: "+tt.dialect)
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

	c.Assert(func() {
		_, _ = renderer.RenderSQL("unsupported", comment)
	}, qt.PanicMatches, "unsupported database dialect: unsupported")
}

func TestRenderer_Interface(t *testing.T) {
	// Test that all dialect renderers implement the RenderVisitor interface
	dialects := []string{"postgresql", "mysql", "mariadb"}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			c := qt.New(t)

			r := renderer.NewRenderer(dialect)

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

			r := renderer.NewRenderer(tt.dialect)

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

			r := renderer.NewRenderer(tt.dialect)

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

// TestNewVisitorMethods_UnitTests tests the new visitor methods without database dependencies
func TestNewVisitorMethods_UnitTests(t *testing.T) {
	dialects := []string{"postgresql", "mysql", "mariadb"}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			t.Run("DropIndex", func(t *testing.T) {
				c := qt.New(t)

				dropIndex := ast.NewDropIndex("test_index").
					SetTable("test_table").
					SetIfExists().
					SetComment("Test comment")

				sql, err := renderer.RenderSQL(dialect, dropIndex)
				c.Assert(err, qt.IsNil)
				c.Assert(sql, qt.IsNotNil)
				c.Assert(sql, qt.Contains, "DROP INDEX")
				c.Assert(sql, qt.Contains, "test_index")

				if dialect == "postgresql" {
					c.Assert(sql, qt.Not(qt.Contains), "ON test_table")
				} else {
					c.Assert(sql, qt.Contains, "ON test_table")
				}
			})

			t.Run("CreateType", func(t *testing.T) {
				c := qt.New(t)

				enumDef := ast.NewEnumTypeDef("value1", "value2")
				createType := ast.NewCreateType("test_type", enumDef).
					SetComment("Test type")

				sql, err := renderer.RenderSQL(dialect, createType)
				c.Assert(err, qt.IsNil)
				c.Assert(sql, qt.IsNotNil)

				if dialect == "postgresql" {
					c.Assert(sql, qt.Contains, "CREATE TYPE test_type AS ENUM")
					c.Assert(sql, qt.Contains, "'value1'")
					c.Assert(sql, qt.Contains, "'value2'")
				} else {
					c.Assert(sql, qt.Contains, "does not support CREATE TYPE")
				}
			})

			t.Run("AlterType", func(t *testing.T) {
				c := qt.New(t)

				alterType := ast.NewAlterType("test_type").
					AddOperation(ast.NewAddEnumValueOperation("new_value"))

				sql, err := renderer.RenderSQL(dialect, alterType)
				c.Assert(err, qt.IsNil)
				c.Assert(sql, qt.IsNotNil)

				if dialect == "postgresql" {
					c.Assert(sql, qt.Contains, "ALTER TYPE test_type ADD VALUE 'new_value'")
				} else {
					c.Assert(sql, qt.Contains, "does not support ALTER TYPE")
				}
			})
		})
	}
}

func TestPlatformSpecificOverrides(t *testing.T) {
	c := qt.New(t)

	result, err := goschema.ParseDir("../../stubs")
	c.Assert(err, qt.IsNil)

	// Test PostgreSQL (default)
	postgresStatements := renderer.GetOrderedCreateStatements(result, "postgres")
	var postgresArticlesSQL string
	for _, statement := range postgresStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			postgresArticlesSQL = statement
			break
		}
	}
	c.Assert(postgresArticlesSQL, qt.Contains, "meta_data JSONB")

	// Test MySQL (override)
	mysqlStatements := renderer.GetOrderedCreateStatements(result, "mysql")
	var mysqlArticlesSQL string
	for _, statement := range mysqlStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			mysqlArticlesSQL = statement
			break
		}
	}
	c.Assert(mysqlArticlesSQL, qt.Contains, "meta_data JSON")

	// Test MariaDB (override with check constraint)
	mariadbStatements := renderer.GetOrderedCreateStatements(result, "mariadb")
	var mariadbArticlesSQL string
	for _, statement := range mariadbStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			mariadbArticlesSQL = statement
			break
		}
	}
	c.Assert(mariadbArticlesSQL, qt.Contains, "meta_data LONGTEXT")
	c.Assert(mariadbArticlesSQL, qt.Contains, "JSON_VALID(meta_data)")
}

func TestEmbeddedFieldsInPackageParser(t *testing.T) {
	c := qt.New(t)

	result, err := goschema.ParseDir("../../stubs")
	c.Assert(err, qt.IsNil)

	// Find the articles table statement
	statements := renderer.GetOrderedCreateStatements(result, "postgres")
	var articlesSQL string
	for _, statement := range statements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			articlesSQL = statement
			break
		}
	}

	c.Assert(articlesSQL, qt.Not(qt.Equals), "")

	// Verify embedded fields are included
	c.Assert(articlesSQL, qt.Contains, "created_at", qt.Commentf("Should contain created_at from Timestamps"))
	c.Assert(articlesSQL, qt.Contains, "updated_at", qt.Commentf("Should contain updated_at from Timestamps"))
	c.Assert(articlesSQL, qt.Contains, "audit_by", qt.Commentf("Should contain audit_by from AuditInfo"))
	c.Assert(articlesSQL, qt.Contains, "audit_reason", qt.Commentf("Should contain audit_reason from AuditInfo"))
	c.Assert(articlesSQL, qt.Contains, "meta_data", qt.Commentf("Should contain meta_data from Meta"))
	c.Assert(articlesSQL, qt.Contains, "author_id", qt.Commentf("Should contain author_id from User relation"))
}

func TestGetOrderedCreateStatements(t *testing.T) {
	c := qt.New(t)

	result, err := goschema.ParseDir("../../stubs")
	c.Assert(err, qt.IsNil)

	statements := renderer.GetOrderedCreateStatements(result, "postgres")
	c.Assert(len(statements), qt.Equals, len(result.Tables)+3) // 1 type + 2 indexes

	c.Assert(statements[0], qt.Contains, "CREATE TYPE")
	c.Assert(statements[17], qt.Contains, "CREATE INDEX")
	c.Assert(statements[18], qt.Contains, "CREATE INDEX")

	for i := 1; i < 17; i++ {
		c.Assert(statements[i], qt.Contains, "CREATE TABLE")
	}
}
