package migrate

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denisvmedia/inventario/internal/errkit"
	ptahintegration "github.com/denisvmedia/inventario/registry/ptah"
)

// NewPtahMigrateCommand creates the new Ptah-based migrate command
func NewPtahMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration management using Ptah",
		Long: `Manage database migrations using the Ptah migration tool.

This command provides comprehensive migration management including:
- Apply pending migrations (up)
- Rollback migrations (down) 
- Check migration status
- Generate schema diffs
- Dry run mode for safe testing

All migrations are embedded in the binary and use PostgreSQL-specific features.`,
	}

	// Add subcommands
	cmd.AddCommand(newMigrateUpCommand())
	cmd.AddCommand(newMigrateDownCommand())
	cmd.AddCommand(newMigrateStatusCommand())
	cmd.AddCommand(newMigrateGenerateCommand())

	return cmd
}

// newMigrateUpCommand creates the migrate up subcommand
func newMigrateUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		Long: `Apply all pending database migrations to bring the schema up to date.

Each migration runs in its own transaction, so if any migration fails,
it will be rolled back and the migration process will stop.

Examples:
  inventario migrate up                    # Apply all pending migrations
  inventario migrate up --dry-run          # Preview what would be applied`,
		RunE: migrateUpCommand,
	}

	cmd.Flags().Bool("dry-run", false, "Show what migrations would be applied without executing them")

	return cmd
}

// newMigrateDownCommand creates the migrate down subcommand
func newMigrateDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down [target-version]",
		Short: "Rollback migrations to a specific version",
		Long: `Rollback database migrations to a specific version.

WARNING: Down migrations can cause data loss! Always backup your database
before running down migrations in production.

Examples:
  inventario migrate down 5                # Rollback to version 5
  inventario migrate down 5 --dry-run      # Preview rollback to version 5
  inventario migrate down 5 --confirm      # Skip confirmation prompt`,
		Args: cobra.ExactArgs(1),
		RunE: migrateDownCommand,
	}

	cmd.Flags().Bool("dry-run", false, "Show what migrations would be rolled back without executing them")
	cmd.Flags().Bool("confirm", false, "Skip confirmation prompt (dangerous!)")

	return cmd
}

// newMigrateStatusCommand creates the migrate status subcommand
func newMigrateStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current migration status",
		Long: `Display the current migration status including:
- Current database version
- Total number of migrations
- Number of pending migrations
- List of pending migrations (with --verbose)

Examples:
  inventario migrate status                # Show basic status
  inventario migrate status --verbose      # Show detailed status with pending migrations`,
		RunE: migrateStatusCommand,
	}

	cmd.Flags().Bool("verbose", false, "Show detailed status information")

	return cmd
}

// newMigrateGenerateCommand creates the migrate generate subcommand
func newMigrateGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate migration SQL from schema differences",
		Long: `Generate migration SQL by comparing Go entity definitions with the current database schema.

This command uses Ptah's schema introspection to compare your Go struct
definitions with the actual database schema and generates the SQL needed
to sync them.

Examples:
  inventario migrate generate              # Generate migration SQL
  inventario migrate generate --schema-dir ./models  # Use custom schema directory`,
		RunE: migrateGenerateCommand,
	}

	cmd.Flags().String("schema-dir", "./schema", "Directory containing Go entity definitions")

	return cmd
}

// migrateUpCommand handles the migrate up subcommand
func migrateUpCommand(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	migrator, err := createPtahMigrator()
	if err != nil {
		return err
	}

	// Register embedded migrations
	if err := migrator.RegisterEmbeddedMigrations(); err != nil {
		return errkit.Wrap(err, "failed to register embedded migrations")
	}

	fmt.Println("=== MIGRATE UP ===")
	fmt.Printf("Database: %s\n", viper.GetString("db-dsn"))
	fmt.Println()

	return migrator.MigrateUp(context.Background(), dryRun)
}

// migrateDownCommand handles the migrate down subcommand
func migrateDownCommand(cmd *cobra.Command, args []string) error {
	targetVersionStr := args[0]
	targetVersion, err := strconv.Atoi(targetVersionStr)
	if err != nil {
		return fmt.Errorf("invalid target version: %s", targetVersionStr)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	confirm, _ := cmd.Flags().GetBool("confirm")

	migrator, err := createPtahMigrator()
	if err != nil {
		return err
	}

	// Register embedded migrations
	if err := migrator.RegisterEmbeddedMigrations(); err != nil {
		return errkit.Wrap(err, "failed to register embedded migrations")
	}

	fmt.Println("=== MIGRATE DOWN ===")
	fmt.Printf("Database: %s\n", viper.GetString("db-dsn"))
	fmt.Printf("Target version: %d\n", targetVersion)
	fmt.Println()

	return migrator.MigrateDown(context.Background(), targetVersion, dryRun, confirm)
}

// migrateStatusCommand handles the migrate status subcommand
func migrateStatusCommand(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	migrator, err := createPtahMigrator()
	if err != nil {
		return err
	}

	// Register embedded migrations
	if err := migrator.RegisterEmbeddedMigrations(); err != nil {
		return errkit.Wrap(err, "failed to register embedded migrations")
	}

	return migrator.PrintMigrationStatus(context.Background(), verbose)
}

// migrateGenerateCommand handles the migrate generate subcommand
func migrateGenerateCommand(cmd *cobra.Command, args []string) error {
	schemaDir, _ := cmd.Flags().GetString("schema-dir")

	migrator, err := createPtahMigrator()
	if err != nil {
		return err
	}

	fmt.Println("=== GENERATE MIGRATION ===")
	fmt.Printf("Database: %s\n", viper.GetString("db-dsn"))
	fmt.Printf("Schema directory: %s\n", schemaDir)
	fmt.Println()

	// Generate migration SQL from schema differences
	statements, err := migrator.GenerateMigrationSQL(context.Background())
	if err != nil {
		return errkit.Wrap(err, "failed to generate migration SQL")
	}

	if len(statements) == 0 {
		fmt.Println("✅ No schema differences found - database is in sync with Go entities")
		return nil
	}

	fmt.Printf("Generated %d migration statements:\n\n", len(statements))
	for i, stmt := range statements {
		fmt.Printf("-- Statement %d\n%s;\n\n", i+1, stmt)
	}

	fmt.Println("⚠️  Review the SQL carefully before creating a migration file!")
	return nil
}

// createPtahMigrator creates a Ptah migrator instance
func createPtahMigrator() (*ptahintegration.PtahMigrator, error) {
	dsn := viper.GetString("db-dsn")
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is required (set --db-dsn or DB_DSN)")
	}

	// Validate that this is a PostgreSQL DSN
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return nil, fmt.Errorf("Ptah migrations only support PostgreSQL databases")
	}

	// Create the migrator with just the DSN for now
	migrator, err := ptahintegration.NewPtahMigrator(nil, dsn, "./schema")
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create Ptah migrator")
	}

	return migrator, nil
}
