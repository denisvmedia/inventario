package migrate

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/errkit"
	ptahintegration "github.com/denisvmedia/inventario/registry/ptah"
)

const (
	dbDSNFlag = "db-dsn"
)

var migrateFlags = map[string]cobraflags.Flag{
	dbDSNFlag: &cobraflags.StringFlag{
		Name:       dbDSNFlag,
		Value:      "", // No default for migrate command - must be explicitly provided
		Usage:      "PostgreSQL database connection string (required)",
		Persistent: true, // Make this flag available to all subcommands
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "PostgreSQL database migration management",
	Long: `Advanced PostgreSQL database migration management using Ptah.

This command provides comprehensive PostgreSQL migration capabilities including:
- Apply pending migrations (up)
- Rollback migrations (down)
- Check migration status
- Dry run mode for safe testing

All migrations are embedded in the binary and support PostgreSQL-specific features
like triggers, functions, JSONB operators, and advanced indexing.

IMPORTANT: This migration system ONLY supports PostgreSQL databases.
Other database types are no longer supported in this version.

USAGE EXAMPLES:

  Apply all pending migrations:
    inventario migrate
    inventario migrate up

  Rollback to specific version:
    inventario migrate down 5

  Check migration status:
    inventario migrate status

  Preview migrations without applying:
    inventario migrate up --dry-run

CONFIGURATION:

  The command reads database configuration from:
  1. Command line flag: --db-dsn
  2. Environment variable: DB_DSN
  3. Configuration file: db-dsn setting

  PostgreSQL connection string format:
    postgres://user:password@host:port/database?sslmode=disable

MIGRATION SAFETY:

  • Each migration runs in its own transaction
  • Failed migrations are automatically rolled back
  • Migration state is tracked in schema_migrations table
  • Always backup your database before running migrations in production`,
	RunE: migrateCommand,
}

// NewMigrateCommand creates the main migrate command using Ptah
func NewMigrateCommand() *cobra.Command {
	// Register cobraflags which automatically handles viper binding and persistent flags
	cobraflags.RegisterMap(migrateCmd, migrateFlags)

	// Add subcommands
	migrateCmd.AddCommand(newMigrateUpCommand())
	migrateCmd.AddCommand(newMigrateDownCommand())
	migrateCmd.AddCommand(newMigrateStatusCommand())
	migrateCmd.AddCommand(newMigrateGenerateCommand())
	migrateCmd.AddCommand(newMigrateResetCommand())
	migrateCmd.AddCommand(newMigrateDropCommand())

	return migrateCmd
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
		Use:   "generate [migration-name]",
		Short: "Generate timestamped migration files from Go entity annotations",
		Long: `Generate timestamped migration files using Ptah's migration generator.

This command uses Ptah's migration generator to compare your Go struct
annotations with the actual database schema and generates both UP and DOWN
migration files with proper timestamps.

Examples:
  inventario migrate generate                    # Generate migration files from schema differences
  inventario migrate generate add_user_table    # Generate migration with custom name
  inventario migrate generate --schema           # Generate complete schema SQL (preview only)
  inventario migrate generate --initial          # Generate initial migration for empty database`,
		RunE: migrateGenerateCommand,
	}

	cmd.Flags().Bool("schema", false, "Generate complete schema SQL (preview only, no files created)")
	cmd.Flags().Bool("initial", false, "Generate initial migration for empty database")

	return cmd
}

// migrateCommand is the default action (same as migrate up)
func migrateCommand(cmd *cobra.Command, args []string) error {
	dsn := getDatabaseDSN()
	if dsn == "" {
		return fmt.Errorf("database DSN is required (set --db-dsn, DB_DSN environment variable, or db-dsn in config file)")
	}

	// Validate that this is a PostgreSQL DSN
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return fmt.Errorf(`This migration system only supports PostgreSQL databases.

Current DSN: %s

Please use a PostgreSQL connection string like:
  postgres://user:password@localhost:5432/database?sslmode=disable

For other database types, this version no longer provides migration support.`, dsn)
	}

	fmt.Println("Running default migration (migrate up)...")                                //nolint:forbidigo // CLI output is OK
	fmt.Println("Use 'inventario migrate --help' to see all available migration commands.") //nolint:forbidigo // CLI output is OK
	fmt.Println()                                                                           //nolint:forbidigo // CLI output is OK

	// Delegate to migrate up command
	return migrateUpCommand(cmd, args)
}

// migrateUpCommand handles the migrate up subcommand
func migrateUpCommand(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	migrator, err := createPtahMigrator()
	if err != nil {
		return err
	}

	dsn := getDatabaseDSN()
	fmt.Println("=== MIGRATE UP ===") //nolint:forbidigo // CLI output is OK
	fmt.Printf("Database: %s\n", dsn) //nolint:forbidigo // CLI output is OK
	fmt.Println()                     //nolint:forbidigo // CLI output is OK

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

	dsn := getDatabaseDSN()
	fmt.Println("=== MIGRATE DOWN ===")               //nolint:forbidigo // CLI output is OK
	fmt.Printf("Database: %s\n", dsn)                 //nolint:forbidigo // CLI output is OK
	fmt.Printf("Target version: %d\n", targetVersion) //nolint:forbidigo // CLI output is OK
	fmt.Println()                                     //nolint:forbidigo // CLI output is OK

	return migrator.MigrateDown(context.Background(), targetVersion, dryRun, confirm)
}

// migrateStatusCommand handles the migrate status subcommand
func migrateStatusCommand(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	migrator, err := createPtahMigrator()
	if err != nil {
		return err
	}

	return migrator.PrintMigrationStatus(context.Background(), verbose)
}

// migrateGenerateCommand handles the migrate generate subcommand
func migrateGenerateCommand(cmd *cobra.Command, args []string) error {
	generateSchema, _ := cmd.Flags().GetBool("schema")
	generateInitial, _ := cmd.Flags().GetBool("initial")

	migrator, err := createPtahMigrator()
	if err != nil {
		return err
	}

	// Handle schema preview mode (no files created)
	if generateSchema {
		statements, err := migrator.GenerateSchemaSQL(context.Background())
		if err != nil {
			return errkit.Wrap(err, "failed to generate schema SQL")
		}

		if len(statements) == 0 {
			fmt.Println("✅ No schema found in Go annotations") //nolint:forbidigo // CLI output is OK
			return nil
		}

		fmt.Println("=== COMPLETE SCHEMA SQL (PREVIEW) ===")                             //nolint:forbidigo // CLI output is OK
		fmt.Println()                                                                    //nolint:forbidigo // CLI output is OK
		fmt.Println("-- Complete schema generated from Go entity annotations")           //nolint:forbidigo // CLI output is OK
		fmt.Printf("-- Generated from: %s\n", "./models")                                //nolint:forbidigo // CLI output is OK
		fmt.Println("-- NOTE: This is a preview only. No migration files were created.") //nolint:forbidigo // CLI output is OK
		fmt.Println()                                                                    //nolint:forbidigo // CLI output is OK
		for i, stmt := range statements {
			fmt.Printf("-- Statement %d\n%s;\n\n", i+1, stmt) //nolint:forbidigo // CLI output is OK
		}

		fmt.Printf("Generated %d SQL statements (preview only).\n", len(statements))                              //nolint:forbidigo // CLI output is OK
		fmt.Println("💡 Use 'migrate generate --initial' to create actual migration files for an empty database.") //nolint:forbidigo // CLI output is OK
		return nil
	}

	// Handle initial migration generation
	if generateInitial {
		_, err := migrator.GenerateInitialMigration(context.Background())
		if err != nil {
			return errkit.Wrap(err, "failed to generate initial migration")
		}

		fmt.Println("🎉 Initial migration files created successfully!")          //nolint:forbidigo // CLI output is OK
		fmt.Printf("Next steps:\n")                                             //nolint:forbidigo // CLI output is OK
		fmt.Printf("  1. Review the generated migration files\n")               //nolint:forbidigo // CLI output is OK
		fmt.Printf("  2. Run 'inventario migrate up' to apply the migration\n") //nolint:forbidigo // CLI output is OK

		return nil
	}

	// Handle regular migration generation from schema differences
	migrationName := "migration"
	if len(args) > 0 {
		migrationName = args[0]
	}

	files, err := migrator.GenerateMigrationFiles(context.Background(), migrationName)
	if err != nil {
		return errkit.Wrap(err, "failed to generate migration files")
	}

	fmt.Println("🎉 Migration files created successfully!")                                        //nolint:forbidigo // CLI output is OK
	fmt.Printf("Next steps:\n")                                                                   //nolint:forbidigo // CLI output is OK
	fmt.Printf("  1. Review the generated migration files\n")                                     //nolint:forbidigo // CLI output is OK
	fmt.Printf("  2. Run 'inventario migrate up' to apply the migration\n")                       //nolint:forbidigo // CLI output is OK
	fmt.Printf("  3. Test rollback with 'inventario migrate down %d' if needed\n", files.Version) //nolint:forbidigo // CLI output is OK

	return nil
}

// getDatabaseDSN gets the database DSN from various sources (flag, env, config)
func getDatabaseDSN() string {
	// cobraflags automatically binds to viper, so this includes:
	// 1. Command line flag
	// 2. Environment variables
	// 3. Configuration file
	return migrateFlags[dbDSNFlag].GetString()
}

// createPtahMigrator creates a Ptah migrator instance
func createPtahMigrator() (*ptahintegration.PtahMigrator, error) {
	dsn := getDatabaseDSN()
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	// Validate that this is a PostgreSQL DSN
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return nil, fmt.Errorf("Ptah migrations only support PostgreSQL databases")
	}

	// Create the migrator with the models directory for schema parsing
	migrator, err := ptahintegration.NewPtahMigrator(nil, dsn, "./models")
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create Ptah migrator")
	}

	return migrator, nil
}

// newMigrateResetCommand creates the migrate reset subcommand
func newMigrateResetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Drop all tables and recreate from scratch",
		Long: `Drop all database tables and recreate the schema from scratch.

This command performs a complete database reset by:
1. Dropping all existing tables, indexes, and constraints
2. Applying all migrations from the beginning

WARNING: This operation will DELETE ALL DATA in the database!
Always backup your database before running this command in production.

Examples:
  inventario migrate reset                     # Reset database (with confirmation)
  inventario migrate reset --confirm           # Reset without confirmation prompt
  inventario migrate reset --dry-run           # Preview what would be reset`,
		RunE: migrateResetCommand,
	}

	cmd.Flags().Bool("dry-run", false, "Show what would be reset without executing")
	cmd.Flags().Bool("confirm", false, "Skip confirmation prompt (dangerous!)")

	return cmd
}

// newMigrateDropCommand creates the migrate drop subcommand
func newMigrateDropCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Drop all database tables and data",
		Long: `Drop all database tables, indexes, constraints, and data.

This command completely cleans the database by dropping all tables.
Unlike 'reset', this command does NOT recreate the schema afterward.

WARNING: This operation will DELETE ALL DATA and SCHEMA in the database!
Always backup your database before running this command in production.

Examples:
  inventario migrate drop                      # Drop all tables (with confirmation)
  inventario migrate drop --confirm            # Drop without confirmation prompt
  inventario migrate drop --dry-run            # Preview what would be dropped`,
		RunE: migrateDropCommand,
	}

	cmd.Flags().Bool("dry-run", false, "Show what would be dropped without executing")
	cmd.Flags().Bool("confirm", false, "Skip confirmation prompt (dangerous!)")

	return cmd
}

// migrateResetCommand handles the migrate reset subcommand
func migrateResetCommand(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	confirm, _ := cmd.Flags().GetBool("confirm")

	migrator, err := createPtahMigrator()
	if err != nil {
		return err
	}

	dsn := getDatabaseDSN()
	fmt.Println("=== MIGRATE RESET ===") //nolint:forbidigo // CLI output is OK
	fmt.Printf("Database: %s\n", dsn)    //nolint:forbidigo // CLI output is OK
	fmt.Println()                        //nolint:forbidigo // CLI output is OK

	return migrator.ResetDatabase(context.Background(), dryRun, confirm)
}

// migrateDropCommand handles the migrate drop subcommand
func migrateDropCommand(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	confirm, _ := cmd.Flags().GetBool("confirm")

	migrator, err := createPtahMigrator()
	if err != nil {
		return err
	}

	dsn := getDatabaseDSN()
	fmt.Println("=== MIGRATE DROP ===") //nolint:forbidigo // CLI output is OK
	fmt.Printf("Database: %s\n", dsn)   //nolint:forbidigo // CLI output is OK
	fmt.Println()                       //nolint:forbidigo // CLI output is OK

	return migrator.DropDatabase(context.Background(), dryRun, confirm)
}
