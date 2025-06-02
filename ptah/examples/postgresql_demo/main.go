package main

import (
	"fmt"
	"log"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/parser"
)

func main() {
	// PostgreSQL-style DDL statements with advanced features
	sql := `
		CREATE TYPE status_enum AS ENUM ('pending', 'active', 'archived');

		CREATE DOMAIN email_domain AS TEXT
			CHECK (VALUE ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');

		CREATE TABLE public.full_demo (
			serial_id SERIAL PRIMARY KEY,
			big_id BIGSERIAL UNIQUE,
			uuid_id UUID DEFAULT gen_random_uuid() NOT NULL,
			char_fixed CHAR(10),
			varchar_var VARCHAR(255) NOT NULL,
			text_field TEXT CHECK (char_length(text_field) <= 5000),
			small_value SMALLINT DEFAULT 1 CHECK (small_value > 0),
			numeric_precise NUMERIC(12,4) NOT NULL DEFAULT 0.0000,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT now(),
			status status_enum DEFAULT 'pending',
			tags TEXT[] DEFAULT ARRAY[]::TEXT[],
			matrix INT[][],
			json_field JSON,
			jsonb_field JSONB NOT NULL DEFAULT '{}'::jsonb,
			data BYTEA,
			email_address email_domain,
			full_name TEXT GENERATED ALWAYS AS (char_fixed || ' ' || varchar_var) STORED,
			case_insensitive TEXT COLLATE "C",
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE ON UPDATE SET NULL,
			CONSTRAINT uq_tag_and_status UNIQUE (tags, status),
			CONSTRAINT chk_price_non_negative CHECK (numeric_precise >= 0),
			CHECK (created_at <= updated_at)
		);

		COMMENT ON TABLE public.full_demo IS 'Comprehensive PostgreSQL demo table';
		COMMENT ON COLUMN public.full_demo.varchar_var IS 'Variable length string';
	`

	fmt.Println("Parsing PostgreSQL DDL statements:")
	fmt.Println("==================================")
	fmt.Println(sql)
	fmt.Println()

	// Create parser and parse the SQL
	p := parser.NewParser(sql)
	statements, err := p.Parse()
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	fmt.Printf("Successfully parsed %d SQL statement(s):\n\n", len(statements.Statements))

	// Print detailed information about each parsed statement
	for i, stmt := range statements.Statements {
		fmt.Printf("Statement %d:\n", i+1)

		switch s := stmt.(type) {
		case *ast.EnumNode:
			fmt.Printf("  Type: CREATE TYPE (ENUM)\n")
			fmt.Printf("  Name: %s\n", s.Name)
			fmt.Printf("  Values: %v\n", s.Values)

		case *ast.CommentNode:
			fmt.Printf("  Type: COMMENT/DOMAIN\n")
			fmt.Printf("  Content: %s\n", s.Text)

		case *ast.CreateTableNode:
			fmt.Printf("  Type: CREATE TABLE\n")
			fmt.Printf("  Table: %s\n", s.Name)
			fmt.Printf("  Columns: %d\n", len(s.Columns))
			fmt.Printf("  Constraints: %d\n", len(s.Constraints))

			fmt.Println("\n  PostgreSQL Column Features:")
			for j, col := range s.Columns {
				fmt.Printf("    %d. %s %s", j+1, col.Name, col.Type)

				var features []string
				if !col.Nullable {
					features = append(features, "NOT NULL")
				}
				if col.Primary {
					features = append(features, "PRIMARY KEY")
				}
				if col.Unique {
					features = append(features, "UNIQUE")
				}
				if col.Default != nil {
					if col.Default.Expression != "" {
						features = append(features, fmt.Sprintf("DEFAULT %s", col.Default.Expression))
					} else {
						features = append(features, fmt.Sprintf("DEFAULT %s", col.Default.Value))
					}
				}
				if col.Check != "" {
					if len(col.Check) > 50 {
						features = append(features, "CHECK(...)")
					} else {
						features = append(features, fmt.Sprintf("CHECK(%s)", col.Check))
					}
				}
				if col.ForeignKey != nil {
					features = append(features, fmt.Sprintf("REFERENCES %s(%s)", col.ForeignKey.Table, col.ForeignKey.Column))
				}
				if col.Comment != "" {
					features = append(features, fmt.Sprintf("COLLATE %s", col.Comment))
				}

				if len(features) > 0 {
					fmt.Printf(" [%s]", fmt.Sprintf("%v", features))
				}
				fmt.Println()
			}

			if len(s.Constraints) > 0 {
				fmt.Println("\n  Table Constraints:")
				for j, constraint := range s.Constraints {
					fmt.Printf("    %d. %s", j+1, constraint.Type.String())
					if constraint.Name != "" {
						fmt.Printf(" [%s]", constraint.Name)
					}
					if len(constraint.Columns) > 0 {
						fmt.Printf(" (%v)", constraint.Columns)
					}
					if constraint.Expression != "" {
						fmt.Printf(" CHECK(%s)", constraint.Expression)
					}
					fmt.Println()
				}
			}

		default:
			fmt.Printf("  Type: Unknown (%T)\n", s)
		}
		fmt.Println()
	}

	fmt.Println("PostgreSQL parser demo completed successfully!")
	fmt.Println("\nKey PostgreSQL features demonstrated:")
	fmt.Println("- ENUM types with CREATE TYPE")
	fmt.Println("- DOMAIN types with CHECK constraints")
	fmt.Println("- Schema-qualified table names (public.table_name)")
	fmt.Println("- SERIAL and BIGSERIAL auto-increment types")
	fmt.Println("- UUID with gen_random_uuid() function")
	fmt.Println("- Array types (TEXT[], INT[][])")
	fmt.Println("- JSON and JSONB types with type casting ('{}'::jsonb)")
	fmt.Println("- PostgreSQL-specific data types (TIMESTAMPTZ, BYTEA)")
	fmt.Println("- GENERATED ALWAYS AS computed columns")
	fmt.Println("- COLLATE specifications")
	fmt.Println("- Complex CHECK constraints with functions")
	fmt.Println("- Named table constraints")
	fmt.Println("- COMMENT ON statements")
	fmt.Println("- Foreign keys with CASCADE and SET NULL actions")
}
