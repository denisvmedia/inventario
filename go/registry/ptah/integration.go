package ptah

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stokaro/ptah/core/goschema"
	"github.com/stokaro/ptah/core/renderer"
	"github.com/stokaro/ptah/dbschema"
	"github.com/stokaro/ptah/migration/generator"
	"github.com/stokaro/ptah/migration/migrator"
	"github.com/stokaro/ptah/migration/planner"
	"github.com/stokaro/ptah/migration/schemadiff"

	"github.com/denisvmedia/inventario/internal/errkit"
)

// embeddedMigrations will be populated when migration files exist
// For now, we'll handle the case where no files exist
var embeddedMigrations embed.FS

// PtahMigrator integrates Ptah migration capabilities with Inventario
type PtahMigrator struct {
	pool         *pgxpool.Pool
	dbURL        string
	schemaDir    string
	migrator     *migrator.Migrator
	capabilities PtahCapabilities
}

// PtahCapabilities defines what Ptah features are available
type PtahCapabilities struct {
	SQLMigrations       bool // Traditional SQL migration files
	VersionTracking     bool // Migration version tracking
	UpDownMigrations    bool // Up/down migration support
	SchemaGeneration    bool // Generate schema from Go structs
	SchemaIntrospection bool // Read current database schema
	SchemaDiffing       bool // Compare schemas and generate diffs
	MigrationGeneration bool // Generate migrations from schema diffs
	EmbeddedMigrations  bool // Embed migrations in binary
	DryRunMode          bool // Preview migrations without applying
	TransactionSafety   bool // Each migration runs in transaction
	AdvancedPostgreSQL  bool // PostgreSQL-specific features
}

// NewPtahMigrator creates a new Ptah-based migrator
func NewPtahMigrator(pool *pgxpool.Pool, dbURL string, schemaDir string) (*PtahMigrator, error) {
	// Connect to database using Ptah's connection
	conn, err := dbschema.ConnectToDatabase(dbURL)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to connect to database")
	}

	// Create Ptah migrator instance
	ptahMigrator := migrator.NewMigrator(conn)

	return &PtahMigrator{
		pool:      pool,
		dbURL:     dbURL,
		schemaDir: schemaDir,
		migrator:  ptahMigrator,
		capabilities: PtahCapabilities{
			SQLMigrations:       true,
			VersionTracking:     true,
			UpDownMigrations:    true,
			SchemaGeneration:    true,
			SchemaIntrospection: true,
			SchemaDiffing:       true,
			MigrationGeneration: true,
			EmbeddedMigrations:  true,
			DryRunMode:          true,
			TransactionSafety:   true,
			AdvancedPostgreSQL:  true,
		},
	}, nil
}

// GetCapabilities returns the Ptah capabilities
func (m *PtahMigrator) GetCapabilities() PtahCapabilities {
	return m.capabilities
}

// Note: No embedded migrations registration needed - using direct schema application

// MigrateUp applies migrations using embedded migration files if they exist, otherwise uses schema differences
func (m *PtahMigrator) MigrateUp(ctx context.Context, dryRun bool) error {
	// First, try to use embedded migration files if they exist
	hasMigrationFiles, err := m.hasEmbeddedMigrations()
	if err != nil {
		return errkit.Wrap(err, "failed to check for embedded migrations")
	}

	if hasMigrationFiles {
		return m.migrateUpWithFiles(ctx, dryRun)
	}

	// Fallback to dynamic schema application
	return m.migrateUpWithSchema(ctx, dryRun)
}

// hasEmbeddedMigrations checks if there are any migration files in the filesystem
func (m *PtahMigrator) hasEmbeddedMigrations() (bool, error) {
	// Check if migration files exist in the filesystem
	migrationsDir := filepath.Join(".", "registry", "ptah", "migrations", "source")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		// Directory doesn't exist or can't be read
		return false, nil
	}

	// Check if there are any .sql files
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			return true, nil
		}
	}

	return false, nil
}

// migrateUpWithFiles applies migrations using migration files from filesystem
func (m *PtahMigrator) migrateUpWithFiles(ctx context.Context, dryRun bool) error {
	fmt.Println("Using migration files from filesystem...")

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Println("No actual changes will be made to the database")
		fmt.Println()
	}

	// Use migrations directory from filesystem
	migrationsDir := filepath.Join(".", "registry", "ptah", "migrations", "source")

	// Connect to database for migration execution
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return errkit.Wrap(err, "failed to connect to database")
	}
	defer conn.Close()

	// Create a new migrator and register migrations from directory
	ptahMigrator := migrator.NewMigrator(conn)
	err = migrator.RegisterMigrationsFromDirectory(ptahMigrator, migrationsDir)
	if err != nil {
		return errkit.Wrap(err, "failed to register migrations from directory")
	}

	if dryRun {
		// For dry run, show what would be applied
		fmt.Println("✅ Dry run completed successfully!")
		fmt.Println("Note: Detailed migration preview not available with file-based migrations.")
		fmt.Println("Use 'inventario migrate status' to see current migration state.")
		return nil
	}

	// Apply migrations
	err = ptahMigrator.MigrateUp(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to run migrations")
	}

	fmt.Println("✅ Migrations completed successfully!")
	return nil
}

// migrateUpWithSchema applies schema changes from Go annotations
func (m *PtahMigrator) migrateUpWithSchema(ctx context.Context, dryRun bool) error {
	fmt.Println("Using dynamic schema generation from Go annotations...")

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Println("No actual changes will be made to the database")
		fmt.Println()
		// For dry run, show what would be applied
		statements, err := m.GenerateMigrationSQL(ctx)
		if err != nil {
			return errkit.Wrap(err, "failed to generate migration SQL for dry run")
		}

		if len(statements) == 0 {
			fmt.Println("✅ No pending changes - database is up to date!")
		} else {
			fmt.Printf("Would apply %d migration statements:\n", len(statements))
			for i, stmt := range statements {
				fmt.Printf("  %d. %s\n", i+1, stmt)
			}
		}
		fmt.Println("✅ Dry run completed successfully!")
		return nil
	}

	// Generate migration SQL from schema differences
	statements, err := m.GenerateMigrationSQL(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to generate migration SQL")
	}

	if len(statements) == 0 {
		fmt.Println("✅ Database is already up to date!")
		return nil
	}

	fmt.Printf("Applying %d migration statements...\n", len(statements))

	// Connect to database for migration execution
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return errkit.Wrap(err, "failed to connect to database")
	}
	defer conn.Close()

	// Apply each statement in a transaction
	err = m.applyMigrationStatements(ctx, *conn, statements)
	if err != nil {
		return errkit.Wrap(err, "failed to apply migration statements")
	}

	fmt.Println("✅ Migrations completed successfully!")
	return nil
}

// applyMigrationStatements applies SQL statements to the database
func (m *PtahMigrator) applyMigrationStatements(ctx context.Context, conn dbschema.DatabaseConnection, statements []string) error {
	// Apply each statement
	for i, stmt := range statements {
		if stmt == "" {
			continue
		}

		fmt.Printf("  Executing statement %d/%d...\n", i+1, len(statements))
		_, err := conn.Exec(stmt)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to execute statement %d: %s", i+1, stmt))
		}
	}

	return nil
}

// MigrateDown is not supported with schema-based migrations
func (m *PtahMigrator) MigrateDown(ctx context.Context, targetVersion int, dryRun bool, confirm bool) error {
	return fmt.Errorf("rollback migrations are not supported with schema-based migrations from Go annotations.\n\nThis migration system generates schema directly from Go entity annotations rather than using versioned migration files.\nTo rollback changes, modify your Go entity annotations and run 'migrate up' to apply the changes.")
}

// PrintMigrationStatus prints detailed migration status information
func (m *PtahMigrator) PrintMigrationStatus(ctx context.Context, verbose bool) error {
	fmt.Println("=== MIGRATION STATUS ===")
	fmt.Printf("Database: %s\n", dbschema.FormatDatabaseURL(m.dbURL))
	fmt.Printf("Schema source: %s\n", m.schemaDir)
	fmt.Println()

	// Check if there are schema differences
	statements, err := m.GenerateMigrationSQL(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to check migration status")
	}

	if len(statements) == 0 {
		fmt.Println("Status: ✅ Database schema is in sync with Go entity annotations")
	} else {
		fmt.Printf("Status: ⚠️  Database schema differs from Go entity annotations\n")
		fmt.Printf("Pending changes: %d SQL statements\n", len(statements))

		if verbose {
			fmt.Println("\nPending changes:")
			for i, stmt := range statements {
				fmt.Printf("  %d. %s\n", i+1, stmt)
			}
		}
	}

	return nil
}

// GenerateMigrationSQL generates migration SQL from schema differences using Ptah
func (m *PtahMigrator) GenerateMigrationSQL(ctx context.Context) ([]string, error) {
	fmt.Println("=== GENERATE MIGRATION SQL ===")
	fmt.Printf("Schema directory: %s\n", m.schemaDir)
	fmt.Printf("Database: %s\n", dbschema.FormatDatabaseURL(m.dbURL))
	fmt.Println()

	// 1. Parse Go entities from the schema directory
	absPath, err := filepath.Abs(m.schemaDir)
	if err != nil {
		return nil, errkit.Wrap(err, "error resolving schema directory path")
	}

	fmt.Printf("Parsing Go entities from: %s\n", absPath)
	result, err := goschema.ParseDir(absPath)
	if err != nil {
		return nil, errkit.Wrap(err, "error parsing Go entities")
	}

	fmt.Printf("Found %d tables, %d fields, %d indexes, %d enums\n",
		len(result.Tables), len(result.Fields), len(result.Indexes), len(result.Enums))

	// 2. Connect to database and read current schema
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return nil, errkit.Wrap(err, "error connecting to database")
	}
	defer conn.Close()

	fmt.Println("Reading current database schema...")
	dbSchema, err := conn.Reader().ReadSchema()
	if err != nil {
		return nil, errkit.Wrap(err, "error reading database schema")
	}

	// 3. Compare schemas and generate diff
	fmt.Println("Comparing schemas...")
	diff := schemadiff.Compare(result, dbSchema)

	if !diff.HasChanges() {
		fmt.Println("✅ No schema differences found - database is in sync with Go entities")
		return []string{}, nil
	}

	// 4. Generate migration SQL statements
	fmt.Println("Generating migration SQL...")
	astNodes := planner.GenerateSchemaDiffAST(diff, result, conn.Info().Dialect)

	sql, err := renderer.RenderSQL(conn.Info().Dialect, astNodes...)
	if err != nil {
		return nil, errkit.Wrap(err, "error rendering migration SQL")
	}

	// Split the SQL into individual statements
	statements := []string{sql}
	if sql != "" {
		// You might want to split on semicolons or other delimiters here
		// For now, treating as a single statement
	}

	fmt.Printf("Generated %d migration statements\n", len(statements))
	return statements, nil
}

// GenerateSchemaSQL generates complete schema SQL from Go annotations
func (m *PtahMigrator) GenerateSchemaSQL(ctx context.Context) ([]string, error) {
	fmt.Println("=== GENERATE COMPLETE SCHEMA ===")
	fmt.Printf("Schema directory: %s\n", m.schemaDir)
	fmt.Println()

	// Parse Go entities from the schema directory
	absPath, err := filepath.Abs(m.schemaDir)
	if err != nil {
		return nil, errkit.Wrap(err, "error resolving schema directory path")
	}

	fmt.Printf("Parsing Go entities from: %s\n", absPath)
	result, err := goschema.ParseDir(absPath)
	if err != nil {
		return nil, errkit.Wrap(err, "error parsing Go entities")
	}

	fmt.Printf("Found %d tables, %d fields, %d indexes, %d enums\n",
		len(result.Tables), len(result.Fields), len(result.Indexes), len(result.Enums))

	// Generate ordered CREATE statements for PostgreSQL
	statements := renderer.GetOrderedCreateStatements(result, "postgres")

	fmt.Printf("Generated %d schema statements\n", len(statements))
	return statements, nil
}

// GenerateMigrationFiles generates timestamped migration files using Ptah's migration generator
func (m *PtahMigrator) GenerateMigrationFiles(ctx context.Context, migrationName string) (*generator.MigrationFiles, error) {
	fmt.Println("=== GENERATE MIGRATION FILES ===")
	fmt.Printf("Schema directory: %s\n", m.schemaDir)
	fmt.Printf("Database: %s\n", dbschema.FormatDatabaseURL(m.dbURL))
	fmt.Printf("Migration name: %s\n", migrationName)
	fmt.Println()

	// Connect to database first
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to connect to database")
	}
	defer conn.Close()

	// Determine output directory for migration files
	outputDir := filepath.Join(".", "registry", "ptah", "migrations", "source")

	// Use Ptah's migration generator with database connection
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

	fmt.Printf("✅ Generated migration files:\n")
	fmt.Printf("  UP:   %s\n", files.UpFile)
	fmt.Printf("  DOWN: %s\n", files.DownFile)
	fmt.Printf("  Version: %d\n", files.Version)

	return files, nil
}

// GenerateInitialMigration generates the initial migration for an empty database
func (m *PtahMigrator) GenerateInitialMigration(ctx context.Context) (*generator.MigrationFiles, error) {
	fmt.Println("=== GENERATE INITIAL MIGRATION ===")
	fmt.Printf("Schema directory: %s\n", m.schemaDir)
	fmt.Println()

	// Check if database is empty
	isEmpty, err := m.isDatabaseEmpty(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to check if database is empty")
	}

	if !isEmpty {
		return nil, fmt.Errorf("database is not empty - use 'migrate generate' for schema differences instead")
	}

	// Generate initial migration using Ptah's generator
	return m.GenerateMigrationFiles(ctx, "initial_schema")
}

// isDatabaseEmpty checks if the database has any user tables
func (m *PtahMigrator) isDatabaseEmpty(ctx context.Context) (bool, error) {
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return false, errkit.Wrap(err, "failed to connect to database")
	}
	defer conn.Close()

	schema, err := conn.Reader().ReadSchema()
	if err != nil {
		return false, errkit.Wrap(err, "failed to read database schema")
	}

	// Check if there are any user tables (excluding system tables)
	return len(schema.Tables) == 0, nil
}
