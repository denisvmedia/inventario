// Migrator Parser - A comprehensive database schema parser and DDL generator
//
// This tool parses Go struct definitions with migration annotations and generates
// database-specific DDL statements for PostgreSQL, MySQL, and MariaDB. It demonstrates
// the full capabilities of the Ptah migration system including:
//
// - Parsing Go files with embedded type dependencies
// - Extracting table, field, index, and enum definitions
// - Generating dialect-specific CREATE statements
// - Handling foreign key relationships and constraints
// - Supporting platform-specific overrides
//
// Usage:
//   migrator_parser <filename.go>
//
// Example:
//   migrator_parser ./entities/user.go
//   migrator_parser ./models/product.go
//
// The tool will automatically discover and parse related files in the same directory
// to resolve embedded type references and generate complete schema definitions.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/renderer"
)

func main() {
	// Validate command line arguments
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	filename := os.Args[1]

	// Parse the Go file and discover dependencies
	fmt.Printf("Parsing Go file: %s\n", filename)
	fmt.Println("Discovering embedded type dependencies...")

	embeddedFields, fields, indexes, tables, enums := goschema.ParseFileWithDependencies(filename)

	// Build the complete database schema
	database := goschema.Database{
		Tables:         tables,
		Fields:         fields,
		Indexes:        indexes,
		Enums:          enums,
		EmbeddedFields: embeddedFields,
	}

	// Generate DDL statements for all supported database dialects
	fmt.Printf("\nFound %d tables, %d fields, %d indexes, %d enums\n",
		len(tables), len(fields), len(indexes), len(enums))
	fmt.Println("\nGenerating DDL statements for supported dialects:")
	fmt.Println(strings.Repeat("=", 60))

	generateDDLForDialects(&database)

	// Display detailed schema information if requested
	if shouldShowDetails() {
		fmt.Println("\nDetailed Schema Information:")
		fmt.Println(strings.Repeat("=", 60))
		displaySchemaDetails(embeddedFields, fields, indexes, tables, enums)
	}
}

// printUsage displays the command usage information
func printUsage() {
	fmt.Println("Migrator Parser - Database Schema DDL Generator")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  migrator_parser <filename.go>")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  filename.go    Path to the Go file containing entity definitions")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  SHOW_DETAILS=1    Show detailed schema information")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  migrator_parser ./entities/user.go")
	fmt.Println("  migrator_parser ./models/product.go")
	fmt.Println("  SHOW_DETAILS=1 migrator_parser ./entities/user.go")
}

// shouldShowDetails checks if detailed schema information should be displayed
func shouldShowDetails() bool {
	return os.Getenv("SHOW_DETAILS") == "1"
}

// generateDDLForDialects generates and displays DDL statements for all supported dialects
func generateDDLForDialects(database *goschema.Database) {
	supportedDialects := []string{"postgres", "mysql", "mariadb"}

	for _, dialect := range supportedDialects {
		fmt.Printf("\n=== %s ===\n", strings.ToUpper(dialect))

		statements := renderer.GetOrderedCreateStatements(database, dialect)
		if len(statements) == 0 {
			fmt.Printf("No statements generated for %s\n", dialect)
			continue
		}

		for _, stmt := range statements {
			fmt.Println(stmt)
		}
		fmt.Println()
	}
}

// displaySchemaDetails shows detailed information about all parsed schema elements
func displaySchemaDetails(embeddedFields []goschema.EmbeddedField, fields []goschema.Field, indexes []goschema.Index, tables []goschema.Table, enums []goschema.Enum) {
	// Display embedded fields
	if len(embeddedFields) > 0 {
		fmt.Printf("\nEmbedded Fields (%d):\n", len(embeddedFields))
		fmt.Println(strings.Repeat("-", 40))
		for i, embedded := range embeddedFields {
			fmt.Printf("%d. Struct: %s, Embedded Type: %s, Mode: %s\n",
				i+1, embedded.StructName, embedded.EmbeddedTypeName, embedded.Mode)
		}
	}

	// Display tables
	if len(tables) > 0 {
		fmt.Printf("\nTables (%d):\n", len(tables))
		fmt.Println(strings.Repeat("-", 40))
		for i, table := range tables {
			fmt.Printf("%d. %s (struct: %s)\n", i+1, table.Name, table.StructName)
			if table.Comment != "" {
				fmt.Printf("   Comment: %s\n", table.Comment)
			}
			if table.Engine != "" {
				fmt.Printf("   Engine: %s\n", table.Engine)
			}
			if len(table.PrimaryKey) > 0 {
				fmt.Printf("   Primary Key: %s\n", strings.Join(table.PrimaryKey, ", "))
			}
		}
	}

	// Display fields
	if len(fields) > 0 {
		fmt.Printf("\nFields (%d):\n", len(fields))
		fmt.Println(strings.Repeat("-", 40))
		for i, field := range fields {
			fmt.Printf("%d. %s.%s -> %s (%s)\n",
				i+1, field.StructName, field.FieldName, field.Name, field.Type)

			var attributes []string
			if field.Primary {
				attributes = append(attributes, "PRIMARY KEY")
			}
			if field.AutoInc {
				attributes = append(attributes, "AUTO_INCREMENT")
			}
			if field.Unique {
				attributes = append(attributes, "UNIQUE")
			}
			if !field.Nullable {
				attributes = append(attributes, "NOT NULL")
			}
			if field.Default != "" {
				attributes = append(attributes, fmt.Sprintf("DEFAULT '%s'", field.Default))
			}
			if field.Foreign != "" {
				attributes = append(attributes, fmt.Sprintf("FOREIGN KEY -> %s", field.Foreign))
			}

			if len(attributes) > 0 {
				fmt.Printf("   Attributes: %s\n", strings.Join(attributes, ", "))
			}
		}
	}

	// Display indexes
	if len(indexes) > 0 {
		fmt.Printf("\nIndexes (%d):\n", len(indexes))
		fmt.Println(strings.Repeat("-", 40))
		for i, index := range indexes {
			indexType := "INDEX"
			if index.Unique {
				indexType = "UNIQUE INDEX"
			}
			fmt.Printf("%d. %s %s ON %s (%s)\n",
				i+1, indexType, index.Name, index.StructName, strings.Join(index.Fields, ", "))
		}
	}

	// Display enums
	if len(enums) > 0 {
		fmt.Printf("\nEnums (%d):\n", len(enums))
		fmt.Println(strings.Repeat("-", 40))
		for i, enum := range enums {
			fmt.Printf("%d. %s: [%s]\n",
				i+1, enum.Name, strings.Join(enum.Values, ", "))
		}
	}
}
