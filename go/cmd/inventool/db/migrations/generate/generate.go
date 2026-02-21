package generate

import (
	"context"
	"fmt"
	"os"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/schema/migrations/generator"
)

type Command struct {
	command.Base

	config Config
}

func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "generate [migration-name]",
		Short: "Generate timestamped migration files from Go entity annotations",
		Long: `Generate timestamped migration files using Ptah's migration generator.

This command uses Ptah's migration generator to compare your Go struct
annotations with the actual database schema and generates both UP and DOWN
migration files with proper timestamps.

Examples:
  inventario migrate generate                    # Generate migration files from schema differences
  inventario migrate generate add_user_table     # Generate migration with custom name
  inventario migrate generate --preview          # Generate complete schema SQL and print on screen for preview
  inventario migrate generate --check            # Exit 1 if pending schema changes exist (CI lint gate)
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.migrateGenerate(&c.config, dbConfig, args)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	shared.TryReadSection("migrate", &c.config)
	c.Cmd().Flags().StringVar(&c.config.GoEntitiesDir, "go-entities-dir", c.config.GoEntitiesDir, "Directory containing Go entity files")
	c.Cmd().Flags().StringVar(&c.config.MigrationsDir, "migrations-dir", c.config.MigrationsDir, "Directory containing migration files")
	c.Cmd().Flags().BoolVar(&c.config.Preview, "preview", c.config.Preview, "Generate complete schema SQL (preview only, no files created)")
	c.Cmd().Flags().BoolVar(&c.config.Check, "check", c.config.Check, "Check for pending schema changes and exit 1 if any are found (no files created)")
}

// migrateGenerate handles the migrate generate subcommand
func (c *Command) migrateGenerate(cfg *Config, dbConfig *shared.DatabaseConfig, args []string) error {
	generateSchemaPreview := cfg.Preview
	dsn := dbConfig.DBDSN

	fmt.Println("=== MIGRATE GENERATE ===")
	fmt.Printf("Database: %s\n", dsn)
	fmt.Println()

	gen, err := generator.New(dsn, cfg.GoEntitiesDir)
	if err != nil {
		return err
	}

	// Handle check mode: detect pending schema changes without writing any files.
	if cfg.Check {
		hasChanges, checkErr := gen.CheckPendingChanges(context.Background())
		if checkErr != nil {
			return errxtrace.Wrap("failed to check pending schema changes", checkErr)
		}
		if hasChanges {
			fmt.Println("❌ Pending schema changes detected.")
			fmt.Println("   Run 'inventool db migrations generate' to create the migration files.")
			os.Exit(1) //revive:disable-line:deep-exit -- intentional non-zero exit so CI/make treat this as a lint failure
		}
		fmt.Println("✅ Schema is in sync — no pending migrations.")
		return nil
	}

	// Handle schema preview mode (no files created)
	if generateSchemaPreview {
		statements, err := gen.GenerateSchemaSQL(context.Background())
		if err != nil {
			return errxtrace.Wrap("failed to generate schema SQL", err)
		}

		if len(statements) == 0 {
			fmt.Println("✅ No schema found in Go annotations")
			return nil
		}

		fmt.Println("=== COMPLETE SCHEMA SQL (PREVIEW) ===")
		fmt.Println()
		fmt.Println("-- Complete schema generated from Go entity annotations")
		fmt.Printf("-- Generated from: %s\n", cfg.GoEntitiesDir)
		fmt.Println("-- NOTE: This is a preview only. No migration files were created.")
		fmt.Println()
		for i, stmt := range statements {
			fmt.Printf("-- Statement %d\n%s;\n\n", i+1, stmt)
		}

		fmt.Printf("Generated %d SQL statements (preview only).\n", len(statements))
		fmt.Println("💡 Use 'migrate generate' without --preview to create actual migration files.")
		return nil
	}

	// Handle regular migration generation from schema differences
	migrationName := "migration"
	if len(args) > 0 {
		migrationName = args[0]
	}

	files, err := gen.GenerateMigrationFiles(context.Background(), migrationName, cfg.MigrationsDir)
	if err != nil {
		return errxtrace.Wrap("failed to generate migration files", err)
	}

	// Check if no migration was needed (files will be nil when no changes detected)
	if files == nil {
		fmt.Println("✅ No schema changes detected - no migration files generated")
		fmt.Printf("The database schema is already in sync with your Go entity annotations.\n")
		fmt.Printf("No migration is needed at this time.\n")
		return nil
	}

	fmt.Printf("🎉 Migration files created for: %s\n", migrationName)
	fmt.Printf("Next steps:\n")
	fmt.Printf("  1. Review the generated migration files\n")
	fmt.Printf("  2. Run 'inventario migrate up' to apply the migration\n")
	fmt.Printf("  3. Test rollback with 'inventario migrate down %d' if needed\n", files.Version)

	return nil
}
