package ptahmigrate

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/errkit"
	ptahintegration "github.com/denisvmedia/inventario/registry/ptah"
)

// NewPtahMigrateCommand creates the new Ptah-based migrate command
func NewPtahMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ptah-migrate",
		Short: "Advanced PostgreSQL migration management using Ptah",
		Long: `Advanced database migration management using the Ptah migration tool.

This command provides comprehensive PostgreSQL migration capabilities including:
- Apply pending migrations (up)
- Rollback migrations (down)
- Check migration status
- Generate schema diffs from Go entities
- Dry run mode for safe testing

All migrations are embedded in the binary and support PostgreSQL-specific features
like triggers, functions, JSONB operators, and advanced indexing.

IMPORTANT: This migration system ONLY supports PostgreSQL databases.
It provides advanced features not available in the standard migration system.

Examples:
  inventario ptah-migrate up --db-dsn="postgres://user:pass@localhost/db"
  inventario ptah-migrate down 5 --db-dsn="postgres://user:pass@localhost/db"
  inventario ptah-migrate status --db-dsn="postgres://user:pass@localhost/db"
  inventario ptah-migrate generate --db-dsn="postgres://user:pass@localhost/db"`,
		RunE: ptahMigrateCommand,
	}

	// Add global flags
	cmd.PersistentFlags().String("db-dsn", "", "PostgreSQL database connection string (required)")

	// Add subcommands
	cmd.AddCommand(newPtahMigrateUpCommand())
	cmd.AddCommand(newPtahMigrateDownCommand())
	cmd.AddCommand(newPtahMigrateStatusCommand())
	cmd.AddCommand(newPtahMigrateGenerateCommand())

	return cmd
}

// newPtahMigrateUpCommand creates the migrate up subcommand
func newPtahMigrateUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		Long: `Apply all pending database migrations to bring the schema up to date.

Each migration runs in its own transaction, so if any migration fails,
it will be rolled back and the migration process will stop.

Examples:
  inventario ptah-migrate up                    # Apply all pending migrations
  inventario ptah-migrate up --dry-run          # Preview what would be applied`,
		RunE: ptahMigrateUpCommand,
	}

	cmd.Flags().Bool("dry-run", false, "Show what migrations would be applied without executing them")

	return cmd
}

// newPtahMigrateDownCommand creates the migrate down subcommand
func newPtahMigrateDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down [target-version]",
		Short: "Rollback migrations to a specific version",
		Long: `Rollback database migrations to a specific version.

WARNING: Down migrations can cause data loss! Always backup your database
before running down migrations in production.

Examples:
  inventario ptah-migrate down 5                # Rollback to version 5
  inventario ptah-migrate down 5 --dry-run      # Preview rollback to version 5
  inventario ptah-migrate down 5 --confirm      # Skip confirmation prompt`,
		Args: cobra.ExactArgs(1),
		RunE: ptahMigrateDownCommand,
	}

	cmd.Flags().Bool("dry-run", false, "Show what migrations would be rolled back without executing them")
	cmd.Flags().Bool("confirm", false, "Skip confirmation prompt (dangerous!)")

	return cmd
}

// newPtahMigrateStatusCommand creates the migrate status subcommand
func newPtahMigrateStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current migration status",
		Long: `Display the current migration status including:
- Current database version
- Total number of migrations
- Number of pending migrations
- List of pending migrations (with --verbose)

Examples:
  inventario ptah-migrate status                # Show basic status
  inventario ptah-migrate status --verbose      # Show detailed status with pending migrations`,
		RunE: ptahMigrateStatusCommand,
	}

	cmd.Flags().Bool("verbose", false, "Show detailed status information")

	return cmd
}

// newPtahMigrateGenerateCommand creates the migrate generate subcommand
func newPtahMigrateGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate migration SQL from schema differences",
		Long: `Generate migration SQL by comparing Go entity definitions with the current database schema.

This command uses Ptah's schema introspection to compare your Go struct
definitions with the actual database schema and generates the SQL needed
to sync them.

Examples:
  inventario ptah-migrate generate              # Generate migration SQL
  inventario ptah-migrate generate --schema-dir ./models  # Use custom schema directory`,
		RunE: ptahMigrateGenerateCommand,
	}

	cmd.Flags().String("schema-dir", "./schema", "Directory containing Go entity definitions")

	return cmd
}

// ptahMigrateCommand is the default action (same as migrate up)
func ptahMigrateCommand(cmd *cobra.Command, args []string) error {
	dsn, err := cmd.Flags().GetString("db-dsn")
	if err != nil {
		return errkit.Wrap(err, "failed to get db-dsn flag")
	}

	if dsn == "" {
		return fmt.Errorf("database DSN is required (set --db-dsn)")
	}

	// Validate that this is a PostgreSQL DSN
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return fmt.Errorf(`Ptah migrations only support PostgreSQL databases.

Current DSN: %s

Please use a PostgreSQL connection string like:
  postgres://user:password@localhost:5432/database

For other database types, use the standard 'inventario migrate' command.`, dsn)
	}

	fmt.Println("Running default Ptah migration (migrate up)...")
	fmt.Println("Use 'inventario ptah-migrate --help' to see all available migration commands.")
	fmt.Println()

	// Delegate to migrate up command
	return ptahMigrateUpCommand(cmd, args)
}

// ptahMigrateUpCommand handles the migrate up subcommand
func ptahMigrateUpCommand(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	migrator, err := createPtahMigrator(cmd)
	if err != nil {
		return err
	}

	// Register embedded migrations
	if err := migrator.RegisterEmbeddedMigrations(); err != nil {
		return errkit.Wrap(err, "failed to register embedded migrations")
	}

	dsn, _ := cmd.Flags().GetString("db-dsn")
	fmt.Println("=== PTAH MIGRATE UP ===")
	fmt.Printf("Database: %s\n", dsn)
	fmt.Println()

	return migrator.MigrateUp(context.Background(), dryRun)
}

// ptahMigrateDownCommand handles the migrate down subcommand
func ptahMigrateDownCommand(cmd *cobra.Command, args []string) error {
	targetVersionStr := args[0]
	targetVersion, err := strconv.Atoi(targetVersionStr)
	if err != nil {
		return fmt.Errorf("invalid target version: %s", targetVersionStr)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	confirm, _ := cmd.Flags().GetBool("confirm")

	migrator, err := createPtahMigrator(cmd)
	if err != nil {
		return err
	}

	// Register embedded migrations
	if err := migrator.RegisterEmbeddedMigrations(); err != nil {
		return errkit.Wrap(err, "failed to register embedded migrations")
	}

	dsn, _ := cmd.Flags().GetString("db-dsn")
	fmt.Println("=== PTAH MIGRATE DOWN ===")
	fmt.Printf("Database: %s\n", dsn)
	fmt.Printf("Target version: %d\n", targetVersion)
	fmt.Println()

	return migrator.MigrateDown(context.Background(), targetVersion, dryRun, confirm)
}

// ptahMigrateStatusCommand handles the migrate status subcommand
func ptahMigrateStatusCommand(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	migrator, err := createPtahMigrator(cmd)
	if err != nil {
		return err
	}

	// Register embedded migrations
	if err := migrator.RegisterEmbeddedMigrations(); err != nil {
		return errkit.Wrap(err, "failed to register embedded migrations")
	}

	return migrator.PrintMigrationStatus(context.Background(), verbose)
}

// ptahMigrateGenerateCommand handles the migrate generate subcommand
func ptahMigrateGenerateCommand(cmd *cobra.Command, args []string) error {
	schemaDir, _ := cmd.Flags().GetString("schema-dir")

	migrator, err := createPtahMigrator(cmd)
	if err != nil {
		return err
	}

	dsn, _ := cmd.Flags().GetString("db-dsn")
	fmt.Println("=== GENERATE MIGRATION ===")
	fmt.Printf("Database: %s\n", dsn)
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
func createPtahMigrator(cmd *cobra.Command) (*ptahintegration.PtahMigrator, error) {
	dsn, err := cmd.Flags().GetString("db-dsn")
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get db-dsn flag")
	}

	if dsn == "" {
		return nil, fmt.Errorf("database DSN is required (set --db-dsn)")
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
