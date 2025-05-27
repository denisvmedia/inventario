// Full implementation with PostgreSQL + MySQL + MariaDB DDL generation, foreign keys, and ALTER scaffolding

package main

import (
	"fmt"
	"os"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrator_parser <filename.go>")
		return
	}
	filename := os.Args[1]
	emb, fields, indexes, tables, enums := migratorlib.ParseFile(filename)

	dialects := []string{"postgres", "mysql", "mariadb"}
	for _, dialect := range dialects {
		for _, table := range tables {
			fmt.Println(migratorlib.GenerateCreateTable(table, fields, indexes, enums, dialect))
		}
	}

	fmt.Println(migratorlib.GenerateAlterStatements([]migratorlib.SchemaField{
		{StructName: "User", Name: "email", Type: "VARCHAR(255)", Nullable: true},
		{StructName: "User", Name: "name", Type: "TEXT", Nullable: false},
	}, fields))

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
