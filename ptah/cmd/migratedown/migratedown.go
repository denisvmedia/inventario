package migratedown

import (
	"context"
	"fmt"
	"os"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/migrator"
)

var migrateDownCmd = &cobra.Command{
	Use:   "migrate-down",
	Short: "Roll back migrations to a specific version",
	Long: `Roll back database migrations to a specific target version.

This command applies down migrations to revert the database schema to an earlier
version. All migrations with versions higher than the target version will be
rolled back in reverse order.

Each migration rollback is run in a transaction, so if any rollback fails, it will
be rolled back and the migration process will stop.

⚠️  WARNING: This operation can result in data loss! Make sure you have backups
before running down migrations in production.`,
	RunE: migrateDownCommand,
}

const (
	dbURLFlag        = "db-url"
	migrationsFlag   = "migrations-dir"
	targetFlag       = "target"
	dryRunFlag       = "dry-run"
	verboseFlag      = "verbose"
	confirmFlag      = "confirm"
)

var migrateDownFlags = map[string]cobraflags.Flag{
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
	targetFlag: &cobraflags.IntFlag{
		Name:  targetFlag,
		Value: 0,
		Usage: "Target version to migrate down to (required)",
	},
	dryRunFlag: &cobraflags.BoolFlag{
		Name:  dryRunFlag,
		Value: false,
		Usage: "Show what migrations would be rolled back without actually running them",
	},
	verboseFlag: &cobraflags.BoolFlag{
		Name:  verboseFlag,
		Value: false,
		Usage: "Enable verbose output",
	},
	confirmFlag: &cobraflags.BoolFlag{
		Name:  confirmFlag,
		Value: false,
		Usage: "Skip confirmation prompt (use with caution!)",
	},
}

func NewMigrateDownCommand() *cobra.Command {
	cobraflags.RegisterMap(migrateDownCmd, migrateDownFlags)
	return migrateDownCmd
}

func migrateDownCommand(_ *cobra.Command, _ []string) error {
	dbURL := migrateDownFlags[dbURLFlag].GetString()
	migrationsDir := migrateDownFlags[migrationsFlag].GetString()
	targetVersion := migrateDownFlags[targetFlag].GetInt()
	dryRun := migrateDownFlags[dryRunFlag].GetBool()
	verbose := migrateDownFlags[verboseFlag].GetBool()
	skipConfirm := migrateDownFlags[confirmFlag].GetBool()

	if dbURL == "" {
		return fmt.Errorf("database URL is required")
	}

	if migrationsDir == "" {
		return fmt.Errorf("migrations directory is required")
	}

	if targetVersion < 0 {
		return fmt.Errorf("target version must be >= 0")
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

	fmt.Println("=== MIGRATE DOWN ===")
	fmt.Printf("Database: %s\n", executor.FormatDatabaseURL(dbURL))
	fmt.Printf("Dialect: %s\n", conn.Info().Dialect)
	fmt.Printf("Migrations directory: %s\n", migrationsDir)
	fmt.Printf("Target version: %d\n", targetVersion)
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

	if status.CurrentVersion <= targetVersion {
		fmt.Printf("✅ Database is already at or below target version %d!\n", targetVersion)
		return nil
	}

	// Calculate which migrations will be rolled back
	var migrationsToRollback []int
	for _, version := range status.PendingMigrations {
		if version > targetVersion && version <= status.CurrentVersion {
			migrationsToRollback = append(migrationsToRollback, version)
		}
	}

	// Also need to check applied migrations that are above target
	// This is a simplified approach - in practice you'd query the database
	// for applied migrations above the target version
	fmt.Printf("Migrations to roll back: %d\n", status.CurrentVersion-targetVersion)

	if verbose {
		fmt.Printf("Will roll back from version %d to %d\n", status.CurrentVersion, targetVersion)
	}

	fmt.Println()

	// Safety confirmation (unless skipped or dry run)
	if !dryRun && !skipConfirm {
		fmt.Println("⚠️  WARNING: Rolling back migrations can result in data loss!")
		fmt.Printf("This will roll back the database from version %d to version %d.\n", status.CurrentVersion, targetVersion)
		fmt.Print("Are you sure you want to continue? Type 'YES' to confirm: ")
		
		var confirmation string
		fmt.Scanln(&confirmation)
		
		if confirmation != "YES" {
			fmt.Println("Migration rollback cancelled.")
			return nil
		}
		fmt.Println()
	}

	// Run down migrations
	err = migrator.RunMigrationsDown(context.Background(), conn, targetVersion, migrationsFS)
	if err != nil {
		return fmt.Errorf("error running down migrations: %w", err)
	}

	// Get final status
	finalStatus, err := migrator.GetMigrationStatus(context.Background(), conn, migrationsFS)
	if err != nil {
		return fmt.Errorf("error getting final migration status: %w", err)
	}

	fmt.Println()
	if dryRun {
		fmt.Println("✅ Dry run completed successfully!")
		fmt.Printf("Would have rolled back to version: %d\n", targetVersion)
	} else {
		fmt.Println("✅ Migration rollback completed successfully!")
		fmt.Printf("Database is now at version: %d\n", finalStatus.CurrentVersion)
	}

	return nil
}
