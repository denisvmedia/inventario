// Full implementation with PostgreSQL + MySQL + MariaDB DDL generation, foreign keys, and ALTER scaffolding

package main

import (
	"fmt"
	"os"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/platform"
	"github.com/denisvmedia/inventario/ptah/renderer/generators"
	"github.com/denisvmedia/inventario/ptah/schema/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrator_parser <filename.go>")
		return
	}
	filename := os.Args[1]
	emb, fields, indexes, tables, enums := parser.ParseFileWithDependencies(filename)

	dialects := []string{"postgres", "mysql", "mariadb"}
	for _, dialect := range dialects {
		fmt.Printf("=== %s ===\n", dialect)
		for _, table := range tables {
			// Use the new embedded field support
			fmt.Println(generators.GenerateCreateTableWithEmbedded(table, fields, indexes, enums, emb, dialect))
		}
		fmt.Println()
	}

	fmt.Println(generators.GenerateAlterStatements([]goschema.Field{
		{StructName: "User", Name: "email", Type: "VARCHAR(255)", Nullable: true},
		{StructName: "User", Name: "name", Type: "TEXT", Nullable: false},
	}, fields, platform.Postgres))

	for _, e := range emb {
		fmt.Printf(`Embedded: %+v
		`, e)
	}
	for _, f := range fields {
		fmt.Printf(`Field: %+v
		`, f)
	}
	for _, i := range indexes {
		fmt.Printf(`Index: %+v
		`, i)
	}
	for _, t := range tables {
		fmt.Printf(`Table: %+v
		`, t)
	}
	for _, e := range enums {
		fmt.Printf(`Enum: %+v
		`, e)
	}
}
