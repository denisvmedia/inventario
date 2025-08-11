package reset

import (
	"context"
	"fmt"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/migrate/common"
	"github.com/denisvmedia/inventario/registry/ptah"
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

// New creates the migrate reset subcommand
func New(dsnFlag cobraflags.Flag) *cobra.Command {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateResetCommand(cmd, args, dsnFlag)
		},
	}

	// Register cobraflags which automatically handles viper binding and persistent flags
	cobraflags.RegisterMap(cmd, flags)
	common.RegisterOperationalUserFlag(cmd)

	return cmd
}

// migrateResetCommand handles the migrate reset subcommand
func migrateResetCommand(cmd *cobra.Command, args []string, dsnFlag cobraflags.Flag) error {
	dryRun := flags[dryRunFlag].GetBool()
	confirm := flags[confirmFlag].GetBool()
	dsn := dsnFlag.GetString()
	opUser, _ := cmd.Flags().GetString("operational-user")
	operationalUser := common.GetOperationalUser(opUser, dsn)

	migrator, err := common.CreatePtahMigrator(dsn)
	if err != nil {
		return err
	}

	fmt.Println("=== MIGRATE RESET ===") //nolint:forbidigo // CLI output is OK
	fmt.Printf("Database: %s\n", dsn)    //nolint:forbidigo // CLI output is OK
	fmt.Println()                        //nolint:forbidigo // CLI output is OK

	return migrator.ResetDatabase(context.Background(), ptah.MigrateArgs{
		OperationalUser: operationalUser,
		DryRun:          dryRun,
	}, confirm)
}
