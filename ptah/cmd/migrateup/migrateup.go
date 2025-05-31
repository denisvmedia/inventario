package migrateup

import (
	"context"
	"fmt"
	"os"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/migrator"
)

var migrateUpCmd = &cobra.Command{
	Use:   "migrate-up",
	Short: "Run pending migrations up to the latest version",
	Long: `Run all pending database migrations up to the latest version.

This command applies all migrations that haven't been applied yet, bringing
the database schema up to the latest version defined in the migration files.

Each migration is run in a transaction, so if any migration fails, it will
be rolled back and the migration process will stop.`,
	RunE: migrateUpCommand,
}

const (
	dbURLFlag        = "db-url"
	migrationsFlag   = "migrations-dir"
	dryRunFlag       = "dry-run"
	verboseFlag      = "verbose"
)

var migrateUpFlags = map[string]cobraflags.Flag{
	dbURLFlag: &cobraflags.StringFlag{
		Name:  dbURLFlag,
		Value: "",
		Usage: "Database URL (required). Example: postgres://user:pass@localhost/db",
	},
	migrationsFlag: &cobraflags.StringFlag{
		Name:  migrationsFlag,
		Value: "",
		Usage: "Directory containing migration files (required)",
	},
	dryRunFlag: &cobraflags.BoolFlag{
		Name:  dryRunFlag,
		Value: false,
		Usage: "Show what migrations would be applied without actually running them",
	},
	verboseFlag: &cobraflags.BoolFlag{
		Name:  verboseFlag,
		Value: false,
		Usage: "Enable verbose output",
	},
}

func NewMigrateUpCommand() *cobra.Command {
	cobraflags.RegisterMap(migrateUpCmd, migrateUpFlags)
	return migrateUpCmd
}

func migrateUpCommand(_ *cobra.Command, _ []string) error {
	dbURL := migrateUpFlags[dbURLFlag].GetString()
	migrationsDir := migrateUpFlags[migrationsFlag].GetString()
	dryRun := migrateUpFlags[dryRunFlag].GetBool()
	verbose := migrateUpFlags[verboseFlag].GetBool()

	if dbURL == "" {
		return fmt.Errorf("database URL is required")
	}

	if migrationsDir == "" {
		return fmt.Errorf("migrations directory is required")
	}

	if verbose {
		fmt.Printf("Connecting to database: %s\n", executor.FormatDatabaseURL(dbURL))
	}

	// Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer conn.Close()

	// Set dry run mode if requested
	conn.Writer().SetDryRun(dryRun)

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Println("No actual changes will be made to the database")
		fmt.Println()
	}

	fmt.Println("=== MIGRATE UP ===")
	fmt.Printf("Database: %s\n", executor.FormatDatabaseURL(dbURL))
	fmt.Printf("Dialect: %s\n", conn.Info().Dialect)
	fmt.Printf("Migrations directory: %s\n", migrationsDir)
	fmt.Println()

	// Create filesystem from migrations directory
	migrationsFS := os.DirFS(migrationsDir)

	// Get migration status before running
	status, err := migrator.GetMigrationStatus(context.Background(), conn, migrationsFS)
	if err != nil {
		return fmt.Errorf("error getting migration status: %w", err)
	}

	fmt.Printf("Current version: %d\n", status.CurrentVersion)
	fmt.Printf("Total migrations: %d\n", status.TotalMigrations)
	fmt.Printf("Pending migrations: %d\n", len(status.PendingMigrations))

	if !status.HasPendingChanges {
		fmt.Println("✅ Database is already up to date!")
		return nil
	}

	if verbose {
		fmt.Printf("Pending migration versions: %v\n", status.PendingMigrations)
	}

	fmt.Println()

	// Run migrations
	err = migrator.RunMigrations(context.Background(), conn, migrationsFS)
	if err != nil {
		return fmt.Errorf("error running migrations: %w", err)
	}

	// Get final status
	finalStatus, err := migrator.GetMigrationStatus(context.Background(), conn, migrationsFS)
	if err != nil {
		return fmt.Errorf("error getting final migration status: %w", err)
	}

	fmt.Println()
	if dryRun {
		fmt.Println("✅ Dry run completed successfully!")
		fmt.Printf("Would have applied %d migrations\n", len(status.PendingMigrations))
	} else {
		fmt.Println("✅ Migrations completed successfully!")
		fmt.Printf("Database is now at version: %d\n", finalStatus.CurrentVersion)
	}

	return nil
}
