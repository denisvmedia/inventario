// Package-wide schema generator with dependency-ordered table creation
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dbschema"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: package_migrator <command> [options]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  generate <root_directory> [dialect]  - Generate schema from Go entities")
		fmt.Println("  write-db <root_directory> <db_url>   - Write schema to database")
		fmt.Println("  read-db <db_url>                     - Read schema from database")
		fmt.Println("  compare <root_directory> <db_url>    - Compare generated schema with database")
		fmt.Println("  migrate <root_directory> <db_url>    - Generate migration SQL from differences")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  package_migrator generate ./")
		fmt.Println("  package_migrator write-db ./models postgres://user:pass@localhost/db")
		fmt.Println("  package_migrator read-db postgres://user:pass@localhost/db")
		fmt.Println("  package_migrator compare ./models postgres://user:pass@localhost/db")
		fmt.Println("  package_migrator migrate ./models postgres://user:pass@localhost/db")
		return
	}

	command := os.Args[1]

	switch command {
	case "generate":
		if len(os.Args) < 3 {
			fmt.Println("Usage: package_migrator generate <root_directory> [dialect]")
			return
		}
		generateSchema(os.Args[2:])
	case "write-db":
		if len(os.Args) < 4 {
			fmt.Println("Usage: package_migrator write-db <root_directory> <db_url>")
			return
		}
		writeSchema(os.Args[2], os.Args[3])
	case "read-db":
		if len(os.Args) < 3 {
			fmt.Println("Usage: package_migrator read-db <db_url>")
			return
		}
		readDatabaseSchema(os.Args[2])
	case "compare":
		if len(os.Args) < 4 {
			fmt.Println("Usage: package_migrator compare <root_directory> <db_url>")
			return
		}
		compareSchema(os.Args[2], os.Args[3])
	case "migrate":
		if len(os.Args) < 4 {
			fmt.Println("Usage: package_migrator migrate <root_directory> <db_url>")
			return
		}
		generateMigration(os.Args[2], os.Args[3])
	default:
		// Backward compatibility - if first arg is not a command, treat as generate
		generateSchema(os.Args[1:])
	}
}

func generateSchema(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: package_migrator generate <root_directory> [dialect]")
		return
	}

	rootDir := args[0]

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
	if len(args) >= 2 {
		dialects = []string{args[1]}
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
	if len(args) >= 2 && args[1] == "migration" {
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

// compareSchema compares generated schema with database schema
func compareSchema(rootDir, dbURL string) {
	fmt.Printf("Comparing schema from %s with database %s\n", rootDir, dbschema.FormatDatabaseURL(dbURL))
	fmt.Println("=== SCHEMA COMPARISON ===")
	fmt.Println()

	// 1. Parse Go entities
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		return
	}

	result, err := migratorlib.ParsePackageRecursively(absPath)
	if err != nil {
		fmt.Printf("Error parsing Go entities: %v\n", err)
		return
	}

	// 2. Connect to database and read schema
	conn, err := dbschema.ConnectToDatabase(dbURL)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		return
	}
	defer conn.Close()

	dbSchema, err := conn.Reader.ReadSchema()
	if err != nil {
		fmt.Printf("Error reading database schema: %v\n", err)
		return
	}

	// 3. Compare schemas
	diff := dbschema.CompareSchemas(result, dbSchema)

	// 4. Display differences
	output := dbschema.FormatSchemaDiff(diff)
	fmt.Print(output)
}

// readDatabaseSchema reads and displays schema from database
func readDatabaseSchema(dbURL string) {
	fmt.Printf("Reading schema from database: %s\n", dbschema.FormatDatabaseURL(dbURL))
	fmt.Println("=== DATABASE SCHEMA ===")
	fmt.Println()

	// Connect to the database
	conn, err := dbschema.ConnectToDatabase(dbURL)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		fmt.Println()
		fmt.Println("Make sure:")
		fmt.Println("1. The database URL is correct")
		fmt.Println("2. The database server is running")
		fmt.Println("3. You have the correct permissions")
		fmt.Println("4. The database exists")
		return
	}
	defer conn.Close()

	fmt.Printf("Connected to %s database successfully!\n", conn.Info.Dialect)
	fmt.Println()

	// Read the schema
	schema, err := conn.Reader.ReadSchema()
	if err != nil {
		fmt.Printf("Error reading schema: %v\n", err)
		return
	}

	// Format and display the schema
	output := dbschema.FormatSchema(schema, conn.Info)
	fmt.Print(output)
}

// writeSchema writes the generated schema to the database
func writeSchema(rootDir, dbURL string) {
	fmt.Printf("Writing schema from %s to database %s\n", rootDir, dbschema.FormatDatabaseURL(dbURL))
	fmt.Println("=== WRITE SCHEMA TO DATABASE ===")
	fmt.Println()

	// 1. Parse Go entities
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		return
	}

	result, err := migratorlib.ParsePackageRecursively(absPath)
	if err != nil {
		fmt.Printf("Error parsing Go entities: %v\n", err)
		return
	}

	fmt.Printf("Parsed %d tables, %d enums from Go entities\n", len(result.Tables), len(result.Enums))

	// 2. Connect to database
	conn, err := dbschema.ConnectToDatabase(dbURL)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("Connected to %s database successfully!\n", conn.Info.Dialect)

	// 3. Check if schema already exists
	existingTables, err := conn.Writer.CheckSchemaExists(result)
	if err != nil {
		fmt.Printf("Error checking existing schema: %v\n", err)
		return
	}

	if len(existingTables) > 0 {
		fmt.Printf("⚠️  WARNING: The following tables already exist: %v\n", existingTables)
		fmt.Println("This operation will skip existing tables.")
		fmt.Println("Use 'compare' command to see differences, or 'migrate' to generate update SQL.")
		fmt.Println()
	}

	// 4. Write schema
	fmt.Println("Writing schema to database...")
	err = conn.Writer.WriteSchema(result)
	if err != nil {
		fmt.Printf("Error writing schema: %v\n", err)
		return
	}

	fmt.Println("✅ Schema written successfully!")
}

// generateMigration generates migration SQL from schema differences
func generateMigration(rootDir, dbURL string) {
	fmt.Printf("Generating migration from %s to database %s\n", rootDir, dbschema.FormatDatabaseURL(dbURL))
	fmt.Println("=== GENERATE MIGRATION SQL ===")
	fmt.Println()

	// 1. Parse Go entities
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		return
	}

	result, err := migratorlib.ParsePackageRecursively(absPath)
	if err != nil {
		fmt.Printf("Error parsing Go entities: %v\n", err)
		return
	}

	// 2. Connect to database and read schema
	conn, err := dbschema.ConnectToDatabase(dbURL)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		return
	}
	defer conn.Close()

	dbSchema, err := conn.Reader.ReadSchema()
	if err != nil {
		fmt.Printf("Error reading database schema: %v\n", err)
		return
	}

	// 3. Compare schemas
	diff := dbschema.CompareSchemas(result, dbSchema)

	// 4. Display differences summary
	fmt.Print(dbschema.FormatSchemaDiff(diff))

	if !diff.HasChanges() {
		return
	}

	// 5. Generate migration SQL
	fmt.Println("=== MIGRATION SQL ===")
	fmt.Println()

	statements := diff.GenerateMigrationSQL(result, conn.Info.Dialect)

	fmt.Println("-- Migration generated from schema differences")
	fmt.Printf("-- Generated on: %s\n", "now") // You could add actual timestamp
	fmt.Printf("-- Source: %s\n", rootDir)
	fmt.Printf("-- Target: %s\n", dbschema.FormatDatabaseURL(dbURL))
	fmt.Println()

	for _, statement := range statements {
		fmt.Println(statement)
	}

	fmt.Println()
	fmt.Printf("Generated %d migration statements.\n", len(statements))
	fmt.Println("⚠️  Review the SQL carefully before executing!")
}
