package main

import (
	"fmt"
	"log"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/parser"
)

func main() {
	// MySQL-style CREATE TABLE statement with backticks, specific data types, and MySQL-specific syntax
	sql := `CREATE TABLE ` + "`sample`" + ` (
		` + "`id`" + ` int NOT NULL AUTO_INCREMENT,
		` + "`name`" + ` varchar(50) DEFAULT 'John',
		` + "`age`" + ` int DEFAULT 30,
		` + "`created_at`" + ` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
		` + "`active`" + ` tinyint(1) DEFAULT 1,
		PRIMARY KEY (` + "`id`" + `)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;`

	fmt.Println("Parsing MySQL-style CREATE TABLE statement:")
	fmt.Println("==========================================")
	fmt.Println(sql)
	fmt.Println()

	// Create parser and parse the SQL
	p := parser.NewParser(sql)
	statements, err := p.Parse()
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	fmt.Printf("Successfully parsed %d SQL statement(s):\n\n", len(statements.Statements))

	// Print detailed information about the parsed statement
	for i, stmt := range statements.Statements {
		fmt.Printf("Statement %d:\n", i+1)

		if createTable, ok := stmt.(*ast.CreateTableNode); ok {
			fmt.Printf("  Type: CREATE TABLE\n")
			fmt.Printf("  Table: %s\n", createTable.Name)
			fmt.Printf("  Columns: %d\n", len(createTable.Columns))
			fmt.Printf("  Constraints: %d\n", len(createTable.Constraints))

			fmt.Println("\n  Column Details:")
			for j, col := range createTable.Columns {
				fmt.Printf("    %d. %s %s", j+1, col.Name, col.Type)

				var attributes []string
				if !col.Nullable {
					attributes = append(attributes, "NOT NULL")
				}
				if col.AutoInc {
					attributes = append(attributes, "AUTO_INCREMENT")
				}
				if col.Primary {
					attributes = append(attributes, "PRIMARY KEY")
				}
				if col.Unique {
					attributes = append(attributes, "UNIQUE")
				}
				if col.Default != nil {
					if col.Default.Expression != "" {
						attributes = append(attributes, fmt.Sprintf("DEFAULT %s", col.Default.Expression))
					} else {
						attributes = append(attributes, fmt.Sprintf("DEFAULT %s", col.Default.Value))
					}
				}

				if len(attributes) > 0 {
					fmt.Printf(" [%s]", fmt.Sprintf("%v", attributes))
				}
				fmt.Println()
			}

			if len(createTable.Constraints) > 0 {
				fmt.Println("\n  Constraints:")
				for j, constraint := range createTable.Constraints {
					fmt.Printf("    %d. %s", j+1, constraint.Type.String())
					if len(constraint.Columns) > 0 {
						fmt.Printf(" (%v)", constraint.Columns)
					}
					if constraint.Name != "" {
						fmt.Printf(" [name: %s]", constraint.Name)
					}
					fmt.Println()
				}
			}

			if len(createTable.Options) > 0 {
				fmt.Println("\n  Table Options:")
				for key, value := range createTable.Options {
					fmt.Printf("    %s = %s\n", key, value)
				}
			}

			if createTable.Comment != "" {
				fmt.Printf("\n  Comment: %s\n", createTable.Comment)
			}
		}
		fmt.Println()
	}

	fmt.Println("MySQL parser demo completed successfully!")
	fmt.Println("\nKey MySQL features demonstrated:")
	fmt.Println("- Backticked identifiers (`table_name`, `column_name`)")
	fmt.Println("- MySQL-specific data types (int, varchar, tinyint, timestamp)")
	fmt.Println("- Numeric default values (30, 1)")
	fmt.Println("- String default values ('John')")
	fmt.Println("- Function default values (CURRENT_TIMESTAMP)")
	fmt.Println("- MySQL table options (ENGINE, CHARSET, COLLATE)")
	fmt.Println("- AUTO_INCREMENT columns")
	fmt.Println("- NULL/NOT NULL specifications")
	fmt.Println("- PRIMARY KEY constraints")
}
