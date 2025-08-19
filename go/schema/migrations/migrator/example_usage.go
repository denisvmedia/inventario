package migrator

import (
	"context"
	"fmt"
	"log"
)

// ExampleDropDatabase demonstrates how to use the DropDatabase method
func ExampleDropDatabase() {
	// Create a migrator instance with a PostgreSQL DSN
	dbURL := "postgres://user:password@localhost:5432/testdb?sslmode=disable"
	migrator := New(dbURL, nil)

	ctx := context.Background()

	// Example 1: Dry run mode (safe to test)
	fmt.Println("=== Example 1: Dry Run Mode ===")
	err := migrator.DropDatabase(ctx, true, false) // dryRun=true, confirm=false
	if err != nil {
		log.Printf("Dry run failed: %v", err)
	}

	// Example 2: With confirmation prompt (would prompt user in real scenario)
	fmt.Println("\n=== Example 2: With Confirmation ===")
	err = migrator.DropDatabase(ctx, false, false) // dryRun=false, confirm=false
	if err != nil {
		log.Printf("Drop with confirmation failed: %v", err)
	}

	// Example 3: Skip confirmation (dangerous - use with caution!)
	fmt.Println("\n=== Example 3: Skip Confirmation (Dangerous!) ===")
	err = migrator.DropDatabase(ctx, false, true) // dryRun=false, confirm=true
	if err != nil {
		log.Printf("Drop without confirmation failed: %v", err)
	}

	// Output:
	// === Example 1: Dry Run Mode ===
	// === DRY RUN MODE ===
	// No actual changes will be made to the database
	//
	// Target database: testdb
	//
	// Would drop database: testdb
	// ✅ Dry run completed successfully!
	//
	// === Example 2: With Confirmation ===
	// Target database: testdb
	//
	// ⚠️  WARNING: This will COMPLETELY DELETE the entire database and ALL its data!
	// This operation cannot be undone!
	// Are you sure you want to continue? (type 'yes' to confirm): [user input required]
	//
	// === Example 3: Skip Confirmation (Dangerous!) ===
	// Target database: testdb
	//
	// Terminating connections to database: testdb
	// Dropping database: testdb
	// ✅ Database dropped successfully!
}

// ExampleParsePostgreSQLDSN demonstrates DSN parsing functionality
func ExampleParsePostgreSQLDSN() {
	testCases := []string{
		"postgres://user:pass@localhost:5432/myapp?sslmode=disable",
		"postgresql://admin:secret@db.example.com:5432/production",
		"postgres://readonly:pass@localhost/analytics",
	}

	for _, dsn := range testCases {
		migrator := New(dsn, nil)
		
		// This would normally be called internally by DropDatabase
		dbName, adminDSN, err := migrator.parsePostgreSQLDSN()
		if err != nil {
			log.Printf("Failed to parse DSN %s: %v", dsn, err)
			continue
		}
		
		fmt.Printf("Original DSN: %s\n", dsn)
		fmt.Printf("Database Name: %s\n", dbName)
		fmt.Printf("Admin DSN: %s\n", adminDSN)
		fmt.Println()
	}

	// Output:
	// Original DSN: postgres://user:pass@localhost:5432/myapp?sslmode=disable
	// Database Name: myapp
	// Admin DSN: postgres://user:pass@localhost:5432/postgres?sslmode=disable
	//
	// Original DSN: postgresql://admin:secret@db.example.com:5432/production
	// Database Name: production
	// Admin DSN: postgresql://admin:secret@db.example.com:5432/postgres
	//
	// Original DSN: postgres://readonly:pass@localhost/analytics
	// Database Name: analytics
	// Admin DSN: postgres://readonly:pass@localhost/postgres
}
