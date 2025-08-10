package ptah

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/lib/pq" // PostgreSQL driver for database/sql
	"github.com/stokaro/ptah/core/goschema"
	"github.com/stokaro/ptah/dbschema"
	"github.com/stokaro/ptah/migration/generator"
	"github.com/stokaro/ptah/migration/migrator"

	"github.com/denisvmedia/inventario/internal/errkit"
)

// PtahMigrator provides a simple interface to Ptah's migration capabilities
type PtahMigrator struct {
	dbURL     string
	schemaDir string
}

// NewPtahMigrator creates a new Ptah-based migrator
func NewPtahMigrator(dbURL string, schemaDir string) (*PtahMigrator, error) {
	return &PtahMigrator{
		dbURL:     dbURL,
		schemaDir: schemaDir,
	}, nil
}

// GenerateMigrationFiles generates timestamped migration files using Ptah's native generator
func (m *PtahMigrator) GenerateMigrationFiles(ctx context.Context, migrationName string) (*generator.MigrationFiles, error) {
	fmt.Println("=== GENERATE MIGRATION FILES ===")   //nolint:forbidigo // CLI output is OK //nolint:forbidigo // CLI output is OK
	fmt.Printf("Schema directory: %s\n", m.schemaDir) //nolint:forbidigo // CLI output is OK //nolint:forbidigo // CLI output is OK
	fmt.Printf("Migration name: %s\n", migrationName) //nolint:forbidigo // CLI output is OK //nolint:forbidigo // CLI output is OK
	fmt.Println()                                     //nolint:forbidigo // CLI output is OK //nolint:forbidigo // CLI output is OK

	// Determine output directory for migration files
	outputDir := filepath.Join(".", "registry", "ptah", "migrations", "source")

	// Ensure output directory exists
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create output directory")
	}

	// Connect to database first
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to connect to database")
	}
	defer conn.Close()

	// Use Ptah's native migration generator with database connection
	opts := generator.GenerateMigrationOptions{
		RootDir:       m.schemaDir,
		DatabaseURL:   m.dbURL,
		DBConn:        conn,
		MigrationName: migrationName,
		OutputDir:     outputDir,
	}

	files, err := generator.GenerateMigration(opts)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to generate migration files")
	}

	// Check if no migration was needed (files will be nil when no changes detected)
	if files == nil {
		fmt.Printf("✅ No schema changes detected - no migration files generated\n") //nolint:forbidigo // CLI output is OK
		return nil, nil
	}

	fmt.Printf("✅ Generated migration files:\n") //nolint:forbidigo // CLI output is OK
	fmt.Printf("  UP:   %s\n", files.UpFile)     //nolint:forbidigo // CLI output is OK
	fmt.Printf("  DOWN: %s\n", files.DownFile)   //nolint:forbidigo // CLI output is OK
	fmt.Printf("  Version: %d\n", files.Version) //nolint:forbidigo // CLI output is OK

	return files, nil
}

// GenerateInitialMigration generates the initial migration for an empty database
func (m *PtahMigrator) GenerateInitialMigration(ctx context.Context) (*generator.MigrationFiles, error) {
	fmt.Println("=== GENERATE INITIAL MIGRATION ===") //nolint:forbidigo // CLI output is OK
	fmt.Printf("Schema directory: %s\n", m.schemaDir) //nolint:forbidigo // CLI output is OK
	fmt.Println()                                     //nolint:forbidigo // CLI output is OK

	// Generate initial migration using Ptah's generator
	return m.GenerateMigrationFiles(ctx, "initial_schema")
}

// MigrateUp applies migrations using embedded migrations or file-based migrations
func (m *PtahMigrator) MigrateUp(ctx context.Context, dryRun bool) error { //nolint:revive // dryRun flag is appropriate for CLI
	fmt.Println("=== MIGRATE UP ===") //nolint:forbidigo // CLI output is OK

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")                           //nolint:forbidigo // CLI output is OK
		fmt.Println("No actual changes will be made to the database") //nolint:forbidigo // CLI output is OK
		fmt.Println()                                                 //nolint:forbidigo // CLI output is OK
	}

	// Connect to database using standard ptah approach
	// When using a shared pool, we still create a separate connection for migrations
	// but the pool limits will prevent connection exhaustion
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return errkit.Wrap(err, "failed to connect to database")
	}
	defer conn.Close()

	// Create migrator
	ptahMigrator := migrator.NewMigrator(conn)

	// Try embedded migrations first
	if HasEmbeddedMigrations() {
		fmt.Println("Using embedded migrations...") //nolint:forbidigo // CLI output is OK
		err := RegisterEmbeddedMigrations(ptahMigrator)
		if err != nil {
			return errkit.Wrap(err, "failed to register embedded migrations")
		}
	} else {
		// Fallback to file-based migrations
		fmt.Println("Using file-based migrations...") //nolint:forbidigo // CLI output is OK
		migrationsDir := filepath.Join(".", "registry", "ptah", "migrations", "source")
		entries, err := os.ReadDir(migrationsDir)
		if err != nil || len(entries) == 0 {
			fmt.Println("No migration files found. Use 'migrate generate --initial' to create initial migration.") //nolint:forbidigo // CLI output is OK
			return nil
		}

		err = migrator.RegisterMigrationsFromDirectory(ptahMigrator, migrationsDir)
		if err != nil {
			return errkit.Wrap(err, "failed to register migrations from directory")
		}
	}

	if dryRun {
		fmt.Println("✅ Dry run completed successfully!") //nolint:forbidigo // CLI output is OK
		return nil
	}

	// Apply migrations
	err = ptahMigrator.MigrateUp(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to run migrations")
	}

	fmt.Println("✅ Migrations completed successfully!") //nolint:forbidigo // CLI output is OK
	return nil
}

// MigrateDown is not supported with Ptah's file-based migrations
func (m *PtahMigrator) MigrateDown(ctx context.Context, targetVersion int, dryRun bool, confirm bool) error {
	return fmt.Errorf("rollback migrations are not supported with Ptah's file-based migrations")
}

// ResetDatabase drops all tables and recreates the schema from scratch
func (m *PtahMigrator) ResetDatabase(ctx context.Context, dryRun bool, confirm bool) error { //nolint:revive // CLI flags are appropriate
	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")                           //nolint:forbidigo // CLI output is OK
		fmt.Println("No actual changes will be made to the database") //nolint:forbidigo // CLI output is OK
		fmt.Println()                                                 //nolint:forbidigo // CLI output is OK
	}

	// First drop all tables
	err := m.DropDatabase(ctx, dryRun, confirm)
	if err != nil {
		return errkit.Wrap(err, "failed to drop database tables")
	}

	if dryRun {
		fmt.Println("After dropping tables, would apply all migrations...") //nolint:forbidigo // CLI output is OK
		fmt.Println("✅ Dry run completed successfully!")                    //nolint:forbidigo // CLI output is OK
		return nil
	}

	fmt.Println()                                          //nolint:forbidigo // CLI output is OK
	fmt.Println("=== RECREATING SCHEMA ===")               //nolint:forbidigo // CLI output is OK
	fmt.Println("Applying all migrations from scratch...") //nolint:forbidigo // CLI output is OK
	fmt.Println()                                          //nolint:forbidigo // CLI output is OK

	// Then apply all migrations
	err = m.MigrateUp(ctx, false)
	if err != nil {
		return errkit.Wrap(err, "failed to recreate schema")
	}

	fmt.Println("✅ Database reset completed successfully!") //nolint:forbidigo // CLI output is OK
	return nil
}

// DropDatabase drops all tables, indexes, and constraints
func (m *PtahMigrator) DropDatabase(ctx context.Context, dryRun bool, confirm bool) error { //nolint:revive // CLI flags are appropriate
	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")                           //nolint:forbidigo // CLI output is OK
		fmt.Println("No actual changes will be made to the database") //nolint:forbidigo // CLI output is OK
		fmt.Println()                                                 //nolint:forbidigo // CLI output is OK
	}

	// Create direct database connection for drop operations
	db, err := sql.Open("postgres", m.dbURL)
	if err != nil {
		return errkit.Wrap(err, "failed to connect to database")
	}
	defer db.Close()

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		return errkit.Wrap(err, "failed to ping database")
	}

	// Get list of all tables
	tables, err := m.getAllTables(ctx, db)
	if err != nil {
		return errkit.Wrap(err, "failed to get table list")
	}

	if len(tables) == 0 {
		fmt.Println("No tables found in database.") //nolint:forbidigo // CLI output is OK
		return nil
	}

	fmt.Printf("Found %d tables to drop:\n", len(tables)) //nolint:forbidigo // CLI output is OK
	for _, table := range tables {
		fmt.Printf("  - %s\n", table) //nolint:forbidigo // CLI output is OK
	}
	fmt.Println() //nolint:forbidigo // CLI output is OK

	// Confirmation prompt
	if !confirm && !dryRun {
		fmt.Print("⚠️  WARNING: This will DELETE ALL DATA and SCHEMA in the database!\n") //nolint:forbidigo // CLI output is OK
		fmt.Print("Are you sure you want to continue? (type 'yes' to confirm): ")         //nolint:forbidigo // CLI output is OK

		var response string
		fmt.Scanln(&response)

		if response != "yes" {
			fmt.Println("Operation cancelled.") //nolint:forbidigo // CLI output is OK
			return nil
		}
		fmt.Println() //nolint:forbidigo // CLI output is OK
	}

	if dryRun {
		fmt.Println("Would drop all tables and their data...") //nolint:forbidigo // CLI output is OK
		fmt.Println("✅ Dry run completed successfully!")       //nolint:forbidigo // CLI output is OK
		return nil
	}

	// Drop all tables
	fmt.Println("Dropping all tables...") //nolint:forbidigo // CLI output is OK
	err = m.dropAllTables(ctx, db, tables)
	if err != nil {
		return errkit.Wrap(err, "failed to drop tables")
	}

	fmt.Println("✅ All tables dropped successfully!") //nolint:forbidigo // CLI output is OK
	return nil
}

// GenerateSchemaSQL generates complete schema SQL from Go annotations (for preview)
func (m *PtahMigrator) GenerateSchemaSQL(ctx context.Context) ([]string, error) {
	fmt.Println("=== GENERATE SCHEMA SQL ===")        //nolint:forbidigo // CLI output is OK
	fmt.Printf("Schema directory: %s\n", m.schemaDir) //nolint:forbidigo // CLI output is OK
	fmt.Println()                                     //nolint:forbidigo // CLI output is OK

	// Parse Go entities from models directory
	goSchema, err := goschema.ParseDir(m.schemaDir)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to parse Go schema")
	}

	fmt.Printf("Found %d tables, %d fields, %d indexes, %d enums, %d extensions\n", //nolint:forbidigo // CLI output is OK
		len(goSchema.Tables), len(goSchema.Fields), len(goSchema.Indexes),
		len(goSchema.Enums), len(goSchema.Extensions))

	// For now, return a simple message indicating the schema was parsed
	// The actual SQL generation is handled by Ptah's migration generator
	fmt.Printf("Schema parsed successfully - use migration generation for SQL output\n") //nolint:forbidigo // CLI output is OK
	return []string{"-- Schema parsed successfully"}, nil
}

// PrintMigrationStatus prints detailed migration status information
func (m *PtahMigrator) PrintMigrationStatus(ctx context.Context, verbose bool) error { //nolint:revive // verbose flag is appropriate for CLI
	fmt.Println("=== MIGRATION STATUS ===")                           //nolint:forbidigo // CLI output is OK
	fmt.Printf("Database: %s\n", dbschema.FormatDatabaseURL(m.dbURL)) //nolint:forbidigo // CLI output is OK
	fmt.Printf("Schema source: %s\n", m.schemaDir)                    //nolint:forbidigo // CLI output is OK
	fmt.Println()                                                     //nolint:forbidigo // CLI output is OK

	// Check if migration files exist
	migrationsDir := filepath.Join(".", "registry", "ptah", "migrations", "source")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil || len(entries) == 0 {
		fmt.Println("Status: ⚠️  No migration files found")                          //nolint:forbidigo // CLI output is OK
		fmt.Println("Use 'migrate generate --initial' to create initial migration.") //nolint:forbidigo // CLI output is OK
		return nil
	}

	fmt.Printf("Status: ✅ Migration files found (%d files)\n", len(entries)) //nolint:forbidigo // CLI output is OK

	if verbose {
		fmt.Println("\nMigration files:") //nolint:forbidigo // CLI output is OK
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
				fmt.Printf("  - %s\n", entry.Name()) //nolint:forbidigo // CLI output is OK
			}
		}
	}

	return nil
}

// getAllTables gets a list of all user tables in the database
func (m *PtahMigrator) getAllTables(ctx context.Context, db *sql.DB) ([]string, error) {
	query := `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to query tables")
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, errkit.Wrap(err, "failed to scan table name")
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating table rows")
	}

	return tables, nil
}

// dropAllTables drops all tables in the correct order (handling foreign key constraints)
func (m *PtahMigrator) dropAllTables(ctx context.Context, db *sql.DB, tables []string) error {
	// Drop all tables with CASCADE to handle foreign key constraints
	for _, table := range tables {
		fmt.Printf("Dropping table: %s\n", table) //nolint:forbidigo // CLI output is OK

		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)
		_, err := db.ExecContext(ctx, dropSQL)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to drop table %s", table))
		}
	}

	// Also drop any remaining sequences that might be left over
	fmt.Println("Cleaning up sequences...") //nolint:forbidigo // CLI output is OK
	cleanupSQL := `
		DO $$
		DECLARE
			seq_name TEXT;
		BEGIN
			FOR seq_name IN
				SELECT sequence_name
				FROM information_schema.sequences
				WHERE sequence_schema = 'public'
			LOOP
				EXECUTE 'DROP SEQUENCE IF EXISTS ' || seq_name || ' CASCADE';
			END LOOP;
		END $$;`

	_, err := db.ExecContext(ctx, cleanupSQL)
	if err != nil {
		// Don't fail if sequence cleanup fails - it's not critical
		fmt.Printf("Warning: Failed to clean up sequences: %v\n", err) //nolint:forbidigo // CLI output is OK
	}

	return nil
}
