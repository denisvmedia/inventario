package renderer_test

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/renderer"
)

func TestDebugMySQLSQL(t *testing.T) {
	c := qt.New(t)

	r, err := renderer.NewRenderer("mysql")
	c.Assert(err, qt.IsNil)

	table := &ast.CreateTableNode{
		Name: "test_users",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INT",
				Primary:  true,
				AutoInc:  true,
				Nullable: false,
			},
			{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Unique:   true,
				Nullable: false,
			},
			{
				Name:     "status",
				Type:     "ENUM('active', 'inactive')",
				Nullable: false,
			},
		},
		Options: map[string]string{
			"ENGINE": "InnoDB",
		},
	}

	sql, err := r.Render(table)
	c.Assert(err, qt.IsNil)

	fmt.Printf("Generated MySQL SQL:\n%s\n", sql)
	fmt.Printf("Generated MySQL SQL (escaped):\n%q\n", sql)
}

func TestDebugMariaDBSQL(t *testing.T) {
	c := qt.New(t)

	r, err := renderer.NewRenderer("mariadb")
	c.Assert(err, qt.IsNil)

	table := &ast.CreateTableNode{
		Name: "test_products",
		Columns: []*ast.ColumnNode{
			{
				Name:     "id",
				Type:     "INT",
				Primary:  true,
				AutoInc:  true,
				Nullable: false,
			},
			{
				Name:     "name",
				Type:     "VARCHAR(255)",
				Nullable: false,
			},
			{
				Name:     "category",
				Type:     "ENUM('electronics', 'books', 'clothing')",
				Nullable: false,
			},
		},
		Options: map[string]string{
			"ENGINE": "InnoDB",
		},
	}

	sql, err := r.Render(table)
	c.Assert(err, qt.IsNil)

	fmt.Printf("Generated MariaDB SQL:\n%s\n", sql)
}
