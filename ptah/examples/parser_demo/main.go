package main

import (
	"fmt"
	"log"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/parser"
)

func main() {
	// Example SQL DDL statements
	sql := `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTO_INCREMENT,
			email VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(100) NOT NULL,
			age INTEGER CHECK (age >= 0),
			created_at TIMESTAMP DEFAULT NOW(),
			FOREIGN KEY (id) REFERENCES profiles(user_id) ON DELETE CASCADE
		) ENGINE=InnoDB CHARSET=utf8mb4;

		CREATE UNIQUE INDEX idx_users_email ON users (email);

		CREATE TYPE status AS ENUM ('active', 'inactive', 'pending');

		ALTER TABLE users ADD COLUMN status VARCHAR(20) DEFAULT 'active';
	`

	// Create parser and parse the SQL
	p := parser.NewParser(sql)
	statements, err := p.Parse()
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	fmt.Printf("Successfully parsed %d SQL statements:\n\n", len(statements.Statements))

	// Print information about each parsed statement
	for i, stmt := range statements.Statements {
		fmt.Printf("Statement %d:\n", i+1)

		switch s := stmt.(type) {
		case *ast.CreateTableNode:
			fmt.Printf("  Type: CREATE TABLE\n")
			fmt.Printf("  Table: %s\n", s.Name)
			fmt.Printf("  Columns: %d\n", len(s.Columns))
			fmt.Printf("  Constraints: %d\n", len(s.Constraints))
			if len(s.Options) > 0 {
				fmt.Printf("  Options: %v\n", s.Options)
			}
			if s.Comment != "" {
				fmt.Printf("  Comment: %s\n", s.Comment)
			}

		case *ast.IndexNode:
			fmt.Printf("  Type: CREATE INDEX\n")
			fmt.Printf("  Index: %s\n", s.Name)
			fmt.Printf("  Table: %s\n", s.Table)
			fmt.Printf("  Columns: %v\n", s.Columns)
			fmt.Printf("  Unique: %t\n", s.Unique)

		case *ast.EnumNode:
			fmt.Printf("  Type: CREATE TYPE (ENUM)\n")
			fmt.Printf("  Name: %s\n", s.Name)
			fmt.Printf("  Values: %v\n", s.Values)

		case *ast.AlterTableNode:
			fmt.Printf("  Type: ALTER TABLE\n")
			fmt.Printf("  Table: %s\n", s.Name)
			fmt.Printf("  Operations: %d\n", len(s.Operations))

		default:
			fmt.Printf("  Type: Unknown\n")
		}
		fmt.Println()
	}

	fmt.Println("Parser demo completed successfully!")
}
