package drop

import (
	"context"
	"fmt"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/migrate/common"
)

const (
	dryRunFlag  = "dry-run"
	confirmFlag = "confirm"
)

var flags = map[string]cobraflags.Flag{
	dryRunFlag: &cobraflags.BoolFlag{
		Name:  dryRunFlag,
		Usage: "Show what would be dropped without executing",
	},
	confirmFlag: &cobraflags.BoolFlag{
		Name:  confirmFlag,
		Usage: "Skip confirmation prompt (dangerous!)",
	},
}

// New creates the migrate drop subcommand
func New(dsnFlag cobraflags.Flag) *cobra.Command {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateDropCommand(cmd, args, dsnFlag)
		},
	}

	// Register cobraflags which automatically handles viper binding and persistent flags
	cobraflags.RegisterMap(cmd, flags)

	return cmd
}

// migrateDropCommand handles the migrate drop subcommand
func migrateDropCommand(cmd *cobra.Command, args []string, dsnFlag cobraflags.Flag) error {
	dryRun := flags[dryRunFlag].GetBool()
	confirm := flags[confirmFlag].GetBool()

	dsn := dsnFlag.GetString()

	migrator, err := common.CreatePtahMigrator(dsn)
	if err != nil {
		return err
	}

	fmt.Println("=== MIGRATE DROP ===") //nolint:forbidigo // CLI output is OK
	fmt.Printf("Database: %s\n", dsn)   //nolint:forbidigo // CLI output is OK
	fmt.Println()                       //nolint:forbidigo // CLI output is OK

	return migrator.DropDatabase(context.Background(), dryRun, confirm)
}
