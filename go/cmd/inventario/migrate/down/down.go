package down

import (
	"context"
	"fmt"
	"strconv"

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

// New creates the migrate down subcommand
func New(dsnFlag cobraflags.Flag) *cobra.Command {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateDownCommand(cmd, args, dsnFlag)
		},
	}

	// Register cobraflags which automatically handles viper binding and persistent flags
	cobraflags.RegisterMap(cmd, flags)

	return cmd
}

// migrateDownCommand handles the migrate down subcommand
func migrateDownCommand(cmd *cobra.Command, args []string, dsnFlag cobraflags.Flag) error {
	targetVersionStr := args[0]
	targetVersion, err := strconv.Atoi(targetVersionStr)
	if err != nil {
		return fmt.Errorf("invalid target version: %s", targetVersionStr)
	}

	dryRun := flags[dryRunFlag].GetBool()
	confirm := flags[confirmFlag].GetBool()

	dsn := dsnFlag.GetString()

	migrator, err := common.CreatePtahMigrator(dsn)
	if err != nil {
		return err
	}

	fmt.Println("=== MIGRATE DOWN ===")               //nolint:forbidigo // CLI output is OK
	fmt.Printf("Database: %s\n", dsn)                 //nolint:forbidigo // CLI output is OK
	fmt.Printf("Target version: %d\n", targetVersion) //nolint:forbidigo // CLI output is OK
	fmt.Println()                                     //nolint:forbidigo // CLI output is OK

	return migrator.MigrateDown(context.Background(), targetVersion, dryRun, confirm)
}
