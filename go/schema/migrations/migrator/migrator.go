package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	_ "github.com/lib/pq" // PostgreSQL driver for database/sql
	"github.com/stokaro/ptah/dbschema"
	"github.com/stokaro/ptah/migration/migrator"

	"github.com/denisvmedia/inventario/schema/migrations"
)

type Args struct {
	DryRun bool
}

// Migrator provides a simple interface to Ptah's migration capabilities
type Migrator struct {
	dbURL  string
	logger *slog.Logger
	migFS  fs.FS
}

// New creates a new Ptah-based migrator
//
// Parameters:
//   - dbURL:    PostgreSQL database connection string
func New(dbURL string, migFS fs.FS) *Migrator {
	return &Migrator{
		dbURL:  dbURL,
		logger: slog.Default(),
		migFS:  migFS,
	}
}

func NewWithFallback(dbURL, fallbackDir string) *Migrator {
	var migFS fs.FS
	if migrations.HasEmbeddedMigrations() {
		migFS = must.Must(migrations.EmbeddedMigrationsFS())
	} else {
		migFS = migrations.MigrationsFS(fallbackDir)
	}
	return New(dbURL, migFS)
}

func (m *Migrator) SetLogger(logger *slog.Logger) *Migrator {
	tmp := *m
	tmp.logger = logger
	return &tmp
}

// MigrateUp applies migrations using embedded migrations or file-based migrations
func (m *Migrator) MigrateUp(ctx context.Context, args Args) error {
	m.logger.Info("Applying migrations up", "db_url", m.dbURL, "dry_run", args.DryRun)

	if args.DryRun {
		m.logger.Info("Dry run mode enabled - no actual changes will be made")
	}

	// Connect to database using standard ptah approach
	// When using a shared pool, we still create a separate connection for migrations
	// but the pool limits will prevent connection exhaustion
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return errxtrace.Wrap("failed to connect to database", err)
	}
	defer conn.Close()

	// Create migrator
	ptahMigrator, err := migrator.NewFSMigrator(conn, m.migFS)
	if err != nil {
		return errxtrace.Wrap("failed to create Ptah migrator", err)
	}

	// Use file-based migrations from the provided filesystem
	fmt.Println("Using file-based migrations...")

	if args.DryRun {
		m.logger.Info("Dry run completed successfully")
		return nil
	}

	// Apply migrations
	err = ptahMigrator.MigrateUp(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to run migrations", err)
	}

	m.logger.Info("Migrations completed successfully")
	return nil
}

// MigrateDown is not supported with Ptah's file-based migrations
func (m *Migrator) MigrateDown(ctx context.Context, targetVersion int, dryRun bool, confirm bool) error {
	return fmt.Errorf("rollback migrations are supported by Ptah, but integration is not implemented yet")
}

// ResetDatabase drops all tables and recreates the schema from scratch
func (m *Migrator) ResetDatabase(ctx context.Context, args Args, confirm bool) error {
	if args.DryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Println("No actual changes will be made to the database")
		fmt.Println()
	}

	// First drop all tables
	err := m.DropTables(ctx, args.DryRun, confirm)
	if err != nil {
		return errxtrace.Wrap("failed to drop database tables", err)
	}

	if args.DryRun {
		fmt.Println("After dropping tables, would apply all migrations...")
		fmt.Println("✅ Dry run completed successfully!")
		return nil
	}

	fmt.Println()
	fmt.Println("=== RECREATING SCHEMA ===")
	fmt.Println("Applying all migrations from scratch...")
	fmt.Println()

	// Then apply all migrations
	err = m.MigrateUp(ctx, Args{
		DryRun: args.DryRun,
	})
	if err != nil {
		return errxtrace.Wrap("failed to recreate schema", err)
	}

	fmt.Println("✅ Database reset completed successfully!")
	return nil
}

// DropTables drops all tables, indexes, and constraints
//
//revive:disable-next-line:flag-parameter CLI flags are appropriate
func (m *Migrator) DropTables(ctx context.Context, dryRun bool, confirm bool) error {
	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Println("No actual changes will be made to the database")
		fmt.Println()
	}

	// Create direct database connection for drop operations
	db, err := sql.Open("postgres", m.dbURL)
	if err != nil {
		return errxtrace.Wrap("failed to connect to database", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		return errxtrace.Wrap("failed to ping database", err)
	}

	// Get list of all tables
	tables, err := m.getAllTables(ctx, db)
	if err != nil {
		return errxtrace.Wrap("failed to get table list", err)
	}

	if len(tables) == 0 {
		fmt.Println("No tables found in database.")
		return nil
	}

	fmt.Printf("Found %d tables to drop:\n", len(tables))
	for _, table := range tables {
		fmt.Printf("  - %s\n", table)
	}
	fmt.Println()

	// Confirmation prompt
	if !confirm && !dryRun {
		fmt.Print("⚠️  WARNING: This will DELETE ALL DATA and SCHEMA in the database!\n")
		fmt.Print("Are you sure you want to continue? (type 'yes' to confirm): ")

		var response string
		fmt.Scanln(&response)

		if response != "yes" {
			fmt.Println("Operation cancelled.")
			return nil
		}
		fmt.Println()
	}

	if dryRun {
		fmt.Println("Would drop all tables and their data...")
		fmt.Println("✅ Dry run completed successfully!")
		return nil
	}

	// Drop all tables
	fmt.Println("Dropping all tables...")
	err = m.dropAllTables(ctx, db, tables)
	if err != nil {
		return errxtrace.Wrap("failed to drop tables", err)
	}

	fmt.Println("✅ All tables dropped successfully!")
	return nil
}

//revive:disable-next-line:flag-parameter CLI flags are appropriate
func (m *Migrator) DropDatabase(ctx context.Context, dryRun bool, confirm bool) error {
	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Println("No actual changes will be made to the database")
		fmt.Println()
	}

	// Parse the database URL to extract database name and connection info
	dbName, adminDSN, err := m.parsePostgreSQLDSN()
	if err != nil {
		return errxtrace.Wrap("failed to parse database DSN", err)
	}

	m.logger.Info("Preparing to drop database", "database", dbName, "dry_run", dryRun)

	fmt.Printf("Target database: %s\n", dbName)
	fmt.Println()

	// Confirmation prompt
	if !confirm && !dryRun {
		fmt.Print("⚠️  WARNING: This will COMPLETELY DELETE the entire database and ALL its data!\n")
		fmt.Print("This operation cannot be undone!\n")
		fmt.Print("Are you sure you want to continue? (type 'yes' to confirm): ")

		var response string
		fmt.Scanln(&response)

		if response != "yes" {
			fmt.Println("Operation cancelled.")
			return nil
		}
		fmt.Println()
	}

	if dryRun {
		fmt.Printf("Would drop database: %s\n", dbName)
		fmt.Println("✅ Dry run completed successfully!")
		return nil
	}

	// Connect to the postgres database (not the target database) to perform the drop
	db, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return errxtrace.Wrap("failed to connect to PostgreSQL admin database", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		return errxtrace.Wrap("failed to ping PostgreSQL admin database", err)
	}

	// Terminate all connections to the target database
	fmt.Printf("Terminating connections to database: %s\n", dbName)
	terminateSQL := `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = $1 AND pid <> pg_backend_pid()`

	_, err = db.ExecContext(ctx, terminateSQL, dbName)
	if err != nil {
		// Don't fail if we can't terminate connections - the drop might still work
		fmt.Printf("Warning: Failed to terminate connections: %v\n", err)
	}

	// Drop the database
	fmt.Printf("Dropping database: %s\n", dbName)
	dropSQL := fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)
	_, err = db.ExecContext(ctx, dropSQL)
	if err != nil {
		return errxtrace.Wrap(fmt.Sprintf("failed to drop database %s", dbName), err)
	}

	fmt.Println("✅ Database dropped successfully!")
	return nil
}

// PrintMigrationStatus prints detailed migration status information
func (m *Migrator) PrintMigrationStatus(ctx context.Context, verbose bool) error { //revive:disable:flag-parameter
	fmt.Println("=== MIGRATION STATUS ===")
	fmt.Printf("Database: %s\n", dbschema.FormatDatabaseURL(m.dbURL))
	fmt.Println("Schema source: File system")
	fmt.Println()

	// Check if migration files exist
	entries, err := fs.ReadDir(m.migFS, ".")
	if err != nil || len(entries) == 0 {
		fmt.Println("Status: ⚠️  No migration files found")
		fmt.Println("Use 'migrate generate --initial' to create initial migration.")
		return nil
	}

	fmt.Printf("Status: ✅ Migration files found (%d files)\n", len(entries))

	if verbose {
		fmt.Println("\nMigration files:")
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
				fmt.Printf("  - %s\n", entry.Name())
			}
		}
	}

	return nil
}

// getAllTables gets a list of all user tables in the database
func (m *Migrator) getAllTables(ctx context.Context, db *sql.DB) ([]string, error) {
	query := `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errxtrace.Wrap("failed to query tables", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, errxtrace.Wrap("failed to scan table name", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, errxtrace.Wrap("error iterating table rows", err)
	}

	return tables, nil
}

// dropAllTables drops all tables in the correct order (handling foreign key constraints)
func (m *Migrator) dropAllTables(ctx context.Context, db *sql.DB, tables []string) error {
	// Drop all tables with CASCADE to handle foreign key constraints
	for _, table := range tables {
		fmt.Printf("Dropping table: %s\n", table)

		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)
		_, err := db.ExecContext(ctx, dropSQL)
		if err != nil {
			return errxtrace.Wrap(fmt.Sprintf("failed to drop table %s", table), err)
		}
	}

	// Also drop any remaining sequences that might be left over
	fmt.Println("Cleaning up sequences...")
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
		fmt.Printf("Warning: Failed to clean up sequences: %v\n", err)
	}

	return nil
}

// parsePostgreSQLDSN parses a PostgreSQL DSN and returns the database name and an admin DSN
// that connects to the 'postgres' database for administrative operations
func (m *Migrator) parsePostgreSQLDSN() (dbName string, adminDSN string, err error) {
	parsed, err := url.Parse(m.dbURL)
	if err != nil {
		return "", "", errxtrace.Wrap("failed to parse database URL", err)
	}

	// Validate that this is a PostgreSQL DSN
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return "", "", fmt.Errorf("unsupported database scheme: %s (only PostgreSQL is supported)", parsed.Scheme)
	}

	// Extract database name from path (remove leading slash)
	dbName = strings.TrimPrefix(parsed.Path, "/")
	if dbName == "" {
		return "", "", fmt.Errorf("database name not found in DSN path")
	}

	// Create admin DSN by replacing the database name with 'postgres'
	adminURL := *parsed
	adminURL.Path = "/postgres"
	adminDSN = adminURL.String()

	return dbName, adminDSN, nil
}
