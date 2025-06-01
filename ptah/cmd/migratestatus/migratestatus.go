package migratestatus

import (
	"context"
	"fmt"
	"os"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/dbschema"
	"github.com/denisvmedia/inventario/ptah/migration/migrator"
)

var migrateStatusCmd = &cobra.Command{
	Use:   "migrate-status",
	Short: "Show current migration status",
	Long: `Show the current migration status of the database.

This command displays information about:
- Current database schema version
- Total number of available migrations
- Number of pending migrations
- List of pending migration versions

This is useful for checking the state of your database before running
migrations or for debugging migration issues.`,
	RunE: migrateStatusCommand,
}

const (
	dbURLFlag      = "db-url"
	migrationsFlag = "migrations-dir"
	verboseFlag    = "verbose"
	jsonFlag       = "json"
)

var migrateStatusFlags = map[string]cobraflags.Flag{
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
	verboseFlag: &cobraflags.BoolFlag{
		Name:  verboseFlag,
		Value: false,
		Usage: "Enable verbose output with detailed migration information",
	},
	jsonFlag: &cobraflags.BoolFlag{
		Name:  jsonFlag,
		Value: false,
		Usage: "Output status in JSON format",
	},
}

func NewMigrateStatusCommand() *cobra.Command {
	cobraflags.RegisterMap(migrateStatusCmd, migrateStatusFlags)
	return migrateStatusCmd
}

func migrateStatusCommand(_ *cobra.Command, _ []string) error {
	dbURL := migrateStatusFlags[dbURLFlag].GetString()
	migrationsDir := migrateStatusFlags[migrationsFlag].GetString()
	verbose := migrateStatusFlags[verboseFlag].GetBool()
	jsonOutput := migrateStatusFlags[jsonFlag].GetBool()

	if dbURL == "" {
		return fmt.Errorf("database URL is required")
	}

	if migrationsDir == "" {
		return fmt.Errorf("migrations directory is required")
	}

	// Connect to database
	conn, err := dbschema.ConnectToDatabase(dbURL)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer conn.Close()

	// Create filesystem from migrations directory
	migrationsFS := os.DirFS(migrationsDir)

	// Get migration status
	status, err := migrator.GetMigrationStatus(context.Background(), conn, migrationsFS)
	if err != nil {
		return fmt.Errorf("error getting migration status: %w", err)
	}

	if jsonOutput {
		return outputJSON(status)
	}

	return outputHuman(status, conn, verbose)
}

func outputJSON(status *migrator.MigrationStatus) error {
	// Simple JSON output - in a real implementation you might want to use
	// a proper JSON marshaling library
	fmt.Printf(`{
  "current_version": %d,
  "total_migrations": %d,
  "pending_migrations": %v,
  "has_pending_changes": %t
}
`, status.CurrentVersion, status.TotalMigrations, status.PendingMigrations, status.HasPendingChanges)
	return nil
}

func outputHuman(status *migrator.MigrationStatus, conn *dbschema.DatabaseConnection, verbose bool) error {
	fmt.Println("=== MIGRATION STATUS ===")
	fmt.Printf("Database: %s\n", dbschema.FormatDatabaseURL("***"))
	fmt.Printf("Dialect: %s\n", conn.Info().Dialect)
	fmt.Printf("Schema: %s\n", conn.Info().Schema)
	fmt.Println()

	fmt.Printf("Current Version: %d\n", status.CurrentVersion)
	fmt.Printf("Total Migrations: %d\n", status.TotalMigrations)
	fmt.Printf("Pending Migrations: %d\n", len(status.PendingMigrations))

	if status.HasPendingChanges {
		fmt.Println("Status: ⚠️  Pending migrations available")

		if verbose && len(status.PendingMigrations) > 0 {
			fmt.Println("\nPending migration versions:")
			for _, version := range status.PendingMigrations {
				fmt.Printf("  - %d\n", version)
			}
		}

		fmt.Println("\nRun 'migrate-up' to apply pending migrations.")
	} else {
		fmt.Println("Status: ✅ Database is up to date")
	}

	if verbose {
		fmt.Println("\n=== DETAILED INFORMATION ===")

		if status.TotalMigrations == 0 {
			fmt.Println("No migrations found in the migrations directory.")
		} else {
			appliedCount := status.TotalMigrations - len(status.PendingMigrations)
			fmt.Printf("Applied migrations: %d\n", appliedCount)

			if len(status.PendingMigrations) > 0 {
				fmt.Printf("Next migration to apply: %d\n", status.PendingMigrations[0])
			}
		}
	}

	return nil
}
