package migrator_test

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/migrator"
	migrator_examples "github.com/denisvmedia/inventario/ptah/examples/migrator"
)

// Example demonstrates how to use the migrator programmatically
func ExampleMigrator() {
	// This is a demonstration - in real usage you would have a valid database URL
	dbURL := "postgres://user:pass@localhost/db"
	
	// Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	// Create a migrator
	m := migrator.NewMigrator(conn)

	// Register a simple migration
	migration := &migrator.Migration{
		Version:     1,
		Description: "Create users table",
		Up: func(ctx context.Context, conn *executor.DatabaseConnection) error {
			return conn.Writer().ExecuteSQL(`
				CREATE TABLE users (
					id SERIAL PRIMARY KEY,
					email VARCHAR(255) NOT NULL UNIQUE,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`)
		},
		Down: func(ctx context.Context, conn *executor.DatabaseConnection) error {
			return conn.Writer().ExecuteSQL("DROP TABLE users")
		},
	}

	m.Register(migration)

	// Run migrations
	err = m.MigrateUp(context.Background())
	if err != nil {
		fmt.Printf("Migration failed: %v\n", err)
		return
	}

	fmt.Println("Migration completed successfully")
}

// Example demonstrates how to use the high-level migration functions
func ExampleRunMigrations() {
	// This is a demonstration - in real usage you would have a valid database URL
	dbURL := "postgres://user:pass@localhost/db"

	// Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	// Get example migrations filesystem
	exampleFS := migrator_examples.GetExampleMigrations()
	migrationsFS := must.Must(fs.Sub(exampleFS, "migrations"))

	// Run all migrations from the filesystem
	err = migrator.RunMigrations(context.Background(), conn, migrationsFS)
	if err != nil {
		fmt.Printf("Migration failed: %v\n", err)
		return
	}

	fmt.Println("All migrations completed successfully")
}

// Example demonstrates how to check migration status
func ExampleGetMigrationStatus() {
	// This is a demonstration - in real usage you would have a valid database URL
	dbURL := "postgres://user:pass@localhost/db"

	// Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	// Get example migrations filesystem
	exampleFS := migrator_examples.GetExampleMigrations()
	migrationsFS := must.Must(fs.Sub(exampleFS, "migrations"))

	// Get migration status
	status, err := migrator.GetMigrationStatus(context.Background(), conn, migrationsFS)
	if err != nil {
		fmt.Printf("Failed to get status: %v\n", err)
		return
	}

	fmt.Printf("Current version: %d\n", status.CurrentVersion)
	fmt.Printf("Total migrations: %d\n", status.TotalMigrations)
	fmt.Printf("Pending migrations: %d\n", len(status.PendingMigrations))
	fmt.Printf("Has pending changes: %t\n", status.HasPendingChanges)
}

// Example demonstrates how to create migrations from SQL strings
func ExampleCreateMigrationFromSQL() {
	upSQL := `
		CREATE TABLE products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			price DECIMAL(10,2) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX idx_products_name ON products(name);
	`

	downSQL := `
		DROP INDEX IF EXISTS idx_products_name;
		DROP TABLE IF EXISTS products;
	`

	migration := migrator.CreateMigrationFromSQL(2, "Create products table", upSQL, downSQL)

	fmt.Printf("Migration version: %d\n", migration.Version)
	fmt.Printf("Migration description: %s\n", migration.Description)
	fmt.Printf("Has up function: %t\n", migration.Up != nil)
	fmt.Printf("Has down function: %t\n", migration.Down != nil)

	// Output:
	// Migration version: 2
	// Migration description: Create products table
	// Has up function: true
	// Has down function: true
}

// Example demonstrates how to register migrations from different filesystems
func Example_registerMigrationsCustomFilesystem() {
	// This is a demonstration - in real usage you would have a valid database URL
	dbURL := "postgres://user:pass@localhost/db"

	// Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	// Create a migrator
	m := migrator.NewMigrator(conn)

	// Option 1: Register from example migrations
	exampleFS := migrator_examples.GetExampleMigrations()
	migrationsFS := must.Must(fs.Sub(exampleFS, "migrations"))
	err = migrator.RegisterMigrations(m, migrationsFS)
	if err != nil {
		fmt.Printf("Failed to register example migrations: %v\n", err)
		return
	}

	// Option 2: Register from a directory on disk
	// err = migrator.RegisterMigrationsFromDirectory(m, "/path/to/migrations")

	// Option 3: Register from a custom filesystem
	// customFS := os.DirFS("/custom/path")
	// err = migrator.RegisterMigrations(m, customFS)

	fmt.Println("Migrations registered successfully")
}
