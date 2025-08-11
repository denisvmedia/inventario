package generate

import (
	"context"
	"fmt"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/migrate/common"
	"github.com/denisvmedia/inventario/internal/errkit"
)

const (
	schemaFlag  = "schema"
	initialFlag = "initial"
)

var flags = map[string]cobraflags.Flag{
	schemaFlag: &cobraflags.BoolFlag{
		Name:  schemaFlag,
		Usage: "Generate complete schema SQL (preview only, no files created)",
	},
	initialFlag: &cobraflags.BoolFlag{
		Name:  initialFlag,
		Usage: "Generate initial migration for empty database",
	},
}

// New creates the migrate generate subcommand
func New(dsnFlag cobraflags.Flag) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [migration-name]",
		Short: "Generate timestamped migration files from Go entity annotations",
		Long: `Generate timestamped migration files using Ptah's migration generator.

This command uses Ptah's migration generator to compare your Go struct
annotations with the actual database schema and generates both UP and DOWN
migration files with proper timestamps.

Examples:
  inventario migrate generate                    # Generate migration files from schema differences
  inventario migrate generate add_user_table     # Generate migration with custom name
  inventario migrate generate --schema           # Generate complete schema SQL (preview only)
  inventario migrate generate --initial          # Generate initial migration for empty database`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateGenerateCommand(cmd, args, dsnFlag)
		},
	}

	// Register cobraflags which automatically handles viper binding and persistent flags
	cobraflags.RegisterMap(cmd, flags)

	return cmd
}

// migrateGenerateCommand handles the migrate generate subcommand
func migrateGenerateCommand(cmd *cobra.Command, args []string, dsnFlag cobraflags.Flag) error {
	generateSchema := flags[schemaFlag].GetBool()
	generateInitial := flags[initialFlag].GetBool()

	migrator, err := common.CreatePtahMigrator(dsnFlag.GetString())
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
		files, err := migrator.GenerateInitialMigration(context.Background())
		if err != nil {
			return errkit.Wrap(err, "failed to generate initial migration")
		}

		// Check if no migration was needed (files will be nil when no changes detected)
		if files == nil {
			fmt.Println("✅ No schema changes detected - no initial migration files generated")      //nolint:forbidigo // CLI output is OK
			fmt.Printf("The database schema is already in sync with your Go entity annotations.\n") //nolint:forbidigo // CLI output is OK
			fmt.Printf("No initial migration is needed.\n")                                         //nolint:forbidigo // CLI output is OK
			return nil
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

	// Check if no migration was needed (files will be nil when no changes detected)
	if files == nil {
		fmt.Println("✅ No schema changes detected - no migration files generated")              //nolint:forbidigo // CLI output is OK
		fmt.Printf("The database schema is already in sync with your Go entity annotations.\n") //nolint:forbidigo // CLI output is OK
		fmt.Printf("No migration is needed at this time.\n")                                    //nolint:forbidigo // CLI output is OK
		return nil
	}

	fmt.Println("🎉 Migration files created successfully!")                                        //nolint:forbidigo // CLI output is OK
	fmt.Printf("Next steps:\n")                                                                   //nolint:forbidigo // CLI output is OK
	fmt.Printf("  1. Review the generated migration files\n")                                     //nolint:forbidigo // CLI output is OK
	fmt.Printf("  2. Run 'inventario migrate up' to apply the migration\n")                       //nolint:forbidigo // CLI output is OK
	fmt.Printf("  3. Test rollback with 'inventario migrate down %d' if needed\n", files.Version) //nolint:forbidigo // CLI output is OK

	return nil
}
