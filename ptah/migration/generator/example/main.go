package main

import (
	"fmt"
	"log"
	"os"

	"github.com/denisvmedia/inventario/ptah/migration/generator"
)

func main() {
	// Check command line arguments
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run main.go <entities_dir> <database_url> <output_dir> [migration_name]")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  go run main.go ./entities postgres://user:pass@localhost/db ./migrations")
		fmt.Println("  go run main.go ./entities postgres://user:pass@localhost/db ./migrations add_users_table")
		fmt.Println("")
		fmt.Println("Database URL formats:")
		fmt.Println("  PostgreSQL: postgres://user:password@host:port/database")
		fmt.Println("  MySQL:      mysql://user:password@host:port/database")
		fmt.Println("  MariaDB:    mariadb://user:password@host:port/database")
		os.Exit(1)
	}

	entitiesDir := os.Args[1]
	databaseURL := os.Args[2]
	outputDir := os.Args[3]
	
	migrationName := "migration" // default
	if len(os.Args) > 4 {
		migrationName = os.Args[4]
	}

	// Configure migration generation options
	opts := generator.GenerateMigrationOptions{
		RootDir:       entitiesDir,
		DatabaseURL:   databaseURL,
		MigrationName: migrationName,
		OutputDir:     outputDir,
	}

	fmt.Printf("Generating migration...\n")
	fmt.Printf("  Entities directory: %s\n", entitiesDir)
	fmt.Printf("  Database URL: %s\n", maskPassword(databaseURL))
	fmt.Printf("  Output directory: %s\n", outputDir)
	fmt.Printf("  Migration name: %s\n", migrationName)
	fmt.Println()

	// Generate the migration
	files, err := generator.GenerateMigration(opts)
	if err != nil {
		log.Fatalf("Error generating migration: %v", err)
	}

	// Display results
	fmt.Printf("✅ Migration generated successfully!\n")
	fmt.Printf("  Version: %d\n", files.Version)
	fmt.Printf("  UP file:   %s\n", files.UpFile)
	fmt.Printf("  DOWN file: %s\n", files.DownFile)
	fmt.Println()

	// Display file contents for review
	fmt.Println("=== UP MIGRATION ===")
	upContent, err := os.ReadFile(files.UpFile)
	if err != nil {
		log.Printf("Warning: Could not read UP migration file: %v", err)
	} else {
		fmt.Println(string(upContent))
	}

	fmt.Println("=== DOWN MIGRATION ===")
	downContent, err := os.ReadFile(files.DownFile)
	if err != nil {
		log.Printf("Warning: Could not read DOWN migration file: %v", err)
	} else {
		fmt.Println(string(downContent))
	}

	fmt.Println("⚠️  Please review the generated SQL carefully before applying the migration!")
	fmt.Println()
	fmt.Println("To apply the migration:")
	fmt.Printf("  go run ./cmd migrate-up --db-url %s --migrations-dir %s\n", databaseURL, outputDir)
	fmt.Println()
	fmt.Println("To rollback the migration:")
	fmt.Printf("  go run ./cmd migrate-down --db-url %s --migrations-dir %s --target <previous_version>\n", databaseURL, outputDir)
}

// maskPassword masks the password in a database URL for display purposes
func maskPassword(url string) string {
	// Simple password masking - in a real implementation you might want more sophisticated parsing
	// This is just for display purposes to avoid showing passwords in logs
	
	// Find the pattern user:password@
	start := -1
	end := -1
	
	for i := 0; i < len(url)-1; i++ {
		if url[i] == ':' && url[i+1] == '/' && i+2 < len(url) && url[i+2] == '/' {
			// Found ://
			start = i + 3
			break
		}
	}
	
	if start == -1 {
		return url // No protocol found
	}
	
	// Find the @ symbol after the protocol
	for i := start; i < len(url); i++ {
		if url[i] == '@' {
			end = i
			break
		}
	}
	
	if end == -1 {
		return url // No @ found, probably no password
	}
	
	// Find the : between user and password
	colonPos := -1
	for i := start; i < end; i++ {
		if url[i] == ':' {
			colonPos = i
			break
		}
	}
	
	if colonPos == -1 {
		return url // No : found, probably no password
	}
	
	// Replace password with asterisks
	masked := url[:colonPos+1] + "****" + url[end:]
	return masked
}
