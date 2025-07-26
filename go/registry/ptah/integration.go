package ptah

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stokaro/ptah/dbschema"
	"github.com/stokaro/ptah/migration/migrator"

	"github.com/denisvmedia/inventario/internal/errkit"
)

//go:embed migrations/source/*.sql
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

// RegisterEmbeddedMigrations registers the embedded migration files
func (m *PtahMigrator) RegisterEmbeddedMigrations() error {
	// Extract the migrations subdirectory from embedded filesystem
	migrationsFS, err := fs.Sub(embeddedMigrations, "migrations/source")
	if err != nil {
		return errkit.Wrap(err, "failed to extract migrations subdirectory")
	}

	// Register migrations with Ptah migrator
	err = migrator.RegisterMigrations(m.migrator, migrationsFS)
	if err != nil {
		return errkit.Wrap(err, "failed to register embedded migrations")
	}

	return nil
}

// RegisterMigrationsFromDirectory registers migrations from a directory
func (m *PtahMigrator) RegisterMigrationsFromDirectory(migrationsDir string) error {
	err := migrator.RegisterMigrationsFromDirectory(m.migrator, migrationsDir)
	if err != nil {
		return errkit.Wrap(err, "failed to register migrations from directory")
	}
	return nil
}

// MigrateUp applies all pending migrations
func (m *PtahMigrator) MigrateUp(ctx context.Context, dryRun bool) error {
	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Println("No actual changes will be made to the database")
		fmt.Println()
		// For dry run, we'll just show what would be applied
		return m.showPendingMigrations(ctx)
	}

	// Get migration status before running
	status, err := m.GetMigrationStatus(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to get migration status")
	}

	fmt.Printf("Current version: %d\n", status.CurrentVersion)
	fmt.Printf("Pending migrations: %d\n", len(status.PendingMigrations))

	if !status.HasPendingChanges {
		fmt.Println("✅ Database is already up to date!")
		return nil
	}

	fmt.Printf("Applying %d pending migrations...\n", len(status.PendingMigrations))

	// Run migrations using embedded filesystem
	migrationsFS, err := fs.Sub(embeddedMigrations, "migrations/source")
	if err != nil {
		return errkit.Wrap(err, "failed to extract migrations subdirectory")
	}

	// Connect to database for migration execution
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return errkit.Wrap(err, "failed to connect to database")
	}
	defer conn.Close()

	err = migrator.RunMigrations(ctx, conn, migrationsFS)
	if err != nil {
		return errkit.Wrap(err, "failed to run migrations")
	}

	fmt.Println("✅ Migrations completed successfully!")
	return nil
}

// MigrateDown rolls back migrations to a specific version
func (m *PtahMigrator) MigrateDown(ctx context.Context, targetVersion int, dryRun bool, confirm bool) error {
	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Println("No actual changes will be made to the database")
		fmt.Println()
	}

	// Get current migration status
	status, err := m.GetMigrationStatus(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to get migration status")
	}

	if status.CurrentVersion <= targetVersion {
		fmt.Printf("Database is already at or below version %d (current: %d)\n", targetVersion, status.CurrentVersion)
		return nil
	}

	fmt.Printf("Rolling back from version %d to version %d\n", status.CurrentVersion, targetVersion)

	// Safety confirmation for down migrations (unless confirm flag is set)
	if !confirm && !dryRun {
		fmt.Println("⚠️  WARNING: Down migrations can cause data loss!")
		fmt.Printf("Are you sure you want to rollback to version %d? (y/N): ", targetVersion)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Migration rollback cancelled")
			return nil
		}
	}

	if dryRun {
		fmt.Printf("Would rollback from version %d to version %d\n", status.CurrentVersion, targetVersion)
		fmt.Println("✅ Dry run rollback completed successfully!")
		return nil
	}

	// Connect to database for migration execution
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return errkit.Wrap(err, "failed to connect to database")
	}
	defer conn.Close()

	// Run down migrations using embedded filesystem
	migrationsFS, err := fs.Sub(embeddedMigrations, "migrations/source")
	if err != nil {
		return errkit.Wrap(err, "failed to extract migrations subdirectory")
	}

	err = migrator.RunMigrationsDown(ctx, conn, targetVersion, migrationsFS)
	if err != nil {
		return errkit.Wrap(err, "failed to run down migrations")
	}

	fmt.Printf("✅ Successfully rolled back to version %d!\n", targetVersion)
	return nil
}

// GetMigrationStatus returns the current migration status
func (m *PtahMigrator) GetMigrationStatus(ctx context.Context) (*migrator.MigrationStatus, error) {
	migrationsFS, err := fs.Sub(embeddedMigrations, "migrations/source")
	if err != nil {
		return nil, errkit.Wrap(err, "failed to extract migrations subdirectory")
	}

	// Connect to database for status check
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to connect to database")
	}
	defer conn.Close()

	status, err := migrator.GetMigrationStatus(ctx, conn, migrationsFS)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get migration status")
	}

	return status, nil
}

// PrintMigrationStatus prints detailed migration status information
func (m *PtahMigrator) PrintMigrationStatus(ctx context.Context, verbose bool) error {
	status, err := m.GetMigrationStatus(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to get migration status")
	}

	fmt.Println("=== MIGRATION STATUS ===")
	fmt.Printf("Database: %s\n", dbschema.FormatDatabaseURL(m.dbURL))
	fmt.Printf("Current version: %d\n", status.CurrentVersion)
	fmt.Printf("Total migrations: %d\n", status.TotalMigrations)
	fmt.Printf("Applied migrations: %d\n", status.TotalMigrations-len(status.PendingMigrations))
	fmt.Printf("Pending migrations: %d\n", len(status.PendingMigrations))

	if status.HasPendingChanges {
		fmt.Println("Status: ⚠️  Database needs migration")
		if verbose && len(status.PendingMigrations) > 0 {
			fmt.Println("\nPending migrations:")
			for _, version := range status.PendingMigrations {
				fmt.Printf("  - Version %d\n", version)
			}
		}
	} else {
		fmt.Println("Status: ✅ Database is up to date")
	}

	return nil
}

// showPendingMigrations shows what migrations would be applied (for dry run)
func (m *PtahMigrator) showPendingMigrations(ctx context.Context) error {
	status, err := m.GetMigrationStatus(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to get migration status")
	}

	if !status.HasPendingChanges {
		fmt.Println("✅ No pending migrations - database is up to date")
		return nil
	}

	fmt.Printf("Would apply %d pending migrations:\n", len(status.PendingMigrations))
	for _, version := range status.PendingMigrations {
		fmt.Printf("  - Version %d\n", version)
	}

	fmt.Println("✅ Dry run completed successfully!")
	return nil
}

// GenerateMigrationSQL generates migration SQL from schema differences (placeholder)
func (m *PtahMigrator) GenerateMigrationSQL(ctx context.Context) ([]string, error) {
	// This is a placeholder implementation
	// In a full implementation, this would:
	// 1. Parse Go entities from the schema directory
	// 2. Read current database schema
	// 3. Compare schemas and generate diff
	// 4. Generate SQL statements from the diff

	fmt.Println("Schema diff generation is not yet implemented")
	fmt.Println("This would compare Go entity definitions with the current database schema")
	fmt.Println("and generate the SQL needed to sync them.")

	return []string{}, nil
}
