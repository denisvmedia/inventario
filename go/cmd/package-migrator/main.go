// Package-wide schema generator with dependency-ordered table creation
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: package_migrator <root_directory> [dialect]")
		fmt.Println("  root_directory: Path to the root directory to scan for entities")
		fmt.Println("  dialect: Database dialect (postgres, mysql, mariadb) - defaults to all")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  package_migrator ./")
		fmt.Println("  package_migrator ./ postgres")
		fmt.Println("  package_migrator ../models mysql")
		return
	}

	rootDir := os.Args[1]

	// Convert to absolute path
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		return
	}

	// Check if directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Printf("Directory does not exist: %s\n", absPath)
		return
	}

	fmt.Printf("Scanning directory: %s\n", absPath)
	fmt.Println("=" + strings.Repeat("=", len(absPath)+19))
	fmt.Println()

	// Parse the entire package recursively
	result, err := migratorlib.ParsePackageRecursively(absPath)
	if err != nil {
		fmt.Printf("Error parsing package: %v\n", err)
		return
	}

	// Print summary
	fmt.Printf("Found %d tables, %d fields, %d indexes, %d enums, %d embedded fields\n",
		len(result.Tables), len(result.Fields), len(result.Indexes), len(result.Enums), len(result.EmbeddedFields))
	fmt.Println()

	// Print dependency information
	fmt.Println(result.GetDependencyInfo())
	fmt.Println()

	// Determine which dialects to generate
	dialects := []string{"postgres", "mysql", "mariadb"}
	if len(os.Args) >= 3 {
		dialects = []string{os.Args[2]}
	}

	// Generate SQL for each dialect
	for _, dialect := range dialects {
		fmt.Printf("=== %s SCHEMA ===\n", strings.ToUpper(dialect))
		fmt.Println()

		// Generate enum statements first (only once per dialect)
		if len(result.Enums) > 0 {
			fmt.Println("-- ENUMS --")
			for _, enum := range result.Enums {
				if dialect == "postgres" {
					fmt.Printf("CREATE TYPE %s AS ENUM (%s);\n", enum.Name,
						strings.Join(func() []string {
							quoted := make([]string, len(enum.Values))
							for i, v := range enum.Values {
								quoted[i] = "'" + v + "'"
							}
							return quoted
						}(), ", "))
				} else {
					fmt.Printf("-- Enum %s: %v (handled in table definitions)\n", enum.Name, enum.Values)
				}
			}
			fmt.Println()
		}

		// Generate table statements
		statements := result.GetOrderedCreateStatements(dialect)

		for i, statement := range statements {
			fmt.Printf("-- Table %d/%d\n", i+1, len(result.Tables))
			fmt.Println(statement)
			fmt.Println()
		}

		fmt.Println()
	}

	// Generate migration file template
	if len(os.Args) >= 3 && os.Args[2] == "migration" {
		generateMigrationTemplate(result, dialects[0])
	}
}

func generateMigrationTemplate(result *migratorlib.PackageParseResult, dialect string) {
	fmt.Println("=== MIGRATION TEMPLATE ===")
	fmt.Println()

	fmt.Println("-- Migration: Create all tables")
	fmt.Println("-- Generated automatically from Go entity definitions")
	fmt.Println()

	statements := result.GetOrderedCreateStatements(dialect)
	for _, statement := range statements {
		fmt.Println(statement)
		fmt.Println()
	}

	fmt.Println("-- End of migration")
}
